// Package model holds the core data structures shared across Flagit.
package model

import "time"

// TicketType distinguishes bug reports from feature requests.
type TicketType string

const (
	TypeBug     TicketType = "bug"
	TypeFeature TicketType = "feature"
)

// Valid reports whether t is a known ticket type.
func (t TicketType) Valid() bool {
	return t == TypeBug || t == TypeFeature
}

// Status is a point in the ticket lifecycle.
//
// The suggested flow is open → in-progress → resolved → shipped → closed, but
// admins may move a ticket to any status at any time.
type Status string

const (
	StatusOpen       Status = "open"
	StatusInProgress Status = "in-progress"
	StatusResolved   Status = "resolved"
	StatusShipped    Status = "shipped"
	StatusClosed     Status = "closed"
)

// Statuses lists every valid status in lifecycle order.
var Statuses = []Status{StatusOpen, StatusInProgress, StatusResolved, StatusShipped, StatusClosed}

// Valid reports whether s is a known status.
func (s Status) Valid() bool {
	for _, known := range Statuses {
		if s == known {
			return true
		}
	}
	return false
}

// allowedTransitions is the workflow the product documents:
//
//	open → in-progress → resolved → shipped → closed
//
// Each step may also skip straight to closed (a ticket can be dismissed at any
// point) and step back one stage (work reopens, a release is rolled back).
// Anything else is a mistake worth catching — an admin who genuinely means it
// passes the override, see CanTransitionTo.
var allowedTransitions = map[Status][]Status{
	StatusOpen:       {StatusInProgress, StatusResolved, StatusClosed},
	StatusInProgress: {StatusOpen, StatusResolved, StatusClosed},
	StatusResolved:   {StatusInProgress, StatusShipped, StatusClosed},
	StatusShipped:    {StatusResolved, StatusClosed},
	StatusClosed:     {StatusOpen, StatusInProgress},
}

// CanTransitionTo reports whether a ticket may move from s to next under the
// documented workflow. Moving to the status it already holds is always allowed,
// so re-saving a ticket without changing its status is not an error.
func (s Status) CanTransitionTo(next Status) bool {
	if !s.Valid() || !next.Valid() {
		return false
	}
	if s == next {
		return true
	}
	for _, allowed := range allowedTransitions[s] {
		if next == allowed {
			return true
		}
	}
	return false
}

// NextStatuses lists the statuses a ticket in status s may move to, for a UI
// that wants to offer only the legal ones.
func (s Status) NextStatuses() []Status {
	if !s.Valid() {
		return nil
	}
	return append([]Status{s}, allowedTransitions[s]...)
}

// Role identifies who wrote a message.
type Role string

const (
	RoleUser  Role = "user"
	RoleAgent Role = "agent"
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	return r == RoleUser || r == RoleAgent
}

// Ticket is a single bug report or feature request.
//
// DeviceTokenHash is the SHA-256 of the reporter's device token; the raw token
// never touches the database and is never serialised back to a client.
type Ticket struct {
	ID              string     `json:"id"`
	Type            TicketType `json:"type"`
	Title           string     `json:"title"`
	Body            string     `json:"body"`
	Status          Status     `json:"status"`
	AppName         string     `json:"appName"`
	AppVersion      string     `json:"appVersion"`
	OS              string     `json:"os"`
	Platform        string     `json:"platform"`
	DeviceModel     string     `json:"deviceModel"`
	DeviceTokenHash string     `json:"-"`
	LogRingBuffer   string     `json:"logs,omitempty"`
	LogsDurationMin int        `json:"logsDurationMin,omitempty"`
	ShippedVersion  string     `json:"shippedVersion,omitempty"`
	CreatedAt       time.Time  `json:"createdAt"`
	UpdatedAt       time.Time  `json:"updatedAt"`
}

// Message is one entry in a ticket's conversation.
type Message struct {
	ID        int64     `json:"id"`
	TicketID  string    `json:"ticketId"`
	Body      string    `json:"body"`
	Role      Role      `json:"role"`
	CreatedAt time.Time `json:"createdAt"`
}

// CommitInfo records a commit an agent produced for a ticket. Dev-only
// metadata: it is exposed through the admin dashboard, never the public API.
type CommitInfo struct {
	ID         int64     `json:"id"`
	TicketID   string    `json:"ticketId"`
	CommitHash string    `json:"commitHash"`
	Branch     string    `json:"branch"`
	Message    string    `json:"message"`
	CreatedAt  time.Time `json:"createdAt"`
}

// App is a client application Flagit has seen at least one ticket from.
type App struct {
	Name               string    `json:"name"`
	AutoProcessEnabled bool      `json:"autoProcessEnabled"`
	CreatedAt          time.Time `json:"createdAt"`
}

// Setting keys stored in the settings table.
const (
	SettingGlobalAutoProcess = "global_auto_process"
	SettingHermesWebhookURL  = "hermes_webhook_url"
	// SettingAdminKeyHash holds the SHA-256 of the admin key, never the key.
	SettingAdminKeyHash = "admin_key_hash"
)
