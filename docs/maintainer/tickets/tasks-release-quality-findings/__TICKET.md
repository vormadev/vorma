# vorma-tasks thermo-nuclear findings (P012) — held for batched Phase D-end triage

Same disposition as `matcher-release-quality-findings`: held for the single batched
maintainer ruling at Phase D's end. Full analyses in
`docs/maintainer/fable/packets/P012-tasks-release-quality/REPORT.md`. All four are
protocol-adjacent (the findings-mode exclusion class); none blocks anything.

1. Cancellation observe-and-return idiom duplicated at ~10 protocol call sites — a private
   helper would collapse each to one line; every site is inside the loom-guarded
   wait/notify, poll-once, or shared-cache protocol.
2. RunStarted/RunCompleted observability bracketing duplicated verbatim between
   `run_in_exec_ctx` and `resolve_shared`'s spawned closure — same protocol-adjacency.
3. `RunningGuard`/`SharedRunningGuard` structural twins across the loom module boundary —
   the pair is a correctness property of the protocol, not incidental structure;
   unification crosses the loom gate.
4. `task.rs` at 1004 lines — the ExecCtx resolution engine is plausibly its own module,
   but a split changes loom's cfg-gate structure even with zero logic moved.

Explicitly checked and cleared by P012 (recorded so nobody re-litigates): the single-task
inline branch in ParallelBatch::run (deliberate, matches the engine pattern);
joined_result vs wait_for_shared_runner (coincidental rhyme); TasksOptions::default's
minimal bound; cross-crate error-type coherence.

## Status note (Fable, 2026-07-02)

This ticket was briefly deleted under a Fable ruling made outside its authority (the batch
triage is the maintainer's sitting, not Fable's); restored verbatim the same day. Fable's
RECOMMENDATION for all four rows: reject — the duplication is loom-protocol legibility and
unification perturbs the exact surface the 7-model loom gate freezes. Awaiting the
maintainer's ruling.
