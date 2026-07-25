package db

import (
	"testing"
	"time"

	"flagit/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTicket(appName string) *model.Ticket {
	return &model.Ticket{
		Type:            model.TypeBug,
		Title:           "Crash on save",
		Body:            "Tapping save closes the app",
		AppName:         appName,
		AppVersion:      "1.4.2",
		OS:              "iOS 18.2",
		Platform:        "ios",
		DeviceModel:     "iPhone 15",
		DeviceTokenHash: HashDeviceToken("device-token-1"),
		LogRingBuffer:   "panic: nil map",
		LogsDurationMin: 5,
	}
}

func TestCreateTicketGeneratesIDAndTimestamps(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")

	require.NoError(t, d.CreateTicket(ticket))

	assert.True(t, ValidTicketID(ticket.ID), "generated %q", ticket.ID)
	assert.Equal(t, model.StatusOpen, ticket.Status, "status defaults to open")
	assert.False(t, ticket.CreatedAt.IsZero())
	assert.Equal(t, ticket.CreatedAt, ticket.UpdatedAt)

	got, err := d.GetTicket(ticket.ID)
	require.NoError(t, err)
	assert.Equal(t, ticket.Title, got.Title)
	assert.Equal(t, ticket.Body, got.Body)
	assert.Equal(t, ticket.AppVersion, got.AppVersion)
	assert.Equal(t, ticket.OS, got.OS)
	assert.Equal(t, ticket.Platform, got.Platform)
	assert.Equal(t, ticket.DeviceModel, got.DeviceModel)
	assert.Equal(t, ticket.DeviceTokenHash, got.DeviceTokenHash)
	assert.Equal(t, ticket.LogRingBuffer, got.LogRingBuffer)
	assert.Equal(t, 5, got.LogsDurationMin)
	assert.True(t, ticket.CreatedAt.Equal(got.CreatedAt))
}

func TestCreateTicketHonoursExplicitID(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")
	ticket.ID = "FLG-FIXED1"
	ticket.Status = model.StatusResolved

	require.NoError(t, d.CreateTicket(ticket))

	got, err := d.GetTicket("FLG-FIXED1")
	require.NoError(t, err)
	assert.Equal(t, model.StatusResolved, got.Status)
}

func TestCreateTicketRejectsDuplicateExplicitID(t *testing.T) {
	d := newTestDB(t)
	first := newTicket("notes")
	first.ID = "FLG-DUPES1"
	require.NoError(t, d.CreateTicket(first))

	second := newTicket("notes")
	second.ID = "FLG-DUPES1"

	err := d.CreateTicket(second)
	require.Error(t, err)
	assert.True(t, isUniqueViolation(err))
}

func TestCreateTicketRetriesPastIDCollisions(t *testing.T) {
	d := newTestDB(t)

	// Occupy a run of IDs, then verify inserts still succeed: the retry loop
	// is what makes a collision survivable rather than fatal.
	for i := 0; i < 25; i++ {
		taken := newTicket("notes")
		require.NoError(t, d.CreateTicket(taken))
	}

	fresh := newTicket("notes")
	require.NoError(t, d.CreateTicket(fresh))

	all, err := d.ListTickets(TicketFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 26, "every ticket got a distinct id")
}

func TestGetTicketNotFound(t *testing.T) {
	d := newTestDB(t)

	_, err := d.GetTicket("FLG-NOPE12")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestListTicketsFilters(t *testing.T) {
	d := newTestDB(t)

	bug := newTicket("notes")
	require.NoError(t, d.CreateTicket(bug))

	feature := newTicket("notes")
	feature.Type = model.TypeFeature
	require.NoError(t, d.CreateTicket(feature))

	other := newTicket("timer")
	require.NoError(t, d.CreateTicket(other))
	require.NoError(t, d.UpdateTicketStatus(other.ID, model.StatusClosed, ""))

	tests := []struct {
		name   string
		filter TicketFilter
		want   int
	}{
		{"all", TicketFilter{}, 3},
		{"by app", TicketFilter{AppName: "notes"}, 2},
		{"by type", TicketFilter{Type: model.TypeFeature}, 1},
		{"by status", TicketFilter{Status: model.StatusClosed}, 1},
		{"by app and status", TicketFilter{AppName: "notes", Status: model.StatusOpen}, 2},
		{"unknown app", TicketFilter{AppName: "ghost"}, 0},
		{"limited", TicketFilter{Limit: 2}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := d.ListTickets(tt.filter)
			require.NoError(t, err)
			assert.Len(t, got, tt.want)
		})
	}
}

