// Package tasks defines shared task type constants used across the codebase.
// This ensures consistency between task enqueueing (handlers) and task processing (worker).
package tasks

const (
	// TypeWorkerPing is used to verify the worker is alive and processing tasks.
	TypeWorkerPing = "worker:ping"
)
