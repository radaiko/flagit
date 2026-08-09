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

// TicketIDChars is the length of the random part of a ticket ID. Six
// characters (36^6 ≈ 2.2 billion) matches the documented FLG-7X3K9Q shape.
const TicketIDChars = 6

// ticketIDAlphabet is uppercase-only on purpose, and that is a correctness
// requirement rather than a style choice.
//
// A ticket ID is the one thing a reporter has to carry away and hand back: it
// is printed on a tag, read aloud, and retyped into a field that is styled
// uppercase and asks the keyboard to capitalise. Lookups then land on
// `WHERE id = ?` against a TEXT primary key, which SQLite compares with its
// default BINARY collation — so an ID holding a lowercase character stops
// matching itself the moment it passes through a person or an uppercasing UI.
//
// Case is what makes the alphabet 36 rather than 62. 2.2 billion IDs is ample
// (CreateTicket retries past the rare collision), and an ID is not a
// credential: ownership is proved by the device token, so guessing an ID buys
// a 403 rather than a ticket.
const ticketIDAlphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// ticketIDPattern stays deliberately wider than ticketIDAlphabet. IDs issued
// before the alphabet was narrowed contain lowercase characters, and they are
// still valid tickets: tightening this to uppercase would reject them at the
// door instead of letting the lookup decide.
var ticketIDPattern = regexp.MustCompile(`^FLG-[0-9A-Za-z]{4,12}$`)

// GenerateTicketID returns a fresh, unvalidated ticket ID such as FLG-7X3K9Q.
// Uniqueness is enforced by the caller retrying on a primary-key collision.
func GenerateTicketID() (string, error) {
	var sb strings.Builder
	sb.WriteString(TicketIDPrefix)
	max := big.NewInt(int64(len(ticketIDAlphabet)))
	for i := 0; i < TicketIDChars; i++ {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}
		sb.WriteByte(ticketIDAlphabet[n.Int64()])
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

// HashAdminKey returns the hex SHA-256 of an admin key. Like device tokens,
// only the hash is persisted, so a leaked database does not hand over admin
// access. An empty key hashes to the empty string so "no key" can never match
// a stored hash.
func HashAdminKey(key string) string {
	return HashDeviceToken(key)
}

// GenerateAdminKey returns a random 32-byte hex string for admin API auth.
func GenerateAdminKey() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}