func TestListTicketsNewestFirst(t *testing.T) {
	d := newTestDB(t)

	first := newTicket("notes")
	require.NoError(t, d.CreateTicket(first))
	second := newTicket("notes")
	require.NoError(t, d.CreateTicket(second))

	got, err := d.ListTickets(TicketFilter{})
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, second.ID, got[0].ID)
	assert.Equal(t, first.ID, got[1].ID)
}

func TestCountTickets(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.CreateTicket(newTicket("notes")))
	require.NoError(t, d.CreateTicket(newTicket("timer")))

	n, err := d.CountTickets(TicketFilter{AppName: "notes"})
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestUpdateTicketStatus(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")
	require.NoError(t, d.CreateTicket(ticket))

	require.NoError(t, d.UpdateTicketStatus(ticket.ID, model.StatusInProgress, ""))

	got, err := d.GetTicket(ticket.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusInProgress, got.Status)
	assert.Empty(t, got.ShippedVersion)
	assert.True(t, got.UpdatedAt.After(got.CreatedAt), "updated_at moved forward")
}

func TestUpdateTicketStatusRecordsShippedVersion(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")
	require.NoError(t, d.CreateTicket(ticket))

	require.NoError(t, d.UpdateTicketStatus(ticket.ID, model.StatusShipped, "1.5.0"))

	got, err := d.GetTicket(ticket.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusShipped, got.Status)
	assert.Equal(t, "1.5.0", got.ShippedVersion)
}

func TestUpdateTicketStatusUnknownTicket(t *testing.T) {
	d := newTestDB(t)

	assert.ErrorIs(t, d.UpdateTicketStatus("FLG-NOPE12", model.StatusClosed, ""), ErrNotFound)
	assert.ErrorIs(t, d.UpdateTicketStatus("FLG-NOPE12", model.StatusShipped, "9.9"), ErrNotFound)
	assert.ErrorIs(t, d.TouchTicket("FLG-NOPE12"), ErrNotFound)
}

func TestPollTickets(t *testing.T) {
	d := newTestDB(t)

	first := newTicket("notes")
	require.NoError(t, d.CreateTicket(first))
	cutoff := first.CreatedAt

	second := newTicket("notes")
	require.NoError(t, d.CreateTicket(second))

	changed, err := d.PollTickets(cutoff)
	require.NoError(t, err)
	require.Len(t, changed, 1)
	assert.Equal(t, second.ID, changed[0].ID)

	// An update to an older ticket brings it back into the poll window.
	require.NoError(t, d.UpdateTicketStatus(first.ID, model.StatusInProgress, ""))
	changed, err = d.PollTickets(cutoff)
	require.NoError(t, err)
	require.Len(t, changed, 2)
	assert.Equal(t, second.ID, changed[0].ID, "oldest change first")
	assert.Equal(t, first.ID, changed[1].ID)
}

func TestPollTicketsFromEpochReturnsEverything(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.CreateTicket(newTicket("notes")))
	require.NoError(t, d.CreateTicket(newTicket("timer")))

	got, err := d.PollTickets(time.Time{})
	require.NoError(t, err)
	assert.Len(t, got, 2)
}

func TestPollTicketsEmpty(t *testing.T) {
	d := newTestDB(t)

	got, err := d.PollTickets(time.Now())
	require.NoError(t, err)
	assert.NotNil(t, got, "an empty poll serialises as [] not null")
	assert.Empty(t, got)
}

