// Package memory defines conversational memory contracts and the reference
// in-process implementations used by gaal-lib.
//
// Conversational memory is session-scoped state that may survive multiple runs
// through a Store. Working memory is run-scoped, always local to one execution,
// and may include transient artifacts that are never persisted. Workflow
// execution history is a separate concern handled by pkg/workflow and must not
// be mixed into conversational memory by default.
package memory
