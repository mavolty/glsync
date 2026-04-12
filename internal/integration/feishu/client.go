// Package feishu will hold the Feishu/Lark group notification integration.
// This is a stub — the interface defines the contract; implementation is deferred.
package feishu

import "context"

// Notification is the message payload sent to a Feishu group.
type Notification struct {
	GroupID  string
	Title    string
	Body     string
	IssueKey string
	MRLink   string
}

// Notifier is the interface the worker will call to send Feishu messages.
type Notifier interface {
	Send(ctx context.Context, n Notification) error
}

// NoopNotifier is a no-op implementation used until the real one is built.
type NoopNotifier struct{}

func (NoopNotifier) Send(_ context.Context, _ Notification) error {
	return nil
}