func TestMessagesRoundTrip(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")
	require.NoError(t, d.CreateTicket(ticket))
	createdAt := ticket.CreatedAt

	user := &model.Message{TicketID: ticket.ID, Body: "Still broken", Role: model.RoleUser}
	require.NoError(t, d.CreateMessage(user))
	agent := &model.Message{TicketID: ticket.ID, Body: "Looking into it", Role: model.RoleAgent}
	require.NoError(t, d.CreateMessage(agent))

	assert.NotZero(t, user.ID)
	assert.Greater(t, agent.ID, user.ID)

	got, err := d.ListMessagesByTicket(ticket.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "Still broken", got[0].Body)
	assert.Equal(t, model.RoleUser, got[0].Role)
	assert.Equal(t, model.RoleAgent, got[1].Role)
	assert.False(t, got[0].CreatedAt.IsZero())

	// Posting a message must resurface the ticket for pollers.
	reloaded, err := d.GetTicket(ticket.ID)
	require.NoError(t, err)
	assert.True(t, reloaded.UpdatedAt.After(createdAt))
}

func TestListMessagesByTicketEmpty(t *testing.T) {
	d := newTestDB(t)

	got, err := d.ListMessagesByTicket("FLG-NOPE12")
	require.NoError(t, err)
	assert.NotNil(t, got)
	assert.Empty(t, got)
}

func TestCreateMessageRejectsUnknownTicket(t *testing.T) {
	d := newTestDB(t)

	err := d.CreateMessage(&model.Message{TicketID: "FLG-NOPE12", Body: "hi", Role: model.RoleUser})
	assert.Error(t, err, "foreign key constraint rejects orphan messages")
}

func TestCommitsRoundTrip(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")
	require.NoError(t, d.CreateTicket(ticket))

	commit := &model.CommitInfo{
		TicketID:   ticket.ID,
		CommitHash: "a1b2c3d",
		Branch:     "fix/FLG-crash",
		Message:    "fix: guard against nil map",
	}
	require.NoError(t, d.CreateCommit(commit))
	assert.NotZero(t, commit.ID)

	got, err := d.ListCommitsByTicket(ticket.ID)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "a1b2c3d", got[0].CommitHash)
	assert.Equal(t, "fix/FLG-crash", got[0].Branch)
	assert.False(t, got[0].CreatedAt.IsZero())

	empty, err := d.ListCommitsByTicket("FLG-NOPE12")
	require.NoError(t, err)
	assert.Empty(t, empty)
}

func TestCreateCommitRejectsUnknownTicket(t *testing.T) {
	d := newTestDB(t)

	assert.Error(t, d.CreateCommit(&model.CommitInfo{TicketID: "FLG-NOPE12", CommitHash: "abc"}))
}

func TestDeletingTicketCascades(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")
	require.NoError(t, d.CreateTicket(ticket))
	require.NoError(t, d.CreateMessage(&model.Message{TicketID: ticket.ID, Body: "hi", Role: model.RoleUser}))
	require.NoError(t, d.CreateCommit(&model.CommitInfo{TicketID: ticket.ID, CommitHash: "abc"}))

	_, err := d.SQL().Exec(`DELETE FROM tickets WHERE id = ?`, ticket.ID)
	require.NoError(t, err)

	messages, err := d.ListMessagesByTicket(ticket.ID)
	require.NoError(t, err)
	assert.Empty(t, messages)
	commits, err := d.ListCommitsByTicket(ticket.ID)
	require.NoError(t, err)
	assert.Empty(t, commits)
}

func TestAppsCRUD(t *testing.T) {
	d := newTestDB(t)

	require.NoError(t, d.CreateApp(&model.App{Name: "notes"}))
	require.NoError(t, d.CreateApp(&model.App{Name: "timer", AutoProcessEnabled: true}))

	notes, err := d.GetApp("notes")
	require.NoError(t, err)
	assert.False(t, notes.AutoProcessEnabled, "auto-process is off by default")
	assert.False(t, notes.CreatedAt.IsZero())

	timer, err := d.GetApp("timer")
	require.NoError(t, err)
	assert.True(t, timer.AutoProcessEnabled)

	notes.AutoProcessEnabled = true
	require.NoError(t, d.UpdateApp(notes))
	notes, err = d.GetApp("notes")
	require.NoError(t, err)
	assert.True(t, notes.AutoProcessEnabled)

	apps, err := d.ListApps()
	require.NoError(t, err)
	require.Len(t, apps, 2)
	assert.Equal(t, "notes", apps[0].Name, "alphabetical")
	assert.Equal(t, "timer", apps[1].Name)
}

