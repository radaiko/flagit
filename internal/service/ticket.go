// Package service holds Flagit's business rules: validation, ownership
// checks, and deciding when Hermes gets told about a ticket.
package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/url"
	"strings"
	"time"

	"flagit/internal/db"
	"flagit/internal/model"
	"flagit/internal/webhook"
)

// Errors returned by the service. The API layer maps these onto status codes.
var (
	// ErrNotFound means no such ticket/app/setting.
	ErrNotFound = db.ErrNotFound
	// ErrForbidden means the caller's device token does not own the ticket.
	ErrForbidden = errors.New("forbidden")
	// ErrInvalid means the request failed validation; the message is safe to
	// show to the caller.
	ErrInvalid = errors.New("invalid request")
)

// Field limits. Long-form fields are truncated rather than rejected: a user
// who pasted a huge stack trace should still get a ticket.
const (
	MaxTitleLen   = 200
	MaxBodyLen    = 20000
	MaxLogsLen    = 200000
	MaxMessageLen = 20000
	MaxAppNameLen = 100
	MaxShortLen   = 100
)

// UnknownAppName is used when a client creates a ticket without naming its
// app, so those tickets still group somewhere in the dashboard.
const UnknownAppName = "unknown"

// Sender delivers webhook payloads. *webhook.Sender satisfies it; tests
// substitute a recorder.
type Sender interface {
	Send(ctx context.Context, url string, payload webhook.Payload) error
}

// Service is the business layer over the database.
type Service struct {
	DB        *db.DB
	Webhook   Sender
	PublicURL string
	Logger    *slog.Logger

	// Dispatch runs webhook delivery. It defaults to a goroutine so a slow or
	// unreachable Hermes never blocks the user's ticket submission; tests
	// replace it with a synchronous call.
	Dispatch func(func())
}

// New returns a Service with production defaults.
func New(database *db.DB, sender Sender, publicURL string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		DB:        database,
		Webhook:   sender,
		PublicURL: publicURL,
		Logger:    logger,
		Dispatch:  func(fn func()) { go fn() },
	}
}

// NewTicketInput is the validated shape of a ticket-creation request.
type NewTicketInput struct {
	Type            model.TicketType
	Title           string
	Body            string
	DeviceToken     string
	AppName         string
	AppVersion      string
	OS              string
	Platform        string
	DeviceModel     string
	Logs            string
	LogsDurationMin int
}

// CreateTicket validates the input, stores the ticket, registers the app if it
// is new, and notifies Hermes when auto-processing applies.
func (s *Service) CreateTicket(ctx context.Context, in NewTicketInput) (*model.Ticket, error) {
	if !in.Type.Valid() {
		return nil, fmt.Errorf("%w: type must be %q or %q", ErrInvalid, model.TypeBug, model.TypeFeature)
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, fmt.Errorf("%w: title is required", ErrInvalid)
	}
	token := strings.TrimSpace(in.DeviceToken)
	if token == "" {
		return nil, fmt.Errorf("%w: deviceToken is required", ErrInvalid)
	}

	appName := strings.TrimSpace(in.AppName)
	if appName == "" {
		appName = UnknownAppName
	}

	ticket := &model.Ticket{
		Type:            in.Type,
		Title:           truncate(title, MaxTitleLen),
		Body:            truncate(strings.TrimSpace(in.Body), MaxBodyLen),
		Status:          model.StatusOpen,
		AppName:         truncate(appName, MaxAppNameLen),
		AppVersion:      truncate(strings.TrimSpace(in.AppVersion), MaxShortLen),
		OS:              truncate(strings.TrimSpace(in.OS), MaxShortLen),
		Platform:        truncate(strings.TrimSpace(in.Platform), MaxShortLen),
		DeviceModel:     truncate(strings.TrimSpace(in.DeviceModel), MaxShortLen),
		DeviceTokenHash: db.HashDeviceToken(token),
		LogRingBuffer:   truncate(in.Logs, MaxLogsLen),
		LogsDurationMin: max(in.LogsDurationMin, 0),
	}

	if err := s.DB.CreateTicket(ticket); err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}

	autoProcess, err := s.registerApp(ticket.AppName)
	if err != nil {
		// The ticket exists and the user should not see a failure; the only
		// casualty is auto-processing, which an admin can trigger manually.
		s.logger().Error("could not register app for ticket", "ticket", ticket.ID, "app", ticket.AppName, "error", err)
	} else if autoProcess {
		s.notifyHermes(ticket)
	} else {
		s.logger().Info("auto-processing disabled, skipping webhook", "ticket", ticket.ID, "app", ticket.AppName)
	}

	return ticket, nil
}

