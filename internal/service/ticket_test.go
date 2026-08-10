package service

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"flagit/internal/db"
	"flagit/internal/model"
	"flagit/internal/webhook"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const deviceToken = "3f6b1a4e-0000-4000-8000-000000000001"

// recorder captures webhook deliveries instead of making HTTP calls.
type recorder struct {
	mu       sync.Mutex
	urls     []string
	payloads []webhook.Payload
	err      error
}

func (r *recorder) Send(_ context.Context, url string, p webhook.Payload) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.urls = append(r.urls, url)
	r.payloads = append(r.payloads, p)
	return r.err
}

func (r *recorder) calls() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.payloads)
}

func newService(t *testing.T) (*Service, *recorder) {
	t.Helper()
	database, err := db.InitDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, database.Close()) })

	tick := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	database.Now = func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}

	rec := &recorder{}
	s := New(database, rec, "https://flagit.example", slog.New(slog.DiscardHandler))
	// Deliver synchronously so assertions do not race the dispatcher.
	s.Dispatch = func(fn func()) { fn() }
	return s, rec
}

func validInput() NewTicketInput {
	return NewTicketInput{
		Type:            model.TypeBug,
		Title:           "Crash on save",
		Body:            "Tapping save closes the app",
		DeviceToken:     deviceToken,
		AppName:         "notes",
		AppVersion:      "1.4.2",
		OS:              "iOS 18.2",
		Platform:        "ios",
		DeviceModel:     "iPhone 15",
		Logs:            "panic: nil map",
		LogsDurationMin: 5,
	}
}

// mustCreate creates a ticket and fails the test if it does not succeed.
func mustCreate(t *testing.T, s *Service, in NewTicketInput) *model.Ticket {
	t.Helper()
	ticket, err := s.CreateTicket(context.Background(), in)
	require.NoError(t, err)
	return ticket
}

func TestCreateTicket(t *testing.T) {
	s, _ := newService(t)

	ticket := mustCreate(t, s, validInput())

	assert.True(t, db.ValidTicketID(ticket.ID))
	assert.Equal(t, model.StatusOpen, ticket.Status)
	assert.Equal(t, model.TypeBug, ticket.Type)
	assert.Equal(t, "notes", ticket.AppName)
	assert.Equal(t, db.HashDeviceToken(deviceToken), ticket.DeviceTokenHash)
	assert.NotContains(t, ticket.DeviceTokenHash, deviceToken, "the raw token is never stored")

	// The app is registered on first sighting.
	apps, err := s.ListApps()
	require.NoError(t, err)
	require.Len(t, apps, 1)
	assert.Equal(t, "notes", apps[0].Name)
	assert.False(t, apps[0].AutoProcessEnabled, "auto-process defaults to off")
}

func TestCreateTicketTrimsAndDefaults(t *testing.T) {
	s, _ := newService(t)
	in := validInput()
	in.Title = "  Crash on save  "
	in.Body = "\n details \n"
	in.AppName = "   "
	in.LogsDurationMin = -3

	ticket := mustCreate(t, s, in)

	assert.Equal(t, "Crash on save", ticket.Title)
	assert.Equal(t, "details", ticket.Body)
	assert.Equal(t, UnknownAppName, ticket.AppName, "an unnamed app still groups somewhere")
	assert.Equal(t, 0, ticket.LogsDurationMin)
}

func TestCreateTicketTruncatesOverlongFields(t *testing.T) {
	s, _ := newService(t)
	in := validInput()
	in.Title = strings.Repeat("ä", MaxTitleLen+50)
	in.Body = strings.Repeat("b", MaxBodyLen+50)
	in.Logs = strings.Repeat("l", MaxLogsLen+50)

	ticket := mustCreate(t, s, in)

	assert.Equal(t, MaxTitleLen, len([]rune(ticket.Title)), "truncation counts runes, not bytes")
	assert.Equal(t, MaxBodyLen, len(ticket.Body))
	assert.Equal(t, MaxLogsLen, len(ticket.LogRingBuffer))
}