func TestAppErrors(t *testing.T) {
	d := newTestDB(t)
	require.NoError(t, d.CreateApp(&model.App{Name: "notes"}))

	_, err := d.GetApp("ghost")
	assert.ErrorIs(t, err, ErrNotFound)

	assert.ErrorIs(t, d.UpdateApp(&model.App{Name: "ghost"}), ErrNotFound)
	assert.Error(t, d.CreateApp(&model.App{Name: "notes"}), "duplicate app name")
}

func TestListAppsEmpty(t *testing.T) {
	d := newTestDB(t)

	apps, err := d.ListApps()
	require.NoError(t, err)
	assert.NotNil(t, apps)
	assert.Empty(t, apps)
}

func TestEnsureApp(t *testing.T) {
	d := newTestDB(t)

	app, created, err := d.EnsureApp("notes", true)
	require.NoError(t, err)
	assert.True(t, created, "first sighting registers the app")
	assert.True(t, app.AutoProcessEnabled, "the global default applies on creation")

	// A second sighting must not re-apply the global default.
	app, created, err = d.EnsureApp("notes", false)
	require.NoError(t, err)
	assert.False(t, created)
	assert.True(t, app.AutoProcessEnabled, "existing per-app setting wins")
}

func TestEnsureAppDefaultsOff(t *testing.T) {
	d := newTestDB(t)

	app, created, err := d.EnsureApp("timer", false)
	require.NoError(t, err)
	assert.True(t, created)
	assert.False(t, app.AutoProcessEnabled)
}

func TestSettings(t *testing.T) {
	d := newTestDB(t)

	got, err := d.GetSetting("missing", "fallback")
	require.NoError(t, err)
	assert.Equal(t, "fallback", got)

	require.NoError(t, d.SetSetting(model.SettingHermesWebhookURL, "https://hermes.example/hook"))
	got, err = d.GetSetting(model.SettingHermesWebhookURL, "")
	require.NoError(t, err)
	assert.Equal(t, "https://hermes.example/hook", got)

	// Upsert, not insert-only.
	require.NoError(t, d.SetSetting(model.SettingHermesWebhookURL, "https://other.example/hook"))
	got, err = d.GetSetting(model.SettingHermesWebhookURL, "")
	require.NoError(t, err)
	assert.Equal(t, "https://other.example/hook", got)

	all, err := d.ListSettings()
	require.NoError(t, err)
	assert.Equal(t, map[string]string{model.SettingHermesWebhookURL: "https://other.example/hook"}, all)
}

func TestBoolSettings(t *testing.T) {
	d := newTestDB(t)

	on, err := d.GetBoolSetting(model.SettingGlobalAutoProcess, false)
	require.NoError(t, err)
	assert.False(t, on, "global auto-process defaults to off")

	require.NoError(t, d.SetBoolSetting(model.SettingGlobalAutoProcess, true))
	on, err = d.GetBoolSetting(model.SettingGlobalAutoProcess, false)
	require.NoError(t, err)
	assert.True(t, on)

	require.NoError(t, d.SetBoolSetting(model.SettingGlobalAutoProcess, false))
	on, err = d.GetBoolSetting(model.SettingGlobalAutoProcess, true)
	require.NoError(t, err)
	assert.False(t, on)

	// Legacy/hand-edited "1" also counts as true.
	require.NoError(t, d.SetSetting("legacy", "1"))
	on, err = d.GetBoolSetting("legacy", false)
	require.NoError(t, err)
	assert.True(t, on)
}

