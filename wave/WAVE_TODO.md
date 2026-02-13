WAVE TODO

Context:

- This list is inferred from canonical_refactor/\*.md as untrusted input.
- Treat every item as a candidate until explicitly accepted.

## Unify event execution flow to reduce duplication.

- Merge single-event and batched-event orchestration into one deterministic
  classify -> plan -> execute pipeline.
- Remove duplicated pre/concurrent/post hook and restart handling logic.
- Target files: `wave/tooling/events.go`.

## Make refresh-action reduction deterministic.

- Reduce all hook-returned refresh actions in a stable order before
  restart/reload decisions.
- Ensure concurrent hook completion order cannot change final restart/reload
  outcomes.
- Target files: `wave/tooling/events.go`.

## Keep lifecycle phases explicit inside dev tooling internals.

- Keep boundaries clear between classification, planning, build execution,
  restart, and browser-phase signaling.
- Avoid hidden branching and mixed responsibilities.
- Target files: `wave/tooling/devserver.go`, `wave/tooling/events.go`,
  `wave/tooling/builder.go`.

## Replace bool-heavy event internals with explicit plan/result types.

- Reduce scattered flag mutation in watcher execution code.
- Make decision data explicit before side effects run.
- Target files: `wave/tooling/events.go`.

## Make restart request merge/upgrade policy a pure function.

- Move restart-channel upgrade semantics behind a deterministic function.
- Add exhaustive table tests around config-restart precedence and upgrade rules.
- Target files: `wave/tooling/devserver.go`.

## Consolidate path/dependency normalization helpers.

- Reduce duplicated path cleaning, slash conversion, abs/rel resolution helpers.
- Keep one canonical normalization path per concern.
- Target files: `wave/parse.go`, `wave/tooling/devserver.go`,
  `wave/tooling/watcher.go`.

---

## MAYBE (do last after discussion): Look for file-processing efficiency improvements.

- Profile static/CSS file-processing paths under large asset trees and frequent
  edit cycles.
- Reduce unnecessary full-tree work while preserving deletion correctness and
  deterministic outputs.
- Prefer targeted planner/file-processing optimizations before introducing a
  broader artifact-DAG abstraction.
- Target files: `wave/tooling/static.go`, `wave/tooling/builder.go`,
  `wave/tooling/events.go`.

## MAYBE (do last after discussion): Revisit diagnostics from a clean slate.

- Current state goal: no diagnostics command surface while Wave internals are
  being simplified.
- If reintroduced later, require explicit ROI and minimal scope (no speculative
  framework).
- Target files: `wave/tooling/cli.go`, `wave/tooling/events.go`.

---

## RULES:

### Preserve Wave guardrails during cleanup.

- No non-JSON authored config path.
- No builder-pattern APIs in Go.
- No duplicate default-path APIs.
- No back-compat adapters while sub-1.0.
- Keep runtime/build boundaries strict.
