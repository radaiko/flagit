package api

import (
	"fmt"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"flagit/internal/db"
	"flagit/internal/model"
	"flagit/internal/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postAgentMessage posts an agent reply through the internal API.
func postAgentMessage(t *testing.T, h *harness, ticketID, body string) {
	t.Helper()
	rec := do(t, h.internal, http.MethodPost, "/internal/tickets/"+ticketID+"/messages",
		map[string]any{"body": body}, adminHeaders())
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
}

// ------------------------------------------------------------------ auth --

func TestInternalRoutesRequireAdminKey(t *testing.T) {
	h := newHarness(t)

	routes := []struct{ method, path string }{
		{http.MethodGet, "/internal/poll"},
		{http.MethodGet, "/internal/tickets"},
		{http.MethodGet, "/internal/tickets/FLG-ABC123"},
		{http.MethodPatch, "/internal/tickets/FLG-ABC123"},
		{http.MethodGet, "/internal/tickets/FLG-ABC123/messages"},
		{http.MethodPost, "/internal/tickets/FLG-ABC123/messages"},
		{http.MethodGet, "/internal/tickets/FLG-ABC123/commits"},
		{http.MethodPost, "/internal/tickets/FLG-ABC123/commits"},
		{http.MethodPost, "/internal/tickets/batch"},
		{http.MethodGet, "/internal/apps"},
		{http.MethodPatch, "/internal/apps/notes"},
		{http.MethodGet, "/internal/settings"},
		{http.MethodPatch, "/internal/settings"},
	}
	for _, route := range routes {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			rec := do(t, h.internal, route.method, route.path, map[string]any{}, nil)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Contains(t, errorMessage(t, rec), "admin key")
		})
	}
}

func TestAdminKeyRejectsWrongValue(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets", nil,
		map[string]string{HeaderAdminKey: "wrong-key"})

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestAdminKeyAcceptsBearerToken(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets", nil,
		map[string]string{"Authorization": "Bearer " + testAdminKey})

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestAdminKeyIgnoresNonBearerAuthorization(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets", nil,
		map[string]string{"Authorization": "Basic " + testAdminKey})

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUnconfiguredAdminKeyFailsClosed(t *testing.T) {
	// A server built with an empty hash must reject every request regardless
	// of what the caller sends.
	db, err := db.InitDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })
	svc := service.New(db, nil, "", slog.New(slog.DiscardHandler))
	srv := NewServer(svc, "", slog.New(slog.DiscardHandler))
	router := srv.InternalRouter()

	// An empty configured key must not be matchable by an empty header.
	rec := do(t, router, http.MethodGet, "/internal/tickets", nil, map[string]string{HeaderAdminKey: ""})

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Contains(t, errorMessage(t, rec), "not configured")
}

// ------------------------------------------------------------------ poll --

func TestPollEndpoint(t *testing.T) {
	h := newHarness(t)
	first := createTicket(t, h)
	second := createTicket(t, h)

	rec := do(t, h.internal, http.MethodGet, "/internal/poll", nil, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var resp pollResponse
	decodeData(t, rec, &resp)
	assert.Equal(t, 2, resp.Count)
	require.Len(t, resp.Tickets, 2)
	assert.Equal(t, first.ID, resp.Tickets[0].ID, "oldest change first")
	assert.Equal(t, second.ID, resp.Tickets[1].ID)
	assert.False(t, resp.Now.IsZero())
}

func TestPollSince(t *testing.T) {
	h := newHarness(t)
	first := createTicket(t, h)
	second := createTicket(t, h)

	since := first.CreatedAt.Format(time.RFC3339Nano)
	rec := do(t, h.internal, http.MethodGet, "/internal/poll?since="+since, nil, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var resp pollResponse
	decodeData(t, rec, &resp)
	require.Len(t, resp.Tickets, 1)
	assert.Equal(t, second.ID, resp.Tickets[0].ID)
}

func TestPollIncludesLogsForHermes(t *testing.T) {
	h := newHarness(t)
	createTicket(t, h)

	rec := do(t, h.internal, http.MethodGet, "/internal/poll", nil, adminHeaders())

	assert.Contains(t, rec.Body.String(), "panic: nil map", "Hermes needs the diagnostics")
	assert.NotContains(t, rec.Body.String(), testToken)
}

func TestPollRejectsBadSince(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.internal, http.MethodGet, "/internal/poll?since=yesterday", nil, adminHeaders())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, errorMessage(t, rec), "invalid since timestamp")
}

