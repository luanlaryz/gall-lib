// Package workflow defines the minimal workflow contracts consumed by app.
package workflow

import (
	"context"
	"time"

	"github.com/luanlima/gaal-lib/pkg/types"
)

// Request is the minimal public request envelope for a workflow.
type Request struct{}

// Response is the minimal public response envelope for a workflow.
type Response struct{}

// Workflow is the minimal runnable workflow contract.
type Workflow interface {
	Name() string
	Run(ctx context.Context, req Request) (Response, error)
}

// Event is the minimal public workflow event envelope.
type Event struct {
	Type     string
	Metadata types.Metadata
}

// Hook observes workflow events.
type Hook interface {
	OnEvent(ctx context.Context, event Event)
}

// HistoryEntry is the minimal history payload recorded for a workflow.
type HistoryEntry struct {
	Kind     string
	Metadata types.Metadata
}

// HistorySink persists workflow history entries.
type HistorySink interface {
	Append(ctx context.Context, entry HistoryEntry) error
}

// RetryPolicy decides whether a failed workflow step should retry.
type RetryPolicy interface {
	Next(attempt int, err error) (time.Duration, bool)
}

// Descriptor identifies a workflow in registries and runtime views.
type Descriptor struct {
	Name string
	ID   string
}