func TestCreateTicketValidation(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*NewTicketInput)
		wantMsg string
	}{
		{"unknown type", func(in *NewTicketInput) { in.Type = "question" }, "type must be"},
		{"empty type", func(in *NewTicketInput) { in.Type = "" }, "type must be"},
		{"missing title", func(in *NewTicketInput) { in.Title = "   " }, "title is required"},
		{"missing device token", func(in *NewTicketInput) { in.DeviceToken = "" }, "deviceToken is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, _ := newService(t)
			in := validInput()
			tt.mutate(&in)

			_, err := s.CreateTicket(context.Background(), in)

			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalid)
			assert.ErrorContains(t, err, tt.wantMsg)
		})
	}
}

func TestCreateTicketFeatureType(t *testing.T) {
	s, _ := newService(t)
	in := validInput()
	in.Type = model.TypeFeature

	ticket := mustCreate(t, s, in)

	assert.Equal(t, model.TypeFeature, ticket.Type)
}

func TestCreateTicketDBFailure(t *testing.T) {
	s, _ := newService(t)
	require.NoError(t, s.DB.Close())

	_, err := s.CreateTicket(context.Background(), validInput())

	assert.ErrorContains(t, err, "create ticket")
}

// -------------------------------------------------------- webhook firing --

func TestCreateTicketSkipsWebhookWhenAutoProcessOff(t *testing.T) {
	s, rec := newService(t)
	require.NoError(t, s.DB.SetSetting(model.SettingHermesWebhookURL, "https://hermes.example/hook"))

	mustCreate(t, s, validInput())

	assert.Zero(t, rec.calls(), "auto-process is off by default")
}

func TestCreateTicketFiresWebhookWhenAppEnabled(t *testing.T) {
	s, rec := newService(t)
	require.NoError(t, s.DB.SetSetting(model.SettingHermesWebhookURL, "https://hermes.example/hook"))
	require.NoError(t, s.DB.CreateApp(&model.App{Name: "notes", AutoProcessEnabled: true}))

	ticket := mustCreate(t, s, validInput())

	require.Equal(t, 1, rec.calls())
	assert.Equal(t, "https://hermes.example/hook", rec.urls[0])
	assert.Equal(t, ticket.ID, rec.payloads[0].TicketID)
	assert.Equal(t, "https://flagit.example/api/tickets/"+ticket.ID, rec.payloads[0].TicketURL)
}

func TestCreateTicketAppliesGlobalToggleToUnknownAppsOnly(t *testing.T) {
	s, rec := newService(t)
	require.NoError(t, s.DB.SetSetting(model.SettingHermesWebhookURL, "https://hermes.example/hook"))
	require.NoError(t, s.DB.SetBoolSetting(model.SettingGlobalAutoProcess, true))

	// First ticket from an unknown app inherits the global toggle.
	mustCreate(t, s, validInput())
	assert.Equal(t, 1, rec.calls())

	app, err := s.DB.GetApp("notes")
	require.NoError(t, err)
	assert.True(t, app.AutoProcessEnabled)

	// Turning the app off must survive the global toggle staying on.
	_, err = s.UpdateApp("notes", false)
	require.NoError(t, err)
	mustCreate(t, s, validInput())
	assert.Equal(t, 1, rec.calls(), "the per-app setting wins for known apps")
}

func TestCreateTicketSkipsWebhookWhenURLUnset(t *testing.T) {
	s, rec := newService(t)
	require.NoError(t, s.DB.CreateApp(&model.App{Name: "notes", AutoProcessEnabled: true}))

	mustCreate(t, s, validInput())

	assert.Zero(t, rec.calls(), "no Hermes configured yet")
}