func TestPollDBFailure(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.Service.DB.Close())

	rec := do(t, h.internal, http.MethodGet, "/internal/poll", nil, adminHeaders())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --------------------------------------------------------------- tickets --

func TestListTicketsEndpoint(t *testing.T) {
	h := newHarness(t)
	createTicket(t, h)

	other := createTicketBody()
	other["appName"] = "timer"
	other["type"] = "feature"
	rec := do(t, h.public, http.MethodPost, "/api/tickets", other, nil)
	require.Equal(t, http.StatusCreated, rec.Code)

	tests := []struct {
		query string
		want  int
	}{
		{"", 2},
		{"?app=notes", 1},
		{"?type=feature", 1},
		{"?status=open", 2},
		{"?status=closed", 0},
		{"?limit=1", 1},
		{"?app=notes&type=feature", 0},
	}
	for _, tt := range tests {
		t.Run("query="+tt.query, func(t *testing.T) {
			rec := do(t, h.internal, http.MethodGet, "/internal/tickets"+tt.query, nil, adminHeaders())

			require.Equal(t, http.StatusOK, rec.Code)
			var page ticketPage
			decodeData(t, rec, &page)
			assert.Len(t, page.Tickets, tt.want)
		})
	}
}

func TestListTicketsRejectsBadLimit(t *testing.T) {
	h := newHarness(t)

	for _, q := range []string{"?limit=abc", "?limit=-1"} {
		t.Run(q, func(t *testing.T) {
			rec := do(t, h.internal, http.MethodGet, "/internal/tickets"+q, nil, adminHeaders())

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

func TestListTicketsDBFailure(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.Service.DB.Close())

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets", nil, adminHeaders())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminGetTicket(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)
	postAgentMessage(t, h, ticket.ID, "On it")
	rec := do(t, h.internal, http.MethodPost, "/internal/tickets/"+ticket.ID+"/commits",
		map[string]any{"commitHash": "a1b2c3d", "branch": "fix/crash", "message": "fix: guard nil map"}, adminHeaders())
	require.Equal(t, http.StatusCreated, rec.Code)

	got := do(t, h.internal, http.MethodGet, "/internal/tickets/"+ticket.ID, nil, adminHeaders())

	require.Equal(t, http.StatusOK, got.Code)
	var view adminTicketView
	decodeData(t, got, &view)
	assert.Equal(t, ticket.ID, view.ID)
	assert.Equal(t, "panic: nil map", view.LogRingBuffer, "admins see the diagnostics")
	require.Len(t, view.Messages, 1)
	require.Len(t, view.Commits, 1)
	assert.Equal(t, "a1b2c3d", view.Commits[0].CommitHash)
}

func TestAdminGetTicketNotFound(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets/FLG-ZZZZZZ", nil, adminHeaders())

	assert.Equal(t, http.StatusNotFound, rec.Code)
}

func TestAdminGetTicketPartialFailures(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	_, err := h.Service.DB.SQL().Exec(`DROP TABLE commits`)
	require.NoError(t, err)
	rec := do(t, h.internal, http.MethodGet, "/internal/tickets/"+ticket.ID, nil, adminHeaders())
	assert.Equal(t, http.StatusInternalServerError, rec.Code)

	_, err = h.Service.DB.SQL().Exec(`DROP TABLE messages`)
	require.NoError(t, err)
	rec = do(t, h.internal, http.MethodGet, "/internal/tickets/"+ticket.ID, nil, adminHeaders())
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestAdminListMessages(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)
	postAgentMessage(t, h, ticket.ID, "On it")

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets/"+ticket.ID+"/messages", nil, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var messages []*model.Message
	decodeData(t, rec, &messages)
	require.Len(t, messages, 1)
	assert.Equal(t, model.RoleAgent, messages[0].Role)

	missing := do(t, h.internal, http.MethodGet, "/internal/tickets/FLG-ZZZZZZ/messages", nil, adminHeaders())
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

func TestAgentMessageEndpointErrors(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	tests := []struct {
		name     string
		id       string
		body     any
		wantCode int
	}{
		{"empty body", ticket.ID, map[string]any{"body": " "}, http.StatusBadRequest},
		{"malformed", ticket.ID, "{", http.StatusBadRequest},
		{"unknown ticket", "FLG-ZZZZZZ", map[string]any{"body": "hi"}, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h.internal, http.MethodPost, "/internal/tickets/"+tt.id+"/messages", tt.body, adminHeaders())

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}

func TestUpdateTicketEndpoint(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"status": "in-progress"}, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var got model.Ticket
	decodeData(t, rec, &got)
	assert.Equal(t, model.StatusInProgress, got.Status)
}

func TestUpdateTicketWithComment(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"status": "resolved", "comment": "Fixed in 1.5.0"}, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)

	messages := do(t, h.internal, http.MethodGet, "/internal/tickets/"+ticket.ID+"/messages", nil, adminHeaders())
	var got []*model.Message
	decodeData(t, messages, &got)
	require.Len(t, got, 1)
	assert.Equal(t, "Fixed in 1.5.0", got[0].Body)
	assert.Equal(t, model.RoleAgent, got[0].Role)
}

func TestUpdateTicketShippedVersion(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	// Walk the documented workflow to reach shipped.
	for _, status := range []string{"in-progress", "resolved"} {
		step := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
			map[string]any{"status": status}, adminHeaders())
		require.Equal(t, http.StatusOK, step.Code, "body: %s", step.Body.String())
	}

	rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"status": "shipped", "shippedVersion": "1.5.0"}, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var got model.Ticket
	decodeData(t, rec, &got)
	assert.Equal(t, "1.5.0", got.ShippedVersion)
}

