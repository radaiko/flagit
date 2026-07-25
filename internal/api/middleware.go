package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
	"time"
)

// HeaderAdminKey authenticates internal API callers.
const HeaderAdminKey = "X-Admin-Key"

// HeaderDeviceToken carries the reporter's ownership token on public calls.
const HeaderDeviceToken = "X-Device-Token"

// statusRecorder captures the status code for the logging middleware.
type statusRecorder struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (r *statusRecorder) WriteHeader(status int) {
	if !r.wrote {
		r.status = status
		r.wrote = true
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if !r.wrote {
		// An implicit 200 from a handler that never called WriteHeader.
		r.status = http.StatusOK
		r.wrote = true
	}
	return r.ResponseWriter.Write(b)
}

// RequestLogger logs method, path, status and duration for every request.
func (s *Server) RequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		next.ServeHTTP(rec, r)

		s.logger().Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).String(),
		)
	})
}

// AdminKeyAuth rejects requests without a matching admin key. When no key is
// configured the middleware fails closed: an unset key must never mean "open".
func (s *Server) AdminKeyAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.AdminKey == "" {
			s.writeError(w, http.StatusServiceUnavailable, "admin key is not configured")
			return
		}
		if !s.validAdminKey(adminKeyFrom(r)) {
			s.writeError(w, http.StatusUnauthorized, "invalid or missing admin key")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validAdminKey compares in constant time so the key cannot be recovered by
// timing repeated guesses.
func (s *Server) validAdminKey(provided string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), []byte(s.AdminKey)) == 1
}

// adminKeyFrom reads the admin key from X-Admin-Key, falling back to a bearer
// token so standard HTTP clients work unchanged.
func adminKeyFrom(r *http.Request) string {
	if key := strings.TrimSpace(r.Header.Get(HeaderAdminKey)); key != "" {
		return key
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if after, ok := strings.CutPrefix(auth, "Bearer "); ok {
		return strings.TrimSpace(after)
	}
	return ""
}

// deviceTokenFrom reads the ownership token from the header, falling back to a
// query parameter for overlay links opened directly in a browser.
func deviceTokenFrom(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get(HeaderDeviceToken)); token != "" {
		return token
	}
	return strings.TrimSpace(r.URL.Query().Get("token"))
}

// RequireDeviceToken rejects public requests that carry no ownership token.
func (s *Server) RequireDeviceToken(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if deviceTokenFrom(r) == "" {
			s.writeError(w, http.StatusUnauthorized, "missing device token")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// CORS allows browser-based overlays hosted on an app's own origin to call the
// public API. Safe to be permissive here: every endpoint is authorised by an
// unguessable device token, and no cookies or credentials are involved.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+HeaderDeviceToken)
		w.Header().Set("Access-Control-Max-Age", "86400")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
