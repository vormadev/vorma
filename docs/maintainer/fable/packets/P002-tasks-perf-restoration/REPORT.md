# P002 Report

Tasks perf restoration on the Linux machine (`sjc-z390-aorus-pro-wifi`, Intel i9-9900K,
Ubuntu 26.04, rustc 1.96.0). Executed as three measured steps, one mechanical change each,
measured on an idle box. Cause 1 (blake3 → keyed SipHash) and cause 2 (poll-once fast
path) landed and are recorded. Cause 3 (ParallelBatch overhead) was attributed by
measurement; the one plausible internal reduction was implemented, A/B-measured against
the existing code, found to be a regression at scale, and reverted — the recovered
per-sibling overhead came from cause 2.

## What changed

- `crates/vorma-tasks/src/key.rs` — fingerprints now use a process-global keyed SipHash
  (std `RandomState` in a `LazyLock`) instead of blake3. `fingerprint_for` writes the
  `TaskId` value then hashes the input in one pass. `KeyFingerprint` now implements `Hash`
  manually (writes only its precomputed `hash` u64), making the passthrough
  `FingerprintHasher` structurally correct instead of dependent on derived field order.
  The `Blake3Hasher` and the derived `Hash` are removed; the field-order comments are
  deleted/rewritten.
- `crates/vorma-tasks/src/task.rs` — `run_in_exec_ctx` regains its poll-once fast path:
  the call future is pinned and polled once with `Waker::noop()`; on `Poll::Ready` the
  outcome is used directly and the `tokio::select!` cancellation subscription is skipped;
  on `Poll::Pending` it drops into the same pinned future's `select!` (the pre-existing
  cancellation branch, unchanged).
- `crates/vorma-tasks/Cargo.toml` — `blake3` removed from `[dependencies]` (workspace-level
  entry left alone; other crates still use it).
- `docs/maintainer/bench-results/vorma-tasks/sjc-z390-aorus-pro-wifi.bench.results.txt` —
  re-recorded via `make bench-tasks` (post-restoration numbers).
- `docs/maintainer/fable/STATE.md` — Linux tasks table replaced with the P002 before/after
  recording; regression-causes note updated to resolved; fingerprint-hasher open flag
  closed; "where work stands" updated; mac-side "beat Go" re-recording flagged as the
  remaining acceptance check.

`crates/vorma-tasks/src/parallel.rs` was modified during step 3 and then fully reverted;
it is byte-identical to HEAD (verified `git diff HEAD` empty).

## Decisions made

1. **`TaskId` mixed into the fingerprint hash (was blake3-over-input-only).** The packet
   directs writing the `TaskId` value first, then the input. This changes the hash value
   but not observable behavior: `KeyFingerprint` still carries `task_id` and `input_type`
   as `Eq`-compared fields, and slot equality is always verified on the full typed key
   (`Slot::matches` / `PathKey::matches` downcast and compare the actual input). Hash
   changes only bucket routing. The `equal_hashes_still_use_input_equality` test
   (`COLLISION_TASK`, inputs engineered to collide on the fingerprint hash) is the direct
   guard and stays green.

2. **`SiblingTask`/`PreparedSibling` single-box refactor implemented, measured, and
   reverted (step 3).** Diagnosis said ParallelBatch had per-sibling cost beyond spawn.
   Scratch attribution (below) showed the largest plausible internal lever was the
   double-box in `add`/`run` (an outer `Box<dyn FnOnce>` closure that returns a boxed
   future). I implemented a trait-object design that spawns the sibling's concrete future
   directly (one box). A direct A/B in a scratch harness with real task resolves at
   tasks-2 and tasks-50 scale measured the "single-box" trait-object approach 5-9% SLOWER
   than the existing closure double-box (tasks-50: ~51.4µs vs ~47.5µs across two runs). The
   in-workspace bench agreed (tasks-50 ticked up). Per the benchmark-attribution doctrine —
   a mechanical change that does not measurably help is not kept — I reverted to the
   original. The earlier trivial-future micro-bench that suggested a win was not faithful
   to the required `Vec<Box<dyn ...>>` storage + dispatch; the faithful A/B is
   authoritative. No public API or semantics changed in either direction.

3. **No further ParallelBatch reduction applied.** The attribution shows the per-sibling
   cost is dominated by the irreducible `tokio::spawn` floor (~2.4µs/task on this box,
   maintainer-accepted) plus sub-100ns structural plumbing (output mutex ~44ns/sibling,
   child-ctx alloc ~112ns/batch) that is structurally required by the ratified
   spawned-parallelism + heterogeneous-output design and is invisible against the floor.
   The material per-sibling recovery came from cause 2 (poll-once eliminates the
   per-sibling cancellation-subscription setup), which is why the parallel rows improved.
   Forcing an unmeasurable micro-change here would be the exact anti-pattern LEARNINGS
   warns against.

