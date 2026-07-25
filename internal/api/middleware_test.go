package api

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCORSPreflight(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.public, http.MethodOptions, "/api/tickets", nil, nil)

	assert.Equal(t, http.StatusNoContent, rec.Code)
	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Methods"), "POST")
	assert.Contains(t, rec.Header().Get("Access-Control-Allow-Headers"), HeaderDeviceToken)
}

func TestCORSHeadersOnNormalRequests(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.public, http.MethodGet, "/healthz", nil, nil)

	assert.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
}

func TestInternalRouterHasNoCORS(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets", nil, adminHeaders())

	assert.Empty(t, rec.Header().Get("Access-Control-Allow-Origin"),
		"the internal API is not meant to be reachable from a browser on another origin")
}

func TestRequestLoggerRecordsMethodPathAndStatus(t *testing.T) {
	var logs bytes.Buffer
	srv := NewServer(nil, "key", slog.New(slog.NewJSONHandler(&logs, nil)))

	handler := srv.RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/tickets", nil))

	var entry map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))
	assert.Equal(t, "POST", entry["method"])
	assert.Equal(t, "/api/tickets", entry["path"])
	assert.Equal(t, float64(http.StatusTeapot), entry["status"])
	assert.NotEmpty(t, entry["duration"])
}

func TestRequestLoggerDefaultsToOKWhenHandlerOnlyWrites(t *testing.T) {
	var logs bytes.Buffer
	srv := NewServer(nil, "key", slog.New(slog.NewJSONHandler(&logs, nil)))

	// A handler that never calls WriteHeader still produces a 200.
	handler := srv.RequestLogger(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello"))
		_, _ = w.Write([]byte(" again"))
	}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	assert.Equal(t, "hello again", rec.Body.String())
	var entry map[string]any
	require.NoError(t, json.Unmarshal(logs.Bytes(), &entry))
	assert.Equal(t, float64(http.StatusOK), entry["status"])
}

func TestStatusRecorderKeepsTheFirstStatus(t *testing.T) {
	inner := httptest.NewRecorder()
	rec := &statusRecorder{ResponseWriter: inner, status: http.StatusOK}

	rec.WriteHeader(http.StatusNotFound)
	rec.WriteHeader(http.StatusInternalServerError)

	assert.Equal(t, http.StatusNotFound, rec.status)
}

func TestRecovererTurnsPanicsInto500(t *testing.T) {
	h := newHarness(t)
	// A handler failure must not take the process down with it.
	h.Overlay = http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	router := h.PublicRouter()

	rec := do(t, router, http.MethodGet, "/overlay", nil, nil)

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminKeyFrom(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]string
		want    string
	}{
		{"header", map[string]string{HeaderAdminKey: "abc"}, "abc"},
		{"header trimmed", map[string]string{HeaderAdminKey: "  abc  "}, "abc"},
		{"bearer", map[string]string{"Authorization": "Bearer abc"}, "abc"},
		{"header wins", map[string]string{HeaderAdminKey: "abc", "Authorization": "Bearer xyz"}, "abc"},
		{"basic ignored", map[string]string{"Authorization": "Basic abc"}, ""},
		{"none", nil, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			for k, v := range tt.headers {
				req.Header.Set(k, v)
			}

			assert.Equal(t, tt.want, adminKeyFrom(req))
		})
	}
}

func TestDeviceTokenFrom(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		header string
		want   string
	}{
		{"header", "/", "abc", "abc"},
		{"query", "/?token=abc", "", "abc"},
		{"header wins", "/?token=xyz", "abc", "abc"},
		{"none", "/", "", ""},
		{"blank header falls through", "/?token=abc", "   ", "abc"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			if tt.header != "" {
				req.Header.Set(HeaderDeviceToken, tt.header)
			}

			assert.Equal(t, tt.want, deviceTokenFrom(req))
		})
	}
}

