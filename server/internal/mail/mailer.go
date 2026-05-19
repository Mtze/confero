// Package mail defines the Mailer interface and shared message types.
package mail

import "context"

// Message is the envelope passed to a Mailer. BodyText and BodyHTML are
// pre-rendered by the caller; the Mailer is responsible only for transport.
type Message struct {
	To       string
	Subject  string
	BodyText string
	BodyHTML string
}

// Mailer sends email messages.
type Mailer interface {
	Send(ctx context.Context, msg Message) error
}
