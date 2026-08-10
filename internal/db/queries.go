package db

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"flagit/internal/model"
)

// ErrNotFound is returned when a lookup by primary key matches no row.
var ErrNotFound = errors.New("not found")

// maxIDAttempts bounds the retry loop that resolves ticket ID collisions.
// With 62^6 possible IDs a single retry is already vanishingly unlikely.
const maxIDAttempts = 10

const ticketColumns = `id, type, title, body, status, app_name, app_version, os, platform,
	device_model, device_token_hash, log_ring_buffer, logs_duration_min, shipped_version,
	created_at, updated_at`

// ---------------------------------------------------------------- tickets --

// CreateTicket inserts t, generating an ID when one is not already set and
// stamping CreatedAt/UpdatedAt. On success t is updated in place.
func (d *DB) CreateTicket(t *model.Ticket) error {
	now := d.Now().UTC()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	t.UpdatedAt = t.CreatedAt
	if t.Status == "" {
		t.Status = model.StatusOpen
	}

	// An explicit ID is inserted as-is; a generated one retries past collisions.
	if t.ID != "" {
		return d.insertTicket(t)
	}

	var lastErr error
	for i := 0; i < maxIDAttempts; i++ {
		id, err := GenerateTicketID()
		if err != nil {
			return fmt.Errorf("generate ticket id: %w", err)
		}
		t.ID = id
		lastErr = d.insertTicket(t)
		if lastErr == nil {
			return nil
		}
		if !isUniqueViolation(lastErr) {
			return lastErr
		}
	}
	t.ID = ""
	return fmt.Errorf("could not allocate a unique ticket id after %d attempts: %w", maxIDAttempts, lastErr)
}

