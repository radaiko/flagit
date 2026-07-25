// Package webhook delivers new-ticket notifications to Hermes.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"flagit/internal/model"
)

// DefaultMaxAttempts is the total number of delivery attempts, including the
// first: one try plus two retries.
const DefaultMaxAttempts = 3

// DefaultBaseDelay is the first backoff interval; it doubles per retry.
const DefaultBaseDelay = time.Second

// Payload is the JSON body POSTed to Hermes for a new ticket.
type Payload struct {
	Event           string           `json:"event"`
	TicketID        string           `json:"ticketId"`
	Type            model.TicketType `json:"type"`
	Title           string           `json:"title"`
	Body            string           `json:"body"`
	Status          model.Status     `json:"status"`
	AppName         string           `json:"appName"`
	AppVersion      string           `json:"appVersion"`
	OS              string           `json:"os"`
	Platform        string           `json:"platform"`
	DeviceModel     string           `json:"deviceModel"`
	Logs            string           `json:"logs,omitempty"`
	LogsDurationMin int              `json:"logsDurationMin,omitempty"`
	CreatedAt       time.Time        `json:"createdAt"`
	TicketURL       string           `json:"ticketUrl,omitempty"`
}

// PayloadFor builds the Hermes payload for a ticket. publicURL may be empty.
func PayloadFor(t *model.Ticket, publicURL string) Payload {
	p := Payload{
		Event:           "ticket.created",
		TicketID:        t.ID,
		Type:            t.Type,
		Title:           t.Title,
		Body:            t.Body,
		Status:          t.Status,
		AppName:         t.AppName,
		AppVersion:      t.AppVersion,
		OS:              t.OS,
		Platform:        t.Platform,
		DeviceModel:     t.DeviceModel,
		Logs:            t.LogRingBuffer,
		LogsDurationMin: t.LogsDurationMin,
		CreatedAt:       t.CreatedAt,
	}
	if publicURL != "" {
		p.TicketURL = fmt.Sprintf("%s/api/tickets/%s", publicURL, t.ID)
	}
	return p
}

// Sender POSTs payloads to a Hermes webhook URL, retrying transient failures
// with exponential backoff.
type Sender struct {
	Client      *http.Client
	MaxAttempts int
	BaseDelay   time.Duration
	Logger      *slog.Logger

	// Sleep is the backoff hook; tests replace it to avoid real waiting.
	Sleep func(time.Duration)
}

// NewSender returns a Sender with production defaults.
func NewSender(logger *slog.Logger) *Sender {
	if logger == nil {
		logger = slog.Default()
	}
	return &Sender{
		Client:      &http.Client{Timeout: 10 * time.Second},
		MaxAttempts: DefaultMaxAttempts,
		BaseDelay:   DefaultBaseDelay,
		Logger:      logger,
		Sleep:       time.Sleep,
	}
}

// Send POSTs payload to url, retrying up to MaxAttempts times with exponential
// backoff. It returns the last error if every attempt fails. An empty url is
// not an error: it means Hermes has not been configured yet.
func (s *Sender) Send(ctx context.Context, url string, payload Payload) error {
	if url == "" {
		return nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal webhook payload: %w", err)
	}

	attempts := s.MaxAttempts
	if attempts < 1 {
		attempts = DefaultMaxAttempts
	}

	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		lastErr = s.post(ctx, url, body)
		if lastErr == nil {
			s.logger().Info("webhook delivered", "ticket", payload.TicketID, "attempt", attempt)
			return nil
		}
		s.logger().Warn("webhook attempt failed",
			"ticket", payload.TicketID, "attempt", attempt, "of", attempts, "error", lastErr)

		// A 4xx means Hermes understood the request and rejected it. Sending
		// the identical body again cannot change that answer, so retrying only
		// delays the error and adds load.
		if !isRetryable(lastErr) {
			s.logger().Error("webhook rejected, not retrying",
				"ticket", payload.TicketID, "error", lastErr)
			return fmt.Errorf("webhook rejected: %w", lastErr)
		}

		if attempt == attempts {
			break
		}
		if err := s.wait(ctx, attempt); err != nil {
			return err
		}
	}
	return fmt.Errorf("webhook delivery failed after %d attempts: %w", attempts, lastErr)
}

// StatusError is a non-2xx response from the webhook endpoint.
type StatusError struct {
	StatusCode int
}

func (e *StatusError) Error() string {
	return fmt.Sprintf("webhook returned HTTP %d", e.StatusCode)
}

// Permanent reports whether the status is a client error, which will not
// resolve itself on a retry. 408 (Request Timeout) and 429 (Too Many Requests)
// are the exceptions: both explicitly invite the caller to come back.
func (e *StatusError) Permanent() bool {
	if e.StatusCode == http.StatusRequestTimeout || e.StatusCode == http.StatusTooManyRequests {
		return false
	}
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// isRetryable reports whether another attempt could plausibly succeed. Network
// and 5xx failures are transient; a rejected request is not.
func isRetryable(err error) bool {
	var status *StatusError
	if errors.As(err, &status) {
		return !status.Permanent()
	}
	// Anything else is a transport-level failure: DNS, TLS, connection reset.
	return true
}

// wait backs off before the next attempt, honouring context cancellation.
func (s *Sender) wait(ctx context.Context, attempt int) error {
	base := s.BaseDelay
	if base <= 0 {
		base = DefaultBaseDelay
	}
	delay := base << (attempt - 1) // 1×, 2×, 4× …

	if err := ctx.Err(); err != nil {
		return err
	}
	sleep := s.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	sleep(delay)
	return ctx.Err()
}

func (s *Sender) post(ctx context.Context, url string, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build webhook request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "flagit-webhook/1")

	client := s.Client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Drain so the connection can be reused.
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return &StatusError{StatusCode: resp.StatusCode}
	}
	return nil
}

func (s *Sender) logger() *slog.Logger {
	if s.Logger == nil {
		return slog.Default()
	}
	return s.Logger
}
