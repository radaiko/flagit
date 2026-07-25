package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"flagit/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testTicket() *model.Ticket {
	return &model.Ticket{
		ID:              "FLG-7X3K9Q",
		Type:            model.TypeBug,
		Title:           "Crash on save",
		Body:            "Tapping save closes the app",
		Status:          model.StatusOpen,
		AppName:         "notes",
		AppVersion:      "1.4.2",
		OS:              "iOS 18.2",
		Platform:        "ios",
		DeviceModel:     "iPhone 15",
		DeviceTokenHash: "secret-hash",
		LogRingBuffer:   "panic: nil map",
		LogsDurationMin: 5,
		CreatedAt:       time.Date(2026, 7, 25, 9, 0, 0, 0, time.UTC),
	}
}

// newTestSender returns a Sender that records its backoff delays instead of
// sleeping, so retry tests run instantly.
func newTestSender(t *testing.T) (*Sender, *[]time.Duration) {
	t.Helper()
	var delays []time.Duration
	s := NewSender(slog.New(slog.DiscardHandler))
	s.BaseDelay = time.Second
	s.Sleep = func(d time.Duration) { delays = append(delays, d) }
	return s, &delays
}

func TestPayloadFor(t *testing.T) {
	p := PayloadFor(testTicket(), "https://flagit.example")

	assert.Equal(t, "ticket.created", p.Event)
	assert.Equal(t, "FLG-7X3K9Q", p.TicketID)
	assert.Equal(t, model.TypeBug, p.Type)
	assert.Equal(t, "notes", p.AppName)
	assert.Equal(t, "panic: nil map", p.Logs)
	assert.Equal(t, 5, p.LogsDurationMin)
	assert.Equal(t, "https://flagit.example/api/tickets/FLG-7X3K9Q", p.TicketURL)
}

func TestPayloadForWithoutPublicURL(t *testing.T) {
	p := PayloadFor(testTicket(), "")

	assert.Empty(t, p.TicketURL)
}

func TestPayloadNeverLeaksDeviceTokenHash(t *testing.T) {
	raw, err := json.Marshal(PayloadFor(testTicket(), ""))
	require.NoError(t, err)

	assert.NotContains(t, string(raw), "secret-hash")
}

func TestSendSuccess(t *testing.T) {
	var got Payload
	var contentType, userAgent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		userAgent = r.Header.Get("User-Agent")
		body, _ := io.ReadAll(r.Body)
		require.NoError(t, json.Unmarshal(body, &got))
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	s, delays := newTestSender(t)
	require.NoError(t, s.Send(context.Background(), srv.URL, PayloadFor(testTicket(), "")))

	assert.Equal(t, "application/json", contentType)
	assert.Equal(t, "flagit-webhook/1", userAgent)
	assert.Equal(t, "FLG-7X3K9Q", got.TicketID)
	assert.Empty(t, *delays, "a first-try success must not back off")
}

func TestSendEmptyURLIsANoOp(t *testing.T) {
	s, _ := newTestSender(t)

	assert.NoError(t, s.Send(context.Background(), "", PayloadFor(testTicket(), "")))
}

func TestSendRetriesThenSucceeds(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s, delays := newTestSender(t)
	require.NoError(t, s.Send(context.Background(), srv.URL, PayloadFor(testTicket(), "")))

	assert.Equal(t, int32(3), calls.Load())
	assert.Equal(t, []time.Duration{time.Second, 2 * time.Second}, *delays, "exponential backoff")
}

func TestSendGivesUpAfterMaxAttempts(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	s, delays := newTestSender(t)
	err := s.Send(context.Background(), srv.URL, PayloadFor(testTicket(), ""))

	require.Error(t, err)
	assert.ErrorContains(t, err, "after 3 attempts")
	assert.ErrorContains(t, err, "HTTP 502")
	assert.Equal(t, int32(DefaultMaxAttempts), calls.Load())
	assert.Len(t, *delays, 2, "no backoff after the final attempt")
}

func TestSend4xxIsNotRetried(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer srv.Close()

	s, delays := newTestSender(t)
	err := s.Send(context.Background(), srv.URL, PayloadFor(testTicket(), ""))

	// Hermes understood the request and rejected it. Resending the identical
	// body cannot change that answer, so retrying only delays the error.
	require.Error(t, err)
	assert.ErrorContains(t, err, "webhook rejected")
	assert.ErrorContains(t, err, "HTTP 404")
	assert.Equal(t, int32(1), calls.Load(), "one attempt, no retries")
	assert.Empty(t, *delays, "no backoff for a permanent failure")
}

func TestSendRetriesTransientClientErrors(t *testing.T) {
	// 408 and 429 are 4xx, but both explicitly invite the caller back.
	for _, status := range []int{http.StatusRequestTimeout, http.StatusTooManyRequests} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			var calls atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(status)
			}))
			defer srv.Close()

			s, _ := newTestSender(t)
			assert.Error(t, s.Send(context.Background(), srv.URL, PayloadFor(testTicket(), "")))
			assert.Equal(t, int32(DefaultMaxAttempts), calls.Load())
		})
	}
}

