package mail

import (
	"bytes"
	"embed"
	"fmt"
	htmpl "html/template"
	"strings"
	ttmpl "text/template"
	"time"
)

//go:embed templates/*.tmpl
var templateFS embed.FS

// ReminderData is the template context for per-deadline reminder emails.
type ReminderData struct {
	UserName          string
	ConferenceName    string
	ConferenceAcronym string
	DeadlineKind      string
	DeadlineDate      time.Time
	LeadTimeDays      int32
}

// DigestData is the template context for weekly digest emails.
type DigestData struct {
	UserName  string
	WeekStart time.Time
	Items     []DigestItem
}

// DigestItem is one deadline entry in the digest.
type DigestItem struct {
	ConferenceName    string
	ConferenceAcronym string
	DeadlineKind      string
	DeadlineDate      time.Time
}

var funcMap = ttmpl.FuncMap{
	"formatDate": func(t time.Time) string {
		return t.Format("02 Jan 2006")
	},
	"title": strings.Title, //nolint:staticcheck
}

// RenderReminder renders both text and HTML bodies for a reminder email.
func RenderReminder(data ReminderData) (text, html string, err error) {
	text, err = renderText("templates/reminder.txt.tmpl", data)
	if err != nil {
		return "", "", fmt.Errorf("reminder text: %w", err)
	}
	html, err = renderHTML("templates/reminder.html.tmpl", data)
	if err != nil {
		return "", "", fmt.Errorf("reminder html: %w", err)
	}
	return text, html, nil
}

// RenderDigest renders both text and HTML bodies for a digest email.
func RenderDigest(data DigestData) (text, html string, err error) {
	text, err = renderText("templates/digest.txt.tmpl", data)
	if err != nil {
		return "", "", fmt.Errorf("digest text: %w", err)
	}
	html, err = renderHTML("templates/digest.html.tmpl", data)
	if err != nil {
		return "", "", fmt.Errorf("digest html: %w", err)
	}
	return text, html, nil
}

func renderText(name string, data any) (string, error) {
	src, err := templateFS.ReadFile(name)
	if err != nil {
		return "", err
	}
	t, err := ttmpl.New(name).Funcs(funcMap).Parse(string(src))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func renderHTML(name string, data any) (string, error) {
	src, err := templateFS.ReadFile(name)
	if err != nil {
		return "", err
	}
	hfuncs := htmpl.FuncMap{
		"formatDate": funcMap["formatDate"],
		"title":      funcMap["title"],
	}
	t, err := htmpl.New(name).Funcs(hfuncs).Parse(string(src))
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}
