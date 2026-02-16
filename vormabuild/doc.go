// Package vormabuild contains Vorma build and dev entrypoints.
//
// This package is build-time only and is intended for build commands (for
// example, ./cmd/build). Do not import it from runtime request-serving code.
// Keeping build-time orchestration out of production binaries avoids pulling in
// unnecessary tooling dependencies and keeps app binaries smaller.
//
// Build and fast-rebuild flows follow a deterministic plan/stage/commit model:
//  1. plan: capture immutable runtime snapshots and compute build decisions.
//  2. stage: run filesystem/codegen side effects outside runtime write locks.
//  3. commit: apply bounded runtime-state commits under lock.
//
// Runtime invariants:
//   - runtime state writes commit through the shared runtime-state commit path so
//     readers never observe mixed-field updates.
//   - rollback of captured runtime snapshots is attempt-scoped and guarded by
//     build-ID token checks so stale attempts cannot overwrite newer commits.
//   - artifact writers treat superseded runtime snapshots as no-op completion
//     rather than mutating newer runtime state.
package vormabuild
