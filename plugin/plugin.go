// Package plugin provides extension hooks for glsync.
//
// Register handlers from your main package (or an init() function) before
// the server starts. All registered handlers are called synchronously in
// registration order.
//
// Example usage in a private fork's main.go:
//
//	import _ "github.com/your-org/glsync-ext" // registers handlers via init()
package plugin

import (
	"context"

	"github.com/mavolty/glsync/internal/domain"
)

// EventHandler is called after a webhook event is parsed and persisted,
// before jobs are enqueued. A non-nil error is logged as a warning and does
// NOT abort job enqueuing. Implementations must be safe for concurrent use.
type EventHandler interface {
	HandleEvent(ctx context.Context, event domain.NormalizedEvent) error
}

// JobHandler is called after a job is executed (success or failure).
// execErr is nil on success. Implementations must be safe for concurrent use.
type JobHandler interface {
	HandleJobResult(ctx context.Context, job domain.Job, execErr error)
}

var (
	eventHandlers []EventHandler
	jobHandlers   []JobHandler
)

// RegisterEventHandler adds h to the global event handler registry.
// Call before the server starts (e.g. from an init() function).
func RegisterEventHandler(h EventHandler) {
	eventHandlers = append(eventHandlers, h)
}

// RegisterJobHandler adds h to the global job handler registry.
// Call before the server starts (e.g. from an init() function).
func RegisterJobHandler(h JobHandler) {
	jobHandlers = append(jobHandlers, h)
}

// EventHandlers returns the registered event handlers (read-only view).
func EventHandlers() []EventHandler { return eventHandlers }

// JobHandlers returns the registered job handlers (read-only view).
func JobHandlers() []JobHandler { return jobHandlers }
