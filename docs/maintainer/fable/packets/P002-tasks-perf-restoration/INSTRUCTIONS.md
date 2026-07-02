# P002 — Tasks Perf Restoration

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md` (especially the tasks doctrine and performance conventions, including the
per-machine bench-recording doctrine) and `STATE.md`. Repo-root `AGENTS.md` also binds
you. Prerequisites (met): P001/P001b accepted, all gates green on the Linux machine, and
the Linux baseline recorded (2026-07-01).

## Baselines and machine discipline (maintainer-ruled)

This packet executes on the Linux machine (`sjc-z390-aorus-pro-wifi`). Bench recordings
are per-machine and additive under `docs/maintainer/bench-results/vorma-tasks/`; your
working baseline is this machine's file and the matching Linux table in `STATE.md`. The Go
/ pre-gap / staged-now columns in `STATE.md` are Apple M3 Max recordings: use them to
understand the regression's shape and magnitude, never as absolute targets for Linux
numbers — cross-machine comparison is meaningless in both directions. All of your
measurements are same-machine before/after on an otherwise idle box (no concurrent builds
or tests while timing). The absolute maintainer bar — beat Go on every row except the two
with accepted structural explanations — was defined on the mac and is verified on the mac:
after this packet is accepted, the maintainer re-records there
(`BENCH_MACHINE_ID=m3-max make bench-tasks`). Your job is to make that verification
succeed by eliminating the diagnosed causes, with each win attributed on this machine.

## Context

`crates/vorma-tasks` recently gained its final API shape (the `task!` static-node macro,
three cache policies, `ParallelBatch` with true spawned parallelism, loom-verified wait
protocol). That work was good — but it also regressed every benchmark row and recorded the
regressed numbers. The maintainer's bar is explicit: beat the Go baselines, not match
them. The causes are already diagnosed; this packet is the mechanical restoration. The
`STATE.md` mac table gives Go / pre-gap / staged-now for every row (regression shape); the
`STATE.md` Linux table is your measurable baseline.

Diagnosed causes:

1. `fingerprint_for` in `src/key.rs` runs blake3 over the input on every resolve —
   including pure memo hits. Blake3 initialization plus finalization costs roughly 50–80ns
   on small inputs; the entire Go memo-hit operation is 17ns. Blake3 here is also unkeyed
   and truncated to 64 bits, which is weaker against offline collision precomputation than
   a keyed hash (see LEARNINGS on why this hash is a denial-of-service surface at all).
2. `run_in_exec_ctx` in `src/task.rs` lost its poll-once fast path: every task run now
   wires cancellation-subscription machinery (`tokio::select!` plus, for child tokens, a
   boxed parent wait inside `CancelToken::cancelled`) even when the task body completes on
   its first poll — which memo-dependency and pure-compute bodies always do.
3. `ParallelBatch::run` (`src/parallel.rs`) shows per-sibling cost well beyond expected
   `tokio::spawn` overhead (a two-task batch measures ~10µs; spawn cost predicts low
   single-digit µs). Unattributed; cause 2 is likely a large component (every sibling run
   pays the subscription setup), the rest needs measurement.

## Scope

### 1. Keyed-SipHash fingerprints (replaces blake3 on the hot path)

In `src/key.rs`:

- Create one process-global `std::hash::RandomState` (in a `std::sync::LazyLock`).
  `RandomState` is std's keyed SipHash-1-3 with per-process random keys: fast (~10–15ns on
  small inputs) and flooding-resistant, with no new dependency.
- `fingerprint_for` builds its hasher from that state and hashes the task id and the input
  in one pass: write the `TaskId` value first, then `input.hash(&mut hasher)`. Store the
  result in `KeyFingerprint.hash` as today. `TypeId` stays an `Eq`-compared field only.
- Replace the derived `Hash` on `KeyFingerprint` with a manual implementation that writes
  only `self.hash` (`state.write_u64(self.hash)`). This makes the existing passthrough
  `FingerprintHasher` structurally correct instead of dependent on derived field order —
  its comment about field ordering then gets deleted, and its `write_u64` no longer relies
  on "last write wins".
- Remove the blake3 usage and drop `blake3` from `crates/vorma-tasks/Cargo.toml`
  dependencies (leave the workspace-level entry alone; other crates use it).

Semantics note: equality on slots is always verified on the full typed key, so hash
changes cannot change observable behavior — only bucket routing.

### 2. Restore the poll-once fast path

In `run_in_exec_ctx` (`src/task.rs`): before entering `tokio::select!`, pin the call
future and poll it once with a noop waker (`std::task::Waker::noop`). If it is
`Poll::Ready`, use that outcome and skip the select entirely; if `Poll::Pending`, proceed
into the existing `select!` with the same pinned future. This is safe because each poll
re-registers wakers (the noop registration is superseded by the select's), and observable
cancellation semantics are unchanged: cancellation is checked before the run starts and
after the outcome is produced, exactly as now. Restore the explanatory comment about why
bodies that never suspend cannot miss prompt cancellation.

Tests that pin this area and must stay green untouched:
`cross_exec_ctx_runner_respects_cancellation_set_before_success_return`,
`cancellation_results_are_not_memoized_inside_one_exec_ctx`,
`waiting_on_local_work_respects_cancellation`, all loom models.

### 3. Attribute and reduce ParallelBatch overhead

Measurement first: using scratch-only experiments (never committed), attribute the
per-sibling cost of a two-task batch across: task spawn, the prepared-closure boxing, the
child cancel token, the output-slot mutex, and cause 2 above. Report the attribution
table. Then apply only internal-only reductions (fewer boxes, fewer allocations, cheaper
token wiring) that keep the public `ParallelBatch` API and all semantics identical: true
spawned parallelism stays (maintainer ruling — never replace spawning with single-poller
concurrency), first-error-wins error selection stays, eager sibling cancellation stays,
panic resume stays, cancel-on-drop stays.

## Hard constraints

- Public API: frozen. No signature, type, or export changes anywhere in the crate.
- Semantics: frozen. Anything that would change observable behavior — retention,
  cancellation timing classes, error selection, panic propagation — is an escalation, not
  a change.
- Loom: `make loom-tasks` must pass with models unweakened. If your change touches the
  slot protocol (it should not need to), new transitions require new models and an
  escalation note.
- No new dependencies. No unsafe. Benchmarks recorded only via `make bench-tasks`.
- If a target row cannot be met without violating any of the above, stop and escalate with
  your measurements. Do not force a number.

## Definition of done

- Full gate green: workspace tests, clippy `-D warnings`, fmt check, `make loom-tasks`.
- `make bench-tasks` re-recorded on this machine (per-machine file under
  `docs/maintainer/bench-results/vorma-tasks/`); `REPORT.md` contains a per-row
  Linux-baseline vs Linux-after table with deltas for all 13 rows, with the mac Go/pre-gap
  columns included for shape reference only (clearly marked cross-machine).
- Targets, on this machine: every row improves or has an explicitly accepted structural
  explanation. Cause-level proof required — the blake3 resolve-path cost is gone (code +
  bench delta on memo-hit-dominated rows like `repeated_task_calls` and `single_task`),
  the poll-once fast path is restored (bench delta on immediately-ready bodies), and the
  ParallelBatch attribution table shows where the per-sibling µs went and what was
  recovered (`parallel_independent_tasks` and the `parallel_scaling` rows must improve
  materially; spawn cost itself is real and accepted). `high_contention` keeps its
  structural explanation; `context_cancellation` already wins by design — just record.
- Per the benchmark-attribution doctrine in `LEARNINGS.md`: one mechanical change per
  measured step; never bundle. Fixes here must not change semantics at all.
- `docs/maintainer/fable/STATE.md`: Linux tasks table updated to the post-restoration
  recording; open-flags list updated (fingerprint-hasher flag closes with this packet);
  add a one-line reminder that the mac-side "beat Go" verification is pending the
  maintainer's next mac recording.
- `REPORT.md` per the template, including the ParallelBatch attribution table and every
  judgment call listed.