func TestCreateTicketSucceedsWhenWebhookFails(t *testing.T) {
	s, rec := newService(t)
	rec.err = errors.New("hermes is down")
	require.NoError(t, s.DB.SetSetting(model.SettingHermesWebhookURL, "https://hermes.example/hook"))
	require.NoError(t, s.DB.CreateApp(&model.App{Name: "notes", AutoProcessEnabled: true}))

	ticket := mustCreate(t, s, validInput())

	assert.NotEmpty(t, ticket.ID, "the user still gets their ticket")
	assert.Equal(t, 1, rec.calls())
}

func TestCreateTicketWithoutWebhookSender(t *testing.T) {
	s, _ := newService(t)
	s.Webhook = nil
	require.NoError(t, s.DB.SetSetting(model.SettingHermesWebhookURL, "https://hermes.example/hook"))
	require.NoError(t, s.DB.CreateApp(&model.App{Name: "notes", AutoProcessEnabled: true}))

	assert.NotNil(t, mustCreate(t, s, validInput()))
}

func TestNotifyHermesWithNilDispatchRunsInline(t *testing.T) {
	s, rec := newService(t)
	s.Dispatch = nil
	require.NoError(t, s.DB.SetSetting(model.SettingHermesWebhookURL, "https://hermes.example/hook"))
	require.NoError(t, s.DB.CreateApp(&model.App{Name: "notes", AutoProcessEnabled: true}))

	mustCreate(t, s, validInput())

	assert.Equal(t, 1, rec.calls())
}

func TestNewUsesAsyncDispatchByDefault(t *testing.T) {
	database, err := db.InitDB(":memory:")
	require.NoError(t, err)
	defer database.Close()
	require.NoError(t, database.SetSetting(model.SettingHermesWebhookURL, "https://hermes.example/hook"))
	require.NoError(t, database.CreateApp(&model.App{Name: "notes", AutoProcessEnabled: true}))

	done := make(chan struct{})
	s := New(database, senderFunc(func(context.Context, string, webhook.Payload) error {
		close(done)
		return nil
	}), "", slog.New(slog.DiscardHandler))
	require.NotNil(t, s.Dispatch)

	mustCreate(t, s, validInput())

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the default dispatcher never delivered the webhook")
	}
}

func TestNewDefaultsLogger(t *testing.T) {
	s := New(nil, nil, "", nil)

	assert.NotNil(t, s.Logger)
	assert.NotNil(t, s.logger())
}

type senderFunc func(context.Context, string, webhook.Payload) error

func (f senderFunc) Send(ctx context.Context, url string, p webhook.Payload) error {
	return f(ctx, url, p)
}

// ------------------------------------------------------------- ownership --

func TestGetTicketRequiresMatchingToken(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	got, err := s.GetTicket(ticket.ID, deviceToken)
	require.NoError(t, err)
	assert.Equal(t, ticket.ID, got.ID)

	_, err = s.GetTicket(ticket.ID, "someone-elses-token")
	assert.ErrorIs(t, err, ErrForbidden)

	_, err = s.GetTicket(ticket.ID, "")
	assert.ErrorIs(t, err, ErrForbidden, "an empty token must never match")
}

