package api

import (
	"net/http"

	"flagit/internal/model"
	"flagit/internal/service"

	"github.com/go-chi/chi/v5"
)

// createTicketRequest is the POST /api/tickets body.
type createTicketRequest struct {
	Type            model.TicketType `json:"type"`
	Title           string           `json:"title"`
	Body            string           `json:"body"`
	DeviceToken     string           `json:"deviceToken"`
	AppName         string           `json:"appName"`
	AppVersion      string           `json:"appVersion"`
	OS              string           `json:"os"`
	Platform        string           `json:"platform"`
	DeviceModel     string           `json:"deviceModel"`
	Logs            string           `json:"logs"`
	LogsDurationMin int              `json:"logsDurationMin"`
}

// ticketWithMessages is what a reporter sees when fetching their ticket.
type ticketWithMessages struct {
	*model.Ticket
	Messages []*model.Message `json:"messages"`
}

// handleCreateTicket creates a ticket. This is the only public endpoint that
// does not require the device token as a header: it arrives in the body,
// because this is the call that establishes ownership.
func (s *Server) handleCreateTicket(w http.ResponseWriter, r *http.Request) {
	var req createTicketRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	token := req.DeviceToken
	if token == "" {
		token = deviceTokenFrom(r)
	}

	ticket, err := s.Service.CreateTicket(r.Context(), service.NewTicketInput{
		Type:            req.Type,
		Title:           req.Title,
		Body:            req.Body,
		DeviceToken:     token,
		AppName:         req.AppName,
		AppVersion:      req.AppVersion,
		OS:              req.OS,
		Platform:        req.Platform,
		DeviceModel:     req.DeviceModel,
		Logs:            req.Logs,
		LogsDurationMin: req.LogsDurationMin,
	})
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	s.writeJSON(w, http.StatusCreated, ticket, "ticket created")
}

// handleGetTicket returns a ticket and its conversation to its reporter.
func (s *Server) handleGetTicket(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ticket, err := s.Service.GetTicket(id, deviceTokenFrom(r))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	messages, err := s.Service.DB.ListMessagesByTicket(ticket.ID)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	// The reporter sent these logs; echoing them back adds nothing and makes
	// the response needlessly large.
	view := *ticket
	view.LogRingBuffer = ""

	s.writeJSON(w, http.StatusOK, ticketWithMessages{Ticket: &view, Messages: messages}, "")
}

// handleListMessages returns a ticket's conversation to its reporter.
func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := s.Service.ListMessages(chi.URLParam(r, "id"), deviceTokenFrom(r))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, messages, "")
}

// messageRequest is the body for posting a message.
type messageRequest struct {
	Body string `json:"body"`
}

// handlePostMessage appends a reporter's reply to their ticket.
func (s *Server) handlePostMessage(w http.ResponseWriter, r *http.Request) {
	var req messageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	m, err := s.Service.PostUserMessage(chi.URLParam(r, "id"), deviceTokenFrom(r), req.Body)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, m, "message posted")
}