func TestStatusErrorPermanent(t *testing.T) {
	tests := []struct {
		status    int
		permanent bool
	}{
		{http.StatusBadRequest, true},
		{http.StatusUnauthorized, true},
		{http.StatusNotFound, true},
		{http.StatusUnprocessableEntity, true},
		{http.StatusRequestTimeout, false},
		{http.StatusTooManyRequests, false},
		{http.StatusInternalServerError, false},
		{http.StatusBadGateway, false},
		{http.StatusServiceUnavailable, false},
	}
	for _, tt := range tests {
		t.Run(http.StatusText(tt.status), func(t *testing.T) {
			err := &StatusError{StatusCode: tt.status}

			assert.Equal(t, tt.permanent, err.Permanent())
			assert.Equal(t, !tt.permanent, isRetryable(err))
			assert.Contains(t, err.Error(), fmt.Sprint(tt.status))
		})
	}
}

func TestIsRetryableTreatsTransportErrorsAsTransient(t *testing.T) {
	// DNS, TLS and connection resets are worth another attempt.
	assert.True(t, isRetryable(errors.New("connection reset by peer")))
	assert.True(t, isRetryable(fmt.Errorf("wrapped: %w", errors.New("dial tcp: timeout"))))

	// A wrapped StatusError is still recognised through the chain.
	assert.False(t, isRetryable(fmt.Errorf("wrapped: %w", &StatusError{StatusCode: 400})))
}

func TestSendUnreachableHost(t *testing.T) {
	s, _ := newTestSender(t)
	s.MaxAttempts = 2

	err := s.Send(context.Background(), "http://127.0.0.1:1/hook", PayloadFor(testTicket(), ""))

	assert.ErrorContains(t, err, "after 2 attempts")
}

func TestSendInvalidURL(t *testing.T) {
	s, _ := newTestSender(t)
	s.MaxAttempts = 1

	err := s.Send(context.Background(), "://not a url", PayloadFor(testTicket(), ""))

	assert.ErrorContains(t, err, "build webhook request")
}

func TestSendStopsWhenContextCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	s, delays := newTestSender(t)
	// Cancel while backing off: the retry loop must abandon delivery.
	s.Sleep = func(d time.Duration) {
		*delays = append(*delays, d)
		cancel()
	}

	err := s.Send(ctx, srv.URL, PayloadFor(testTicket(), ""))

	assert.ErrorIs(t, err, context.Canceled)
	assert.Len(t, *delays, 1)
}

func TestSendWithAlreadyCancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s, _ := newTestSender(t)
	assert.Error(t, s.Send(ctx, srv.URL, PayloadFor(testTicket(), "")))
}

func TestSendZeroValueSenderUsesDefaults(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	// No Client, Logger, Sleep or MaxAttempts set: every field must fall back.
	s := &Sender{}
	require.NoError(t, s.Send(context.Background(), srv.URL, PayloadFor(testTicket(), "")))
	assert.Equal(t, int32(1), calls.Load())
}

func TestZeroValueSenderBackoffDefaults(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	var slept time.Duration
	s := &Sender{MaxAttempts: 2, Sleep: func(d time.Duration) { slept = d }}
	require.NoError(t, s.Send(context.Background(), srv.URL, PayloadFor(testTicket(), "")))

	assert.Equal(t, DefaultBaseDelay, slept, "a zero BaseDelay falls back to the default")
}

func TestNewSenderDefaults(t *testing.T) {
	s := NewSender(nil)

	assert.NotNil(t, s.Logger)
	assert.Equal(t, DefaultMaxAttempts, s.MaxAttempts)
	assert.Equal(t, DefaultBaseDelay, s.BaseDelay)
	assert.Equal(t, 10*time.Second, s.Client.Timeout)
}

func TestGoTracksDeliveriesForShutdown(t *testing.T) {
	release := make(chan struct{})
	started := make(chan struct{})
	s := NewSender(slog.New(slog.DiscardHandler))

	s.Go(func() {
		close(started)
		<-release
	})
	<-started

	// A delivery is still running, so Wait must not return.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.False(t, s.Wait(ctx), "Wait returned while a delivery was in flight")

	close(release)

	drained, cancelDrain := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDrain()
	assert.True(t, s.Wait(drained), "Wait did not return after the delivery finished")
}

func TestWaitReturnsImmediatelyWhenIdle(t *testing.T) {
	s := NewSender(slog.New(slog.DiscardHandler))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	assert.True(t, s.Wait(ctx))
}

func TestGoRunsTheDelivery(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	s := NewSender(slog.New(slog.DiscardHandler))
	s.Go(func() {
		_ = s.Send(context.Background(), srv.URL, PayloadFor(testTicket(), ""))
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.True(t, s.Wait(ctx))
	assert.Equal(t, int32(1), calls.Load())
}