4. **Comment style / DRY.** The `write` passthrough comment and the fingerprint-map comment
   were rewritten (not just deleted) to describe the new keyed-SipHash reality, per the
   packet's instruction to delete the stale field-ordering comment. No conversational or
   changelog comments were added.

No other judgment calls were made. No semantic changes, no API changes, no new
dependencies, no unsafe, no git actions.

## Gate results

All run on the Linux machine at the final code state (steps 1+2 applied, step 3 reverted).

- **Workspace tests** — `cargo test --workspace --all-targets`: all `test result: ok`,
  0 failed (vorma-tasks: 48 integration + lib tests ok; vorma 215 ok; vorma-build 134 ok;
  every other crate ok). `cargo test --workspace --doc`: ok, 0 failed.
  - vorma-tasks named pin tests explicitly re-run and green:
    `cross_exec_ctx_runner_respects_cancellation_set_before_success_return`,
    `cancellation_results_are_not_memoized_inside_one_exec_ctx`,
    `waiting_on_local_work_respects_cancellation`; plus the four `parallel_*` tests and
    `equal_hashes_still_use_input_equality`.
- **clippy** — `cargo clippy --workspace --all-targets -- -D warnings`: clean, exit 0.
- **fmt** — `cargo fmt --all --check`: clean, exit 0.
- **loom** — `make loom-tasks` (`RUSTFLAGS="--cfg loom" LOOM_MAX_PREEMPTIONS=3 cargo test
  -p vorma-tasks --lib --release`): 7/7 models pass, unweakened. (key.rs compiles under
  loom; it was re-run green after each of steps 1, 2, and the step-3 revert.)

The slot wait/notify protocol was not touched; no new loom transitions were introduced.

## Benchmarks

Recorded via `make bench-tasks` on the idle Linux machine (verified 99.0% CPU idle over a
2s sample immediately before recording; no concurrent builds/tests/agents). ns/op.

**Linux before/after (same machine, authoritative for P002):**

| Row                        | Before (regressed) | After (restored) | Delta   |
| -------------------------- | ------------------ | ---------------- | ------- |
| single_task                | 404.2              | 313.5            | -22.4%  |
| parallel_independent_tasks | 9,100              | 7,942            | -12.7%  |
| high_contention            | 11,136             | 11,520           | +3.4%   |
| task_with_dependencies     | 776.6              | 596.4            | -23.2%  |
| allocations                | 451.7              | 348.7            | -22.8%  |
| parallel_scaling tasks-1   | 661.0              | 525.3            | -20.5%  |
| parallel_scaling tasks-2   | 7,598              | 6,821            | -10.2%  |
| parallel_scaling tasks-5   | 12,216             | 10,647           | -12.8%  |
| parallel_scaling tasks-10  | 16,623             | 15,332           | -7.8%   |
| parallel_scaling tasks-20  | 27,175             | 23,453           | -13.7%  |
| parallel_scaling tasks-50  | 56,437             | 48,884           | -13.4%  |
| context_cancellation       | 6,771              | 5,031            | -25.7%  |
| repeated_task_calls        | 126.0              | 75.87            | -39.8%  |

Every row improved except `high_contention` (+3.4%). That row is dominated by ten
`tokio::spawn` calls in the benchmark harness itself (Go-goroutine fan-out mirror); +3.4%
is within its run-to-run variance (readings across the session: 10933 / 11049 / 11103 /
11520 / 11554, baseline 11136), and it carries the maintainer-accepted structural
explanation. `context_cancellation` already wins by design (preemptive cancellation vs Go
running the body to completion); it also improved as a side effect of the poll-once change.

Cross-machine reference only (mac M3 Max, still the pre-restoration regressed shape — NOT
re-recorded post-P002; the mac "beat Go" bar is verified by the maintainer's next mac
recording): Go / pre-gap / staged-now columns live in STATE.md and must not be compared to
Linux numbers in either direction.

**Per-cause attribution (same machine, staged before/after; ns/op):**

Cause-isolating readings taken between the mechanical steps (scratch redirects of the
release bench binary; the official recording above is the make-target output at the final
state).

