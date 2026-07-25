package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// PublicRouter serves /api/* for apps, plus the embedded web overlay.
func (s *Server) PublicRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.RequestLogger)
	r.Use(CORS)

	r.Get("/healthz", s.handleHealth)

	r.Route("/api", func(r chi.Router) {
		r.Post("/tickets", s.handleCreateTicket)

		r.Group(func(r chi.Router) {
			r.Use(s.RequireDeviceToken)
			r.Get("/tickets/{id}", s.handleGetTicket)
			r.Get("/tickets/{id}/messages", s.handleListMessages)
			r.Post("/tickets/{id}/messages", s.handlePostMessage)
		})
	})

	if s.Overlay != nil {
		r.Handle("/*", s.Overlay)
	}
	return r
}

// InternalRouter serves /internal/* for Hermes and the admin dashboard. Every
// route below /internal requires the admin key; only /healthz is open, so a
// container healthcheck does not need credentials.
func (s *Server) InternalRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.RequestLogger)

	r.Get("/healthz", s.handleHealth)

	r.Route("/internal", func(r chi.Router) {
		r.Use(s.AdminKeyAuth)

		r.Get("/poll", s.handlePoll)
		r.Get("/tickets", s.handleListTickets)
		r.Post("/tickets/batch", s.handleBatchUpdate)
		r.Get("/tickets/{id}", s.handleAdminGetTicket)
		r.Patch("/tickets/{id}", s.handleUpdateTicket)
		r.Get("/tickets/{id}/messages", s.handleAdminListMessages)
		r.Post("/tickets/{id}/messages", s.handleAgentMessage)
		r.Get("/tickets/{id}/commits", s.handleListCommits)
		r.Post("/tickets/{id}/commits", s.handleCreateCommit)
		r.Get("/apps", s.handleListApps)
		r.Patch("/apps/{name}", s.handleUpdateApp)
		r.Get("/settings", s.handleGetSettings)
		r.Patch("/settings", s.handleUpdateSettings)
	})

	// The dashboard SPA authenticates in the browser with the admin key, so it
	// is served without the middleware that its own API calls go through.
	if s.Dashboard != nil {
		r.Handle("/internal/admin", s.Dashboard)
		r.Handle("/internal/admin/*", s.Dashboard)
	}
	return r
}

// handleHealth reports liveness. It is unauthenticated on both routers.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, "")
}