func TestGetTicketUnknownIDsLookIdentical(t *testing.T) {
	s, _ := newService(t)

	// Malformed and merely-absent IDs both report not-found, so a caller
	// cannot probe for the existence of other people's tickets.
	_, err := s.GetTicket("not-a-ticket", deviceToken)
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = s.GetTicket("FLG-ZZZZZZ", deviceToken)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetTicketByIDSkipsOwnershipCheck(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	got, err := s.GetTicketByID(" " + ticket.ID + " ")
	require.NoError(t, err)
	assert.Equal(t, ticket.ID, got.ID)
}

// -------------------------------------------------------------- messages --

func TestPostUserMessage(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	m, err := s.PostUserMessage(ticket.ID, deviceToken, "  Still broken  ")
	require.NoError(t, err)
	assert.Equal(t, model.RoleUser, m.Role)
	assert.Equal(t, "Still broken", m.Body)

	messages, err := s.ListMessages(ticket.ID, deviceToken)
	require.NoError(t, err)
	require.Len(t, messages, 1)
}

func TestPostUserMessageRejectsWrongToken(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	_, err := s.PostUserMessage(ticket.ID, "wrong", "hello")
	assert.ErrorIs(t, err, ErrForbidden)

	_, err = s.ListMessages(ticket.ID, "wrong")
	assert.ErrorIs(t, err, ErrForbidden)
}

func TestPostMessageRejectsEmptyBody(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	_, err := s.PostUserMessage(ticket.ID, deviceToken, "   ")
	assert.ErrorIs(t, err, ErrInvalid)

	_, err = s.PostAgentMessage(ticket.ID, "")
	assert.ErrorIs(t, err, ErrInvalid)
}

func TestPostMessageTruncates(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	m, err := s.PostAgentMessage(ticket.ID, strings.Repeat("x", MaxMessageLen+10))
	require.NoError(t, err)
	assert.Len(t, m.Body, MaxMessageLen)
}

func TestPostAgentMessage(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	m, err := s.PostAgentMessage(ticket.ID, "On it")
	require.NoError(t, err)
	assert.Equal(t, model.RoleAgent, m.Role)

	messages, err := s.ListMessagesByID(ticket.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, model.RoleAgent, messages[0].Role)
}

func TestMessagesOnUnknownTicket(t *testing.T) {
	s, _ := newService(t)

	_, err := s.PostAgentMessage("FLG-ZZZZZZ", "hi")
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = s.PostUserMessage("FLG-ZZZZZZ", deviceToken, "hi")
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = s.ListMessagesByID("FLG-ZZZZZZ")
	assert.ErrorIs(t, err, ErrNotFound)
	_, err = s.ListMessages("FLG-ZZZZZZ", deviceToken)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestPostMessageDBFailure(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())
	require.NoError(t, s.DB.Close())

	_, err := s.PostAgentMessage(ticket.ID, "hi")
	assert.Error(t, err)
}

// ---------------------------------------------------------------- status --

func TestUpdateStatus(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	updated, err := s.UpdateStatus(ticket.ID, model.StatusInProgress, nil)
	require.NoError(t, err)
	assert.Equal(t, model.StatusInProgress, updated.Status)
	assert.True(t, updated.UpdatedAt.After(ticket.CreatedAt))
}

func TestUpdateStatusAllowsAnyTransition(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	// Admins may jump the workflow in either direction.
	_, err := s.UpdateStatus(ticket.ID, model.StatusClosed, nil)
	require.NoError(t, err)
	reopened, err := s.UpdateStatus(ticket.ID, model.StatusOpen, nil)
	require.NoError(t, err)
	assert.Equal(t, model.StatusOpen, reopened.Status)
}

func TestUpdateStatusRecordsShippedVersion(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	// Walk the documented workflow: open → in-progress → resolved → shipped.
	_, err := s.UpdateStatus(ticket.ID, model.StatusInProgress, nil)
	require.NoError(t, err)
	_, err = s.UpdateStatus(ticket.ID, model.StatusResolved, nil)
	require.NoError(t, err)

	updated, err := s.UpdateStatus(ticket.ID, model.StatusShipped, ptr("  1.5.0 "))
	require.NoError(t, err)
	assert.Equal(t, "1.5.0", updated.ShippedVersion)
}

func TestUpdateStatusEnforcesTheWorkflow(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	// open → shipped skips two stages, which is almost always a mistake.
	_, err := s.UpdateStatus(ticket.ID, model.StatusShipped, nil)
	require.ErrorIs(t, err, ErrInvalid)
	assert.ErrorContains(t, err, "cannot move a ticket")
	assert.ErrorContains(t, err, "force")

	still, err := s.GetTicketByID(ticket.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusOpen, still.Status, "a rejected move changes nothing")
}

// Declining is a decision, so it travels the same way any other status change
// does: along the workflow, with the reason recorded in the same transaction.
func TestUpdateStatusDeclinesAnOpenTicketWithAReason(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	updated, err := s.UpdateStatusWithComment(
		ticket.ID, model.StatusDeclined, nil, "Out of scope for this app.")

	require.NoError(t, err)
	assert.Equal(t, model.StatusDeclined, updated.Status)

	messages, err := s.ListMessagesByID(ticket.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "Out of scope for this app.", messages[0].Body)
}

func TestDeclinedTicketCanBeReconsidered(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())
	_, err := s.UpdateStatus(ticket.ID, model.StatusDeclined, nil)
	require.NoError(t, err)

	reopened, err := s.UpdateStatus(ticket.ID, model.StatusOpen, nil)

	require.NoError(t, err)
	assert.Equal(t, model.StatusOpen, reopened.Status)
}