func TestUpdateTicketRejectsAWorkflowSkip(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"status": "shipped"}, adminHeaders())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, errorMessage(t, rec), "cannot move a ticket")
}

func TestUpdateTicketForceOverridesTheWorkflow(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"status": "shipped", "force": true}, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var got model.Ticket
	decodeData(t, rec, &got)
	assert.Equal(t, model.StatusShipped, got.Status)
}

func TestUpdateTicketAcceptsACommentAlone(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"comment": "Still looking into this"}, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code, "body: %s", rec.Body.String())
	var got model.Ticket
	decodeData(t, rec, &got)
	assert.Equal(t, model.StatusOpen, got.Status, "the status is untouched")

	messages := do(t, h.internal, http.MethodGet, "/internal/tickets/"+ticket.ID+"/messages", nil, adminHeaders())
	var conversation []*model.Message
	decodeData(t, messages, &conversation)
	require.Len(t, conversation, 1)
	assert.Equal(t, "Still looking into this", conversation[0].Body)
}

func TestUpdateTicketRejectsAnEmptyPatch(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{}, adminHeaders())

	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, errorMessage(t, rec), "provide a status, a comment, or both")
}

func TestUpdateTicketErrors(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	tests := []struct {
		name     string
		id       string
		body     any
		wantCode int
	}{
		{"bad status", ticket.ID, map[string]any{"status": "done"}, http.StatusBadRequest},
		{"malformed", ticket.ID, "{", http.StatusBadRequest},
		{"unknown ticket", "FLG-ZZZZZZ", map[string]any{"status": "closed"}, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+tt.id, tt.body, adminHeaders())

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}
}

func TestUpdateTicketCommentFailure(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)
	_, err := h.Service.DB.SQL().Exec(`DROP TABLE messages`)
	require.NoError(t, err)

	rec := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"status": "resolved", "comment": "done"}, adminHeaders())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// --------------------------------------------------------------- commits --

func TestCommitEndpoints(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.internal, http.MethodPost, "/internal/tickets/"+ticket.ID+"/commits",
		map[string]any{"commitHash": "a1b2c3d", "branch": "fix/crash", "message": "fix: guard nil map"}, adminHeaders())
	require.Equal(t, http.StatusCreated, rec.Code)

	var c model.CommitInfo
	decodeData(t, rec, &c)
	assert.Equal(t, "a1b2c3d", c.CommitHash)

	list := do(t, h.internal, http.MethodGet, "/internal/tickets/"+ticket.ID+"/commits", nil, adminHeaders())
	require.Equal(t, http.StatusOK, list.Code)
	var commits []*model.CommitInfo
	decodeData(t, list, &commits)
	assert.Len(t, commits, 1)
}

