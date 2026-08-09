package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"flagit/internal/api"
	"flagit/internal/db"
	"flagit/internal/model"
	"flagit/internal/overlay"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseFlagsDefaults(t *testing.T) {
	cfg, err := parseFlags(nil, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, 8080, cfg.port)
	assert.Equal(t, 3000, cfg.adminPort)
	assert.Equal(t, "./data/flagit.db", cfg.dbPath)
	assert.Empty(t, cfg.adminKey)
	assert.Empty(t, cfg.publicURL)
	assert.False(t, cfg.dev)
	assert.Equal(t, "info", cfg.logLevel)
	assert.Empty(t, cfg.commit)
}

func TestParseFlagsOverrides(t *testing.T) {
	cfg, err := parseFlags([]string{
		"--port", "9090",
		"--admin-port", "9091",
		"--db-path", "/tmp/x.db",
		"--admin-key", "secret",
		"--public-url", "https://flagit.example/",
		"--dev",
		"--log-level", "debug",
		"--commit", "212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa",
	}, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.port)
	assert.Equal(t, 9091, cfg.adminPort)
	assert.Equal(t, "/tmp/x.db", cfg.dbPath)
	assert.Equal(t, "secret", cfg.adminKey)
	assert.Equal(t, "https://flagit.example", cfg.publicURL, "a trailing slash is trimmed")
	assert.True(t, cfg.dev)
	assert.Equal(t, "debug", cfg.logLevel)
	assert.Equal(t, "212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa", cfg.commit)
}

func TestParseFlagsReadsEnvironment(t *testing.T) {
	t.Setenv("FLAGIT_PORT", "7070")
	t.Setenv("FLAGIT_ADMIN_PORT", "7071")
	t.Setenv("FLAGIT_DB_PATH", "/tmp/env.db")
	t.Setenv("FLAGIT_ADMIN_KEY", "env-key")
	t.Setenv("FLAGIT_PUBLIC_URL", "https://env.example")
	t.Setenv("FLAGIT_DEV", "true")
	t.Setenv("FLAGIT_COMMIT", "aaaaaaabbbbbbbcccccccddddddd0000000eeeee")

	cfg, err := parseFlags(nil, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, 7070, cfg.port)
	assert.Equal(t, 7071, cfg.adminPort)
	assert.Equal(t, "/tmp/env.db", cfg.dbPath)
	assert.Equal(t, "env-key", cfg.adminKey)
	assert.Equal(t, "https://env.example", cfg.publicURL)
	assert.True(t, cfg.dev)
	assert.Equal(t, "aaaaaaabbbbbbbcccccccddddddd0000000eeeee", cfg.commit,
		"Coolify exposes the deployed revision as SOURCE_COMMIT, mapped to FLAGIT_COMMIT in compose")
}

func TestParseFlagsAcceptsSourceCommitAsTheRevision(t *testing.T) {
	// Coolify names the deployed revision SOURCE_COMMIT. Compose maps it onto
	// FLAGIT_COMMIT, but that mapping only works when Coolify puts the variable
	// where compose interpolates it — which raw Docker Compose deployments do
	// not guarantee. Reading it directly costs nothing and covers that gap.
	t.Setenv("SOURCE_COMMIT", "aaaaaaabbbbbbbcccccccddddddd0000000eeeee")

	cfg, err := parseFlags(nil, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, "aaaaaaabbbbbbbcccccccddddddd0000000eeeee", cfg.commit)
}

func TestFlagitCommitBeatsSourceCommit(t *testing.T) {
	// An operator who pins FLAGIT_COMMIT by hand has said the last word.
	t.Setenv("FLAGIT_COMMIT", "1111111111111111111111111111111111111111")
	t.Setenv("SOURCE_COMMIT", "2222222222222222222222222222222222222222")

	cfg, err := parseFlags(nil, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, "1111111111111111111111111111111111111111", cfg.commit)
}