// Work that is already done cannot be turned down; that is a slip, and the
// workflow catches it the way it catches any other skipped stage.
func TestDecliningShippedWorkNeedsForce(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())
	_, err := s.ApplyUpdate(ticket.ID, StatusUpdate{Status: ptr(model.StatusShipped), Force: true})
	require.NoError(t, err)

	_, err = s.UpdateStatus(ticket.ID, model.StatusDeclined, nil)

	require.ErrorIs(t, err, ErrInvalid)
	still, err := s.GetTicketByID(ticket.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusShipped, still.Status)
}

func TestBatchUpdateStatusDeclinesManyAtOnce(t *testing.T) {
	s, _ := newService(t)
	first := mustCreate(t, s, validInput())
	second := mustCreate(t, s, validInput())

	result, err := s.BatchUpdateStatus(
		[]string{first.ID, second.ID}, model.StatusDeclined, "", false)

	require.NoError(t, err)
	assert.ElementsMatch(t, []string{first.ID, second.ID}, result.Updated)
	assert.Empty(t, result.Failed)
	for _, id := range []string{first.ID, second.ID} {
		got, err := s.GetTicketByID(id)
		require.NoError(t, err)
		assert.Equal(t, model.StatusDeclined, got.Status)
	}
}

func TestForceBypassesTheWorkflow(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	updated, err := s.ApplyUpdate(ticket.ID, StatusUpdate{
		Status: ptr(model.StatusShipped),
		Force:  true,
	})

	require.NoError(t, err)
	assert.Equal(t, model.StatusShipped, updated.Status)
}

func TestApplyUpdateAcceptsACommentWithoutAStatus(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	updated, err := s.ApplyUpdate(ticket.ID, StatusUpdate{Comment: "Still looking into this"})

	require.NoError(t, err)
	assert.Equal(t, model.StatusOpen, updated.Status, "the status is left alone")

	messages, err := s.ListMessagesByID(ticket.ID)
	require.NoError(t, err)
	require.Len(t, messages, 1)
	assert.Equal(t, "Still looking into this", messages[0].Body)
	assert.Equal(t, model.RoleAgent, messages[0].Role)
}

func TestApplyUpdateRejectsAnEmptyUpdate(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	// Neither a status nor a comment: accepting this would let a client
	// believe it had done something.
	_, err := s.ApplyUpdate(ticket.ID, StatusUpdate{})

	assert.ErrorIs(t, err, ErrInvalid)
	assert.ErrorContains(t, err, "provide a status, a comment, or both")
}

func TestApplyUpdateAllowsRestatingTheCurrentStatus(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	updated, err := s.ApplyUpdate(ticket.ID, StatusUpdate{Status: ptr(model.StatusOpen)})

	require.NoError(t, err, "re-saving without a change is not an error")
	assert.Equal(t, model.StatusOpen, updated.Status)
}

func TestUpdateStatusErrors(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	_, err := s.UpdateStatus(ticket.ID, "done", nil)
	assert.ErrorIs(t, err, ErrInvalid)

	_, err = s.UpdateStatus("FLG-ZZZZZZ", model.StatusClosed, nil)
	assert.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, s.DB.Close())
	_, err = s.UpdateStatus(ticket.ID, model.StatusClosed, nil)
	assert.Error(t, err)
}

