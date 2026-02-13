WAVE TODO

Context:

- This list is inferred from canonical_refactor/\*.md as untrusted input.
- Treat every item as a candidate until explicitly accepted.

## 1) File-processing efficiency (active)

- Extend contract tests for changed-path delete/rename/create bursts and mixed
  event op combinations.
- Target files: `wave/tooling/static.go`, `wave/tooling/events.go`,
  `wave/tooling/builder.go`, `wave/tooling/static_processing_test.go`.

Acceptance:

- No full-tree walk for pure changed-path batches in steady-state development
  flows.
- Cleanup and hash outputs remain bit-for-bit consistent with full-scan
  behavior.

## 2) CSS build and hot-reload efficiency

- Add end-to-end watcher-cycle contracts that assert failed CSS rebuilds emit no
  CSS payload and that the next successful rebuild resumes payload emission.
- Add explicit contract coverage for config-reload transitions to verify CSS
  hot-reload caches never leak stale outputs across builder replacement.
- Target files: `wave/tooling/css.go`, `wave/tooling/events.go`,
  `wave/tooling/css_build_test.go`, `wave/tooling/broadcast_behavior_test.go`.

Acceptance:

- CSS hot-reload payload generation avoids disk reads when fresh build outputs
  are already available.
- Browser payload semantics remain unchanged.

## 3) Event/build consistency contracts

- Continue extracting event decision logic into pure functions and lock behavior
  with table-driven contract tests.
- Add end-to-end contract tests for shared CSS dependencies (critical and
  normal), run-on-change-only mixed batches, and normalized changed-path
  aggregation.
- Add deterministic assertions that path-shape variants (`a/./b` and equivalent
  normalized paths) produce identical planning outcomes.
- Target files: `wave/tooling/events.go`, `wave/tooling/events_*_test.go`.

Acceptance:

- Equivalent logical change sets produce identical build/restart/browser
  decisions.

## 4) Diagnostics (deferred)

- Revisit only after core behavior work above is stable.
- If reintroduced, scope must be minimal and directly actionable.
- Target files: `wave/tooling/cli.go`, `wave/tooling/events.go`.

## RULES

- No non-JSON authored config path.
- No builder-pattern APIs in Go.
- No duplicate default-path APIs.
- No back-compat adapters while sub-1.0.
- Keep runtime/build boundaries strict.
