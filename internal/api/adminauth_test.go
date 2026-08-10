package api

import (
	"net/http"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authModeBody mirrors the JSON the dashboard reads to decide whether to put
// up its sign-in screen.
type authModeBody struct {
	AdminKeyRequired bool `json:"adminKeyRequired"`
}

// adminOnlyPaths are internal endpoints that must never answer without the
// admin key unless the operator disabled it for the admin listener.
var adminOnlyPaths = []string{"/internal/tickets", "/internal/version", "/internal/settings", "/internal/apps"}

func TestAdminListenerRequiresTheKeyByDefault(t *testing.T) {
	h := newHarness(t)

	require.False(t, h.Server.AdminAuthDisabled,
		"dropping the admin key must be an explicit choice, never a default")
	for _, path := range adminOnlyPaths {
		rec := do(t, h.internal, http.MethodGet, path, nil, nil)
		assert.Equal(t, http.StatusUnauthorized, rec.Code, path)
	}
}

func TestAdminListenerServesWithoutAKeyWhenAuthIsDisabled(t *testing.T) {
	h := newHarness(t)
	h.Server.AdminAuthDisabled = true

	for _, path := range adminOnlyPaths {
		rec := do(t, h.internal, http.MethodGet, path, nil, nil)
		assert.Equal(t, http.StatusOK, rec.Code, "%s body: %s", path, rec.Body.String())
	}
}

// A key that is no longer needed must not become a way to lock the operator
// out: with auth disabled the header is simply not consulted.
func TestAdminListenerIgnoresAWrongKeyWhenAuthIsDisabled(t *testing.T) {
	h := newHarness(t)
	h.Server.AdminAuthDisabled = true

	rec := do(t, h.internal, http.MethodGet, "/internal/version", nil,
		map[string]string{HeaderAdminKey: "a-stale-key-from-a-previous-deploy"})

	assert.Equal(t, http.StatusOK, rec.Code)
}

// The bypass belongs to the admin listener's router, not to the middleware.
// Anything mounted behind AdminKeyAuth — the public listener above all — has to
// keep rejecting exactly as before.
func TestDisablingAdminAuthDoesNotDisarmTheAdminKeyMiddleware(t *testing.T) {
	h := newHarness(t)
	h.Server.AdminAuthDisabled = true

	guarded := chi.NewRouter()
	guarded.With(h.Server.AdminKeyAuth).Get("/guarded", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	missing := do(t, guarded, http.MethodGet, "/guarded", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, missing.Code, "no key must still be no entry")

	wrong := do(t, guarded, http.MethodGet, "/guarded", nil, map[string]string{HeaderAdminKey: "not-the-key"})
	assert.Equal(t, http.StatusUnauthorized, wrong.Code, "a wrong key must still be no entry")

	right := do(t, guarded, http.MethodGet, "/guarded", nil, adminHeaders())
	assert.Equal(t, http.StatusOK, right.Code)
}

// The public listener serves the overlay's SPA catch-all, so "not routed" has
// to mean a JSON 404 rather than a page of dashboard HTML.
func TestPublicRouterKeepsInternalUnavailableWhenAdminAuthIsDisabled(t *testing.T) {
	h := newHarness(t)
	h.Server.AdminAuthDisabled = true
	h.Server.Overlay = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html>overlay</html>"))
	})
	public := h.Server.PublicRouter()

	paths := []string{
		"/internal/admin",
		"/internal/admin/tickets",
		"/internal/auth",
		"/internal/version",
		"/internal/tickets",
		"/internal/settings",
	}
	credentials := map[string]map[string]string{
		"no key":    nil,
		"wrong key": {HeaderAdminKey: "not-the-key"},
		"right key": adminHeaders(),
	}

	for _, path := range paths {
		for name, headers := range credentials {
			t.Run(path+" with "+name, func(t *testing.T) {
				rec := do(t, public, http.MethodGet, path, nil, headers)

				assert.Equal(t, http.StatusNotFound, rec.Code)
				assert.NotContains(t, rec.Body.String(), "overlay",
					"the public listener must not answer an internal path with the SPA")
			})
		}
	}
}

func TestAuthModeSaysAKeyIsRequired(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.internal, http.MethodGet, "/internal/auth", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var got authModeBody
	decodeData(t, rec, &got)
	assert.True(t, got.AdminKeyRequired)
}

func TestAuthModeSaysNoKeyIsRequiredWhenAuthIsDisabled(t *testing.T) {
	h := newHarness(t)
	h.Server.AdminAuthDisabled = true

	rec := do(t, h.internal, http.MethodGet, "/internal/auth", nil, nil)

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var got authModeBody
	decodeData(t, rec, &got)
	assert.False(t, got.AdminKeyRequired,
		"the dashboard has no other way to know it should skip its sign-in screen")
}

// It answers a yes/no question about the listener the caller already reached,
// so it carries no ticket data — but it is still admin-listener-only.
func TestAuthModeIsNotOnThePublicRouter(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.public, http.MethodGet, "/internal/auth", nil, adminHeaders())

	assert.Equal(t, http.StatusNotFound, rec.Code)
}