func TestParseFlagsIgnoresABlankSourceCommit(t *testing.T) {
	// An unset compose variable arrives as an empty string, not as an absent one.
	t.Setenv("FLAGIT_COMMIT", "")
	t.Setenv("SOURCE_COMMIT", "   ")

	cfg, err := parseFlags(nil, io.Discard)

	require.NoError(t, err)
	assert.Empty(t, cfg.commit, "the build-time value must still get its turn")
}

func TestFlagsBeatEnvironment(t *testing.T) {
	t.Setenv("FLAGIT_PORT", "7070")

	cfg, err := parseFlags([]string{"--port", "9090"}, io.Discard)

	require.NoError(t, err)
	assert.Equal(t, 9090, cfg.port)
}

func TestParseFlagsErrors(t *testing.T) {
	_, err := parseFlags([]string{"--nope"}, io.Discard)
	assert.Error(t, err)

	_, err = parseFlags([]string{"stray-argument"}, io.Discard)
	assert.ErrorContains(t, err, "unexpected argument")

	_, err = parseFlags([]string{"--help"}, io.Discard)
	assert.ErrorIs(t, err, flag.ErrHelp)
}

func TestHelpIsNotAnError(t *testing.T) {
	var out bytes.Buffer

	err := run(context.Background(), []string{"--help"}, &out, io.Discard)

	require.NoError(t, err, "--help must exit 0")
	assert.Contains(t, out.String(), "self-hosted in-app ticket system")
	assert.Contains(t, out.String(), "-admin-key")
	assert.Contains(t, out.String(), "-db-path")
}

func TestRunRejectsBadFlags(t *testing.T) {
	err := run(context.Background(), []string{"--nope"}, io.Discard, io.Discard)

	assert.Error(t, err)
}

func TestRunRejectsUnusableDBPath(t *testing.T) {
	// A file where a directory is needed.
	file := filepath.Join(t.TempDir(), "blocker")
	require.NoError(t, writeEmptyFile(file))

	err := run(context.Background(), []string{"--db-path", filepath.Join(file, "flagit.db")}, io.Discard, io.Discard)

	assert.Error(t, err)
}

func TestEnvHelpers(t *testing.T) {
	assert.Equal(t, "fallback", env("FLAGIT_MISSING_VAR", "fallback"))
	t.Setenv("FLAGIT_TEST_VAR", "  value  ")
	assert.Equal(t, "value", env("FLAGIT_TEST_VAR", "fallback"))

	t.Setenv("FLAGIT_TEST_VAR", "   ")
	assert.Equal(t, "fallback", env("FLAGIT_TEST_VAR", "fallback"), "a blank value is not a value")
}

func TestEnvInt(t *testing.T) {
	assert.Equal(t, 5, envInt("FLAGIT_MISSING_INT", 5))

	t.Setenv("FLAGIT_TEST_INT", "42")
	assert.Equal(t, 42, envInt("FLAGIT_TEST_INT", 5))

	t.Setenv("FLAGIT_TEST_INT", "not-a-number")
	assert.Equal(t, 5, envInt("FLAGIT_TEST_INT", 5), "garbage falls back to the default")
}

func TestEnvBool(t *testing.T) {
	assert.True(t, envBool("FLAGIT_MISSING_BOOL", true))
	assert.False(t, envBool("FLAGIT_MISSING_BOOL", false))

	for _, truthy := range []string{"1", "true", "TRUE", "yes", "on"} {
		t.Setenv("FLAGIT_TEST_BOOL", truthy)
		assert.True(t, envBool("FLAGIT_TEST_BOOL", false), "%q should be true", truthy)
	}
	for _, falsy := range []string{"0", "false", "no", "off", "nonsense"} {
		t.Setenv("FLAGIT_TEST_BOOL", falsy)
		assert.False(t, envBool("FLAGIT_TEST_BOOL", true), "%q should be false", falsy)
	}
}

func TestNewLogger(t *testing.T) {
	var out bytes.Buffer

	logger := newLogger(&out, "warn")
	logger.Info("hidden")
	logger.Warn("shown")

	assert.NotContains(t, out.String(), "hidden")
	assert.Contains(t, out.String(), "shown")

	out.Reset()
	// An unparseable level falls back to info rather than failing to start.
	newLogger(&out, "loud").Info("visible")
	assert.Contains(t, out.String(), "visible")
}