func TestQueriesFailOnClosedDB(t *testing.T) {
	d, err := InitDB(":memory:")
	require.NoError(t, err)
	require.NoError(t, d.Close())

	assert.Error(t, d.CreateTicket(newTicket("notes")))
	_, err = d.GetTicket("FLG-ABC123")
	assert.Error(t, err)
	_, err = d.ListTickets(TicketFilter{})
	assert.Error(t, err)
	_, err = d.PollTickets(time.Time{})
	assert.Error(t, err)
	assert.Error(t, d.UpdateTicketStatus("FLG-ABC123", model.StatusOpen, ""))
	assert.Error(t, d.UpdateTicketStatus("FLG-ABC123", model.StatusShipped, "1.0"))
	assert.Error(t, d.CreateMessage(&model.Message{TicketID: "FLG-ABC123"}))
	_, err = d.ListMessagesByTicket("FLG-ABC123")
	assert.Error(t, err)
	assert.Error(t, d.CreateCommit(&model.CommitInfo{TicketID: "FLG-ABC123"}))
	_, err = d.ListCommitsByTicket("FLG-ABC123")
	assert.Error(t, err)
	assert.Error(t, d.CreateApp(&model.App{Name: "notes"}))
	_, err = d.GetApp("notes")
	assert.Error(t, err)
	_, err = d.ListApps()
	assert.Error(t, err)
	assert.Error(t, d.UpdateApp(&model.App{Name: "notes"}))
	_, _, err = d.EnsureApp("notes", false)
	assert.Error(t, err)
	_, err = d.GetSetting("k", "")
	assert.Error(t, err)
	assert.Error(t, d.SetSetting("k", "v"))
	_, err = d.ListSettings()
	assert.Error(t, err)
	_, err = d.GetBoolSetting("k", false)
	assert.Error(t, err)
}

func TestScanTicketRejectsCorruptTimestamps(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")
	require.NoError(t, d.CreateTicket(ticket))

	_, err := d.SQL().Exec(`UPDATE tickets SET created_at = 'not-a-time' WHERE id = ?`, ticket.ID)
	require.NoError(t, err)
	_, err = d.GetTicket(ticket.ID)
	assert.ErrorContains(t, err, "parse created_at")

	_, err = d.SQL().Exec(`UPDATE tickets SET created_at = updated_at, updated_at = 'nope' WHERE id = ?`, ticket.ID)
	require.NoError(t, err)
	_, err = d.GetTicket(ticket.ID)
	assert.ErrorContains(t, err, "parse updated_at")

	_, err = d.ListTickets(TicketFilter{})
	assert.Error(t, err)
}

func TestScanRowsRejectCorruptTimestamps(t *testing.T) {
	d := newTestDB(t)
	ticket := newTicket("notes")
	require.NoError(t, d.CreateTicket(ticket))
	require.NoError(t, d.CreateMessage(&model.Message{TicketID: ticket.ID, Body: "hi", Role: model.RoleUser}))
	require.NoError(t, d.CreateCommit(&model.CommitInfo{TicketID: ticket.ID, CommitHash: "abc"}))
	require.NoError(t, d.CreateApp(&model.App{Name: "notes"}))

	_, err := d.SQL().Exec(`UPDATE messages SET created_at = 'nope'`)
	require.NoError(t, err)
	_, err = d.ListMessagesByTicket(ticket.ID)
	assert.Error(t, err)

	_, err = d.SQL().Exec(`UPDATE commits SET created_at = 'nope'`)
	require.NoError(t, err)
	_, err = d.ListCommitsByTicket(ticket.ID)
	assert.Error(t, err)

	_, err = d.SQL().Exec(`UPDATE apps SET created_at = 'nope'`)
	require.NoError(t, err)
	_, err = d.ListApps()
	assert.Error(t, err)
	_, err = d.GetApp("notes")
	assert.Error(t, err)
}

func TestIsUniqueViolation(t *testing.T) {
	assert.False(t, isUniqueViolation(nil))
	assert.False(t, isUniqueViolation(assertError("disk full")))
	assert.True(t, isUniqueViolation(assertError("constraint failed: UNIQUE constraint failed: tickets.id (1555)")))
}

type assertError string

func (e assertError) Error() string { return string(e) }
