package model

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTicketTypeValid(t *testing.T) {
	tests := []struct {
		in   TicketType
		want bool
	}{
		{TypeBug, true},
		{TypeFeature, true},
		{TicketType("question"), false},
		{TicketType(""), false},
		{TicketType("BUG"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.in), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.in.Valid())
		})
	}
}

func TestStatusValid(t *testing.T) {
	for _, s := range Statuses {
		assert.True(t, s.Valid(), "%s should be valid", s)
	}
	assert.False(t, Status("done").Valid())
	assert.False(t, Status("").Valid())
	assert.Len(t, Statuses, 5)
}

func TestRoleValid(t *testing.T) {
	assert.True(t, RoleUser.Valid())
	assert.True(t, RoleAgent.Valid())
	assert.False(t, Role("admin").Valid())
	assert.False(t, Role("").Valid())
}

func TestTicketJSONHidesDeviceTokenHash(t *testing.T) {
	raw, err := json.Marshal(Ticket{ID: "FLG-ABC123", DeviceTokenHash: "supersecret"})
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "supersecret")
	assert.Contains(t, string(raw), "FLG-ABC123")
}

func TestStatusCanTransitionTo(t *testing.T) {
	tests := []struct {
		from, to Status
		want     bool
	}{
		// The documented workflow.
		{StatusOpen, StatusInProgress, true},
		{StatusInProgress, StatusResolved, true},
		{StatusResolved, StatusShipped, true},
		{StatusShipped, StatusClosed, true},

		// Stepping back one stage: work reopens, a release is rolled back.
		{StatusInProgress, StatusOpen, true},
		{StatusResolved, StatusInProgress, true},
		{StatusShipped, StatusResolved, true},

		// A ticket can be dismissed at any point, and reopened after closing.
		{StatusOpen, StatusClosed, true},
		{StatusInProgress, StatusClosed, true},
		{StatusClosed, StatusOpen, true},
		{StatusClosed, StatusInProgress, true},

		// Restating the current status is a no-op, not an error.
		{StatusOpen, StatusOpen, true},
		{StatusShipped, StatusShipped, true},

		// Skipping stages is almost always a mistake.
		{StatusOpen, StatusShipped, false},
		{StatusInProgress, StatusShipped, false},
		{StatusOpen, StatusResolved, true},
		{StatusClosed, StatusShipped, false},
		{StatusClosed, StatusResolved, false},

		// Unknown statuses never transition.
		{Status("nonsense"), StatusOpen, false},
		{StatusOpen, Status("nonsense"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.from)+"->"+string(tt.to), func(t *testing.T) {
			assert.Equal(t, tt.want, tt.from.CanTransitionTo(tt.to))
		})
	}
}

func TestNextStatuses(t *testing.T) {
	next := StatusOpen.NextStatuses()

	assert.Contains(t, next, StatusOpen, "staying put is always offered")
	assert.Contains(t, next, StatusInProgress)
	assert.NotContains(t, next, StatusShipped)

	// Everything offered must actually be a legal move.
	for _, status := range Statuses {
		for _, candidate := range status.NextStatuses() {
			assert.True(t, status.CanTransitionTo(candidate),
				"%s offers %s but rejects it", status, candidate)
		}
	}

	assert.Nil(t, Status("nonsense").NextStatuses())
}