func TestResolveAdminKeyUsesProvidedValue(t *testing.T) {
	database := newDB(t)
	var out bytes.Buffer

	key, err := resolveAdminKey(database, "  provided-key  ", &out)

	require.NoError(t, err)
	assert.Len(t, key, 64, "the returned value is a SHA-256 hex hash, not the raw key")
	assert.Empty(t, out.String(), "a supplied key is not echoed to stdout")

	stored, err := database.GetSetting(model.SettingAdminKeyHash, "")
	require.NoError(t, err)
	assert.Equal(t, key, stored)
}

func TestResolveAdminKeyGeneratesAndPrintsOnce(t *testing.T) {
	database := newDB(t)
	var out bytes.Buffer

	key, err := resolveAdminKey(database, "", &out)
	require.NoError(t, err)
	assert.Len(t, key, 64)
	// The output shows the raw generated key, not the hash.
	assert.Regexp(t, `[0-9a-f]{64}`, out.String())
	assert.Contains(t, out.String(), "the only time it can")
	assert.Contains(t, out.String(), "be shown")

	// A restart reuses the stored key and stays quiet.
	out.Reset()
	again, err := resolveAdminKey(database, "", &out)
	require.NoError(t, err)
	assert.Equal(t, key, again)
	assert.Empty(t, out.String())
}

func TestResolveAdminKeyDBFailures(t *testing.T) {
	database := newDB(t)
	require.NoError(t, database.Close())

	_, err := resolveAdminKey(database, "provided", io.Discard)
	assert.ErrorContains(t, err, "store admin key")

	_, err = resolveAdminKey(database, "", io.Discard)
	assert.ErrorContains(t, err, "read admin key")
}

func TestMountFrontendDevModeProxies(t *testing.T) {
	srv := api.NewServer(nil, "key", slog.New(slog.DiscardHandler))

	require.NoError(t, mountFrontend(srv, &config{dev: true, viteURL: "http://localhost:5173"},
		slog.New(slog.DiscardHandler)))

	assert.NotNil(t, srv.Overlay)
	assert.NotNil(t, srv.Dashboard)
}

func TestMountFrontendDevModeRejectsBadViteURL(t *testing.T) {
	srv := api.NewServer(nil, "key", slog.New(slog.DiscardHandler))

	err := mountFrontend(srv, &config{dev: true, viteURL: "://nope"}, slog.New(slog.DiscardHandler))

	assert.Error(t, err)
}

func TestMountFrontend(t *testing.T) {
	srv := api.NewServer(nil, "key", slog.New(slog.DiscardHandler))
	var logs bytes.Buffer

	err := mountFrontend(srv, &config{}, newLogger(&logs, "warn"))
	require.NoError(t, err)

	// A binary built after `make web` embeds the frontend and serves it; one
	// built without it still starts and serves the API alone.
	if overlay.Built() {
		assert.NotNil(t, srv.Overlay)
		assert.NotNil(t, srv.Dashboard)
	} else {
		assert.Nil(t, srv.Overlay)
		assert.Contains(t, logs.String(), "no frontend build embedded")
	}
}

func TestRunRejectsBadViteURL(t *testing.T) {
	err := run(context.Background(), []string{
		"--dev", "--vite-url", "://nope",
		"--db-path", filepath.Join(t.TempDir(), "flagit.db"),
		"--admin-key", "key",
	}, io.Discard, io.Discard)

	assert.Error(t, err)
}

