package version

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// setBuildCommit swaps the build-time value for one test and puts it back.
func setBuildCommit(t *testing.T, value string) {
	t.Helper()
	previous := Commit
	Commit = value
	t.Cleanup(func() { Commit = previous })
}

func TestResolvePrefersTheRuntimeOverride(t *testing.T) {
	setBuildCommit(t, "1111111111111111111111111111111111111111")

	assert.Equal(t, "2222222222222222222222222222222222222222",
		Resolve("2222222222222222222222222222222222222222"))
}

func TestResolveFallsBackToTheBuildTimeValue(t *testing.T) {
	setBuildCommit(t, "212b0004b9b1a0f0f0f0f0f0f0f0f0f0f0f0f0f0")

	assert.Equal(t, "212b0004b9b1a0f0f0f0f0f0f0f0f0f0f0f0f0f0", Resolve(""))
}

func TestResolveIgnoresBlankValues(t *testing.T) {
	setBuildCommit(t, "  ")

	// An unset Coolify variable arrives as an empty string, not as an absent
	// one, so whitespace has to count as "nothing was provided".
	assert.Equal(t, Unknown, Resolve("   "))
}

func TestResolveIsUnknownWithoutAnySource(t *testing.T) {
	setBuildCommit(t, "")

	assert.Equal(t, Unknown, Resolve(""))
}

func TestResolveTrimsSurroundingWhitespace(t *testing.T) {
	setBuildCommit(t, "")

	assert.Equal(t, "abcdef1234567890", Resolve("  abcdef1234567890\n"))
}

func TestResolveIsIdempotent(t *testing.T) {
	setBuildCommit(t, "")

	// main resolves once and the handler resolves again; the second pass must
	// not turn a real commit into something else.
	assert.Equal(t, Unknown, Resolve(Resolve("")))
	assert.Equal(t, "abc1234", Resolve(Resolve("abc1234")))
}

func TestShortTruncatesAFullSHA(t *testing.T) {
	assert.Equal(t, "212b000", Short("212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa"))
}

func TestShortLeavesAnAlreadyShortValueAlone(t *testing.T) {
	assert.Equal(t, "212b00", Short("212b00"))
}

func TestShortLeavesUnknownReadable(t *testing.T) {
	assert.Equal(t, Unknown, Short(Unknown))
	assert.Equal(t, Unknown, Short(""))
}

func TestShortDoesNotTruncateANonSHALabel(t *testing.T) {
	// A tag or a branch name is more useful whole than cut to seven characters.
	assert.Equal(t, "v1.4.2-dirty", Short("v1.4.2-dirty"))
}

func TestKnown(t *testing.T) {
	assert.True(t, Known("212b000f1e2d3c4b5a69788796a5b4c3d2e1f0aa"))
	assert.False(t, Known(Unknown))
	assert.False(t, Known(""))
}