func (d *DB) insertTicket(t *model.Ticket) error {
	_, err := d.sql.Exec(`INSERT INTO tickets (`+ticketColumns+`)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		t.ID, string(t.Type), t.Title, t.Body, string(t.Status), t.AppName, t.AppVersion,
		t.OS, t.Platform, t.DeviceModel, t.DeviceTokenHash, t.LogRingBuffer,
		t.LogsDurationMin, t.ShippedVersion, FormatTime(t.CreatedAt), FormatTime(t.UpdatedAt))
	return err
}

// GetTicket loads one ticket by ID. It returns ErrNotFound if there is no such
// ticket.
func (d *DB) GetTicket(id string) (*model.Ticket, error) {
	row := d.sql.QueryRow(`SELECT `+ticketColumns+` FROM tickets WHERE id = ?`, id)
	t, err := scanTicket(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	return t, err
}

// DefaultPageSize bounds a listing that does not ask for a specific size, so a
// dashboard on an instance with 50k tickets cannot pull all of them at once.
const DefaultPageSize = 100

// MaxPageSize caps what a caller may request per page.
const MaxPageSize = 1000

// TicketFilter narrows a ticket listing. The zero value lists the first
// DefaultPageSize tickets, newest first.
//
// Limit is a *int so "no limit given" is distinguishable from "limit 0". A nil
// Limit takes DefaultPageSize; a Limit of 0 returns no rows, which is how a
// caller asks for the total count without transferring any tickets.
type TicketFilter struct {
	AppName string
	Status  model.Status
	Type    model.TicketType
	Limit   *int
	Offset  int
}

// page returns the limit and offset to apply, clamped to sane bounds.
func page(limit *int, offset int) (int, int) {
	size := DefaultPageSize
	if limit != nil {
		size = *limit
	}
	if size < 0 {
		size = 0
	}
	if size > MaxPageSize {
		size = MaxPageSize
	}
	if offset < 0 {
		offset = 0
	}
	return size, offset
}

// ListTickets returns tickets matching filter, newest first.
func (d *DB) ListTickets(f TicketFilter) ([]*model.Ticket, error) {
	query := `SELECT ` + ticketColumns + ` FROM tickets`
	var where []string
	var args []any
	if f.AppName != "" {
		where = append(where, "app_name = ?")
		args = append(args, f.AppName)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, string(f.Status))
	}
	if f.Type != "" {
		where = append(where, "type = ?")
		args = append(args, string(f.Type))
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}
	limit, offset := page(f.Limit, f.Offset)
	query += " ORDER BY created_at DESC, id DESC LIMIT ? OFFSET ?"
	args = append(args, limit, offset)
	return d.queryTickets(query, args...)
}

// PollTickets returns tickets updated strictly after since, oldest change
// first, so a poller can walk forward through the stream. The page is bounded;
// a poller that receives a full page should immediately poll again using the
// last returned UpdatedAt as its new cursor.
func (d *DB) PollTickets(since time.Time, limit *int) ([]*model.Ticket, error) {
	size, _ := page(limit, 0)
	return d.queryTickets(`SELECT `+ticketColumns+` FROM tickets
		WHERE updated_at > ? ORDER BY updated_at ASC, id ASC LIMIT ?`,
		FormatTime(since), size)
}

// UpdateTicketStatus moves a ticket to status and touches UpdatedAt.
//
// shippedVersion is written unconditionally, including when it is empty: a
// ticket moved back out of "shipped" must stop claiming it went out in 1.5.0.
// Callers that want to keep the existing value pass it back in.
func (d *DB) UpdateTicketStatus(id string, status model.Status, shippedVersion string) error {
	res, err := d.sql.Exec(
		`UPDATE tickets SET status = ?, shipped_version = ?, updated_at = ? WHERE id = ?`,
		string(status), shippedVersion, FormatTime(d.Now()), id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

// UpdateTicketStatusWithMessage applies a status change and appends an agent
// message in a single transaction, so a ticket can never end up moved without
// the explanation that was meant to accompany it. An empty comment writes no
// message.
func (d *DB) UpdateTicketStatusWithMessage(
	id string, status model.Status, shippedVersion, comment string,
) error {
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	// Rollback after a successful Commit is a no-op, so this is safe to defer
	// unconditionally and covers every early return below.
	defer func() { _ = tx.Rollback() }()

	now := FormatTime(d.Now())
	res, err := tx.Exec(
		`UPDATE tickets SET status = ?, shipped_version = ?, updated_at = ? WHERE id = ?`,
		string(status), shippedVersion, now, id)
	if err != nil {
		return err
	}
	if err := expectOneRow(res); err != nil {
		return err
	}

	if comment != "" {
		if _, err := tx.Exec(
			`INSERT INTO messages (ticket_id, body, role, created_at) VALUES (?,?,?,?)`,
			id, comment, string(model.RoleAgent), now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// DeleteTicket removes a ticket and everything recorded against it, reporting
// whether there was a ticket there to remove.
//
// A miss is not an error. Deleting is idempotent: the caller asked for the
// ticket to be gone, and after this call it is, whether or not this call is
// what made it so. That matters most for the double-click and the retried
// request, which must not turn into a failure the admin has to interpret. The
// boolean is what keeps that honest — it lets the caller say "already gone"
// rather than claim a second deletion.
//
// The rows are removed permanently: there is no archive table and no
// soft-delete flag, so nothing here is recoverable afterwards.
//
// Messages and commits are deleted explicitly rather than left to the schema's
// ON DELETE CASCADE. The cascade only fires when the connection has
// foreign_keys turned on, and that is a per-connection pragma rather than a
// property of the file — leaning on it would make orphaned rows depend on how
// the database happened to be opened. All three statements share one
// transaction, so a ticket can never lose its conversation and survive.
func (d *DB) DeleteTicket(id string) (bool, error) {
	deleted, err := d.DeleteTickets([]string{id})
	if err != nil {
		return false, err
	}
	return len(deleted) == 1, nil
}

// DeleteTickets removes every named ticket, with its conversation and its
// commits, and returns the IDs that were actually there — in the order they
// were given, without the duplicates and the misses.
//
// It is atomic: one transaction covers the whole set, so a failure part-way
// through leaves every ticket in place rather than half a selection deleted
// and no way to tell which half. An empty or all-missing set is a no-op that
// returns no IDs, not an error, for the same reason DeleteTicket tolerates a
// miss.
//
// The IDs are read back inside the transaction before anything is removed,
// which is what makes the return value the set that was genuinely deleted
// rather than the set the caller hoped for.
func (d *DB) DeleteTickets(ids []string) ([]string, error) {
	unique := dedupe(ids)
	if len(unique) == 0 {
		return []string{}, nil
	}

	tx, err := d.sql.Begin()
	if err != nil {
		return nil, err
	}
	// Rollback after a successful Commit is a no-op, so this covers every early
	// return below — and it is what makes the whole set atomic.
	defer func() { _ = tx.Rollback() }()

	placeholders, args := inClause(unique)

	present, err := existingTicketIDs(tx, placeholders, args)
	if err != nil {
		return nil, err
	}
	if len(present) == 0 {
		// Nothing to do, but still commit: an open transaction left dangling
		// would hold the single writer connection.
		return []string{}, tx.Commit()
	}

	for _, stmt := range [...]string{
		`DELETE FROM commits WHERE ticket_id IN (` + placeholders + `)`,
		`DELETE FROM messages WHERE ticket_id IN (` + placeholders + `)`,
		`DELETE FROM tickets WHERE id IN (` + placeholders + `)`,
	} {
		if _, err := tx.Exec(stmt, args...); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	// Ordered by the caller's request rather than by what SQLite returned, so
	// the response reads back in the order the admin selected.
	deleted := make([]string, 0, len(present))
	for _, id := range unique {
		if present[id] {
			deleted = append(deleted, id)
		}
	}
	return deleted, nil
}

// existingTicketIDs reports which of the named tickets exist, as a set.
func existingTicketIDs(tx *sql.Tx, placeholders string, args []any) (map[string]bool, error) {
	rows, err := tx.Query(`SELECT id FROM tickets WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	present := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		present[id] = true
	}
	return present, rows.Err()
}

