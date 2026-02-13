WAVE TODO

Context:

- This list is inferred from canonical_refactor/\*.md as untrusted input.
- Treat every item as a candidate until explicitly accepted.

## 1) Hook-stage duplicate-suppression contract expansion (active)

- Add explicit stage-level contracts for `skipDuplicateHooks` across no-wait,
  pre, concurrent, and post flows so dedupe behavior cannot silently drift.
- Cover both action aggregation and side-effect suppression expectations for
  duplicate events sharing the same watched pattern.
- Target files: `wave/tooling/events_hook_execution_test.go`,
  `wave/tooling/events_pipeline_dedup_test.go`.

Acceptance:

- Duplicate-event hook suppression remains behaviorally identical and is guarded
  by direct stage contracts rather than only broad integration checks.

## 2) Diagnostics (deferred)

- Revisit only after core behavior work above is stable.
- If reintroduced, scope must be minimal and directly actionable.
- Target files: `wave/tooling/cli.go`, `wave/tooling/events.go`.

## RULES

- No non-JSON authored config path.
- No builder-pattern APIs in Go.
- No duplicate default-path APIs.
- No back-compat adapters while sub-1.0.
- Keep runtime/build boundaries strict.
