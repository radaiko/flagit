package api

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"flagit/internal/db"
	"flagit/internal/model"

	"github.com/go-chi/chi/v5"
)

// pollResponse is what Hermes receives from /internal/poll. Now is the
// timestamp to pass back as `since` on the next call; HasMore says the page
// was filled, so the poller should come straight back rather than waiting for
// its next tick.
type pollResponse struct {
	Tickets []*model.Ticket `json:"tickets"`
	Since   time.Time       `json:"since"`
	Now     time.Time       `json:"now"`
	Count   int             `json:"count"`
	Limit   int             `json:"limit"`
	HasMore bool            `json:"hasMore"`
}

// handlePoll returns tickets created or updated after ?since, one page at a
// time. An absent `since` starts from the beginning, so a fresh poller can
// bootstrap.
func (s *Server) handlePoll(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	var since time.Time
	if raw := q.Get("since"); raw != "" {
		parsed, err := db.ParseFlexibleTime(raw)
		if err != nil {
			s.writeError(w, http.StatusBadRequest, "invalid since timestamp: "+err.Error())
			return
		}
		since = parsed
	}

	limit, ok := s.intParam(w, q.Get("limit"), "limit")
	if !ok {
		return
	}

	// Captured before the query so a ticket written mid-query is not skipped
	// by the next poll.
	now := time.Now().UTC()

	tickets, err := s.Service.PollTickets(since, limit)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	effective := limit
	if effective <= 0 {
		effective = db.DefaultPageSize
	}
	s.writeJSON(w, http.StatusOK, pollResponse{
		Tickets: tickets,
		Since:   since,
		Now:     now,
		Count:   len(tickets),
		Limit:   effective,
		HasMore: len(tickets) >= effective,
	}, "")
}

// intParam parses an optional non-negative integer query parameter. It writes
// the error response itself and reports whether the caller should continue.
func (s *Server) intParam(w http.ResponseWriter, raw, name string) (int, bool) {
	if raw == "" {
		return 0, true
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 0 {
		s.writeError(w, http.StatusBadRequest, name+" must be a non-negative integer")
		return 0, false
	}
	return value, true
}

// ticketPage is a page of tickets plus what a caller needs to walk the rest.
type ticketPage struct {
	Tickets []*model.Ticket `json:"tickets"`
	Total   int             `json:"total"`
	Limit   int             `json:"limit"`
	Offset  int             `json:"offset"`
	HasMore bool            `json:"hasMore"`
}

// handleListTickets lists tickets for the admin dashboard, with optional
// app/status/type filters and offset/limit paging.
func (s *Server) handleListTickets(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := db.TicketFilter{
		AppName: q.Get("app"),
		Status:  model.Status(q.Get("status")),
		Type:    model.TicketType(q.Get("type")),
	}

	limit, ok := s.intParam(w, q.Get("limit"), "limit")
	if !ok {
		return
	}
	offset, ok := s.intParam(w, q.Get("offset"), "offset")
	if !ok {
		return
	}
	filter.Limit, filter.Offset = limit, offset

	tickets, err := s.Service.ListTickets(filter)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	total, err := s.Service.CountTickets(filter)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}

	effective := limit
	if effective <= 0 {
		effective = db.DefaultPageSize
	}
	s.writeJSON(w, http.StatusOK, ticketPage{
		Tickets: tickets,
		Total:   total,
		Limit:   effective,
		Offset:  offset,
		HasMore: offset+len(tickets) < total,
	}, "")
}

// adminTicketView is the full ticket, including everything hidden from the
// public API.
type adminTicketView struct {
	*model.Ticket
	Messages []*model.Message    `json:"messages"`
	Commits  []*model.CommitInfo `json:"commits"`
}

// handleAdminGetTicket returns a ticket with its conversation and commits.
func (s *Server) handleAdminGetTicket(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	ticket, err := s.Service.GetTicketByID(id)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	messages, err := s.Service.ListMessagesByID(ticket.ID)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	commits, err := s.Service.ListCommits(ticket.ID)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, adminTicketView{Ticket: ticket, Messages: messages, Commits: commits}, "")
}

// handleAdminListMessages returns a ticket's conversation without an
// ownership check.
func (s *Server) handleAdminListMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := s.Service.ListMessagesByID(chi.URLParam(r, "id"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, messages, "")
}