func TestBatchUpdateStatus(t *testing.T) {
	s, _ := newService(t)
	first := mustCreate(t, s, validInput())
	second := mustCreate(t, s, validInput())

	result, err := s.BatchUpdateStatus([]string{first.ID, " " + second.ID + " "}, model.StatusShipped, "1.5.0", true)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{first.ID, second.ID}, result.Updated)
	assert.Empty(t, result.Failed)

	got, err := s.GetTicketByID(second.ID)
	require.NoError(t, err)
	assert.Equal(t, model.StatusShipped, got.Status)
	assert.Equal(t, "1.5.0", got.ShippedVersion)
}

func TestBatchUpdateStatusReportsPerTicketFailures(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	result, err := s.BatchUpdateStatus([]string{ticket.ID, "FLG-ZZZZZZ"}, model.StatusClosed, "", true)

	require.NoError(t, err, "one stale id must not fail the whole sweep")
	assert.Equal(t, []string{ticket.ID}, result.Updated)
	assert.Equal(t, ReasonNotFound, result.Failed["FLG-ZZZZZZ"],
		"a stable reason code, not a raw error string")
}

func TestBatchUpdateStatusValidation(t *testing.T) {
	s, _ := newService(t)

	_, err := s.BatchUpdateStatus([]string{"FLG-ABC123"}, "nonsense", "", true)
	assert.ErrorIs(t, err, ErrInvalid)

	_, err = s.BatchUpdateStatus(nil, model.StatusClosed, "", true)
	assert.ErrorIs(t, err, ErrInvalid)
}

// --------------------------------------------------------------- commits --

func TestRecordAndListCommits(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	c, err := s.RecordCommit(ticket.ID, " a1b2c3d ", " fix/crash ", " fix: guard nil map ")
	require.NoError(t, err)
	assert.Equal(t, "a1b2c3d", c.CommitHash)
	assert.Equal(t, "fix/crash", c.Branch)
	assert.Equal(t, "fix: guard nil map", c.Message)

	commits, err := s.ListCommits(ticket.ID)
	require.NoError(t, err)
	require.Len(t, commits, 1)
}

func TestRecordCommitErrors(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	_, err := s.RecordCommit(ticket.ID, "  ", "main", "msg")
	assert.ErrorIs(t, err, ErrInvalid)

	_, err = s.RecordCommit("FLG-ZZZZZZ", "abc", "main", "msg")
	assert.ErrorIs(t, err, ErrNotFound)

	_, err = s.ListCommits("FLG-ZZZZZZ")
	assert.ErrorIs(t, err, ErrNotFound)

	require.NoError(t, s.DB.Close())
	_, err = s.RecordCommit(ticket.ID, "abc", "main", "msg")
	assert.Error(t, err)
}

// -------------------------------------------------------- listing/polling --

func TestListAndPollTickets(t *testing.T) {
	s, _ := newService(t)
	first := mustCreate(t, s, validInput())
	cutoff := first.CreatedAt
	second := mustCreate(t, s, validInput())

	all, err := s.ListTickets(db.TicketFilter{})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	changed, err := s.PollTickets(cutoff, nil)
	require.NoError(t, err)
	require.Len(t, changed, 1)
	assert.Equal(t, second.ID, changed[0].ID)
}

// ------------------------------------------------------- apps & settings --

func TestUpdateApp(t *testing.T) {
	s, _ := newService(t)
	mustCreate(t, s, validInput())

	app, err := s.UpdateApp("notes", true)
	require.NoError(t, err)
	assert.True(t, app.AutoProcessEnabled)
}