func TestValidAdminKey(t *testing.T) {
	srv := NewServer(nil, "correct-key", slog.New(slog.DiscardHandler))

	assert.True(t, srv.validAdminKey("correct-key"))
	assert.False(t, srv.validAdminKey("correct-ke"))
	assert.False(t, srv.validAdminKey("correct-keyy"))
	assert.False(t, srv.validAdminKey(""))
}

// failingWriter reports an error on the first Write, so the response-writing
// error paths are exercised.
type failingWriter struct{ header http.Header }

func (f *failingWriter) Header() http.Header {
	if f.header == nil {
		f.header = http.Header{}
	}
	return f.header
}
func (f *failingWriter) Write([]byte) (int, error) { return 0, assertErr("connection reset") }
func (f *failingWriter) WriteHeader(int)           {}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func TestWriteJSONSurvivesABrokenConnection(t *testing.T) {
	var logs bytes.Buffer
	srv := NewServer(nil, "key", slog.New(slog.NewJSONHandler(&logs, nil)))

	srv.writeJSON(&failingWriter{}, http.StatusOK, map[string]string{"a": "b"}, "")
	srv.writeError(&failingWriter{}, http.StatusBadRequest, "nope")

	assert.Equal(t, 2, strings.Count(logs.String(), "connection reset"))
}

func TestOverlayAndDashboardAreOptional(t *testing.T) {
	h := newHarness(t)

	// Without a mounted SPA the routers still work; unknown paths 404.
	assert.Equal(t, http.StatusNotFound, do(t, h.public, http.MethodGet, "/overlay", nil, nil).Code)
	assert.Equal(t, http.StatusNotFound,
		do(t, h.internal, http.MethodGet, "/internal/admin", nil, adminHeaders()).Code)
}

func TestIsReservedPath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{"/api", true},
		{"/api/", true},
		{"/api/tickets", true},
		{"/internal", true},
		{"/internal/poll", true},
		// Same leading characters, different path segment.
		{"/apiary", false},
		{"/internals", false},
		{"/api-docs", false},
		{"/", false},
		{"/overlay", false},
		{"/assets/api.js", false},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			assert.Equal(t, tt.want, isReservedPath(tt.path))
		})
	}
}

func TestMountedOverlayNeverAnswersAPIPaths(t *testing.T) {
	h := newHarness(t)
	h.Overlay = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>overlay</html>"))
	})
	router := h.PublicRouter()

	// An unknown or misspelled API path must fail as an API call, not hand a
	// client the overlay's HTML with a 200.
	for _, path := range []string{
		"/api", "/api/", "/api/nope", "/api/tickets/FLG-ABC123/nope",
		"/internal", "/internal/", "/internal/tickets", "/internal/poll",
	} {
		t.Run(path, func(t *testing.T) {
			rec := do(t, router, http.MethodGet, path, nil, adminHeaders())

			assert.Equal(t, http.StatusNotFound, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.NotContains(t, rec.Body.String(), "<html>")
		})
	}
}

func TestMountedOverlayStillServesFrontendRoutes(t *testing.T) {
	h := newHarness(t)
	h.Overlay = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>overlay</html>"))
	})
	router := h.PublicRouter()

	// "/apiary" starts with "/api" as a string but is not an API path.
	for _, path := range []string{"/", "/overlay", "/ticket/FLG-7X3K9Q", "/assets/app.js", "/apiary"} {
		t.Run(path, func(t *testing.T) {
			rec := do(t, router, http.MethodGet, path, nil, nil)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Contains(t, rec.Body.String(), "overlay")
		})
	}
}

func TestOverlayAndDashboardAreServedWhenMounted(t *testing.T) {
	h := newHarness(t)
	h.Overlay = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("overlay"))
	})
	h.Dashboard = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("dashboard"))
	})

	assert.Equal(t, "overlay", do(t, h.PublicRouter(), http.MethodGet, "/", nil, nil).Body.String())

	internal := h.InternalRouter()
	assert.Equal(t, "dashboard", do(t, internal, http.MethodGet, "/internal/admin", nil, nil).Body.String())
	assert.Equal(t, "dashboard",
		do(t, internal, http.MethodGet, "/internal/admin/settings", nil, nil).Body.String())
}
