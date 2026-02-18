// Package waveport centralizes Wave runtime port and mode environment handling.
//
// This package owns:
// - `PORT` parsing/validation
// - dev vs non-dev mode resolution behavior
// - per-resolver cached port resolution
//
// Keeping this logic in one place avoids inconsistent port semantics between
// runtime and tooling components.
package waveport
