package ical_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"confero/internal/ical"
)

func TestEncode_CRLFTerminators(t *testing.T) {
	cal := &ical.Calendar{ProdID: "-//Test//EN", Version: "2.0"}
	out := cal.Encode()
	require.True(t, bytes.Contains(out, []byte("\r\n")), "output must use CRLF line endings")
	require.False(t, bytes.Contains(out, []byte("\r\n\r\n")), "no blank lines expected")
	// Every line must end with CRLF (split and check non-empty lines)
	lines := strings.Split(string(out), "\r\n")
	for _, l := range lines[:len(lines)-1] { // last element after final CRLF is empty
		require.NotEmpty(t, l, "unexpected empty line")
	}
}

func TestEncode_VCALENDARWrapper(t *testing.T) {
	cal := &ical.Calendar{ProdID: "-//Test//EN", Version: "2.0"}
	out := string(cal.Encode())
	require.True(t, strings.HasPrefix(out, "BEGIN:VCALENDAR\r\n"))
	require.True(t, strings.HasSuffix(out, "END:VCALENDAR\r\n"))
}

func TestEncode_DTSTARTFormatUTC(t *testing.T) {
	ts := time.Date(2026, 6, 15, 23, 59, 0, 0, time.UTC)
	cal := &ical.Calendar{
		ProdID:  "-//Test//EN",
		Version: "2.0",
		Events: []ical.Event{
			{UID: "uid1", Summary: "Test", DTStart: ts, DTEnd: ts.Add(15 * time.Minute)},
		},
	}
	out := string(cal.Encode())
	require.Contains(t, out, "DTSTART:20260615T235900Z")
	require.Contains(t, out, "DTEND:20260616T001400Z")
}

func TestEncode_AllDayDTSTART(t *testing.T) {
	start := time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC)
	cal := &ical.Calendar{
		ProdID: "-//Test//EN",
		Events: []ical.Event{
			{UID: "uid1", Summary: "SIGCSE", DTStart: start, DTEnd: end, AllDay: true},
		},
	}
	out := string(cal.Encode())
	require.Contains(t, out, "DTSTART;VALUE=DATE:20260615")
	require.Contains(t, out, "DTEND;VALUE=DATE:20260618")
}

func TestEncode_LineFolding(t *testing.T) {
	longSummary := strings.Repeat("A", 80)
	cal := &ical.Calendar{
		ProdID: "-//Test//EN",
		Events: []ical.Event{
			{UID: "u", Summary: longSummary, DTStart: time.Now()},
		},
	}
	out := string(cal.Encode())
	for _, line := range strings.Split(out, "\r\n") {
		require.LessOrEqual(t, len([]rune(line)), 75, "line exceeds 75 octets: %q", line)
	}
}

func TestEncode_TextEscaping(t *testing.T) {
	cal := &ical.Calendar{
		ProdID: "-//Test//EN",
		Events: []ical.Event{
			{UID: "u", Summary: `has,comma;semi\back`, DTStart: time.Now()},
		},
	}
	out := string(cal.Encode())
	require.Contains(t, out, `has\,comma\;semi\\back`)
}

func TestEncode_MultipleEvents(t *testing.T) {
	cal := &ical.Calendar{
		ProdID: "-//Test//EN",
		Events: []ical.Event{
			{UID: "u1", Summary: "First", DTStart: time.Now()},
			{UID: "u2", Summary: "Second", DTStart: time.Now()},
		},
	}
	out := string(cal.Encode())
	count := strings.Count(out, "BEGIN:VEVENT")
	require.Equal(t, 2, count)
}
