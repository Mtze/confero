// Package mail defines the Mailer interface and shared message types.
package mail

import "context"

// Message is the envelope passed to a Mailer.
type Message struct {
	To             string
	Subject        string
	ConferenceID   string
	ConferenceName string
	DeadlineKind   string
	LeadTimeDays   int32
}

// Mailer sends email messages.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
