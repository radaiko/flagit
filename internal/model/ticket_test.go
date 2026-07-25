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
