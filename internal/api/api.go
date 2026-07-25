// Package api exposes Flagit over HTTP: a public API for apps and an
// internal API for Hermes and the admin dashboard.
package api

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"flagit/internal/service"
)

// maxRequestBytes caps request bodies. Ring-buffer logs are the largest
// legitimate payload, so the limit is generous but finite.
const maxRequestBytes = 4 << 20 // 4 MiB

// Server holds the dependencies every handler needs.
type Server struct {
	Service *service.Service
	// AdminKeyHash is the SHA-256 of the admin key. The key itself is never
	// held in memory or on disk.
	AdminKeyHash string
	Logger       *slog.Logger

	// CreateLimiter caps unauthenticated ticket creation per client. Nil
	// disables limiting, which is only appropriate in tests.
	CreateLimiter *RateLimiter

	// Overlay serves the embedded web overlay; nil disables it.
	Overlay http.Handler
	// Dashboard serves the embedded admin SPA; nil disables it.
	Dashboard http.Handler
}

// NewServer returns a Server with a usable logger. adminKeyHash is the hash
// of the admin key, as produced by db.HashAdminKey.
func NewServer(svc *service.Service, adminKeyHash string, logger *slog.Logger) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	return &Server{
		Service:       svc,
		AdminKeyHash:  adminKeyHash,
		Logger:        logger,
		CreateLimiter: NewRateLimiter(DefaultRateLimit, DefaultRateWindow),
	}
}

// envelope is the success shape for every JSON response.
type envelope struct {
	Data    any    `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

// errorResponse is the failure shape for every JSON response.
type errorResponse struct {
	Error string `json:"error"`
}

func (s *Server) logger() *slog.Logger {
	if s.Logger == nil {
		return slog.Default()
	}
	return s.Logger
}

// writeJSON renders data under the standard success envelope.
func (s *Server) writeJSON(w http.ResponseWriter, status int, data any, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(envelope{Data: data, Message: message}); err != nil {
		// The status line is already out; all that is left is a record of it.
		s.logger().Error("write response", "error", err)
	}
}

// writeError renders an error message with the given status.
func (s *Server) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(errorResponse{Error: message}); err != nil {
		s.logger().Error("write error response", "error", err)
	}
}

// writeServiceError maps a service-layer error onto an HTTP status. Unknown
// errors become a 500 with a generic message so internals never leak.
func (s *Server) writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrNotFound):
		s.writeError(w, http.StatusNotFound, "ticket not found")
	case errors.Is(err, service.ErrForbidden):
		s.writeError(w, http.StatusForbidden, "invalid device token for this ticket")
	case errors.Is(err, service.ErrInvalid):
		s.writeError(w, http.StatusBadRequest, err.Error())
	default:
		s.logger().Error("unhandled service error", "error", err)
		s.writeError(w, http.StatusInternalServerError, "internal server error")
	}
}

// decodeJSON reads a size-limited JSON body into dst, rejecting unknown fields
// so a typo'd key is reported rather than silently ignored.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	// A second value in the stream means the caller sent something we would
	// have silently dropped.
	if err := dec.Decode(&struct{}{}); err != io.EOF {
		return errors.New("body must contain a single JSON object")
	}
	return nil
}