// dedupe drops blanks and repeats while keeping first-seen order. A selection
// that names the same ticket twice is a UI accident, not a request to delete
// it twice.
func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	return out
}

// inClause builds the "?,?,?" placeholder list and matching argument slice for
// an IN clause over ids.
func inClause(ids []string) (string, []any) {
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return strings.TrimSuffix(strings.Repeat("?,", len(ids)), ","), args
}

// TouchTicket bumps UpdatedAt so a changed ticket resurfaces in polling.
func (d *DB) TouchTicket(id string) error {
	res, err := d.sql.Exec(`UPDATE tickets SET updated_at = ? WHERE id = ?`, FormatTime(d.Now()), id)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

// CountTickets reports how many tickets match filter, ignoring its Limit and
// Offset: this is the total a paginated caller needs in order to know how many
// pages there are.
func (d *DB) CountTickets(f TicketFilter) (int, error) {
	query := `SELECT COUNT(*) FROM tickets`
	var where []string
	var args []any
	if f.AppName != "" {
		where = append(where, "app_name = ?")
		args = append(args, f.AppName)
	}
	if f.Status != "" {
		where = append(where, "status = ?")
		args = append(args, string(f.Status))
	}
	if f.Type != "" {
		where = append(where, "type = ?")
		args = append(args, string(f.Type))
	}
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	var n int
	err := d.sql.QueryRow(query, args...).Scan(&n)
	return n, err
}

func (d *DB) queryTickets(query string, args ...any) ([]*model.Ticket, error) {
	rows, err := d.sql.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tickets := []*model.Ticket{}
	for rows.Next() {
		t, err := scanTicket(rows)
		if err != nil {
			return nil, err
		}
		tickets = append(tickets, t)
	}
	return tickets, rows.Err()
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface{ Scan(dest ...any) error }

func scanTicket(s scanner) (*model.Ticket, error) {
	var t model.Ticket
	var typ, status, createdAt, updatedAt string
	if err := s.Scan(&t.ID, &typ, &t.Title, &t.Body, &status, &t.AppName, &t.AppVersion,
		&t.OS, &t.Platform, &t.DeviceModel, &t.DeviceTokenHash, &t.LogRingBuffer,
		&t.LogsDurationMin, &t.ShippedVersion, &createdAt, &updatedAt); err != nil {
		return nil, err
	}
	t.Type = model.TicketType(typ)
	t.Status = model.Status(status)

	var err error
	if t.CreatedAt, err = ParseTime(createdAt); err != nil {
		return nil, fmt.Errorf("parse created_at of %s: %w", t.ID, err)
	}
	if t.UpdatedAt, err = ParseTime(updatedAt); err != nil {
		return nil, fmt.Errorf("parse updated_at of %s: %w", t.ID, err)
	}
	return &t, nil
}

// --------------------------------------------------------------- messages --

// CreateMessage appends a message to a ticket and bumps the ticket's
// UpdatedAt so the conversation shows up in polling.
func (d *DB) CreateMessage(m *model.Message) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = d.Now().UTC()
	}

	// Both writes go in one transaction: a message that landed without its
	// ticket's updated_at moving would be invisible to Hermes' poller, which
	// is a silently dropped reply rather than a visible failure.
	tx, err := d.sql.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	res, err := tx.Exec(`INSERT INTO messages (ticket_id, body, role, created_at) VALUES (?,?,?,?)`,
		m.TicketID, m.Body, string(m.Role), FormatTime(m.CreatedAt))
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}

	touched, err := tx.Exec(`UPDATE tickets SET updated_at = ? WHERE id = ?`,
		FormatTime(d.Now()), m.TicketID)
	if err != nil {
		return err
	}
	if err := expectOneRow(touched); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	// Assigned only after the commit succeeds, so a caller never holds an ID
	// for a row that was rolled back.
	m.ID = id
	return nil
}

