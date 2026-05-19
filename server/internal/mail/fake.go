package mail

import (
	"context"
	"sync"
)

// FakeMailer captures sent messages in memory. Safe for concurrent use.
// Use it in tests and in dev mode ("would send" logging).
type FakeMailer struct {
	mu   sync.Mutex
	sent []Message
}

// Send records the message and returns nil.
func (f *FakeMailer) Send(_ context.Context, msg Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, msg)
	return nil
}

// Sent returns a snapshot of all recorded messages.
func (f *FakeMailer) Sent() []Message {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]Message, len(f.sent))
	copy(out, f.sent)
	return out
}

// Reset clears the recorded messages.
func (f *FakeMailer) Reset() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = f.sent[:0]
}
