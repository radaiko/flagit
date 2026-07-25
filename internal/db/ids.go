package db

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"regexp"
	"strings"
)

// TicketIDPrefix precedes every generated ticket ID.
const TicketIDPrefix = "FLG-"

// TicketIDChars is the length of the random part of a ticket ID. Six base62
// characters (62^6 ≈ 56.8 billion) matches the documented FLG-7X3K9Q shape.
const TicketIDChars = 6

const base62 = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

var ticketIDPattern = regexp.MustCompile(`^FLG-[0-9A-Za-z]{4,12}$`)

// GenerateTicketID returns a fresh, unvalidated ticket ID such as FLG-7X3K9Q.
// Uniqueness is enforced by the caller retrying on a primary-key collision.
func GenerateTicketID() (string, error) {
	var sb strings.Builder
	sb.WriteString(TicketIDPrefix)
	max := big.NewInt(int64(len(base62)))
	for i := 0; i < TicketIDChars; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(base62[n.Int64()])
	}
	return sb.String(), nil
}

// ValidTicketID reports whether id looks like a Flagit ticket ID. It is a
// cheap guard against nonsense path parameters, not an existence check.
func ValidTicketID(id string) bool {
	return ticketIDPattern.MatchString(id)
}

// HashDeviceToken returns the hex SHA-256 of a raw device token. Only the hash
// is ever persisted. An empty token hashes to the empty string so that
// "no token" can never match a stored hash.
func HashDeviceToken(token string) string {
	token = strings.TrimSpace(token)
	if token == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// GenerateAdminKey returns a random 32-byte hex string for admin API auth.
func GenerateAdminKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
