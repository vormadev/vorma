# P002 — Tasks Perf Restoration

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md` (especially the tasks doctrine and performance conventions) and
`STATE.md` (the three-column benchmark table is your baseline and your target).
Repo-root `AGENTS.md` also binds you. Prerequisite: P001 accepted (gates green).

## Context

`crates/vorma-tasks` recently gained its final API shape (the `task!` static-node
macro, three cache policies, `ParallelBatch` with true spawned parallelism, loom-verified
wait protocol). That work was good — but it also regressed every benchmark row and
recorded the regressed numbers. The maintainer's bar is explicit: beat the Go
baselines, not match them. The causes are already diagnosed; this packet is the
mechanical restoration. The `STATE.md` table gives Go / pre-gap / staged-now for every
row.

Diagnosed causes:

1. `fingerprint_for` in `src/key.rs` runs blake3 over the input on every resolve —
  including pure memo hits. Blake3 initialization plus finalization costs roughly
  50–80ns on small inputs; the entire Go memo-hit operation is 17ns. Blake3 here is
  also unkeyed and truncated to 64 bits, which is weaker against offline collision
  precomputation than a keyed hash (see LEARNINGS on why this hash is a
  denial-of-service surface at all).
2. `run_in_exec_ctx` in `src/task.rs` lost its poll-once fast path: every task run now
  wires cancellation-subscription machinery (`tokio::select!` plus, for child tokens, a
  boxed parent wait inside `CancelToken::cancelled`) even when the task body completes
  on its first poll — which memo-dependency and pure-compute bodies always do.
3. `ParallelBatch::run` (`src/parallel.rs`) shows per-sibling cost well beyond expected
  `tokio::spawn` overhead (a two-task batch measures ~10µs; spawn cost predicts low
  single-digit µs). Unattributed; cause 2 is likely a large component (every sibling
  run pays the subscription setup), the rest needs measurement.

## Scope

### 1. Keyed-SipHash fingerprints (replaces blake3 on the hot path)

In `src/key.rs`:

- Create one process-global `std::hash::RandomState` (in a `std::sync::LazyLock`).
  `RandomState` is std's keyed SipHash-1-3 with per-process random keys: fast (~10–15ns
  on small inputs) and flooding-resistant, with no new dependency.
- `fingerprint_for` builds its hasher from that state and hashes the task id and the
  input in one pass: write the `TaskId` value first, then `input.hash(&mut hasher)`.
  Store the result in `KeyFingerprint.hash` as today. `TypeId` stays an `Eq`-compared
  field only.
- Replace the derived `Hash` on `KeyFingerprint` with a manual implementation that
  writes only `self.hash` (`state.write_u64(self.hash)`). This makes the existing
  passthrough `FingerprintHasher` structurally correct instead of dependent on derived
  field order — its comment about field ordering then gets deleted, and its `write_u64`
  no longer relies on "last write wins".
- Remove the blake3 usage and drop `blake3` from `crates/vorma-tasks/Cargo.toml`
  dependencies (leave the workspace-level entry alone; other crates use it).

Semantics note: equality on slots is always verified on the full typed key, so hash
changes cannot change observable behavior — only bucket routing.

### 2. Restore the poll-once fast path

In `run_in_exec_ctx` (`src/task.rs`): before entering `tokio::select!`, pin the call
future and poll it once with a noop waker (`std::task::Waker::noop`). If it is
`Poll::Ready`, use that outcome and skip the select entirely; if `Poll::Pending`,
proceed into the existing `select!` with the same pinned future. This is safe because
each poll re-registers wakers (the noop registration is superseded by the select's),
and observable cancellation semantics are unchanged: cancellation is checked before the
run starts and after the outcome is produced, exactly as now. Restore the explanatory
comment about why bodies that never suspend cannot miss prompt cancellation.

Tests that pin this area and must stay green untouched:
`cross_exec_ctx_runner_respects_cancellation_set_before_success_return`,
`cancellation_results_are_not_memoized_inside_one_exec_ctx`,
`waiting_on_local_work_respects_cancellation`, all loom models.

### 3. Attribute and reduce ParallelBatch overhead

Measurement first: using scratch-only experiments (never committed), attribute the
per-sibling cost of a two-task batch across: task spawn, the prepared-closure boxing,
the child cancel token, the output-slot mutex, and cause 2 above. Report the
attribution table. Then apply only internal-only reductions (fewer boxes, fewer
allocations, cheaper token wiring) that keep the public `ParallelBatch` API and all
semantics identical: true spawned parallelism stays (maintainer ruling — never replace
spawning with single-poller concurrency), first-error-wins error selection stays, eager
sibling cancellation stays, panic resume stays, cancel-on-drop stays.

## Hard constraints

- Public API: frozen. No signature, type, or export changes anywhere in the crate.
- Semantics: frozen. Anything that would change observable behavior — retention,
  cancellation timing classes, error selection, panic propagation — is an escalation,
  not a change.
- Loom: `make loom-tasks` must pass with models unweakened. If your change touches the
  slot protocol (it should not need to), new transitions require new models and an
  escalation note.
- No new dependencies. No unsafe. Benchmarks recorded only via `make bench-tasks`.
- If a target row cannot be met without violating any of the above, stop and escalate
  with your measurements. Do not force a number.

## Definition of done

- Full gate green: workspace tests, clippy `-D warnings`, fmt check, `make loom-tasks`.
- `make bench-tasks` re-recorded; `REPORT.md` contains the four-column table
  (Go / pre-gap / previous-staged / now) for all 13 rows.
- Targets: every row at or below the Go column, except `high_contention` (must not
  exceed the previous-staged 15,298 and keeps its structural explanation) and
  `context_cancellation` (already winning; just record it). The pre-gap column is the
  stretch target for single-run and repeated rows.
- `docs/maintainer/fable/STATE.md` benchmark section and open-flags list updated
  (fingerprint-hasher flag closes with this packet).
- `REPORT.md` per the template, including the ParallelBatch attribution table and every
  judgment call listed.
