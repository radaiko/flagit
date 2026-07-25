package db

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestDB opens a fresh in-memory database with a deterministic clock that
// advances one second per call, so ordering assertions are stable.
func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := InitDB(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, d.Close()) })

	tick := time.Date(2026, 7, 25, 12, 0, 0, 0, time.UTC)
	d.Now = func() time.Time {
		tick = tick.Add(time.Second)
		return tick
	}
	return d
}

func TestInitDBInMemory(t *testing.T) {
	d := newTestDB(t)

	version, err := d.SchemaVersion()
	require.NoError(t, err)
	assert.Equal(t, len(migrations), version)
	assert.NotNil(t, d.SQL())
}

func TestInitDBCreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "flagit.db")

	d, err := InitDB(path)
	require.NoError(t, err)
	defer d.Close()

	assert.FileExists(t, path)
}

func TestInitDBRejectsUnwritablePath(t *testing.T) {
	// A file where a directory needs to be: MkdirAll cannot succeed.
	file := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(file, nil, 0o600))

	_, err := InitDB(filepath.Join(file, "flagit.db"))
	assert.Error(t, err)
}

func TestRunMigrationsIsIdempotent(t *testing.T) {
	d := newTestDB(t)

	require.NoError(t, d.RunMigrations())
	require.NoError(t, d.RunMigrations())

	version, err := d.SchemaVersion()
	require.NoError(t, err)
	assert.Equal(t, len(migrations), version)
}

func TestRunMigrationsFailsOnClosedDB(t *testing.T) {
	d, err := InitDB(":memory:")
	require.NoError(t, err)
	require.NoError(t, d.Close())

	assert.Error(t, d.RunMigrations())
	_, err = d.SchemaVersion()
	assert.Error(t, err)
}

func TestFormatAndParseTime(t *testing.T) {
	// Deliberately a whole number of milliseconds: the layout is fixed-width,
	// so the round trip must be exact.
	in := time.Date(2026, 7, 25, 9, 30, 15, 123000000, time.UTC)

	s := FormatTime(in)
	assert.Equal(t, "2026-07-25T09:30:15.123000000Z", s)

	out, err := ParseTime(s)
	require.NoError(t, err)
	assert.True(t, in.Equal(out))
}

func TestFormatTimeNormalisesToUTC(t *testing.T) {
	zone := time.FixedZone("CEST", 2*60*60)
	local := time.Date(2026, 7, 25, 11, 0, 0, 0, zone)

	assert.Equal(t, "2026-07-25T09:00:00.000000000Z", FormatTime(local))
}

func TestFormatTimeIsLexicographicallySortable(t *testing.T) {
	// The reason for the custom layout: RFC3339Nano trims trailing zeros and
	// would sort ".2" after ".19".
	earlier := FormatTime(time.Date(2026, 1, 1, 0, 0, 0, 190000000, time.UTC))
	later := FormatTime(time.Date(2026, 1, 1, 0, 0, 0, 200000000, time.UTC))

	assert.Less(t, earlier, later)
}

func TestParseFlexibleTime(t *testing.T) {
	want := time.Date(2026, 7, 25, 9, 30, 15, 0, time.UTC)
	tests := []string{
		"2026-07-25T09:30:15.000000000Z",
		"2026-07-25T09:30:15Z",
		"2026-07-25T09:30:15",
		" 2026-07-25T09:30:15Z ",
		"2026-07-25T11:30:15+02:00",
	}
	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			got, err := ParseFlexibleTime(in)
			require.NoError(t, err)
			assert.True(t, want.Equal(got), "got %s", got)
		})
	}

	dateOnly, err := ParseFlexibleTime("2026-07-25")
	require.NoError(t, err)
	assert.Equal(t, time.Date(2026, 7, 25, 0, 0, 0, 0, time.UTC), dateOnly)
}

func TestParseFlexibleTimeRejectsGarbage(t *testing.T) {
	for _, in := range []string{"", "yesterday", "25.07.2026", "1753440000"} {
		_, err := ParseFlexibleTime(in)
		assert.Error(t, err, "%q should not parse", in)
	}
}

func TestParseTimeRejectsWrongLayout(t *testing.T) {
	_, err := ParseTime("2026-07-25T09:30:15Z")
	assert.Error(t, err)
}