| Row                        | Baseline | After cause 1 (SipHash) | After cause 2 (poll-once) |
| -------------------------- | -------- | ----------------------- | ------------------------- |
| repeated_task_calls        | 122.5    | 75.59 (-38%)            | 76.14 (flat)              |
| single_task                | 404.8    | 361.0 (-11%)            | 313.3 (-13%)              |
| task_with_dependencies     | 799.3    | 694.3 (-13%)            | 597.4 (-14%)              |
| allocations                | 454.9    | 403.2 (-11%)            | 351.5 (-13%)              |
| parallel_scaling tasks-1   | 675.5    | 618.8 (-8%)             | 529.1 (-14%)              |
| parallel_scaling tasks-50  | 56,568   | 51,068 (-10%)           | 48,175 (-6%)              |

Cause-level proof:
- Cause 1 (blake3 -> keyed SipHash): cleanest on `repeated_task_calls` (a pure ExecCtx-memo
  hit that returns at slot-claim and never enters `run_in_exec_ctx`): -38%, 122.5 -> 75.6
  ns. blake3's ~50ns init+finalize on the resolve hot path is gone. The single-run rows
  (`single_task`, `task_with_dependencies`, `allocations`) each dropped ~11-13%.
- Cause 2 (poll-once fast path): `repeated_task_calls` is flat (76.1 vs 75.6) — correct
  negative control, that row never reaches `run_in_exec_ctx`. Immediately-ready bodies
  dropped a further ~13-14% (`single_task` 361 -> 313; `task_with_dependencies` 694 -> 597;
  `allocations` 403 -> 352; `parallel_scaling tasks-1` 619 -> 529). The per-sibling
  subscription setup this removes is also why the multi-task parallel rows improved.

**ParallelBatch per-sibling attribution (two-task batch; scratch micro-benches, ns/op).**
Absolute numbers are from an out-of-workspace scratch project (default bench profile,
opt-level 3); internally consistent for attribution, not comparable 1:1 to the in-workspace
bench.

| Component                                             | Cost (2-task) | Notes                                          |
| ---------------------------------------------------- | ------------- | ---------------------------------------------- |
| Full 2-task `ParallelBatch::run`                     | ~7,531        | the thing being attributed                     |
| Raw `tokio::spawn` of 2 futures + JoinSet join       | ~4,807        | irreducible spawn/schedule/join floor (~2.4us/task) |
| 2 task resolves, serial, in a child ctx (no spawn)   | ~632          | required resolve/memo-miss/output work         |
| `ctx.child()` (child cancel-token alloc)             | ~112          | once per batch, not per sibling                |
| Output-slot mutex (2x `Arc<Mutex<Option<Arc<O>>>>`)  | ~88           | ~44 ns/sibling                                 |
| `exec_ctx` create (root token) baseline              | ~60           | shared fixed cost                              |
| Single-box refactor vs double-box, tasks-2           | +261 slower   | trait-object single-box measured SLOWER        |
| Single-box refactor vs double-box, tasks-50          | ~4,000 slower | ~8-9% slower at scale -> reverted              |

Reading: of the ~7.5us two-task batch, ~4.8us is the raw spawn floor (accepted), ~0.63us
is required resolve work, and the remainder is small structural plumbing. The one lever
that looked reducible (collapsing the closure double-box) measured WORSE in a faithful A/B
(the trait-object's vtable dispatch + a separate non-inlined sibling future produce worse
codegen than the closure, whose returned future inlines/localizes better), so it was
rejected. What was recovered on the parallel rows is cause 2 (the per-sibling
cancellation-subscription setup): `parallel_independent_tasks` -12.7%,
`parallel_scaling tasks-{1,5,20,50}` -20.5 / -12.8 / -13.7 / -13.4%.

## Escalations / open questions

None. Public API and observable semantics are unchanged; all constraints held. The only
remaining acceptance item is process, not code: the absolute "beat Go" bar is defined and
verified on the mac, so the maintainer's next `BENCH_MACHINE_ID=m3-max make bench-tasks`
re-recording is the confirmation that the mac rows beat Go post-restoration. This packet
made the Linux deltas that should make that verification succeed (flagged in STATE.md).

Note on the working tree vs git index: my changes vs HEAD are exactly `key.rs`, `task.rs`,
and `Cargo.toml` (`parallel.rs` reverted to HEAD). A harness checkpoint snapshotted some of
these into the git INDEX mid-session; I took no git actions. The working tree is the source
of truth. This mirrors the pre-existing P001b index caveat already noted in STATE.md open
flags — stage from the working tree before committing.

## Discovered out-of-scope work

None. No new tickets filed. All scratch attribution experiments were kept under `/tmp`
(a throwaway cargo project depending on `vorma-tasks` by path) and removed before
completion; nothing scratch was committed or left in the tracked tree.