// ListMessagesByTicket returns a ticket's conversation in chronological order.
func (d *DB) ListMessagesByTicket(ticketID string) ([]*model.Message, error) {
	rows, err := d.sql.Query(`SELECT id, ticket_id, body, role, created_at FROM messages
		WHERE ticket_id = ? ORDER BY id ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	messages := []*model.Message{}
	for rows.Next() {
		var m model.Message
		var role, createdAt string
		if err := rows.Scan(&m.ID, &m.TicketID, &m.Body, &role, &createdAt); err != nil {
			return nil, err
		}
		m.Role = model.Role(role)
		if m.CreatedAt, err = ParseTime(createdAt); err != nil {
			return nil, err
		}
		messages = append(messages, &m)
	}
	return messages, rows.Err()
}

// ---------------------------------------------------------------- commits --

// CreateCommit records a commit an agent produced for a ticket.
func (d *DB) CreateCommit(c *model.CommitInfo) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = d.Now().UTC()
	}
	res, err := d.sql.Exec(`INSERT INTO commits (ticket_id, commit_hash, branch, message, created_at)
		VALUES (?,?,?,?,?)`, c.TicketID, c.CommitHash, c.Branch, c.Message, FormatTime(c.CreatedAt))
	if err != nil {
		return err
	}
	c.ID, err = res.LastInsertId()
	return err
}

// ListCommitsByTicket returns the commits recorded against a ticket, oldest first.
func (d *DB) ListCommitsByTicket(ticketID string) ([]*model.CommitInfo, error) {
	rows, err := d.sql.Query(`SELECT id, ticket_id, commit_hash, branch, message, created_at
		FROM commits WHERE ticket_id = ? ORDER BY id ASC`, ticketID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	commits := []*model.CommitInfo{}
	for rows.Next() {
		var c model.CommitInfo
		var createdAt string
		if err := rows.Scan(&c.ID, &c.TicketID, &c.CommitHash, &c.Branch, &c.Message, &createdAt); err != nil {
			return nil, err
		}
		if c.CreatedAt, err = ParseTime(createdAt); err != nil {
			return nil, err
		}
		commits = append(commits, &c)
	}
	return commits, rows.Err()
}

// ------------------------------------------------------------------- apps --

// CreateApp registers an app. It fails if the app already exists.
func (d *DB) CreateApp(a *model.App) error {
	if a.CreatedAt.IsZero() {
		a.CreatedAt = d.Now().UTC()
	}
	_, err := d.sql.Exec(`INSERT INTO apps (name, auto_process_enabled, created_at) VALUES (?,?,?)`,
		a.Name, boolToInt(a.AutoProcessEnabled), FormatTime(a.CreatedAt))
	return err
}

// GetApp loads one app by name, or ErrNotFound.
func (d *DB) GetApp(name string) (*model.App, error) {
	var a model.App
	var auto int
	var createdAt string
	err := d.sql.QueryRow(`SELECT name, auto_process_enabled, created_at FROM apps WHERE name = ?`, name).
		Scan(&a.Name, &auto, &createdAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.AutoProcessEnabled = auto != 0
	if a.CreatedAt, err = ParseTime(createdAt); err != nil {
		return nil, err
	}
	return &a, nil
}

// ListApps returns every known app, alphabetically.
func (d *DB) ListApps() ([]*model.App, error) {
	rows, err := d.sql.Query(`SELECT name, auto_process_enabled, created_at FROM apps ORDER BY name ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	apps := []*model.App{}
	for rows.Next() {
		var a model.App
		var auto int
		var createdAt string
		if err := rows.Scan(&a.Name, &auto, &createdAt); err != nil {
			return nil, err
		}
		a.AutoProcessEnabled = auto != 0
		if a.CreatedAt, err = ParseTime(createdAt); err != nil {
			return nil, err
		}
		apps = append(apps, &a)
	}
	return apps, rows.Err()
}

// UpdateApp writes an app's settings. It returns ErrNotFound for unknown apps.
func (d *DB) UpdateApp(a *model.App) error {
	res, err := d.sql.Exec(`UPDATE apps SET auto_process_enabled = ? WHERE name = ?`,
		boolToInt(a.AutoProcessEnabled), a.Name)
	if err != nil {
		return err
	}
	return expectOneRow(res)
}

// EnsureApp returns the app named name, registering it with
// autoProcessOnCreate when it is not yet known. The second return value
// reports whether this call was the one that registered it — the caller uses
// that to apply the global "new unknown app" policy exactly once.
func (d *DB) EnsureApp(name string, autoProcessOnCreate bool) (*model.App, bool, error) {
	app, err := d.GetApp(name)
	if err == nil {
		return app, false, nil
	}
	if !errors.Is(err, ErrNotFound) {
		return nil, false, err
	}

	app = &model.App{Name: name, AutoProcessEnabled: autoProcessOnCreate}
	if err := d.CreateApp(app); err != nil {
		// Lost a race against a concurrent first ticket: adopt the winner's row.
		if isUniqueViolation(err) {
			existing, getErr := d.GetApp(name)
			return existing, false, getErr
		}
		return nil, false, err
	}
	return app, true, nil
}

// --------------------------------------------------------------- settings --

// GetSetting reads a setting, returning def when the key is unset.
func (d *DB) GetSetting(key, def string) (string, error) {
	var value string
	err := d.sql.QueryRow(`SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return def, nil
	}
	if err != nil {
		return "", err
	}
	return value, nil
}

// SetSetting writes (or overwrites) a setting.
func (d *DB) SetSetting(key, value string) error {
	_, err := d.sql.Exec(`INSERT INTO settings (key, value) VALUES (?,?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, key, value)
	return err
}

// ListSettings returns every stored setting as a map.
func (d *DB) ListSettings() (map[string]string, error) {
	rows, err := d.sql.Query(`SELECT key, value FROM settings ORDER BY key ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
}

// GetBoolSetting reads a setting stored as "true"/"false".
func (d *DB) GetBoolSetting(key string, def bool) (bool, error) {
	raw, err := d.GetSetting(key, boolToString(def))
	if err != nil {
		return false, err
	}
	return raw == "true" || raw == "1", nil
}

// SetBoolSetting writes a boolean setting as "true"/"false".
func (d *DB) SetBoolSetting(key string, value bool) error {
	return d.SetSetting(key, boolToString(value))
}

// ---------------------------------------------------------------- helpers --

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func boolToString(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func expectOneRow(res sql.Result) error {
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// isUniqueViolation reports whether err is a primary-key/unique constraint
// failure. modernc.org/sqlite reports these as a message rather than a typed
// error, so matching on the text is the portable option.
func isUniqueViolation(err error) bool {
	return err != nil && strings.Contains(strings.ToUpper(err.Error()), "UNIQUE CONSTRAINT FAILED")
}