// registerApp ensures the ticket's app exists and reports whether
// auto-processing is on for it. An app that Flagit has never seen inherits the
// global "new unknown app" toggle, once, at registration time.
func (s *Service) registerApp(appName string) (bool, error) {
	globalDefault, err := s.DB.GetBoolSetting(model.SettingGlobalAutoProcess, false)
	if err != nil {
		return false, err
	}
	app, created, err := s.DB.EnsureApp(appName, globalDefault)
	if err != nil {
		return false, err
	}
	if created {
		s.logger().Info("registered new app", "app", appName, "autoProcess", app.AutoProcessEnabled)
	}
	return app.AutoProcessEnabled, nil
}

// notifyHermes delivers the new-ticket webhook out of band.
func (s *Service) notifyHermes(t *model.Ticket) {
	if s.Webhook == nil {
		return
	}
	url, err := s.DB.GetSetting(model.SettingHermesWebhookURL, "")
	if err != nil {
		s.logger().Error("could not read webhook url", "ticket", t.ID, "error", err)
		return
	}
	if url == "" {
		s.logger().Info("no Hermes webhook configured, skipping", "ticket", t.ID)
		return
	}

	payload := webhook.PayloadFor(t, s.PublicURL)
	dispatch := s.Dispatch
	if dispatch == nil {
		dispatch = func(fn func()) { fn() }
	}
	dispatch(func() {
		// Detached from the request: delivery outlives the HTTP handler.
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		if err := s.Webhook.Send(ctx, url, payload); err != nil {
			s.logger().Error("webhook delivery failed", "ticket", t.ID, "error", err)
		}
	})
}

// GetTicket loads a ticket on behalf of its reporter, verifying the device
// token owns it.
func (s *Service) GetTicket(id, deviceToken string) (*model.Ticket, error) {
	ticket, err := s.GetTicketByID(id)
	if err != nil {
		return nil, err
	}
	if err := s.assertOwner(ticket, deviceToken); err != nil {
		return nil, err
	}
	return ticket, nil
}

// GetTicketByID loads a ticket without an ownership check. Admin/Hermes only.
func (s *Service) GetTicketByID(id string) (*model.Ticket, error) {
	id = strings.TrimSpace(id)
	if !db.ValidTicketID(id) {
		// Shape errors and misses look the same to a caller, so a wrong ID
		// cannot be distinguished from a valid ID for someone else's ticket.
		return nil, ErrNotFound
	}
	return s.DB.GetTicket(id)
}

// assertOwner reports whether deviceToken hashes to the ticket's stored hash.
// The comparison is constant-time so the hash cannot be reconstructed byte by
// byte from how long a rejection takes.
func (s *Service) assertOwner(t *model.Ticket, deviceToken string) error {
	hash := db.HashDeviceToken(deviceToken)
	if hash == "" {
		return ErrForbidden
	}
	if subtle.ConstantTimeCompare([]byte(hash), []byte(t.DeviceTokenHash)) != 1 {
		return ErrForbidden
	}
	return nil
}

// ListMessages returns a ticket's conversation for its owner.
func (s *Service) ListMessages(ticketID, deviceToken string) ([]*model.Message, error) {
	if _, err := s.GetTicket(ticketID, deviceToken); err != nil {
		return nil, err
	}
	return s.DB.ListMessagesByTicket(strings.TrimSpace(ticketID))
}

// ListMessagesByID returns a ticket's conversation without an ownership
// check. Admin/Hermes only.
func (s *Service) ListMessagesByID(ticketID string) ([]*model.Message, error) {
	if _, err := s.GetTicketByID(ticketID); err != nil {
		return nil, err
	}
	return s.DB.ListMessagesByTicket(strings.TrimSpace(ticketID))
}