func TestUpdateAppErrors(t *testing.T) {
	s, _ := newService(t)

	_, err := s.UpdateApp("  ", true)
	assert.ErrorIs(t, err, ErrInvalid)

	_, err = s.UpdateApp("ghost", true)
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestSettingsRoundTrip(t *testing.T) {
	s, _ := newService(t)

	settings, err := s.GetSettings()
	require.NoError(t, err)
	assert.False(t, settings.GlobalAutoProcess)
	assert.Empty(t, settings.HermesWebhookURL)

	on := true
	url := "https://hermes.example/hook"
	settings, err = s.UpdateSettings(&on, &url)
	require.NoError(t, err)
	assert.True(t, settings.GlobalAutoProcess)
	assert.Equal(t, url, settings.HermesWebhookURL)

	// A nil field is left untouched.
	off := false
	settings, err = s.UpdateSettings(&off, nil)
	require.NoError(t, err)
	assert.False(t, settings.GlobalAutoProcess)
	assert.Equal(t, url, settings.HermesWebhookURL)

	// Clearing the webhook disables Hermes delivery.
	empty := "  "
	settings, err = s.UpdateSettings(nil, &empty)
	require.NoError(t, err)
	assert.Empty(t, settings.HermesWebhookURL)
}

func TestUpdateSettingsRejectsNonHTTPWebhook(t *testing.T) {
	s, _ := newService(t)
	bad := "hermes.example/hook"

	_, err := s.UpdateSettings(nil, &bad)

	assert.ErrorIs(t, err, ErrInvalid)
}

func TestSettingsDBFailures(t *testing.T) {
	s, _ := newService(t)
	require.NoError(t, s.DB.Close())

	_, err := s.GetSettings()
	assert.Error(t, err)

	on := true
	_, err = s.UpdateSettings(&on, nil)
	assert.Error(t, err)

	url := "https://hermes.example/hook"
	_, err = s.UpdateSettings(nil, &url)
	assert.Error(t, err)

	_, err = s.ListApps()
	assert.Error(t, err)
	_, err = s.ListTickets(db.TicketFilter{})
	assert.Error(t, err)
	_, err = s.PollTickets(time.Time{}, nil)
	assert.Error(t, err)
}

func TestRegisterAppFailureDoesNotLoseTheTicket(t *testing.T) {
	s, _ := newService(t)
	// Drop the apps table: registration fails, but the ticket must survive.
	_, err := s.DB.SQL().Exec(`DROP TABLE apps`)
	require.NoError(t, err)

	ticket, err := s.CreateTicket(context.Background(), validInput())

	require.NoError(t, err)
	assert.True(t, db.ValidTicketID(ticket.ID))
}

func TestRegisterAppSettingsFailure(t *testing.T) {
	s, _ := newService(t)
	_, err := s.DB.SQL().Exec(`DROP TABLE settings`)
	require.NoError(t, err)

	_, err = s.CreateTicket(context.Background(), validInput())

	assert.NoError(t, err, "an unreadable settings table only costs auto-processing")
}

func TestNotifyHermesSettingsFailure(t *testing.T) {
	s, rec := newService(t)
	require.NoError(t, s.DB.CreateApp(&model.App{Name: "notes", AutoProcessEnabled: true}))

	// A ticket is created, then the settings lookup inside notifyHermes fails.
	ticket := mustCreate(t, s, validInput())
	_, err := s.DB.SQL().Exec(`DROP TABLE settings`)
	require.NoError(t, err)

	s.notifyHermes(ticket)

	assert.Zero(t, rec.calls())
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "abc", truncate("abc", 5))
	assert.Equal(t, "abc", truncate("abc", 3))
	assert.Equal(t, "ab", truncate("abc", 2))
	assert.Equal(t, "", truncate("abc", 0))
	assert.Equal(t, "äö", truncate("äöü", 2), "runes, not bytes")
}

// ptr returns a pointer to v, for the optional-field arguments that
// distinguish "not specified" from "explicitly set".
func ptr[T any](v T) *T { return &v }

