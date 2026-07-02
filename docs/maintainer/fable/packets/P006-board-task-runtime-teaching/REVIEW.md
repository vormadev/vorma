# P006 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** All three teaching rows landed, the complete F-17-owed API
set has cited call sites (`Tasks::new`, `Tasks::exec_ctx`, `CancelToken`
new/cancel/cancelled/child, `ExecCtx::is_cancelled`, `ExecCtx::child`, the `TaskObserver`
family, `Task::id`, `TasksOptions.observer`), and the teaching quality of the new module
is the best board has — the sovereignty lesson, the fresh-context-per-sweep insight, and
the one-observer-per-process point are exactly what the primitive's documentation should
say.

## Independent verification performed (Fable)

- Change surface matches the report (board-only + census; zero `crates/` changes).
- `maintenance.rs` read in full at the teaching bar; the worker's job is real (sessions
  have `created_at` and no expiry mechanism anywhere).
- Gates re-run by Fable: workspace tests zero failures, clippy clean, rust fmt clean,
  board tsgo clean, vitest 851/851; loom 7/7 (executor run + Fable's prior file-captured
  run; vorma-tasks untouched).
- Repo-wide `make ts-fmt-check` green after Fable's review-boundary housekeeping (see
  Findings).

## Rulings issued at this review (Fable)

- **The tokio feature-flag read is confirmed as standing guidance:** enabling a feature on
  an already-present direct dependency, with the lockfile verified byte-identical, is
  manifest honesty, not a new dependency — allowed with disclosure, exactly as the
  executor did it.
- **Endorsed:** the mod-export scan running through the framework's per-request `ExecCtx`
  (an HTTP-triggered action's natural runtime; only the background worker needs
  `Tasks::new`); `.child()` given real purpose via the existing `COMMENTS_FOR_STORY` task;
  no board test for the `complete: false` path (no supported external mid-flight cancel;
  the cancellation mechanism's proof lives in the loom-verified sovereign suite where it
  belongs — forcing it in board would require timing races the tests-never-cheat rule
  forbids).

## Findings

Exhaustive plain list:

1. Two oxfmt defects surfaced during review housekeeping (not executor faults — the
   executor correctly refused to write-format the drifted census file): a
   content-corruption bug (bold spanning glob code spans; it had silently mangled
   `IN_OPEN` to `IN*OPEN` in P001b's report across earlier passes) and a non-convergence
   bug (indented code blocks in list items gain indentation forever). Both repaired by
   hand, both documented with mitigations in ticket
   `oxfmt-markdown-corruption-and-nonconvergence`; repo-wide fmt check is green again.

No executor issues found.

## Consequence

P006 closes; census F-17 is RESOLVED with the exempt remainder (`TaskOverrides`, `Clock`
family) recorded. Board now teaches the full app-useful task lifecycle. Next: P009
(multi-attachment submit), then P007 (prod build) closes Phase C.