func TestCommitEndpointErrors(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	tests := []struct {
		name     string
		id       string
		body     any
		wantCode int
	}{
		{"missing hash", ticket.ID, map[string]any{"branch": "main"}, http.StatusBadRequest},
		{"malformed", ticket.ID, "{", http.StatusBadRequest},
		{"unknown ticket", "FLG-ZZZZZZ", map[string]any{"commitHash": "abc"}, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h.internal, http.MethodPost, "/internal/tickets/"+tt.id+"/commits", tt.body, adminHeaders())

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}

	missing := do(t, h.internal, http.MethodGet, "/internal/tickets/FLG-ZZZZZZ/commits", nil, adminHeaders())
	assert.Equal(t, http.StatusNotFound, missing.Code)
}

// ----------------------------------------------------------------- batch --

func TestBatchUpdateEndpoint(t *testing.T) {
	h := newHarness(t)
	first := createTicket(t, h)
	second := createTicket(t, h)

	rec := do(t, h.internal, http.MethodPost, "/internal/tickets/batch", map[string]any{
		"ticketIds":      []string{first.ID, second.ID, "FLG-ZZZZZZ"},
		"status":         "shipped",
		"shippedVersion": "1.5.0",
		"force":          true,
	}, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var result service.BatchResult
	decodeData(t, rec, &result)
	assert.ElementsMatch(t, []string{first.ID, second.ID}, result.Updated)
	assert.Equal(t, service.ReasonNotFound, result.Failed["FLG-ZZZZZZ"],
		"a stable reason code, not a raw error string")

	got := do(t, h.internal, http.MethodGet, "/internal/tickets?status=shipped", nil, adminHeaders())
	var page ticketPage
	decodeData(t, got, &page)
	require.Len(t, page.Tickets, 2)
	assert.Equal(t, "1.5.0", page.Tickets[0].ShippedVersion)
}

func TestBatchUpdateErrors(t *testing.T) {
	h := newHarness(t)

	tests := []struct {
		name string
		body any
	}{
		{"no ids", map[string]any{"ticketIds": []string{}, "status": "closed"}},
		{"bad status", map[string]any{"ticketIds": []string{"FLG-ABC123"}, "status": "done"}},
		{"malformed", "{"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h.internal, http.MethodPost, "/internal/tickets/batch", tt.body, adminHeaders())

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}

// ------------------------------------------------------------------ apps --

func TestAppsEndpoints(t *testing.T) {
	h := newHarness(t)
	createTicket(t, h)

	rec := do(t, h.internal, http.MethodGet, "/internal/apps", nil, adminHeaders())
	require.Equal(t, http.StatusOK, rec.Code)
	var apps []*model.App
	decodeData(t, rec, &apps)
	require.Len(t, apps, 1)
	assert.Equal(t, "notes", apps[0].Name)
	assert.False(t, apps[0].AutoProcessEnabled)

	patch := do(t, h.internal, http.MethodPatch, "/internal/apps/notes",
		map[string]any{"autoProcessEnabled": true}, adminHeaders())
	require.Equal(t, http.StatusOK, patch.Code)
	var app model.App
	decodeData(t, patch, &app)
	assert.True(t, app.AutoProcessEnabled)
}

func TestUpdateAppErrors(t *testing.T) {
	h := newHarness(t)
	createTicket(t, h)

	tests := []struct {
		name     string
		name_    string
		body     any
		wantCode int
	}{
		{"missing field", "notes", map[string]any{}, http.StatusBadRequest},
		{"malformed", "notes", "{", http.StatusBadRequest},
		{"unknown app", "ghost", map[string]any{"autoProcessEnabled": true}, http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h.internal, http.MethodPatch, "/internal/apps/"+tt.name_, tt.body, adminHeaders())

			assert.Equal(t, tt.wantCode, rec.Code)
		})
	}

	notFound := do(t, h.internal, http.MethodPatch, "/internal/apps/ghost",
		map[string]any{"autoProcessEnabled": true}, adminHeaders())
	assert.Equal(t, "app not found", errorMessage(t, notFound))
}

