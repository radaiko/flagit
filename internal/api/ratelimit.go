package api

import (
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Ticket creation is the one endpoint anyone on the internet can reach without
// a credential, so it is the one that needs a ceiling. These defaults are
// deliberately loose: a person filing several reports in a row is normal, a
// client filing hundreds is not.
const (
	DefaultRateLimit  = 10
	DefaultRateWindow = time.Minute
)

// bucket is one client's allowance within the current window.
type bucket struct {
	tokens   float64
	lastSeen time.Time
}

// RateLimiter is a per-client token bucket.
//
// Tokens refill continuously rather than resetting on a fixed boundary, so a
// client cannot burst twice its allowance by straddling the edge of a window.
// State is in memory: a restart forgives everyone, and a multi-instance deploy
// would limit per instance. Both are acceptable for a self-hosted single
// container, and neither fails open in a way that matters.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket

	// limit is the burst size and the number of requests allowed per window.
	limit float64
	// window is how long a full bucket takes to refill.
	window time.Duration
	// now is the clock, swapped out in tests.
	now func() time.Time

	// lastSweep bounds map growth: entries are dropped once a client has been
	// idle for longer than the window.
	lastSweep time.Time
}

// NewRateLimiter returns a limiter allowing limit requests per window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	if limit <= 0 {
		limit = DefaultRateLimit
	}
	if window <= 0 {
		window = DefaultRateWindow
	}
	return &RateLimiter{
		buckets: map[string]*bucket{},
		limit:   float64(limit),
		window:  window,
		now:     time.Now,
	}
}

// Allow reports whether key may make a request now, consuming one token if so.
func (l *RateLimiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := l.now()
	l.sweep(now)

	b, ok := l.buckets[key]
	if !ok {
		l.buckets[key] = &bucket{tokens: l.limit - 1, lastSeen: now}
		return true
	}

	// Refill for the time elapsed, capped at a full bucket.
	elapsed := now.Sub(b.lastSeen)
	b.tokens = min(l.limit, b.tokens+elapsed.Seconds()*(l.limit/l.window.Seconds()))
	b.lastSeen = now

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// retryAfter reports how long key must wait for its next token.
func (l *RateLimiter) retryAfter(key string) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[key]
	if !ok || b.tokens >= 1 {
		return 0
	}
	needed := 1 - b.tokens
	return time.Duration(needed / (l.limit / l.window.Seconds()) * float64(time.Second))
}

// sweep drops buckets that have been full and untouched for a whole window.
// Called under the lock, at most once per window.
func (l *RateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	l.lastSweep = now
	for key, b := range l.buckets {
		if now.Sub(b.lastSeen) >= l.window {
			delete(l.buckets, key)
		}
	}
}

// RateLimit rejects requests from a client that has exceeded its allowance.
func (s *Server) RateLimit(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			key := clientIP(r)
			if limiter.Allow(key) {
				next.ServeHTTP(w, r)
				return
			}

			wait := limiter.retryAfter(key)
			w.Header().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
			s.logger().Warn("rate limit exceeded", "ip", key, "path", r.URL.Path)
			s.writeError(w, http.StatusTooManyRequests, "too many requests, try again shortly")
		})
	}
}

// clientIP identifies the caller for rate-limiting purposes.
//
// X-Forwarded-For is honoured because Flagit is expected to sit behind a
// reverse proxy or Tailscale Funnel, where every request would otherwise share
// the proxy's address and one client could exhaust everyone's allowance. The
// header is spoofable by a direct caller, which is the accepted trade for a
// V1 limiter: it raises the cost of abuse without being a security boundary.
func clientIP(r *http.Request) string {
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Left-most entry is the original client.
		if first, _, found := strings.Cut(forwarded, ","); found || first != "" {
			if ip := strings.TrimSpace(first); ip != "" {
				return ip
			}
		}
	}
	if real := strings.TrimSpace(r.Header.Get("X-Real-IP")); real != "" {
		return real
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