// handleAgentMessage records a reply from Hermes or an admin.
func (s *Server) handleAgentMessage(w http.ResponseWriter, r *http.Request) {
	var req messageRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	m, err := s.Service.PostAgentMessage(chi.URLParam(r, "id"), req.Body)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, m, "message posted")
}

// updateTicketRequest patches a ticket's status, optionally adding an agent
// comment in the same call.
//
// ShippedVersion is a pointer so an omitted field ("leave the recorded release
// alone") is distinguishable from an explicit empty one ("this is no longer
// shipped").
type updateTicketRequest struct {
	Status         model.Status `json:"status"`
	ShippedVersion *string      `json:"shippedVersion"`
	Comment        string       `json:"comment"`
}

// handleUpdateTicket applies a status change from Hermes or an admin. The
// status change and the comment are written in one transaction, so a failed
// comment cannot leave the ticket silently moved.
func (s *Server) handleUpdateTicket(w http.ResponseWriter, r *http.Request) {
	var req updateTicketRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	ticket, err := s.Service.UpdateStatusWithComment(
		chi.URLParam(r, "id"), req.Status, req.ShippedVersion, req.Comment)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, ticket, "ticket updated")
}

// commitRequest is the body Hermes posts after committing a fix.
type commitRequest struct {
	CommitHash string `json:"commitHash"`
	Branch     string `json:"branch"`
	Message    string `json:"message"`
}

// handleCreateCommit records a commit against a ticket.
func (s *Server) handleCreateCommit(w http.ResponseWriter, r *http.Request) {
	var req commitRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	c, err := s.Service.RecordCommit(chi.URLParam(r, "id"), req.CommitHash, req.Branch, req.Message)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusCreated, c, "commit recorded")
}

// handleListCommits returns a ticket's commit history. Admin only.
func (s *Server) handleListCommits(w http.ResponseWriter, r *http.Request) {
	commits, err := s.Service.ListCommits(chi.URLParam(r, "id"))
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, commits, "")
}

// batchRequest is a mass status update, e.g. marking a set of tickets shipped
// in a given version.
type batchRequest struct {
	TicketIDs      []string     `json:"ticketIds"`
	Status         model.Status `json:"status"`
	ShippedVersion string       `json:"shippedVersion"`
}

// handleBatchUpdate applies one status to many tickets.
func (s *Server) handleBatchUpdate(w http.ResponseWriter, r *http.Request) {
	var req batchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	result, err := s.Service.BatchUpdateStatus(req.TicketIDs, req.Status, req.ShippedVersion)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, result, "batch update applied")
}

// handleListApps lists every app Flagit has seen, with its settings.
func (s *Server) handleListApps(w http.ResponseWriter, r *http.Request) {
	apps, err := s.Service.ListApps()
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, apps, "")
}

// updateAppRequest patches an app's auto-process setting.
type updateAppRequest struct {
	AutoProcessEnabled *bool `json:"autoProcessEnabled"`
}

// handleUpdateApp changes an app's auto-process setting.
func (s *Server) handleUpdateApp(w http.ResponseWriter, r *http.Request) {
	var req updateAppRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}
	if req.AutoProcessEnabled == nil {
		s.writeError(w, http.StatusBadRequest, "autoProcessEnabled is required")
		return
	}

	app, err := s.Service.UpdateApp(chi.URLParam(r, "name"), *req.AutoProcessEnabled)
	if err != nil {
		if errors.Is(err, db.ErrNotFound) {
			s.writeError(w, http.StatusNotFound, "app not found")
			return
		}
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, app, "app updated")
}

// handleGetSettings returns the global configuration.
func (s *Server) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	settings, err := s.Service.GetSettings()
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, settings, "")
}

// updateSettingsRequest patches the global configuration; omitted fields are
// left unchanged.
type updateSettingsRequest struct {
	GlobalAutoProcess *bool   `json:"globalAutoProcess"`
	HermesWebhookURL  *string `json:"hermesWebhookUrl"`
}

// handleUpdateSettings applies a settings patch.
func (s *Server) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req updateSettingsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		s.writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	settings, err := s.Service.UpdateSettings(req.GlobalAutoProcess, req.HermesWebhookURL)
	if err != nil {
		s.writeServiceError(w, err)
		return
	}
	s.writeJSON(w, http.StatusOK, settings, "settings updated")
}
