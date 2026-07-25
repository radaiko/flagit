package db

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateTicketIDShape(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 200; i++ {
		id, err := GenerateTicketID()
		require.NoError(t, err)
		assert.True(t, strings.HasPrefix(id, TicketIDPrefix), "%s lacks the FLG- prefix", id)
		assert.Len(t, id, len(TicketIDPrefix)+TicketIDChars)
		assert.True(t, ValidTicketID(id), "%s should pass validation", id)
		assert.False(t, seen[id], "generated %s twice in 200 draws", id)
		seen[id] = true
	}
}

func TestGenerateTicketIDUsesBase62Only(t *testing.T) {
	for i := 0; i < 50; i++ {
		id, err := GenerateTicketID()
		require.NoError(t, err)
		for _, c := range id[len(TicketIDPrefix):] {
			assert.True(t, strings.ContainsRune(base62, c), "%q is not base62", c)
		}
	}
}

func TestValidTicketID(t *testing.T) {
	tests := []struct {
		id   string
		want bool
	}{
		{"FLG-7X3K9Q", true},
		{"FLG-abc123", true},
		{"FLG-ABCD", true},
		{"flg-7X3K9Q", false},
		{"FLG-", false},
		{"FLG-abc", false},
		{"7X3K9Q", false},
		{"FLG-7X3K9Q-extra", false},
		{"FLG-7X3K9!", false},
		{"", false},
		{"FLG-0123456789abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.want, ValidTicketID(tt.id))
		})
	}
}

func TestHashDeviceToken(t *testing.T) {
	token := "3f6b1a4e-0000-4000-8000-000000000001"
	want := sha256.Sum256([]byte(token))

	got := HashDeviceToken(token)
	assert.Equal(t, hex.EncodeToString(want[:]), got)
	assert.Len(t, got, 64)

	// Deterministic, whitespace-insensitive, and never leaks the raw token.
	assert.Equal(t, got, HashDeviceToken("  "+token+"\n"))
	assert.NotContains(t, got, token)

	assert.NotEqual(t, got, HashDeviceToken("other-token"))
}

func TestHashDeviceTokenEmptyNeverMatches(t *testing.T) {
	assert.Equal(t, "", HashDeviceToken(""))
	assert.Equal(t, "", HashDeviceToken("   "))
}

func TestGenerateAdminKey(t *testing.T) {
	a, err := GenerateAdminKey()
	require.NoError(t, err)
	b, err := GenerateAdminKey()
	require.NoError(t, err)

	assert.Len(t, a, 64)
	assert.NotEqual(t, a, b)
	_, err = hex.DecodeString(a)
	assert.NoError(t, err)
}
