package api

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// PublicRouter serves /api/* for apps, plus the embedded web overlay.
func (s *Server) PublicRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.RequestLogger)
	r.Use(CORS)
	s.useJSONFallbacks(r)

	r.Get("/healthz", s.handleHealth)

	r.Route("/api", func(r chi.Router) {
		// Ticket creation is the only endpoint reachable without a credential,
		// so it is the only one that needs a ceiling.
		r.With(s.RateLimit(s.CreateLimiter)).Post("/tickets", s.handleCreateTicket)

		r.Group(func(r chi.Router) {
			r.Use(s.RequireDeviceToken)
			r.Get("/tickets/{id}", s.handleGetTicket)
			r.Get("/tickets/{id}/messages", s.handleListMessages)
			r.Post("/tickets/{id}/messages", s.handlePostMessage)
		})
	})

	if s.Overlay != nil {
		r.Handle("/*", s.overlayOrJSON404())
	}
	return r
}

// useJSONFallbacks makes chi's default 404 and 405 responses match the rest of
// the API. Set before any Route() call so sub-routers inherit them; otherwise a
// miss inside /api would answer in plain text while everything else is JSON.
func (s *Server) useJSONFallbacks(r chi.Router) {
	r.NotFound(func(w http.ResponseWriter, _ *http.Request) {
		s.writeError(w, http.StatusNotFound, "no such endpoint")
	})
	r.MethodNotAllowed(func(w http.ResponseWriter, _ *http.Request) {
		s.writeError(w, http.StatusMethodNotAllowed, "method not allowed for this endpoint")
	})
}

// reservedRoots never belong to the frontend. Without this guard the SPA
// catch-all would answer an unknown or misspelled API path with the overlay's
// HTML, which reads as a success to a browser and is useless to an API client.
var reservedRoots = []string{"/api", "/internal"}

// isReservedPath reports whether path is an API path rather than a frontend
// route. The segment boundary matters: "/api/x" is reserved, "/apiary" is an
// ordinary frontend route that merely starts with the same letters.
func isReservedPath(path string) bool {
	for _, root := range reservedRoots {
		if path == root || strings.HasPrefix(path, root+"/") {
			return true
		}
	}
	return false
}

// overlayOrJSON404 serves the overlay for frontend routes and a JSON 404 for
// anything under an API prefix that no route claimed.
func (s *Server) overlayOrJSON404() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isReservedPath(r.URL.Path) {
			s.writeError(w, http.StatusNotFound, "no such endpoint")
			return
		}
		s.Overlay.ServeHTTP(w, r)
	})
}

// dashboardOrJSON404 is overlayOrJSON404 for the internal listener: the admin
// SPA for frontend routes, and a JSON 404 for an API path no route claimed.
//
// The reserved check is what keeps the two apart. Without it this catch-all
// would answer a misspelled or withdrawn /internal endpoint with a page, and an
// API client — the dashboard itself included — cannot tell a page from an empty
// result. A wrong path must stay wrong out loud.
func (s *Server) dashboardOrJSON404() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if isReservedPath(r.URL.Path) {
			s.writeError(w, http.StatusNotFound, "no such endpoint")
			return
		}
		s.Dashboard.ServeHTTP(w, r)
	})
}

// InternalRouter serves /internal/* for Hermes and the admin dashboard. Every
// route below /internal requires the admin key, unless AdminAuthDisabled says
// the listener is trusted on its own; /healthz and /internal/auth are open, so
// a container healthcheck needs no credentials and the dashboard can find out
// whether to ask for one.
func (s *Server) InternalRouter() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(s.RequestLogger)
	s.useJSONFallbacks(r)

	r.Get("/healthz", s.handleHealth)
	// Registered outside the guarded group on purpose: a client that cannot
	// authenticate is exactly the one that needs this answer.
	r.Get("/internal/auth", s.handleAuthMode)

	r.Route("/internal", func(r chi.Router) {
		r.Use(s.internalAuth)

		r.Get("/poll", s.handlePoll)
		r.Get("/tickets", s.handleListTickets)
		r.Post("/tickets/batch", s.handleBatchUpdate)
		r.Post("/tickets/batch/delete", s.handleDeleteTickets)
		r.Get("/tickets/{id}", s.handleAdminGetTicket)
		r.Patch("/tickets/{id}", s.handleUpdateTicket)
		r.Delete("/tickets/{id}", s.handleDeleteTicket)
		r.Get("/tickets/{id}/messages", s.handleAdminListMessages)
		r.Post("/tickets/{id}/messages", s.handleAgentMessage)
		r.Get("/tickets/{id}/commits", s.handleListCommits)
		r.Post("/tickets/{id}/commits", s.handleCreateCommit)
		r.Get("/apps", s.handleListApps)
		r.Patch("/apps/{name}", s.handleUpdateApp)
		r.Get("/settings", s.handleGetSettings)
		r.Patch("/settings", s.handleUpdateSettings)
		r.Get("/version", s.handleVersion)
	})

	// The dashboard SPA authenticates in the browser with the admin key, so it
	// is served without the middleware that its own API calls go through.
	if s.Dashboard != nil {
		// Static assets referenced by the SPA (JS, CSS) are served from the
		// overlay's embedded dist under /assets/. They must be accessible
		// without the admin key — the browser loads them via <script>/<link>
		// tags that don't carry custom headers.
		r.Handle("/assets/*", s.Overlay)
		r.Handle("/internal/admin", s.Dashboard)
		r.Handle("/internal/admin/*", s.Dashboard)
		// This listener is reached through a proxy that gives it a hostname of
		// its own, so what an operator opens is a root, not /internal/admin.
		// Serving the dashboard there too is what lets that proxy stay a plain
		// pass-through: the alternative — a rule rewriting / to /internal/admin
		// — rewrites the dashboard's own API calls with it, and answering
		// /internal/tickets with the dashboard's HTML makes a full database
		// read as an empty one.
		r.Handle("/*", s.dashboardOrJSON404())
	}
	return r
}

// handleHealth reports liveness. It is unauthenticated on both routers.
func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	s.writeJSON(w, http.StatusOK, map[string]string{"status": "ok"}, "")
}