// PostUserMessage appends a message from the reporter, verifying ownership.
func (s *Service) PostUserMessage(ticketID, deviceToken, body string) (*model.Message, error) {
	ticket, err := s.GetTicket(ticketID, deviceToken)
	if err != nil {
		return nil, err
	}
	return s.postMessage(ticket.ID, model.RoleUser, body)
}

// PostAgentMessage appends a message from Hermes. Callers are already
// authenticated by the admin key.
func (s *Service) PostAgentMessage(ticketID, body string) (*model.Message, error) {
	ticket, err := s.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}
	return s.postMessage(ticket.ID, model.RoleAgent, body)
}

func (s *Service) postMessage(ticketID string, role model.Role, body string) (*model.Message, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, fmt.Errorf("%w: message body is required", ErrInvalid)
	}
	m := &model.Message{
		TicketID: ticketID,
		Body:     truncate(body, MaxMessageLen),
		Role:     role,
	}
	if err := s.DB.CreateMessage(m); err != nil {
		return nil, fmt.Errorf("create message: %w", err)
	}
	return m, nil
}

// StatusUpdate is a change to apply to a ticket. Every field is optional, so
// the caller can move a ticket, add a comment, or do both.
type StatusUpdate struct {
	// Status is the status to move to. Nil keeps the current one, which is how
	// a comment is added without touching the workflow.
	Status *model.Status
	// ShippedVersion is the release the fix went out in. Nil leaves the
	// recorded value alone; a pointer to "" clears it.
	ShippedVersion *string
	// Comment is an agent message written in the same transaction as the move.
	Comment string
	// Force skips workflow validation. Admins correcting a ticket's state need
	// this; Hermes walking the normal flow does not.
	Force bool
}

// UpdateStatus moves a ticket to a new status along the documented workflow.
//
// A nil shippedVersion leaves the recorded release alone; a non-nil one is
// written as given, so passing a pointer to "" is how a ticket that is no
// longer shipped stops claiming a version.
func (s *Service) UpdateStatus(
	ticketID string, status model.Status, shippedVersion *string,
) (*model.Ticket, error) {
	return s.ApplyUpdate(ticketID, StatusUpdate{
		Status:         &status,
		ShippedVersion: shippedVersion,
	})
}

// UpdateStatusWithComment moves a ticket and records an agent message in the
// same transaction, so the ticket cannot end up moved without the explanation
// that was meant to go with it.
func (s *Service) UpdateStatusWithComment(
	ticketID string, status model.Status, shippedVersion *string, comment string,
) (*model.Ticket, error) {
	return s.ApplyUpdate(ticketID, StatusUpdate{
		Status:         &status,
		ShippedVersion: shippedVersion,
		Comment:        comment,
	})
}

// ApplyUpdate applies a StatusUpdate. The status change and the comment are
// written in one transaction, so a ticket cannot end up moved without the
// explanation that was meant to accompany it.
func (s *Service) ApplyUpdate(ticketID string, update StatusUpdate) (*model.Ticket, error) {
	ticket, err := s.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}

	comment := truncate(strings.TrimSpace(update.Comment), MaxMessageLen)

	// A no-op is a mistake worth reporting: silently accepting it would let a
	// client believe it had done something.
	status := ticket.Status
	if update.Status != nil {
		status = *update.Status
		if !status.Valid() {
			return nil, fmt.Errorf("%w: unknown status %q", ErrInvalid, status)
		}
		if !update.Force && !ticket.Status.CanTransitionTo(status) {
			return nil, fmt.Errorf(
				"%w: cannot move a ticket from %q to %q; the workflow is %s. "+
					"Pass force to override",
				ErrInvalid, ticket.Status, status, workflowDescription)
		}
	} else if comment == "" {
		return nil, fmt.Errorf("%w: provide a status, a comment, or both", ErrInvalid)
	}

	version := ticket.ShippedVersion
	if update.ShippedVersion != nil {
		version = truncate(strings.TrimSpace(*update.ShippedVersion), MaxShortLen)
	}

	if err := s.DB.UpdateTicketStatusWithMessage(ticket.ID, status, version, comment); err != nil {
		return nil, err
	}
	return s.DB.GetTicket(ticket.ID)
}

