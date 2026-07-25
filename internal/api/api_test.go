package api

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"flagit/internal/db"
	"flagit/internal/model"
	"flagit/internal/service"
	"flagit/internal/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testAdminKey = "test-admin-key"
	testToken    = "3f6b1a4e-0000-4000-8000-000000000001"
)

// recorder captures webhook deliveries instead of making HTTP calls.
type recorder struct {
	mu       sync.Mutex
	payloads []webhook.Payload
}

func (r *recorder) Send(_ context.Context, _ string, p webhook.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.payloads = append(r.payloads, p)
	return nil
}

func (r *recorder) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.payloads)
}

// harness bundles a server with both routers wired up.
type harness struct {
	*Server
	public   http.Handler
	internal http.Handler
	hook     *recorder
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	database, err := db.InitDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })

	tick := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	database.Now = func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}

	hook := &recorder{}
	svc := service.New(database, hook, "https://flagit.example", slog.New(slog.DiscardHandler))
	svc.Dispatch = func(fn func()) { fn() }

	srv := NewServer(svc, testAdminKey, slog.New(slog.DiscardHandler))
	return &harness{Server: srv, public: srv.PublicRouter(), internal: srv.InternalRouter(), hook: hook}
}

// do issues a request against a router and returns the recorded response.
func do(t *testing.T, h http.Handler, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var reader *bytes.Reader
	switch b := body.(type) {
	case nil:
		reader = bytes.NewReader(nil)
	case string:
		reader = bytes.NewReader([]byte(b))
	default:
		raw, err := json.Marshal(b)
		require.NoError(t, err)
		reader = bytes.NewReader(raw)
	}

	req := httptest.NewRequest(method, path, reader)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func adminHeaders() map[string]string {
	return map[string]string{HeaderAdminKey: testAdminKey}
}

func deviceHeaders() map[string]string {
	return map[string]string{HeaderDeviceToken: testToken}
}

// decodeData unmarshals the `data` field of a success envelope into dst.
func decodeData(t *testing.T, rec *httptest.ResponseRecorder, dst any) {
	t.Helper()
	var env struct {
		Data    json.RawMessage `json:"data"`
		Message string          `json:"message"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &env), "body: %s", rec.Body.String())
	require.NoError(t, json.Unmarshal(env.Data, dst))
}

// errorMessage extracts the `error` field of a failure response.
func errorMessage(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	var body errorResponse
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body), "body: %s", rec.Body.String())
	return body.Error
}

func createTicketBody() map[string]any {
	return map[string]any{
		"type":            "bug",
		"title":           "Crash on save",
		"body":            "Tapping save closes the app",
		"deviceToken":     testToken,
		"appName":         "notes",
		"appVersion":      "1.4.2",
		"os":              "iOS 18.2",
		"platform":        "ios",
		"deviceModel":     "iPhone 15",
		"logs":            "panic: nil map",
		"logsDurationMin": 5,
	}
}

// createTicket posts a ticket through the public API and returns it.
func createTicket(t *testing.T, h *harness) *model.Ticket {
	t.Helper()
	rec := do(t, h.public, http.MethodPost, "/api/tickets", createTicketBody(), nil)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())

	var ticket model.Ticket
	decodeData(t, rec, &ticket)
	return &ticket
}

func TestNewServerDefaultsLogger(t *testing.T) {
	srv := NewServer(nil, "key", nil)

	assert.NotNil(t, srv.Logger)
	assert.NotNil(t, srv.logger())
}

func TestHealthEndpoints(t *testing.T) {
	h := newHarness(t)

	for name, router := range map[string]http.Handler{"public": h.public, "internal": h.internal} {
		t.Run(name, func(t *testing.T) {
			rec := do(t, router, http.MethodGet, "/healthz", nil, nil)

			assert.Equal(t, http.StatusOK, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			var status map[string]string
			decodeData(t, rec, &status)
			assert.Equal(t, "ok", status["status"])
		})
	}
}
