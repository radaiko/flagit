package api

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The admin listener is reached through a reverse proxy that owns a hostname of
// its own — the Tailscale sidecar — so what an operator opens is that host's
// root, not /internal/admin. Serving the dashboard only under /internal/admin
// left the root answering `404 no such endpoint`, and the way that got papered
// over in production was a proxy rule rewriting / to /internal/admin. That rule
// also rewrote /internal/tickets, so the dashboard's own API calls came back as
// the dashboard's HTML and every ticket in the database read as none.
//
// The fix is for the root to serve the dashboard itself, so the proxy in front
// can stay a plain pass-through and has nothing left to rewrite.
func TestInternalRouterServesTheDashboardAtItsRoot(t *testing.T) {
	h := newHarness(t)
	h.Server.Overlay = stubFrontend("overlay")
	h.Server.Dashboard = stubFrontend("dashboard")
	internal := h.Server.InternalRouter()

	for _, path := range []string{"/", "/tickets", "/settings"} {
		rec := do(t, internal, http.MethodGet, path, nil, nil)

		assert.Equal(t, http.StatusOK, rec.Code, "path: %s", path)
		assert.Contains(t, rec.Body.String(), "dashboard", "path: %s", path)
	}

	// The address it has always been at keeps working; links and bookmarks to
	// it predate this.
	rec := do(t, internal, http.MethodGet, "/internal/admin", nil, nil)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "dashboard")
}

// The catch-all above must never answer for the API. A dashboard that asks for
// tickets and is handed HTML cannot tell that apart from a database with no
// tickets in it, which is precisely the failure this pair of tests exists to
// keep from coming back.
func TestInternalRouterKeepsTheAPIAheadOfTheDashboard(t *testing.T) {
	h := newHarness(t)
	h.Server.Overlay = stubFrontend("overlay")
	h.Server.Dashboard = stubFrontend("dashboard")
	internal := h.Server.InternalRouter()

	for _, path := range []string{
		"/internal/tickets",
		"/internal/apps",
		"/internal/settings",
		"/internal/version",
	} {
		rec := do(t, internal, http.MethodGet, path, nil, adminHeaders())

		require.Equal(t, http.StatusOK, rec.Code, "path: %s", path)
		assert.Equal(t, "application/json", rec.Header().Get("Content-Type"), "path: %s", path)
		assert.NotContains(t, rec.Body.String(), "dashboard", "path: %s", path)
	}

	// An API path nothing claimed is a mistake by an API client, and answering
	// it with a page would hide that. Reserved prefixes keep their JSON 404.
	rec := do(t, internal, http.MethodGet, "/internal/nonexistent", nil, adminHeaders())
	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Equal(t, "no such endpoint", errorMessage(t, rec))

	// Liveness is a machine's question and keeps its machine-readable answer,
	// even though a browser asking for the same path would get the SPA.
	health := do(t, internal, http.MethodGet, "/healthz", nil, nil)
	assert.Equal(t, http.StatusOK, health.Code)
	assert.Equal(t, "application/json", health.Header().Get("Content-Type"))
}
