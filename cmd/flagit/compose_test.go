package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The deployment descriptors are the one part of the configuration the Go
// tests would otherwise never see: a knob can exist on the binary, be covered
// by its own tests, and still never reach the container because nothing wired
// it through docker-compose.yml. These tests read the shipped files and run
// their values through the very helpers main uses, so the two cannot drift.

// composeService is the subset of a compose service definition that decides
// how Flagit is reached: which ports leave the container, and what the process
// is told.
type composeService struct {
	Ports       []string          `yaml:"ports"`
	Expose      []string          `yaml:"expose"`
	Environment map[string]string `yaml:"environment"`
}

type composeFile struct {
	Services map[string]composeService `yaml:"services"`
}

// readComposeService loads one service from a compose file in the repository
// root.
func readComposeService(t *testing.T, file, service string) composeService {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join("..", "..", file))
	require.NoError(t, err)

	var parsed composeFile
	require.NoError(t, yaml.Unmarshal(raw, &parsed))

	svc, ok := parsed.Services[service]
	require.True(t, ok, "%s defines no service %q", file, service)
	return svc
}

// composeDefault resolves what a compose value becomes when the platform
// supplies nothing: ${NAME:-fallback} collapses to the fallback, a bare
// ${NAME} to the empty string. It is deliberately the pessimistic reading —
// the deployment must be correct without anyone remembering to set a variable.
func composeDefault(raw string) string {
	if !strings.HasPrefix(raw, "${") || !strings.HasSuffix(raw, "}") {
		return raw
	}
	inner := raw[2 : len(raw)-1]
	if _, fallback, ok := strings.Cut(inner, ":-"); ok {
		return composeDefault(fallback)
	}
	return ""
}

// The sidecar proxies the tailnet straight at the admin listener and has no
// admin key to present, so the deployment has to hand the binary the switch
// that stands the key down on that listener. Without it the dashboard answers
// every request with 401 and there is nowhere to type a key.
func TestComposeEnablesTheAdminBypassForTheTailscaleSidecar(t *testing.T) {
	svc := readComposeService(t, "docker-compose.yml", "flagit")

	raw, ok := svc.Environment["FLAGIT_ADMIN_DISABLE_AUTH"]
	require.True(t, ok,
		"docker-compose.yml passes no FLAGIT_ADMIN_DISABLE_AUTH, so the admin listener keeps demanding a key the Tailscale sidecar cannot send")

	// Through the binary's own parser rather than a string comparison: this is
	// the question that actually matters, and "True"/"yes"/"enabled" do not
	// all answer it the same way.
	t.Setenv("FLAGIT_ADMIN_DISABLE_AUTH", composeDefault(raw))
	assert.True(t, envBool("FLAGIT_ADMIN_DISABLE_AUTH", false),
		"compose value %q is not one the binary reads as true", raw)
}

// The bypass is only ever safe because of this: the admin listener is on the
// compose network and nowhere else. Publishing it to the host would put an
// unauthenticated dashboard on the VM's interfaces.
func TestComposeNeverPublishesTheAdminPortToTheHost(t *testing.T) {
	svc := readComposeService(t, "docker-compose.yml", "flagit")

	assert.Empty(t, svc.Ports,
		"the admin listener runs without a key; binding any host port would publish it")
	assert.Contains(t, svc.Expose, "3000", "the sidecar still has to reach it over the compose network")
}

// The local overlay is the one place the admin port is published, and the same
// bypass applies there, so the binding has to stay on loopback.
func TestLocalOverlayPublishesTheAdminPortOnLoopbackOnly(t *testing.T) {
	svc := readComposeService(t, "docker-compose.local.yml", "flagit")

	require.NotEmpty(t, svc.Ports, "the overlay exists to publish the ports for local work")
	for _, mapping := range svc.Ports {
		assert.True(t, strings.HasPrefix(mapping, "127.0.0.1:"),
			"%q reaches beyond loopback, and the dashboard behind it needs no key", mapping)
	}
}
