# Current State

Perishable snapshot. Update whenever a packet lands. Last updated: 2026-07-01, after
review of the post-gap staged tree (no commits yet on top of `fable-2 + codex-summary`).

## Gates

- Workspace tests: **549 passed, 0 failed** (`cargo test --workspace --no-fail-fast`).
- Loom: **7/7 models pass** (`make loom-tasks`).
- fmt: clean (`cargo fmt --all --check`).
- Clippy: **RED** — one `needless_update` error at `examples/board/src/lib.rs:113`
  blocks `-D warnings`. P001 fixes this.
- TS gate (`make ts-gate`) and e2e: **not verified since the gap work landed.** P001
  runs and records them.

## Benchmarks — vorma-matcher (recorded, healthy)

`crates/vorma-matcher/bench.results.txt`, Apple M3 Max, mimalloc. Beats the Go
baselines on every row:

| Row | Go (old repo) | Recorded now |
|---|---|---|
| parse_segments | 42.5 | 15.24 |
| flat static / dynamic / splat | 19.9 / 222 / 107 | 7.23 / 123.1 / 68.5 |
| scale small / medium / large | 92 / 124 / 126 | 42.5 / 71.7 / 72.5 |
| worst-case deep | 218 | 123.2 |
| nested static / dynamic / deep / splat / mixed | 166 / 657 / 745 / 590 / 536 | 69.8 / 198.8 / 266.6 / 205.9 / 193.6 |

(ns per op throughout.)

## Benchmarks — vorma-tasks (recorded, REGRESSED — P002)

`crates/vorma-tasks/bench.results.txt` as staged records a large regression introduced
by the gap work. Pre-gap column is the last known-good recording on the same machine and
is the restoration target; the maintainer bar is beating Go, not matching it.

| Row | Go | Pre-gap (target) | Staged now |
|---|---|---|---|
| single_task | 214.1 | 144.4 | 258.8 |
| parallel_independent_tasks | 2,405 | 589.5 | 10,901 |
| high_contention | 5,922 | 13,193 | 15,298 |
| task_with_dependencies | 331.4 | 263.4 | 502.5 |
| allocations | 271.5 | 175.3 | 293.3 |
| parallel_scaling tasks-1 | 288.7 | 215.5 | 406.6 |
| parallel_scaling tasks-2 | 1,779 | 427.9 | 10,117 |
| parallel_scaling tasks-5 | 3,543 | 974.3 | 16,230 |
| parallel_scaling tasks-10 | 8,473 | 1,842 | 24,172 |
| parallel_scaling tasks-20 | 20,280 | 3,641 | 50,871 |
| parallel_scaling tasks-50 | 49,112 | 8,731 | 118,455 |
| context_cancellation | 10,995,394 | 8,226 | 9,295 |
| repeated_task_calls | 17.03 | 23.3 | 95.35 |

Caveats on the pre-gap column: it predates the `task!` static-node redesign, the
`ParallelBatch` API, and the restoration of spawned parallelism, so parallel rows are
not apples-to-apples (spawn cost is real and accepted); single-run and repeated rows
are directly comparable. `high_contention` is dominated by ten `tokio::spawn` calls in
the benchmark harness itself (mirroring Go's goroutine fan-out) and is the one row with
an accepted structural explanation. `context_cancellation` wins by design (preemptive
cancellation vs Go running the body to completion).

Identified regression causes (see P002): blake3 fingerprinting on the per-resolve hot
path; the removed poll-once fast path (every run pays cancellation-subscription setup,
including a boxed parent wait on child tokens); unattributed per-sibling overhead in
`ParallelBatch` beyond expected spawn cost.

## Open flags

- Notes example deleted per the board-is-canonical policy; its unique coverage (a
  TTL'd/extended-cache task exercised through an app, its request-level test suite) has
  no recorded home. Folded into the board coverage packet (P003).
- `FingerprintHasher` in `vorma-tasks/src/key.rs` relies on derived-`Hash` field order;
  hardened as part of P002.
- Lock-poisoning recovery (`unwrap_or_else(|poison| poison.into_inner())`) in the task
  store is deliberate and test-covered; not a flag, recorded so nobody "fixes" it.
- The census lives at `docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md`
  (moved there during the gap-work docs cleanup).

## Where work stands

- Matcher first-principles review: **complete** (semantics corrected and pinned, benches
  beat Go, recorded).
- Tasks first-principles review: **open pending P002** (perf bar unmet again after the
  gap regression; everything else about the crate — API shape, loom verification,
  test suite — landed well).
- Router/request-path review: **not started** (P004; ticket exists at
  `docs/maintainer/tickets/router-request-path-review/`).
- Board: census features and 100% API coverage audit open (P003 and phase C).