func TestListAppsDBFailure(t *testing.T) {
	h := newHarness(t)
	require.NoError(t, h.Service.DB.Close())

	rec := do(t, h.internal, http.MethodGet, "/internal/apps", nil, adminHeaders())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

func TestUpdateAppDBFailure(t *testing.T) {
	h := newHarness(t)
	createTicket(t, h)
	require.NoError(t, h.Service.DB.Close())

	rec := do(t, h.internal, http.MethodPatch, "/internal/apps/notes",
		map[string]any{"autoProcessEnabled": true}, adminHeaders())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
}

// -------------------------------------------------------------- settings --

func TestSettingsEndpoints(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.internal, http.MethodGet, "/internal/settings", nil, adminHeaders())
	require.Equal(t, http.StatusOK, rec.Code)
	var settings service.Settings
	decodeData(t, rec, &settings)
	assert.False(t, settings.GlobalAutoProcess)
	assert.Empty(t, settings.HermesWebhookURL)

	patch := do(t, h.internal, http.MethodPatch, "/internal/settings", map[string]any{
		"globalAutoProcess": true,
		"hermesWebhookUrl":  "https://hermes.example/hook",
	}, adminHeaders())
	require.Equal(t, http.StatusOK, patch.Code)
	decodeData(t, patch, &settings)
	assert.True(t, settings.GlobalAutoProcess)
	assert.Equal(t, "https://hermes.example/hook", settings.HermesWebhookURL)

	assert.NotContains(t, patch.Body.String(), testAdminKey, "the admin key is never served back")
}

func TestSettingsErrors(t *testing.T) {
	h := newHarness(t)

	bad := do(t, h.internal, http.MethodPatch, "/internal/settings",
		map[string]any{"hermesWebhookUrl": "hermes.example/hook"}, adminHeaders())
	assert.Equal(t, http.StatusBadRequest, bad.Code)

	malformed := do(t, h.internal, http.MethodPatch, "/internal/settings", "{", adminHeaders())
	assert.Equal(t, http.StatusBadRequest, malformed.Code)

	require.NoError(t, h.Service.DB.Close())
	failed := do(t, h.internal, http.MethodGet, "/internal/settings", nil, adminHeaders())
	assert.Equal(t, http.StatusInternalServerError, failed.Code)
	failedPatch := do(t, h.internal, http.MethodPatch, "/internal/settings",
		map[string]any{"globalAutoProcess": true}, adminHeaders())
	assert.Equal(t, http.StatusInternalServerError, failedPatch.Code)
}

// ---------------------------------------------------------- integration --

