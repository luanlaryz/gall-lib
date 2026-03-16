package app

import "errors"

var (
	// ErrInvalidConfig is returned when the app configuration is invalid.
	ErrInvalidConfig = errors.New("app: invalid config")
	// ErrAgentNotFound is returned when an agent cannot be resolved by name.
	ErrAgentNotFound = errors.New("app: agent not found")
	// ErrWorkflowNotFound is returned when a workflow cannot be resolved by name.
	ErrWorkflowNotFound = errors.New("app: workflow not found")
	// ErrDuplicateAgent is returned when the same agent name is registered twice.
	ErrDuplicateAgent = errors.New("app: duplicate agent")
	// ErrDuplicateWorkflow is returned when the same workflow name is registered twice.
	ErrDuplicateWorkflow = errors.New("app: duplicate workflow")
	// ErrRegistrySealed is returned when a registry is mutated after startup.
	ErrRegistrySealed = errors.New("app: registry sealed")
	// ErrAppStopped is returned when a stopped app cannot be started again.
	ErrAppStopped = errors.New("app: stopped")
)
