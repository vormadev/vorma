# P012 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02) was blocked from writing
this file by the recurring harness report-file guardrail and returned the content in its
final message; Fable placed it.

## What changed

Ten files, all in `crates/vorma-tasks/` (+794/−17, doc-dominated):

- `src/lib.rs` — crate-root doc rewritten to a full teaching entry point (getting-started
  doctest, the `task!` static-node model, the three cache policies with exact retention
  semantics, spawned parallelism and when `ParallelBatch` earns its spawn cost,
  cooperative/preemptive-at-boundaries cancellation, cycle detection, observation).
  `#![deny(rustdoc::broken_intra_doc_links)]` added (`deny(missing_docs)` predated). The
  `task!` macro documented in full — grammar, capture-free/Copy guarantees, composition,
  type-path flexibility verified against the macro rules.
- `src/task.rs` — `Task`/`Tasks`/`TasksOptions`/`ExecCtx` and all public methods to the
  bar with doctests. Private mechanical cleanup: duplicate `BoxFuture<T>` alias
  consolidated to `task.rs` as `pub(crate)` (existing dependency direction), the
  `overrides.rs` copy deleted.
- `src/overrides.rs` — `TaskOverrideMode`/`TaskOverrides` to the bar with a
  full-override-cycle doctest; duplicate alias and now-unused import removed.
- `src/parallel.rs` — batch types to the bar, restating the parallelism contract and
  `run`'s exact cancellation/error semantics (error tie-break verified empirically before
  documenting).
- `src/observer.rs` — observer surface to the bar; exact event-ordering contract
  documented (traced through resolve/run_in_exec_ctx/resolve_shared, cross-checked against
  the observer tests). One clippy `doc_lazy_continuation` fix.
- `src/cancel.rs`, `src/clock.rs`, `src/error.rs`, `src/key.rs` (`TaskId` only) — to the
  bar; clock doctests deterministic.
- `tests/tasks.rs` — new
  `parallel_error_tie_break_is_completion_order_not_registration_order`, closing a real
  gap (the prior failure test could never exercise a genuine two-error race).

No public API changes; nothing outside the crate; scratch probes deleted.

## Doc-sweep stats

60 public items (21 types, 1 macro, 23 fns/methods, 15 fields) — all raised from one-line
stubs to the teaching bar. Doctests: 0 → 15, all deterministic.

## Findings (analyzed, not implemented — all protocol-adjacent)

1. Cancellation observe-and-return idiom duplicated at ~10 protocol call sites.
2. RunStarted/RunCompleted bracketing duplicated between `run_in_exec_ctx` and
   `resolve_shared`'s spawned closure.
3. `RunningGuard`/`SharedRunningGuard` structural twins across the loom module boundary (a
   correctness property, not incidental structure).
4. `task.rs` at 1004 lines; the resolution engine is plausibly its own module, but a split
   changes loom's cfg-gate structure.

Explicitly checked and cleared: ParallelBatch's single-task inline branch (deliberate);
joined_result vs wait_for_shared_runner (coincidental rhyme); TasksOptions::default's
minimal bound; cross-crate error-type coherence. Nothing touches the
`tasks-memo-hit-hot-path` ticket's row.

## Checklist verdict

All 13 PASS with evidence; "Performant" carries one disclosed bench deviation
(investigated below); "Nothing overly complicated" carries the four findings. DB calls
N/A.

## Gate results

- vorma-tasks: 49/49 tests (+1 new), 15/15 doctests. Workspace: clean on the confirming
  run; first run hit the pre-existing ticketed vorma-build flake (`AddrInUse` port
  collision — signature captured, ticket advanced), confirmed transient (134/134
  isolated + clean full rerun).
- clippy (crate + workspace) clean; fmt (crate + workspace) clean; doc build under
  `-D warnings` clean; `make loom-tasks` 7/7 MANDATORY-green, re-run after every
  functional edit and at final state; `cargo build -p vorma --lib` clean downstream.

## Benchmarks

No recording (baselines not P012's to move). Direct unrecorded runs: all rows within noise
except `parallel_scaling/tasks-2` (+8-11% across 5 runs) — investigated, not dismissed:
the code change is a compile-time-only alias identity move with no codegen surface; the
desktop was measurably busy (including another concurrent agent session);
`context_cancellation` independently spiked +33% on one run and returned to baseline,
corroborating scheduler jitter; tasks-2 is the most jitter-exposed row of its family.
Attributed to machine contention with the evidence stated, not rounded away.

## Escalations / open questions

None on-the-spot. Findings folded into `tasks-release-quality-findings` (done by Fable at
review) for the batched Phase D-end triage.

## Discovered out-of-scope work

None beyond the findings.
