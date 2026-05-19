package service

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScheduledFor verifies that the send time is correctly computed as lead days
// before the deadline at 09:00 in the given timezone.
func TestScheduledFor(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	// Deadline: 2025-03-15 00:00:00 UTC.
	deadline := time.Date(2025, 3, 15, 0, 0, 0, 0, time.UTC)

	got := scheduledFor(deadline, 7, loc)

	// Expected: 2025-03-08 09:00:00 Europe/Berlin = 2025-03-08 08:00:00 UTC (CET).
	want := time.Date(2025, 3, 8, 8, 0, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

// TestScheduledFor_DST verifies the send time across a DST boundary.
func TestScheduledFor_DST(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Berlin")
	require.NoError(t, err)

	// Deadline: 2025-04-01 (after DST switch on 2025-03-30).
	deadline := time.Date(2025, 4, 1, 0, 0, 0, 0, time.UTC)

	got := scheduledFor(deadline, 7, loc)

	// Expected: 2025-03-25 09:00:00 Europe/Berlin = 2025-03-25 08:00:00 UTC (CET, before DST).
	want := time.Date(2025, 3, 25, 8, 0, 0, 0, time.UTC)
	assert.Equal(t, want, got)
}

// TestConferenceDeadlines verifies that only non-nil deadline fields are returned.
func TestConferenceDeadlines(t *testing.T) {
	now := time.Now().UTC()
	later := now.Add(30 * 24 * time.Hour)

	t.Run("both primary and abstract set", func(t *testing.T) {
		conf := makeConferenceRow(now, later, time.Time{}, time.Time{})
		entries := conferenceDeadlines(conf)
		require.Len(t, entries, 2)
		assert.Equal(t, "submission", entries[0].kind)
		assert.Equal(t, "abstract", entries[1].kind)
	})

	t.Run("no deadlines", func(t *testing.T) {
		conf := makeConferenceRow(time.Time{}, time.Time{}, time.Time{}, time.Time{})
		entries := conferenceDeadlines(conf)
		assert.Empty(t, entries)
	})

	t.Run("all four deadlines", func(t *testing.T) {
		conf := makeConferenceRow(now, later, now.Add(60*24*time.Hour), now.Add(90*24*time.Hour))
		entries := conferenceDeadlines(conf)
		require.Len(t, entries, 4)
		kinds := []string{entries[0].kind, entries[1].kind, entries[2].kind, entries[3].kind}
		assert.Equal(t, []string{"submission", "abstract", "notification", "camera_ready"}, kinds)
	})
}