// TestServerStartsAndServesRequests boots the real binary wiring on ephemeral
// ports and drives a ticket through both APIs.
func TestServerStartsAndServesRequests(t *testing.T) {
	publicPort := freePort(t)
	adminPort := freePort(t)
	dbPath := filepath.Join(t.TempDir(), "flagit.db")

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- run(ctx, []string{
			"--port", fmt.Sprint(publicPort),
			"--admin-port", fmt.Sprint(adminPort),
			"--db-path", dbPath,
			"--admin-key", "integration-key",
			"--log-level", "error",
			"--commit", "212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa",
		}, io.Discard, io.Discard)
	}()

	publicURL := fmt.Sprintf("http://127.0.0.1:%d", publicPort)
	adminURL := fmt.Sprintf("http://127.0.0.1:%d", adminPort)
	waitForServer(t, publicURL+"/healthz")
	waitForServer(t, adminURL+"/healthz")

	// Create a ticket through the public API.
	resp := post(t, publicURL+"/api/tickets", `{
		"type":"bug","title":"Crash on save","body":"Tapping save closes the app",
		"deviceToken":"3f6b1a4e-0000-4000-8000-000000000001","appName":"notes"}`, nil)
	require.Equal(t, http.StatusCreated, resp.StatusCode)
	var created struct {
		Data model.Ticket `json:"data"`
	}
	decodeBody(t, resp, &created)
	assert.True(t, db.ValidTicketID(created.Data.ID))

	// The internal API needs the admin key.
	unauth := get(t, adminURL+"/internal/tickets", nil)
	assert.Equal(t, http.StatusUnauthorized, unauth.StatusCode)

	authed := get(t, adminURL+"/internal/tickets", map[string]string{api.HeaderAdminKey: "integration-key"})
	require.Equal(t, http.StatusOK, authed.StatusCode)
	var listed struct {
		Data struct {
			Tickets []model.Ticket `json:"tickets"`
		} `json:"data"`
	}
	decodeBody(t, authed, &listed)
	require.Len(t, listed.Data.Tickets, 1)
	assert.Equal(t, created.Data.ID, listed.Data.Tickets[0].ID)

	// The deployed commit reaches the dashboard's endpoint from the flag.
	versionResp := get(t, adminURL+"/internal/version", map[string]string{api.HeaderAdminKey: "integration-key"})
	require.Equal(t, http.StatusOK, versionResp.StatusCode)
	var deployed struct {
		Data struct {
			Commit string `json:"commit"`
			Short  string `json:"short"`
			Known  bool   `json:"known"`
		} `json:"data"`
	}
	decodeBody(t, versionResp, &deployed)
	assert.Equal(t, "212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa", deployed.Data.Commit)
	assert.Equal(t, "212b000", deployed.Data.Short)
	assert.True(t, deployed.Data.Known)

	// The public port must not expose the internal API at all.
	leaked := get(t, publicURL+"/internal/tickets", map[string]string{api.HeaderAdminKey: "integration-key"})
	assert.Equal(t, http.StatusNotFound, leaked.StatusCode)

	// ...and least of all the commit, which is admin-dashboard-only.
	leakedVersion := get(t, publicURL+"/internal/version", map[string]string{api.HeaderAdminKey: "integration-key"})
	assert.Equal(t, http.StatusNotFound, leakedVersion.StatusCode)

	// Graceful shutdown on signal.
	cancel()
	select {
	case err := <-done:
		assert.NoError(t, err)
	case <-time.After(20 * time.Second):
		t.Fatal("server did not shut down")
	}

	// The data survived: it was on disk, not in memory.
	reopened, err := db.InitDB(dbPath)
	require.NoError(t, err)
	defer reopened.Close()
	tickets, err := reopened.ListTickets(db.TicketFilter{})
	require.NoError(t, err)
	assert.Len(t, tickets, 1)
}

func TestServeReportsAPortItCannotBind(t *testing.T) {
	// Occupy a port, then ask the server for the same one.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()

	srv := &http.Server{Addr: listener.Addr().String()}
	err = serve(context.Background(), slog.New(slog.DiscardHandler), srv)

	assert.ErrorContains(t, err, "listen on")
}

// ------------------------------------------------------------- test utils --

func newDB(t *testing.T) *db.DB {
	t.Helper()
	database, err := db.InitDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func writeEmptyFile(path string) error {
	return os.WriteFile(path, nil, 0o600)
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port
}

func waitForServer(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("server at %s never became ready", url)
}

func get(t *testing.T, url string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func post(t *testing.T, url, body string, headers map[string]string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func decodeBody(t *testing.T, resp *http.Response, dst any) {
	t.Helper()
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(raw, dst), "body: %s", raw)
}
