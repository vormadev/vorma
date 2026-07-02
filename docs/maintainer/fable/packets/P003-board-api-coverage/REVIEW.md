# P003 Review

Reviewer: Fable (orchestrator). Date: 2026-07-01. Executor: Opus subagent dispatched by
Fable (session survived a mid-run API-outage crash and was resumed with context intact).

## Verdict

**Accepted. Packet closed.** The inventory is methodical and reproducible, the coverage
verdicts are honest (including closing the packet's own stale premise), the implemented
board additions are uniformly at the teaching bar, escalations are well-argued positions
rather than punts, and every definition-of-done item is met — escalation being a valid
terminal state per the packet's DoD. One policy ruling is surfaced to the maintainer
(F-17, below); its outcome determines a small follow-up, not this packet's acceptance.

## Independent verification performed (Fable)

- **Change surface:** exactly as reported — six board files + the inventory artifact +
  census appends; no stray changes; no API changes; `vorma.gen.ts` diff is
  generator-shaped output (new `SiteStats`, field additions), not hand edits.
- **Board code review (teaching bar):** all additions read as user-facing teaching
  material — the cache-policy-trio comment on `LIVE_SITE_STATS`, the
  `with_source`/error-chain comment in LAYOUT, the raw-accessor comment in NOT_FOUND
  (correctly steering readers back to typed input), the `append_header`/`Vary` comment,
  and the build-id-contract test comment. Zero audit/process language. Constants imported
  from `vorma` in tests, never re-defined.
- **Claims spot-checked:** the "extended_cache orphan already covered" claim verified —
  `repo.rs:309` and `:331` are TTL'd `extended_cache` tasks exercised through the docs
  views; the packet premise was genuinely stale. Inventory citations spot-checked
  (`session.rs:81/95/112` are precisely the three middleware constructor teachings).
- **Gates re-run by Fable on the final tree:** fmt clean; clippy `-D warnings` clean;
  `cargo test --workspace --all-targets` green TWICE (551 tests each — the +2 are the new
  board tests; the executor's one-time flake did not recur under either run); loom 7/7;
  full `make ts-gate` green (851/851 vitest, all projects) after Fable's normalization
  pass resolved the docs prose-wrap drift; `tsgo -p examples/board` clean; full
  `make e2e-smoke` aggregate exit 0.
- **Census:** F-16 (single_flight closed), F-17 (tasks-surface escalation), F-18 (oxfmt
  gate friction) present and consistent with the report.

## Findings

Exhaustive plain list, per AGENTS.md review rules:

1. During its gate run, the executor ran `oxfmt --write .`, saw 13 "unexpectedly dirty"
   Fable-owned docs (the write-mode reflow), and bulk-restored them to HEAD content —
   destroying the legitimate uncommitted orchestrator edits those files carried (the mac
   verdict and ratification records). It disclosed this plainly in its report; Fable
   re-applied the records the same day and corrected an initially wrong harness-revert
   theory of the loss. The correct move was to escalate the anomaly, not restore files it
   did not author. Fable's dispatch template now states: an unexpectedly dirty file you
   did not author is an escalation, never cleanup. (Candidate one-line rule for AGENTS.md;
   maintainer's call.)
2. The executor's one-time `vorma-build` lib test failure was observed through a
   summary-only grep that filtered out the failing test's name, so the flake ticket
   (`vorma-build-lib-test-flake-under-load`) lacks the exact test. The capture-discipline
   lesson is recorded in that ticket; Fable's two full-suite re-runs (with full failure
   capture) did not reproduce it.

No other issues found.

## The F-17 ruling (for the maintainer)

The standing policy says "Board must cover 100% of public Vorma APIs. Period." The audit
found ~30 items that cannot fit a real board app flow: the standalone `vorma::tasks`
runtime-lifecycle surface (`Tasks::{new,exec_ctx}`, `CancelToken`,
`ExecCtx::{child,cancel_token,is_cancelled}`, `Clock`/`SystemClock`/`ClockInstant`,
`TaskOverrides`, the `TaskObserver` family) plus low-level head/document type carriers and
informational consts. An HTTP app never constructs its own `Tasks`/`ExecCtx` — the
framework owns the runtime — and the maintainer previously moved `TaskOverrides` coverage
OUT of board deliberately. These items are covered today by the sovereign-crate suites and
`crates/vorma/tests/public_api.rs`.

Options: (a) ratify the "realistic app flow" reading — board covers what an application
author uses in a real app; the runtime-lifecycle surface and type carriers are
sovereign-suite + `public_api.rs` coverage — recorded in
`docs/maintainer/board-example/README.md`; or (b) keep the literal rule, which means a
follow-up packet contriving those items into board (the one app-shaped candidate being a
slow-task-logging `TaskObserver` on the server's `TasksOptions.observer`).

Executor recommendation and Fable recommendation agree: **(a)**, with one addition —
implement the slow-task-logging `TaskObserver` anyway as a small Phase C row, because
production task telemetry IS a realistic app-authoring pattern with a genuine teaching
story, independent of the rule's wording.

### Maintainer ruling (final, 2026-07-01)

The maintainer ruled a sharper test than either recommendation: **framework-author
primitive vs app-useful primitive** — "advanced" is never grounds for exemption, and tasks
are a sovereign crate Vorma builds on, so the runtime-lifecycle surface is app-useful
(applications obviously run background work). Consequences: the exempt set narrows to
`TaskOverrides` (prior ruling: framework-suite home), the `Clock` family (revisitable),
type carriers, and informational consts — discharged by sovereign suites

- `public_api.rs`. Board OWES three new teaching rows, queued as Phase C work: (1) a
  background worker constructing its own `Tasks`/`ExecCtx`/`CancelToken` wired to
  shutdown; (2) cooperative cancellation inside a task body (`ExecCtx::is_cancelled`,
  possibly `child`); (3) the `TaskObserver` slow-task telemetry row (+ `Task::id`). The
  ruling is recorded durably in `docs/maintainer/board-example/README.md`; the inventory
  artifact carries the re-triage.

## Bookkeeping

Escalation 3 (docs prose-wrap drift) resolved during this review: Fable normalized the
maintainer docs through `make ts-fmt`, so the aggregate `ts-gate` is green with no
exclusions. Flake ticket filed. STATE.md hazard flag corrected to the true two-mechanism
story.
