// Package ical is a minimal RFC 5545 iCalendar encoder.
package ical

import (
	"bytes"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	crlf        = "\r\n"
	foldLimit   = 75
	foldCont    = "\r\n " // CRLF + single space continuation
	calTimezone = "UTC"
)

// Calendar represents a VCALENDAR object.
type Calendar struct {
	ProdID  string
	Version string
	Events  []Event
}

// Event represents a VEVENT.
type Event struct {
	UID         string
	Summary     string
	Description string
	Location    string
	URL         string
	DTStart     time.Time
	DTEnd       time.Time
	AllDay      bool // when true DTStart/DTEnd are DATE not DATETIME
}

// Encode serialises the calendar to RFC 5545 bytes.
func (c *Calendar) Encode() []byte {
	var b bytes.Buffer

	writeLine(&b, "BEGIN:VCALENDAR")
	writeLine(&b, "VERSION:2.0")
	writeLine(&b, foldProperty("PRODID", c.ProdID))
	writeLine(&b, "CALSCALE:GREGORIAN")
	writeLine(&b, "METHOD:PUBLISH")

	for _, ev := range c.Events {
		ev.encode(&b)
	}

	writeLine(&b, "END:VCALENDAR")
	return b.Bytes()
}

func (e *Event) encode(b *bytes.Buffer) {
	writeLine(b, "BEGIN:VEVENT")
	writeLine(b, foldProperty("UID", e.UID))
	writeLine(b, foldProperty("SUMMARY", escapeText(e.Summary)))

	if e.AllDay {
		writeLine(b, "DTSTART;VALUE=DATE:"+e.DTStart.UTC().Format("20060102"))
		if !e.DTEnd.IsZero() {
			writeLine(b, "DTEND;VALUE=DATE:"+e.DTEnd.UTC().Format("20060102"))
		}
	} else {
		writeLine(b, "DTSTART:"+e.DTStart.UTC().Format("20060102T150405Z"))
		if !e.DTEnd.IsZero() {
			writeLine(b, "DTEND:"+e.DTEnd.UTC().Format("20060102T150405Z"))
		}
	}

	if e.Description != "" {
		writeLine(b, foldProperty("DESCRIPTION", escapeText(e.Description)))
	}
	if e.Location != "" {
		writeLine(b, foldProperty("LOCATION", escapeText(e.Location)))
	}
	if e.URL != "" {
		writeLine(b, foldProperty("URL", e.URL))
	}

	writeLine(b, "END:VEVENT")
}

// foldProperty folds "NAME:VALUE" at 75 octets per RFC 5545 §3.1.
func foldProperty(name, value string) string {
	line := name + ":" + value
	if utf8.RuneCountInString(line) <= foldLimit {
		return line
	}
	var sb strings.Builder
	runes := []rune(line)
	sb.WriteString(string(runes[:foldLimit]))
	runes = runes[foldLimit:]
	for len(runes) > 0 {
		chunk := 74 // 74 data runes + 1 continuation space = 75
		if chunk > len(runes) {
			chunk = len(runes)
		}
		sb.WriteString(foldCont)
		sb.WriteString(string(runes[:chunk]))
		runes = runes[chunk:]
	}
	return sb.String()
}

// escapeText escapes TEXT values per RFC 5545 §3.3.11.
func escapeText(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, ";", `\;`)
	s = strings.ReplaceAll(s, ",", `\,`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	return s
}

func writeLine(b *bytes.Buffer, s string) {
	_, _ = fmt.Fprintf(b, "%s%s", s, crlf)
}
