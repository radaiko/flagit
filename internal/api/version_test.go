package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCommit = "212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa"

// versionBody mirrors the JSON the dashboard reads.
type versionBody struct {
	Commit string `json:"commit"`
	Short  string `json:"short"`
	Known  bool   `json:"known"`
}

func TestVersionReportsTheDeployedCommit(t *testing.T) {
	h := newHarness(t)
	h.Server.Commit = testCommit

	rec := do(t, h.internal, http.MethodGet, "/internal/version", nil, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var got versionBody
	decodeData(t, rec, &got)
	assert.Equal(t, testCommit, got.Commit, "the full SHA is what an operator inspects")
	assert.Equal(t, "212b000", got.Short)
	assert.True(t, got.Known)
}

func TestVersionFallsBackToUnknown(t *testing.T) {
	h := newHarness(t)
	// Nothing was injected at build time and nothing at runtime: the honest
	// answer is "unknown", not an empty string the dashboard would render blank.

	rec := do(t, h.internal, http.MethodGet, "/internal/version", nil, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var got versionBody
	decodeData(t, rec, &got)
	assert.Equal(t, "unknown", got.Commit)
	assert.Equal(t, "unknown", got.Short)
	assert.False(t, got.Known)
}

func TestVersionRequiresTheAdminKey(t *testing.T) {
	h := newHarness(t)
	h.Server.Commit = testCommit

	rec := do(t, h.internal, http.MethodGet, "/internal/version", nil, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.NotContains(t, rec.Body.String(), testCommit)
}

func TestVersionIsNotOnThePublicRouter(t *testing.T) {
	h := newHarness(t)
	h.Server.Commit = testCommit

	for _, path := range []string{"/internal/version", "/api/version"} {
		t.Run(path, func(t *testing.T) {
			rec := do(t, h.public, http.MethodGet, path, nil, adminHeaders())

			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.NotContains(t, rec.Body.String(), testCommit,
				"the deployed commit belongs to the admin dashboard only")
		})
	}
}

func TestVersionStaysOutOfTheSettingsPayload(t *testing.T) {
	h := newHarness(t)
	h.Server.Commit = testCommit

	rec := do(t, h.internal, http.MethodGet, "/internal/settings", nil, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	assert.NotContains(t, rec.Body.String(), testCommit,
		"settings is a mutable config object; build metadata has its own endpoint")
}
