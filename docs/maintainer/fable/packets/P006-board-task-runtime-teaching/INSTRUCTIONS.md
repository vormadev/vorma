# P006 — Board task-runtime teaching (the F-17-ruled rows)

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md` (tasks doctrine especially), `STATE.md`, repo-root `AGENTS.md`,
`docs/maintainer/REMINDERS.md` (the vorma-tasks sovereignty note is load-bearing), and
`docs/maintainer/board-example/README.md` — the 2026-07-01 coverage ruling in that file is
this packet's charter. Inputs: the post-ruling re-triage section of
`docs/maintainer/tickets/board-api-coverage/INVENTORY_VS_BOARD_P003.md` and census finding
F-17.

## Context

The maintainer ruled: the test for board coverage is **framework-author primitive vs
app-useful primitive** — never "is it advanced." `vorma-tasks` is a sovereign crate that
Vorma builds on, and applications obviously run their own background work, so the task
runtime-lifecycle surface is app-useful and board owes it real teaching. Board already
teaches app-authored tasks broadly (ten `task!` declarations across all three cache
policies, three `ParallelBatch` uses); what is missing is the lifecycle surface an app
uses when IT owns the execution context.

## Scope — three teaching rows

1. **A background worker that owns its runtime.** In board's server main: a worker that
   constructs its own `Tasks` (`Tasks::new`), opens an `ExecCtx` per iteration
   (`Tasks::exec_ctx`), runs real board maintenance on an interval (pick an honest job —
   e.g. pruning expired sessions or refreshing a stats snapshot; it must do something the
   app genuinely wants), and wires a `CancelToken` to graceful shutdown so an in-flight
   iteration is cancelled promptly when the server stops. The teaching story: vorma-tasks
   is a sovereign crate — the same primitive the framework hands you in handlers is yours
   to run anywhere (background jobs, CLIs, daemons); this is how.
2. **Cooperative cancellation inside a task body.** A long-running board task (e.g. a
   mod-only bulk scan/export over stories+comments) that checks `ctx.is_cancelled()`
   between work units and bails promptly, teaching when and why a long body should
   cooperate with cancellation (client gone, shutdown, terminal boundary). Design the scan
   so `ExecCtx::child` is also exercised (a child scope per chunk or sub-unit is the
   natural shape for a long scan). Remember the board README's 2026-07-02 ruling: board is
   a teaching tool, contrived by design — "the feature is invented" is the mechanism, not
   a problem; the only bar is that the code teaches honestly. If you conclude an API
   cannot be used sensibly at all, that is an API-design escalation, never a recorded
   waiver.
3. **Slow-task telemetry via `TaskObserver`.** A slow-task-logging observer wired into the
   server main's `TasksOptions.observer` (the census F11 line that was never built): log
   task name/`Task::id`/duration for runs crossing a threshold. Teaching story: production
   task observability without touching handler code.

Where these naturally close items in census F-17's list, update the census; the `Clock`
family and `TaskOverrides` remain exempt per the ruling — do not contrive them in.

## Hard constraints

- **vorma-tasks sovereignty (REMINDERS.md):** the worker uses the crate directly; do not
  add framework hooks, helpers, or opinions to `vorma` or `vorma-tasks` to make board
  prettier. Friction found = census F-row + ticket, never a framework change here.
- Teaching bar throughout: comments explain why an app reaches for each API, in
  user-facing terms. No process language.
- Board tests only for app-observable behavior (e.g. the worker's effect, the scan's
  endpoint), never framework semantics.
- The interval worker is an app loop — keep it simple and honest (a `tokio::time` interval
  with shutdown select is fine; this is app code, not framework code, and the REMINDERS
  no-polling rule governs dev-event plumbing, not application job scheduling).
- No public API changes anywhere; no new dependencies without escalation; no git actions;
  no network installs; scoped fmt writes only (census F-18); unexpectedly dirty unowned
  files are escalations, never cleanup.

## Definition of done

- All three rows landed at the teaching bar, exercising: `Tasks::new`, `Tasks::exec_ctx`,
  `CancelToken` (shutdown wiring), `ExecCtx::is_cancelled` (and `child` only if honest),
  `TaskObserver` family + `Task::id`.
- Census F-17 updated to reflect what is now covered and what remains exempt.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check, board tsgo,
  vitest; `make loom-tasks` untouched-green (you are not touching vorma-tasks).
- REPORT.md per template (or full content returned in-message if the file-write guardrail
  fires, for the orchestrator to place).