// TestFullTicketLifecycle walks the whole flow a real ticket goes through:
// the app reports it, Hermes polls it up, works it, and ships the fix.
func TestFullTicketLifecycle(t *testing.T) {
	h := newHarness(t)

	// The dev turns on auto-processing globally and points Flagit at Hermes.
	setup := do(t, h.internal, http.MethodPatch, "/internal/settings", map[string]any{
		"globalAutoProcess": true,
		"hermesWebhookUrl":  "https://hermes.example/hook",
	}, adminHeaders())
	require.Equal(t, http.StatusOK, setup.Code)

	// A user reports a bug from inside the app.
	ticket := createTicket(t, h)
	assert.Equal(t, 1, h.hook.calls(), "an unknown app inherits the global toggle")
	assert.Equal(t, ticket.ID, h.hook.payloads[0].TicketID)

	// Hermes polls, picks it up and starts work.
	poll := do(t, h.internal, http.MethodGet, "/internal/poll", nil, adminHeaders())
	var polled pollResponse
	decodeData(t, poll, &polled)
	require.Len(t, polled.Tickets, 1)

	inProgress := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"status": "in-progress", "comment": "Reproduced, investigating"}, adminHeaders())
	require.Equal(t, http.StatusOK, inProgress.Code)

	// The user sees the update and replies.
	view := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID, nil, deviceHeaders())
	var seen ticketWithMessages
	decodeData(t, view, &seen)
	assert.Equal(t, model.StatusInProgress, seen.Status)
	require.Len(t, seen.Messages, 1)

	reply := do(t, h.public, http.MethodPost, "/api/tickets/"+ticket.ID+"/messages",
		map[string]any{"body": "Happens every time on iPhone 15"}, deviceHeaders())
	require.Equal(t, http.StatusCreated, reply.Code)

	// Hermes commits a fix and resolves the ticket.
	commit := do(t, h.internal, http.MethodPost, "/internal/tickets/"+ticket.ID+"/commits",
		map[string]any{"commitHash": "a1b2c3d", "branch": "fix/FLG-crash", "message": "fix: guard nil map"}, adminHeaders())
	require.Equal(t, http.StatusCreated, commit.Code)

	resolved := do(t, h.internal, http.MethodPatch, "/internal/tickets/"+ticket.ID,
		map[string]any{"status": "resolved", "comment": "Fixed, pending release"}, adminHeaders())
	require.Equal(t, http.StatusOK, resolved.Code)

	// The release goes out and the ticket is marked shipped in bulk.
	batch := do(t, h.internal, http.MethodPost, "/internal/tickets/batch", map[string]any{
		"ticketIds":      []string{ticket.ID},
		"status":         "shipped",
		"shippedVersion": "1.5.0",
	}, adminHeaders())
	require.Equal(t, http.StatusOK, batch.Code)

	// Final state, as the admin sees it.
	final := do(t, h.internal, http.MethodGet, "/internal/tickets/"+ticket.ID, nil, adminHeaders())
	var view2 adminTicketView
	decodeData(t, final, &view2)
	assert.Equal(t, model.StatusShipped, view2.Status)
	assert.Equal(t, "1.5.0", view2.ShippedVersion)
	assert.Len(t, view2.Messages, 3, "two agent updates and one user reply")
	assert.Len(t, view2.Commits, 1)

	// And as the reporter sees it.
	userView := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID, nil, deviceHeaders())
	var seenFinal ticketWithMessages
	decodeData(t, userView, &seenFinal)
	assert.Equal(t, model.StatusShipped, seenFinal.Status)
	assert.Len(t, seenFinal.Messages, 3)
}

func TestPollingWalksForwardWithoutGaps(t *testing.T) {
	h := newHarness(t)
	first := createTicket(t, h)

	// Poll everything, then use the returned cursor for the next round.
	rec := do(t, h.internal, http.MethodGet, "/internal/poll", nil, adminHeaders())
	var round1 pollResponse
	decodeData(t, rec, &round1)
	require.Len(t, round1.Tickets, 1)
	cursor := round1.Tickets[0].UpdatedAt

	second := createTicket(t, h)
	require.NoError(t, h.Service.DB.CreateMessage(&model.Message{
		TicketID: first.ID, Body: "ping", Role: model.RoleAgent,
	}))

	rec = do(t, h.internal, http.MethodGet,
		"/internal/poll?since="+cursor.Format(time.RFC3339Nano), nil, adminHeaders())
	var round2 pollResponse
	decodeData(t, rec, &round2)

	ids := []string{}
	for _, ticket := range round2.Tickets {
		ids = append(ids, ticket.ID)
	}
	assert.ElementsMatch(t, []string{first.ID, second.ID}, ids,
		"a touched ticket and a new one both appear exactly once")
}

func TestTicketsAreIsolatedBetweenDevices(t *testing.T) {
	h := newHarness(t)
	mine := createTicket(t, h)

	other := createTicketBody()
	other["deviceToken"] = "aaaaaaaa-0000-4000-8000-000000000002"
	rec := do(t, h.public, http.MethodPost, "/api/tickets", other, nil)
	require.Equal(t, http.StatusCreated, rec.Code)
	var theirs model.Ticket
	decodeData(t, rec, &theirs)

	// My token must not open their ticket, even though the ID is valid.
	forbidden := do(t, h.public, http.MethodGet, "/api/tickets/"+theirs.ID, nil, deviceHeaders())
	assert.Equal(t, http.StatusForbidden, forbidden.Code)

	mineOK := do(t, h.public, http.MethodGet, "/api/tickets/"+mine.ID, nil, deviceHeaders())
	assert.Equal(t, http.StatusOK, mineOK.Code)
}

