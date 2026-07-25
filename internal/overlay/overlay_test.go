package overlay

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBuild stands in for a real Vite build so the handler logic is testable
// without running npm.
func fakeBuild() fs.FS {
	return fstest.MapFS{
		"overlay.html":         {Data: []byte("<html>overlay</html>")},
		"dashboard.html":       {Data: []byte("<html>dashboard</html>")},
		"assets/app.js":        {Data: []byte("console.log('hi')")},
		"assets/app.css":       {Data: []byte("body{}")},
		"_internal/chunk.js":   {Data: []byte("// chunk")},
		"favicon.svg":          {Data: []byte("<svg/>")},
		"nested/deep/file.txt": {Data: []byte("deep")},
	}
}

func request(t *testing.T, h http.Handler, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestSPAHandlerServesEntryAtRoot(t *testing.T) {
	h := spaHandler(fakeBuild(), overlayEntry, "")

	rec := request(t, h, "/")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "overlay")
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")
	assert.Equal(t, "no-cache", rec.Header().Get("Cache-Control"),
		"the entry HTML names hashed bundles, so it must not be cached")
}

func TestSPAHandlerServesStaticAssets(t *testing.T) {
	h := spaHandler(fakeBuild(), overlayEntry, "")

	tests := map[string]string{
		"/assets/app.js":        "console.log('hi')",
		"/assets/app.css":       "body{}",
		"/favicon.svg":          "<svg/>",
		"/nested/deep/file.txt": "deep",
		"/_internal/chunk.js":   "// chunk",
	}
	for path, want := range tests {
		t.Run(path, func(t *testing.T) {
			rec := request(t, h, path)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, want, rec.Body.String())
		})
	}
}

func TestSPAHandlerFallsBackToEntryForClientRoutes(t *testing.T) {
	h := spaHandler(fakeBuild(), overlayEntry, "")

	for _, path := range []string{"/ticket/FLG-7X3K9Q", "/reply", "/assets", "/does-not-exist"} {
		t.Run(path, func(t *testing.T) {
			rec := request(t, h, path)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), "overlay", "client-side routing needs the entry HTML")
		})
	}
}

func TestSPAHandlerStripsMountPrefix(t *testing.T) {
	h := spaHandler(fakeBuild(), dashboardEntry, "/internal/admin")

	entry := request(t, h, "/internal/admin")
	assert.Equal(t, http.StatusOK, entry.Code)
	assert.Contains(t, entry.Body.String(), "dashboard")

	nested := request(t, h, "/internal/admin/settings")
	assert.Contains(t, nested.Body.String(), "dashboard")

	asset := request(t, h, "/internal/admin/assets/app.js")
	assert.Equal(t, http.StatusOK, asset.Code)
	assert.Equal(t, "console.log('hi')", asset.Body.String())
}

func TestSPAHandlerResistsPathTraversal(t *testing.T) {
	h := spaHandler(fakeBuild(), overlayEntry, "")

	// Escaping the asset root must not be possible; the entry page is served.
	rec := request(t, h, "/../../etc/passwd")

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Body.String(), "overlay")
}

func TestSPAHandlerWithoutAnEntryFileIs404(t *testing.T) {
	h := spaHandler(fstest.MapFS{}, overlayEntry, "")

	rec := request(t, h, "/")

	assert.Equal(t, http.StatusNotFound, rec.Code)
	assert.Contains(t, rec.Body.String(), "frontend not built")
}

func TestExists(t *testing.T) {
	assets := fakeBuild()

	assert.True(t, exists(assets, "overlay.html"))
	assert.False(t, exists(assets, "missing.html"))
	assert.False(t, exists(assets, "assets"), "a directory is not a servable file")
}

func TestAssetsAndBuilt(t *testing.T) {
	assets, err := Assets()
	require.NoError(t, err)
	assert.NotNil(t, assets)

	// Built() reflects whether `make web` has run; both answers are valid, but
	// it must agree with what is actually embedded.
	_, statErr := fs.Stat(assets, overlayEntry)
	assert.Equal(t, statErr == nil, Built())
}

func TestHandlerConstructors(t *testing.T) {
	overlayHandler, err := OverlayHandler()
	require.NoError(t, err)
	assert.NotNil(t, overlayHandler)

	dashboardHandler, err := DashboardHandler("/internal/admin")
	require.NoError(t, err)
	assert.NotNil(t, dashboardHandler)
}

func TestDevProxyForwardsRequests(t *testing.T) {
	var gotPath string
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_, _ = w.Write([]byte("from vite"))
	}))
	defer vite.Close()

	proxy, err := DevProxy(vite.URL, "")
	require.NoError(t, err)

	rec := request(t, proxy, "/overlay.html")

	assert.Equal(t, "from vite", rec.Body.String())
	assert.Equal(t, "/overlay.html", gotPath)
}

func TestDevProxyStripsPrefix(t *testing.T) {
	var gotPath string
	vite := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
	}))
	defer vite.Close()

	proxy, err := DevProxy(vite.URL, "/internal/admin")
	require.NoError(t, err)

	request(t, proxy, "/internal/admin/settings")
	assert.Equal(t, "/settings", gotPath)

	// The bare mount point maps to Vite's root, not to an empty path.
	request(t, proxy, "/internal/admin")
	assert.Equal(t, "/", gotPath)
}

func TestDevProxyDefaultsToVite(t *testing.T) {
	proxy, err := DevProxy("", "")

	require.NoError(t, err)
	assert.NotNil(t, proxy)
}

func TestDevProxyRejectsBadURL(t *testing.T) {
	_, err := DevProxy("://nope", "")

	assert.ErrorContains(t, err, "parse vite url")
}