// workflowDescription is the documented order, quoted back in errors so a
// caller does not have to go looking for it.
const workflowDescription = "open → in-progress → resolved → shipped → closed"

// BatchResult reports the outcome of a bulk status update. Failed maps a
// ticket ID to a stable reason code, not to a raw error: these values are
// rendered in the dashboard and would otherwise leak SQL text and schema
// details to whoever is looking at it.
type BatchResult struct {
	Updated []string          `json:"updated"`
	Failed  map[string]string `json:"failed"`
}

// Reasons a ticket can be skipped by a batch update.
const (
	ReasonNotFound          = "not_found"
	ReasonInvalidTransition = "invalid_transition"
	ReasonInternalError     = "internal_error"
)

// batchFailureReason classifies an error into one of the stable reason codes.
func batchFailureReason(err error) string {
	switch {
	case errors.Is(err, ErrNotFound):
		return ReasonNotFound
	case errors.Is(err, ErrInvalid):
		return ReasonInvalidTransition
	default:
		return ReasonInternalError
	}
}

// BatchUpdateStatus applies one status (and optional shipped version) to many
// tickets. Unknown IDs are reported per-ticket rather than failing the batch,
// so one stale ID cannot block a release sweep.
func (s *Service) BatchUpdateStatus(
	ids []string, status model.Status, shippedVersion string, force bool,
) (*BatchResult, error) {
	if !status.Valid() {
		return nil, fmt.Errorf("%w: unknown status %q", ErrInvalid, status)
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("%w: at least one ticket id is required", ErrInvalid)
	}

	result := &BatchResult{Updated: []string{}, Failed: map[string]string{}}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		update := StatusUpdate{Status: &status, ShippedVersion: &shippedVersion, Force: force}
		if _, err := s.ApplyUpdate(id, update); err != nil {
			reason := batchFailureReason(err)
			if reason == ReasonInternalError {
				// The reason the caller sees is deliberately vague, so the
				// detail has to survive somewhere.
				s.logger().Error("batch update failed", "ticket", id, "error", err)
			}
			result.Failed[id] = reason
			continue
		}
		result.Updated = append(result.Updated, id)
	}
	return result, nil
}

// RecordCommit stores a commit an agent produced for a ticket.
func (s *Service) RecordCommit(ticketID, hash, branch, message string) (*model.CommitInfo, error) {
	hash = strings.TrimSpace(hash)
	if hash == "" {
		return nil, fmt.Errorf("%w: commitHash is required", ErrInvalid)
	}
	ticket, err := s.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}
	c := &model.CommitInfo{
		TicketID:   ticket.ID,
		CommitHash: truncate(hash, MaxShortLen),
		Branch:     truncate(strings.TrimSpace(branch), MaxShortLen),
		Message:    truncate(strings.TrimSpace(message), MaxTitleLen),
	}
	if err := s.DB.CreateCommit(c); err != nil {
		return nil, fmt.Errorf("record commit: %w", err)
	}
	return c, nil
}

// ListCommits returns the commits recorded against a ticket. Admin only.
func (s *Service) ListCommits(ticketID string) ([]*model.CommitInfo, error) {
	ticket, err := s.GetTicketByID(ticketID)
	if err != nil {
		return nil, err
	}
	return s.DB.ListCommitsByTicket(ticket.ID)
}

// ListTickets returns one page of tickets matching filter. Admin only.
func (s *Service) ListTickets(f db.TicketFilter) ([]*model.Ticket, error) {
	return s.DB.ListTickets(f)
}

// CountTickets reports how many tickets match filter in total, ignoring its
// paging fields.
func (s *Service) CountTickets(f db.TicketFilter) (int, error) {
	return s.DB.CountTickets(f)
}

// PollTickets returns tickets changed after since, oldest change first, up to
// one page. A caller that gets a full page should poll again straight away.
func (s *Service) PollTickets(since time.Time, limit *int) ([]*model.Ticket, error) {
	return s.DB.PollTickets(since, limit)
}

// ListApps returns every app Flagit has seen a ticket from.
func (s *Service) ListApps() ([]*model.App, error) {
	return s.DB.ListApps()
}