func TestValidateWebhookURL(t *testing.T) {
	valid := []string{
		"https://hermes.example/hook",
		"http://hermes.example:9000/hook",
		"https://203.0.113.10/hook",
	}
	for _, url := range valid {
		t.Run("accepts "+url, func(t *testing.T) {
			assert.NoError(t, ValidateWebhookURL(url))
		})
	}

	// Flagit fetches this URL from inside its own network, so an address that
	// only resolves there is a server-side request forgery target.
	invalid := []string{
		"ftp://hermes.example/hook",
		"file:///etc/passwd",
		"hermes.example/hook",
		"https://",
		"http://127.0.0.1:9000/hook",
		"http://localhost:9000/hook",
		"http://app.localhost/hook",
		"http://10.0.0.5/hook",
		"http://172.16.0.5/hook",
		"http://172.31.255.254/hook",
		"http://192.168.1.5/hook",
		"http://169.254.169.254/latest/meta-data",
		"http://0.0.0.0/hook",
		"http://100.101.102.103/hook",
		"http://[::1]/hook",
	}
	for _, url := range invalid {
		t.Run("rejects "+url, func(t *testing.T) {
			err := ValidateWebhookURL(url)
			require.Error(t, err)
			assert.ErrorIs(t, err, ErrInvalid)
		})
	}
}

func TestValidateWebhookURLAllowsPublicIPsNearPrivateRanges(t *testing.T) {
	// 172.15 and 172.32 sit just outside 172.16.0.0/12, and must not be caught
	// by an over-broad prefix check.
	assert.NoError(t, ValidateWebhookURL("http://172.15.0.1/hook"))
	assert.NoError(t, ValidateWebhookURL("http://172.32.0.1/hook"))
	assert.NoError(t, ValidateWebhookURL("http://11.0.0.1/hook"))
}

func TestUpdateSettingsRejectsAnSSRFWebhook(t *testing.T) {
	s, _ := newService(t)
	internal := "http://169.254.169.254/latest/meta-data"

	_, err := s.UpdateSettings(nil, &internal)

	require.ErrorIs(t, err, ErrInvalid)
	assert.ErrorContains(t, err, "private or loopback")

	// Nothing was stored, so a rejected URL cannot be used later.
	settings, err := s.GetSettings()
	require.NoError(t, err)
	assert.Empty(t, settings.HermesWebhookURL)
}

func TestBatchFailureReason(t *testing.T) {
	assert.Equal(t, ReasonNotFound, batchFailureReason(ErrNotFound))
	assert.Equal(t, ReasonInvalidTransition, batchFailureReason(ErrInvalid))
	assert.Equal(t, ReasonInternalError, batchFailureReason(errors.New("disk full")))
}

func TestBatchReportsAnInvalidTransitionAsSuch(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	// Without force, open → shipped is refused per ticket rather than failing
	// the whole sweep.
	result, err := s.BatchUpdateStatus([]string{ticket.ID}, model.StatusShipped, "1.5.0", false)

	require.NoError(t, err)
	assert.Empty(t, result.Updated)
	assert.Equal(t, ReasonInvalidTransition, result.Failed[ticket.ID])
}

func TestBatchFailureNeverLeaksRawErrors(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())
	require.NoError(t, s.DB.Close())

	result, err := s.BatchUpdateStatus([]string{ticket.ID}, model.StatusClosed, "", true)

	require.NoError(t, err)
	assert.Equal(t, ReasonInternalError, result.Failed[ticket.ID])
	assert.NotContains(t, result.Failed[ticket.ID], "sql")
	assert.NotContains(t, result.Failed[ticket.ID], "database")
}

func TestAssertOwnerRejectsAMismatchedHash(t *testing.T) {
	s, _ := newService(t)
	ticket := mustCreate(t, s, validInput())

	assert.NoError(t, s.assertOwner(ticket, deviceToken))
	assert.ErrorIs(t, s.assertOwner(ticket, "wrong-token"), ErrForbidden)
	assert.ErrorIs(t, s.assertOwner(ticket, ""), ErrForbidden)
	// The stored hash is not itself a credential.
	assert.ErrorIs(t, s.assertOwner(ticket, ticket.DeviceTokenHash), ErrForbidden)
}
