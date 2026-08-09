package api

import (
	"net/http"
	"strings"
	"testing"

	"flagit/internal/model"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCreatedTicketIsReachableByItsOwnID walks the exact path a reporter takes:
// file a ticket, keep the ID that came back, then look that ID up again.
//
// Repeated, because the ID is random: a bug that depends on which characters
// were drawn shows up as a flake in a single run and as a certainty here.
func TestCreatedTicketIsReachableByItsOwnID(t *testing.T) {
	h := newHarness(t)

	for i := 0; i < 100; i++ {
		created := createTicket(t, h)
		require.NotEmpty(t, created.ID)

		got := do(t, h.public, http.MethodGet, "/api/tickets/"+created.ID, nil, deviceHeaders())
		require.Equal(t, http.StatusOK, got.Code,
			"ticket %s could not be fetched with the ID its own creation returned: %s",
			created.ID, got.Body.String())

		var view ticketWithMessages
		decodeData(t, got, &view)
		assert.Equal(t, created.ID, view.ID)
	}
}

// TestCreatedTicketIDSurvivesAnUppercasingClient is the regression for the
// production bug. Ticket IDs are meant to be read off a tag and typed back in:
// the overlay's lookup field is styled uppercase and asks the keyboard to
// capitalise, and the ID is normalised to uppercase before it is sent. The
// store compares IDs byte for byte, so an ID that is not already uppercase
// stops matching itself as soon as it makes that round trip.
func TestCreatedTicketIDSurvivesAnUppercasingClient(t *testing.T) {
	h := newHarness(t)

	for i := 0; i < 100; i++ {
		created := createTicket(t, h)
		typedBack := strings.ToUpper(created.ID)

		require.Equal(t, created.ID, typedBack,
			"a generated ID must already be uppercase, or a person retyping it gets a different string")

		got := do(t, h.public, http.MethodGet, "/api/tickets/"+typedBack, nil, deviceHeaders())
		assert.Equal(t, http.StatusOK, got.Code,
			"ticket %s is unreachable once a client uppercases it: %s", created.ID, got.Body.String())

		admin := do(t, h.internal, http.MethodGet, "/internal/tickets/"+typedBack, nil, adminHeaders())
		assert.Equal(t, http.StatusOK, admin.Code,
			"ticket %s is unreachable through the admin API once uppercased: %s",
			created.ID, admin.Body.String())
	}
}

// TestCreatedTicketAppearsInAdminListing covers the other half of the report:
// a ticket that was accepted must be visible to whoever triages it, with the
// same ID the reporter was given.
func TestCreatedTicketAppearsInAdminListing(t *testing.T) {
	h := newHarness(t)

	created := createTicket(t, h)

	rec := do(t, h.internal, http.MethodGet, "/internal/tickets", nil, adminHeaders())
	require.Equal(t, http.StatusOK, rec.Code)

	var page ticketPage
	decodeData(t, rec, &page)

	assert.Equal(t, 1, page.Total)
	require.Len(t, page.Tickets, 1)
	assert.Equal(t, created.ID, page.Tickets[0].ID)
	assert.Equal(t, created.Title, page.Tickets[0].Title)
}

// TestAdminListingSurvivesTheFrontendBeingMounted pins the production wiring.
// The unit harness leaves Overlay and Dashboard nil, so the SPA catch-all
// routes it registers alongside the API are never exercised; the deployed
// binary always has them.
func TestAdminListingSurvivesTheFrontendBeingMounted(t *testing.T) {
	h := newHarness(t)
	h.Server.Overlay = stubFrontend("overlay")
	h.Server.Dashboard = stubFrontend("dashboard")
	public, internal := h.Server.PublicRouter(), h.Server.InternalRouter()

	rec := do(t, public, http.MethodPost, "/api/tickets", createTicketBody(), nil)
	require.Equal(t, http.StatusCreated, rec.Code, "body: %s", rec.Body.String())
	var created model.Ticket
	decodeData(t, rec, &created)

	got := do(t, public, http.MethodGet, "/api/tickets/"+created.ID, nil, deviceHeaders())
	assert.Equal(t, http.StatusOK, got.Code, "body: %s", got.Body.String())

	one := do(t, internal, http.MethodGet, "/internal/tickets/"+created.ID, nil, adminHeaders())
	assert.Equal(t, http.StatusOK, one.Code, "body: %s", one.Body.String())

	list := do(t, internal, http.MethodGet, "/internal/tickets", nil, adminHeaders())
	require.Equal(t, http.StatusOK, list.Code)
	var page ticketPage
	decodeData(t, list, &page)
	require.Len(t, page.Tickets, 1)
	assert.Equal(t, created.ID, page.Tickets[0].ID)
}

// stubFrontend stands in for the embedded Svelte build. Anything that reaches
// it is a request the API routes failed to claim.
func stubFrontend(name string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<!doctype html><!-- " + name + " -->"))
	})
}