// UpdateApp changes an app's auto-process setting.
func (s *Service) UpdateApp(name string, autoProcess bool) (*model.App, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("%w: app name is required", ErrInvalid)
	}
	if err := s.DB.UpdateApp(&model.App{Name: name, AutoProcessEnabled: autoProcess}); err != nil {
		return nil, err
	}
	return s.DB.GetApp(name)
}

// Settings is the admin-visible configuration.
type Settings struct {
	GlobalAutoProcess bool   `json:"globalAutoProcess"`
	HermesWebhookURL  string `json:"hermesWebhookUrl"`
}

// GetSettings reads the current configuration. The admin key is deliberately
// not part of this struct: it is never served back over the API.
func (s *Service) GetSettings() (*Settings, error) {
	autoProcess, err := s.DB.GetBoolSetting(model.SettingGlobalAutoProcess, false)
	if err != nil {
		return nil, err
	}
	url, err := s.DB.GetSetting(model.SettingHermesWebhookURL, "")
	if err != nil {
		return nil, err
	}
	return &Settings{GlobalAutoProcess: autoProcess, HermesWebhookURL: url}, nil
}

// UpdateSettings applies the non-nil fields of a settings patch.
func (s *Service) UpdateSettings(globalAutoProcess *bool, hermesWebhookURL *string) (*Settings, error) {
	if globalAutoProcess != nil {
		if err := s.DB.SetBoolSetting(model.SettingGlobalAutoProcess, *globalAutoProcess); err != nil {
			return nil, err
		}
	}
	if hermesWebhookURL != nil {
		raw := strings.TrimSpace(*hermesWebhookURL)
		if raw != "" {
			if err := ValidateWebhookURL(raw); err != nil {
				return nil, err
			}
		}
		if err := s.DB.SetSetting(model.SettingHermesWebhookURL, raw); err != nil {
			return nil, err
		}
	}
	return s.GetSettings()
}

func (s *Service) logger() *slog.Logger {
	if s.Logger == nil {
		return slog.Default()
	}
	return s.Logger
}

// truncate clips s to at most n runes, so multi-byte input is never split.
func truncate(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n])
}

// ValidateWebhookURL checks a Hermes webhook URL before it is stored.
//
// Flagit fetches this URL from inside the network it is deployed on, so an
// admin-supplied address is a server-side request forgery vector: pointed at
// 169.254.169.254 or a neighbouring container it would relay ticket contents
// to somewhere it should not reach. The admin API is already privileged, but
// a webhook URL is exactly the kind of field that gets set from a copied
// config, so it is worth refusing the obviously wrong ones.
func ValidateWebhookURL(raw string) error {
	parsed, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("%w: webhook url is not a valid URL", ErrInvalid)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("%w: webhook url must start with http:// or https://", ErrInvalid)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("%w: webhook url must include a host", ErrInvalid)
	}

	// A literal IP can be checked directly. A hostname cannot be settled here
	// without a DNS lookup, which would still be re-resolved at delivery time,
	// so names are accepted and only obvious loopback aliases are refused.
	if ip := net.ParseIP(host); ip != nil {
		if isPrivateIP(ip) {
			return fmt.Errorf("%w: webhook url must not point at a private or loopback address", ErrInvalid)
		}
		return nil
	}
	if isLoopbackName(host) {
		return fmt.Errorf("%w: webhook url must not point at a private or loopback address", ErrInvalid)
	}
	return nil
}

// isPrivateIP reports whether ip is in a range that is not routable on the
// public internet, and therefore not somewhere Hermes legitimately lives.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
		ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() || ip.IsInterfaceLocalMulticast() {
		return true
	}
	// Carrier-grade NAT (100.64.0.0/10) also covers Tailscale addresses, which
	// net.IP.IsPrivate does not classify.
	if v4 := ip.To4(); v4 != nil {
		return v4[0] == 100 && v4[1] >= 64 && v4[1] <= 127
	}
	return false
}

// isLoopbackName catches the hostnames that resolve to loopback everywhere,
// so "http://localhost:9000/hook" is refused like 127.0.0.1 would be.
func isLoopbackName(host string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}