func TestDuplicateTicketIDsNeverHappenAcrossManyCreates(t *testing.T) {
	h := newHarness(t)
	seen := map[string]bool{}

	for i := 0; i < 50; i++ {
		ticket := createTicket(t, h)
		require.False(t, seen[ticket.ID], "duplicate id %s at iteration %d", ticket.ID, i)
		seen[ticket.ID] = true
	}

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets", nil, adminHeaders())
	var page ticketPage
	decodeData(t, rec, &page)
	assert.Len(t, page.Tickets, 50, fmt.Sprintf("expected %d distinct tickets", len(seen)))
	assert.Equal(t, 50, page.Total)
	assert.False(t, page.HasMore)
}

func TestUnknownTicketIDShapesAreRejectedConsistently(t *testing.T) {
	h := newHarness(t)

	for _, id := range []string{"FLG-ZZZZZZ", "abc", "FLG-!!!", "%20"} {
		t.Run(id, func(t *testing.T) {
			public := do(t, h.public, http.MethodGet, "/api/tickets/"+id, nil, deviceHeaders())
			assert.Equal(t, http.StatusNotFound, public.Code)

			admin := do(t, h.internal, http.MethodGet, "/internal/tickets/"+id, nil, adminHeaders())
			assert.Equal(t, http.StatusNotFound, admin.Code)
		})
	}
}

func TestListTicketsPaging(t *testing.T) {
	h := newHarness(t)
	for i := 0; i < 5; i++ {
		createTicket(t, h)
	}

	first := do(t, h.internal, http.MethodGet, "/internal/tickets?limit=2", nil, adminHeaders())
	require.Equal(t, http.StatusOK, first.Code)
	var page ticketPage
	decodeData(t, first, &page)
	assert.Len(t, page.Tickets, 2)
	assert.Equal(t, 5, page.Total, "the total ignores the page size")
	assert.Equal(t, 2, page.Limit)
	assert.True(t, page.HasMore)

	last := do(t, h.internal, http.MethodGet, "/internal/tickets?limit=2&offset=4", nil, adminHeaders())
	decodeData(t, last, &page)
	assert.Len(t, page.Tickets, 1)
	assert.Equal(t, 4, page.Offset)
	assert.False(t, page.HasMore, "the final page says so")
}

func TestListTicketsZeroLimitReturnsTheCountOnly(t *testing.T) {
	h := newHarness(t)
	createTicket(t, h)
	createTicket(t, h)

	// limit=0 is how a caller asks "how many are there?" without paying to
	// transfer any of them. It must not be read as "unlimited".
	rec := do(t, h.internal, http.MethodGet, "/internal/tickets?limit=0", nil, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var page ticketPage
	decodeData(t, rec, &page)
	assert.Empty(t, page.Tickets)
	assert.Equal(t, 2, page.Total)
	assert.True(t, page.HasMore)
}

func TestPollZeroLimitReturnsNothing(t *testing.T) {
	h := newHarness(t)
	createTicket(t, h)

	rec := do(t, h.internal, http.MethodGet, "/internal/poll?limit=0", nil, adminHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var resp pollResponse
	decodeData(t, rec, &resp)
	assert.Empty(t, resp.Tickets)
	assert.Equal(t, 0, resp.Count)
}

func TestPollReportsWhenMoreIsWaiting(t *testing.T) {
	h := newHarness(t)
	createTicket(t, h)
	createTicket(t, h)
	createTicket(t, h)

	rec := do(t, h.internal, http.MethodGet, "/internal/poll?limit=2", nil, adminHeaders())

	var resp pollResponse
	decodeData(t, rec, &resp)
	assert.Len(t, resp.Tickets, 2)
	assert.Equal(t, 2, resp.Limit)
	assert.True(t, resp.HasMore, "a full page means the poller should come straight back")
}

func TestListAndPollRejectBadPagingParams(t *testing.T) {
	h := newHarness(t)

	for _, path := range []string{
		"/internal/tickets?offset=abc",
		"/internal/tickets?offset=-1",
		"/internal/poll?limit=abc",
		"/internal/poll?limit=-1",
	} {
		t.Run(path, func(t *testing.T) {
			rec := do(t, h.internal, http.MethodGet, path, nil, adminHeaders())

			assert.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}
}
