// Package memory defines conversational memory contracts and the reference
// implementations used by gaal-lib.
//
// Conversational memory is session-scoped state that may survive multiple runs
// through a [Store]. Working memory is run-scoped, always local to one
// execution, and may include transient artifacts that are never persisted.
// Workflow execution history is a separate concern handled by pkg/workflow and
// must not be mixed into conversational memory by default.
//
// # Choosing a backend
//
// The package ships two Store implementations:
//
//   - [InMemoryStore] keeps state in process memory. It is deterministic and
//     fast but loses all data when the process exits. This is the default when
//     no store is configured.
//
//   - [FileStore] persists each session as a JSON file on the local filesystem.
//     It survives process restarts and requires no external database. Create one
//     with [NewFileStore] or [MustNewFileStore].
//
// Both implement [Store] and can be injected via [app.AgentDefaults.Memory] for
// a global default or via agent.WithMemory for a per-agent override.
//
// # Implementing a custom adapter
//
// Any type that satisfies the [Store] interface is a valid persistence backend.
// To add support for SQLite, PostgreSQL, Redis or any other database, implement
// Load and Save against your storage and pass the adapter where a Store is
// accepted. No changes to this package or to the runtime are required.
package memory
