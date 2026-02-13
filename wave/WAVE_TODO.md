WAVE TODO

Context:

- This list is inferred from canonical_refactor/\*.md as untrusted input.
- Treat every item as a candidate until explicitly accepted.

## 1) Deterministic event execution model (primary)

- Keep one execution pipeline for watcher events: classify -> plan -> execute.
- Remove behavioral duplication between single-event and batched-event paths.
- Replace scattered flag mutation with explicit decision/result structs for:
  build work, restart decision, and browser action.
- Reduce refresh actions in a stable, deterministic order before restart/reload
  decisions.
- Ensure concurrent hook completion order cannot change final outcomes.
- Target files: `wave/tooling/events.go`.

Acceptance:

- No duplicated orchestration branches for single vs batch behavior.
- Same input event set always yields the same plan/outcome.
- Table-driven tests lock deterministic refresh/restart behavior.

## 2) Restart scheduling policy as pure logic

- Move restart request merge/upgrade policy to a pure function.
- Keep channel code as transport only (enqueue/dequeue), not policy logic.
- Add exhaustive table tests for config-restart precedence and upgrade rules.
- Target files: `wave/tooling/devserver.go`,
  `wave/tooling/devserver_restart_test.go`.

Acceptance:

- Restart policy is testable without goroutines/channels.
- Existing precedence behavior is preserved where intended.

## 3) Path normalization consolidation

- Consolidate abs/clean/slash normalization into shared helpers by concern.
- Eliminate duplicated path-shape logic across parse, watcher, config reload,
  and event classification.
- Target files: `wave/parse.go`, `wave/tooling/watcher.go`,
  `wave/tooling/devserver.go`, `wave/tooling/events.go`.

Acceptance:

- One canonical normalization path per concern.
- No mismatched path-shape behavior across modules.

## 4) File-processing efficiency (after deterministic core)

- Profile static/CSS processing on large asset trees and frequent edit cycles.
- Reduce unnecessary full-tree work while preserving: deletion correctness, hash
  correctness, and deterministic outputs.
- Prefer targeted processing improvements over broad abstraction.
- Target files: `wave/tooling/static.go`, `wave/tooling/css.go`,
  `wave/tooling/events.go`, `wave/tooling/builder.go`.

Acceptance:

- Measured improvement under representative large-tree workloads.
- No regression in file-map correctness or cleanup behavior.

## 5) Diagnostics (deferred)

- Revisit only after core behavior work above is stable.
- If reintroduced, scope must be minimal and directly actionable.
- Target files: `wave/tooling/cli.go`, `wave/tooling/events.go`.

## RULES

- No non-JSON authored config path.
- No builder-pattern APIs in Go.
- No duplicate default-path APIs.
- No back-compat adapters while sub-1.0.
- Keep runtime/build boundaries strict.
