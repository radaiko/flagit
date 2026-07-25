package api

import (
	"net/http"
	"strings"
	"testing"

	"flagit/internal/db"
	"flagit/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTicketEndpoint(t *testing.T) {
	h := newHarness(t)

	rec := do(t, h.public, http.MethodPost, "/api/tickets", createTicketBody(), nil)

	require.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))

	var ticket model.Ticket
	decodeData(t, rec, &ticket)
	assert.True(t, db.ValidTicketID(ticket.ID))
	assert.Equal(t, model.TypeBug, ticket.Type)
	assert.Equal(t, model.StatusOpen, ticket.Status)
	assert.Equal(t, "notes", ticket.AppName)
	assert.NotContains(t, rec.Body.String(), testToken, "the device token is never echoed back")
	assert.NotContains(t, rec.Body.String(), "deviceTokenHash")
}

func TestCreateTicketAcceptsTokenFromHeader(t *testing.T) {
	h := newHarness(t)
	body := createTicketBody()
	delete(body, "deviceToken")

	rec := do(t, h.public, http.MethodPost, "/api/tickets", body, deviceHeaders())

	require.Equal(t, http.StatusCreated, rec.Code)

	var ticket model.Ticket
	decodeData(t, rec, &ticket)
	// The ticket is retrievable with the same token, proving it was recorded.
	got := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID, nil, deviceHeaders())
	assert.Equal(t, http.StatusOK, got.Code)
}

func TestCreateTicketValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(map[string]any)
		wantMsg string
	}{
		{"bad type", func(b map[string]any) { b["type"] = "question" }, "type must be"},
		{"no title", func(b map[string]any) { b["title"] = "  " }, "title is required"},
		{"no token", func(b map[string]any) { delete(b, "deviceToken") }, "deviceToken is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := newHarness(t)
			body := createTicketBody()
			tt.mutate(body)

			rec := do(t, h.public, http.MethodPost, "/api/tickets", body, nil)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, errorMessage(t, rec), tt.wantMsg)
		})
	}
}

func TestCreateTicketRejectsMalformedJSON(t *testing.T) {
	h := newHarness(t)

	tests := map[string]string{
		"not json":        "{oops",
		"unknown field":   `{"type":"bug","title":"x","deviceToken":"t","surprise":1}`,
		"trailing object": `{"type":"bug","title":"x","deviceToken":"t"}{"type":"bug"}`,
		"empty body":      "",
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			rec := do(t, h.public, http.MethodPost, "/api/tickets", body, nil)

			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Contains(t, errorMessage(t, rec), "invalid JSON body")
		})
	}
}

func TestCreateTicketRejectsOversizedBody(t *testing.T) {
	h := newHarness(t)
	body := createTicketBody()
	body["logs"] = strings.Repeat("x", maxRequestBytes+1024)

	rec := do(t, h.public, http.MethodPost, "/api/tickets", body, nil)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestGetTicketEndpoint(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)
	postAgentMessage(t, h, ticket.ID, "On it")

	rec := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID, nil, deviceHeaders())

	require.Equal(t, http.StatusOK, rec.Code)
	var got ticketWithMessages
	decodeData(t, rec, &got)
	assert.Equal(t, ticket.ID, got.ID)
	assert.Equal(t, model.StatusOpen, got.Status)
	require.Len(t, got.Messages, 1)
	assert.Equal(t, "On it", got.Messages[0].Body)
	assert.Empty(t, got.LogRingBuffer, "logs are not echoed back to the reporter")
}

func TestGetTicketAcceptsTokenAsQueryParam(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	// Overlay links open in a browser and cannot set headers.
	rec := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID+"?token="+testToken, nil, nil)

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestGetTicketRejectsWrongToken(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID, nil,
		map[string]string{HeaderDeviceToken: "someone-elses-token"})

	assert.Equal(t, http.StatusForbidden, rec.Code)
	assert.Contains(t, errorMessage(t, rec), "device token")
}

func TestGetTicketRequiresToken(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID, nil, nil)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Contains(t, errorMessage(t, rec), "missing device token")
}

func TestGetTicketUnknownID(t *testing.T) {
	h := newHarness(t)

	for _, id := range []string{"FLG-ZZZZZZ", "not-a-ticket"} {
		t.Run(id, func(t *testing.T) {
			rec := do(t, h.public, http.MethodGet, "/api/tickets/"+id, nil, deviceHeaders())

			assert.Equal(t, http.StatusNotFound, rec.Code)
		})
	}
}

func TestPostAndListMessagesEndpoint(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.public, http.MethodPost, "/api/tickets/"+ticket.ID+"/messages",
		map[string]any{"body": "Still broken"}, deviceHeaders())
	require.Equal(t, http.StatusCreated, rec.Code)

	var m model.Message
	decodeData(t, rec, &m)
	assert.Equal(t, model.RoleUser, m.Role)
	assert.Equal(t, "Still broken", m.Body)

	list := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID+"/messages", nil, deviceHeaders())
	require.Equal(t, http.StatusOK, list.Code)
	var messages []*model.Message
	decodeData(t, list, &messages)
	require.Len(t, messages, 1)
}

func TestPostMessageErrors(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	tests := []struct {
		name     string
		id       string
		body     any
		headers  map[string]string
		wantCode int
	}{
		{"empty body", ticket.ID, map[string]any{"body": "  "}, deviceHeaders(), http.StatusBadRequest},
		{"malformed json", ticket.ID, "{", deviceHeaders(), http.StatusBadRequest},
		{"wrong token", ticket.ID, map[string]any{"body": "hi"},
			map[string]string{HeaderDeviceToken: "nope"}, http.StatusForbidden},
		{"no token", ticket.ID, map[string]any{"body": "hi"}, nil, http.StatusUnauthorized},
		{"unknown ticket", "FLG-ZZZZZZ", map[string]any{"body": "hi"}, deviceHeaders(), http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, h.public, http.MethodPost, "/api/tickets/"+tt.id+"/messages", tt.body, tt.headers)

			assert.Equal(t, tt.wantCode, rec.Code, "body: %s", rec.Body.String())
		})
	}
}

func TestListMessagesRejectsWrongToken(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	rec := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID+"/messages", nil,
		map[string]string{HeaderDeviceToken: "nope"})

	assert.Equal(t, http.StatusForbidden, rec.Code)
}

func TestPublicAPIDoesNotExposeInternalRoutes(t *testing.T) {
	h := newHarness(t)

	for _, path := range []string{"/internal/tickets", "/internal/poll", "/internal/settings"} {
		t.Run(path, func(t *testing.T) {
			rec := do(t, h.public, http.MethodGet, path, nil, adminHeaders())

			assert.Equal(t, http.StatusNotFound, rec.Code, "the public router must not serve %s", path)
		})
	}
}

func TestGetTicketDBFailure(t *testing.T) {
	h := newHarness(t)
	ticket := createTicket(t, h)

	// Drop messages so the ticket loads but its conversation cannot.
	_, err := h.Service.DB.SQL().Exec(`DROP TABLE messages`)
	require.NoError(t, err)

	rec := do(t, h.public, http.MethodGet, "/api/tickets/"+ticket.ID, nil, deviceHeaders())

	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Equal(t, "internal server error", errorMessage(t, rec))
}
