# P001 Report

## What changed

- `examples/board/src/lib.rs` — removed a redundant `..vorma::TsGenConfig::default()`
  spread from a struct literal that already names every field of `TsGenConfig` (the clippy
  `needless_update` lint the packet targeted).
- `docs/maintainer/fable/STATE.md` — Gates section rewritten to reflect the Linux reality
  (see "Decisions made" and "Gate results"); "Open flags" gained the three P001 blockers.
- `docs/maintainer/tickets/linux-waitid-id-moved/__TICKET.md` — new ticket for the
  `vorma-build` Linux compile error discovered while running the gate.
- `docs/maintainer/tickets/vorma-tasks-trybuild-compiler-drift/__TICKET.md` — new ticket
  for the trybuild UI drift discovered while running the gate.

## Decisions made

1. **Deleted the spread, did not delete explicit fields.** The packet preferred "delete
   the spread line" and only fell back to deleting explicit fields if deletion did not
   compile. `TsGenConfig` (in `crates/vorma/src/config.rs:85`) has exactly three fields —
   `out_file`, `extra_types`, `extra_ts` — and the board literal names all three, so
   deleting the spread is both sufficient and the packet-preferred fix. Verified by
   reading the struct definition; the literal now names exactly the public fields and
   nothing else.
2. **Stopped at the board file and did not fix two out-of-scope blockers.** The packet's
   hard constraint is "changes at most the one board file (plus STATE.md)." Running the
   gate surfaced two pre-existing blockers outside that scope (details below). Per the
   packet protocol ("Stopping and reporting a blocker is success; improvising past a
   constraint is failure") and AGENTS.md ("Never change observable semantics on your own
   judgment"; the semantic-change protocol), I filed a ticket for each and stopped rather
   than touching `crates/vorma-build` or `crates/vorma-tasks` unilaterally. The two fixes
   are mechanical (a one-line loop-hoist and a one-line `.stderr` rebless) but they are
   maintainer decisions under the protocol.
3. **Did not attempt `make ts-gate` / `make e2e-smoke`.** Both targets run `pnpm install`
   via `ts-install`. `AGENTS.md` says pnpm installations fail under agent sandbox
   restrictions and must be escalated, not attempted. I escalated instead of burning time
   on a known-ENOTFOUND.
4. **Did not run the full `make rust-gate`.** Every step of `rust-gate` beyond
   `rust-fmt`/`loom-tasks` either is or depends on a workspace `--all-targets` build,
   which is blocked by the `vorma-build` compile error. Running
   `rust-build`/`rust-doc`/`rust-bench`/`rust-build-client-wasm`/`rust-package`/
   `rust-fuzz` individually would fail on the same root cause; recording eight copies of
   the same error would be noise. I ran `rust-fmt` (green), `rust-lint` (red, root cause
   below), `rust-test` (red, same root cause), and `loom-tasks` (green), and stopped.

## Gate results

### cargo fmt --all --check → GREEN

```
(no output; exit 0)
```

### make loom-tasks → GREEN (7/7 models)

```
running 7 tests
test loom_tests::old_claim_first_ordering_loses_wakeups - should panic ... ok
test loom_tests::concurrent_lookups_share_one_slot ... ok
test loom_tests::remove_then_abandon_wakes_waiters_and_reopens_future_lookup ... ok
test loom_tests::remove_then_finish_wakes_waiters_and_reopens_future_lookup ... ok
test loom_tests::abandon_wakes_waiter_and_reopens_slot ... ok
test loom_tests::claim_race_coalesces_to_one_runner ... ok
test loom_tests::register_first_ordering_never_loses_wakeups ... ok

test result: ok. 7 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s
```

### cargo clippy --workspace --all-targets -- -D warnings → RED (blocker #1)

The board `needless_update` lint is fixed, but a pre-existing compile error in
`vorma-build` now surfaces first and blocks the whole workspace `--all-targets` build on
Linux:

```
error[E0382]: use of moved value: `id`
 --> crates/vorma-build/src/process_runner.rs:458:16
    |
456 |     let id = Id::Pid(nix::unistd::Pid::from_raw(pid as i32));
    |         -- move occurs because `id` has type `nix::sys::wait::Id<'_>`, which does not implement the `Copy` trait
457 |     loop {
    |     ---- inside of this loop
458 |         match waitid(id, WaitPidFlag::WEXITED | WaitPidFlag::WNOWAIT) {
    |                      ^^ value moved here, in previous iteration of loop

error: could not compile `vorma-build` (lib) due to 1 previous error
```

This error is invisible on macOS (the failing arm is `#[cfg(target_os = "linux")]`, and
macOS uses the kqueue arm), which is why the pre-P001 STATE recorded only the board lint.
Ticket: `docs/maintainer/tickets/linux-waitid-id-moved/`.

### cargo test --workspace --all-targets --no-fail-fast → RED (same blocker #1)

Fails to compile `vorma-build` with the identical `E0382` shown above; no tests run.

### cargo test --workspace --doc --no-fail-fast → RED (same blocker #1)

Fails to compile `vorma-build` with the identical `E0382`; no doc tests run.

### cargo test -p vorma-matcher --all-targets → GREEN (sovereign crate)

```
test specificity_orders_competing_patterns ... ok
test flat_matching_agrees_with_compare_specificity ... ok
test result: ok. 2 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out; finished in 0.01s
(benches compile and run)
```

### cargo test -p vorma-tasks --lib → GREEN

```
running 0 tests
test result: ok. 0 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out
```

(The vorma-tasks unit tests live behind `cfg(loom)` and in the integration suite
`tests/tasks.rs`; the 48-test integration suite passes — see below.)

### cargo test -p vorma-tasks --tests → RED only on api_diagnostics (blocker #2)

The 48-test `tests/tasks.rs` integration suite passes; the `api_diagnostics` trybuild UI
test fails on rustc-1.96 E0599 wording drift:

```
test tests/ui/task_new_removed.rs ... mismatch
EXPECTED: no function or associated item named `new` found for struct `Task<I, O, E>`
ACTUAL:   no associated function or constant named `new` found for struct `Task<I, O, E>`
test task_api_rejects_removed_or_ambiguous_declaration_forms ... FAILED
test result: FAILED. 0 passed; 1 failed; ...
```

The semantic assertion (`Task::new` is rejected with E0599) still holds; only the
diagnostic wording drifted. Ticket:
`docs/maintainer/tickets/vorma-tasks-trybuild-compiler-drift/`.

### make rust-gate → NOT COMPLETED (blocked by #1)

`rust-fmt` green; `rust-lint`/`rust-test` red (#1); `loom-tasks` green. The remaining
steps (`rust-build`, `rust-doc`, `rust-bench`, `rust-build-client-wasm`, `rust-package`,
`rust-fuzz`) all depend on a workspace `--all-targets` build that cannot compile until #1
is fixed. Not run individually to avoid repeating the same root-cause failure eight times.

### make ts-gate → NOT RUN (sandbox blocker)

Requires `pnpm install` via `ts-install`. `AGENTS.md` mandates escalation rather than
attempting pnpm under agent sandbox restrictions. Escalated; not attempted.

### make e2e-smoke → NOT RUN (sandbox blocker)

Depends on `ts-build` → `ts-install` → `pnpm install`. Same sandbox blocker as `ts-gate`.
Escalated; not attempted.

## Benchmarks

Not applicable to P001. Per the packet: "Do not touch `bench.results.txt` files; benchmark
recording belongs to P002." Note: the sovereign-crate bench targets compile and run
incidentally as part of `cargo test --all-targets` (matcher and tasks), but no recording
was performed and `bench.results.txt` files were not modified.

## Escalations / open questions

Three blockers require maintainer rulings before the Linux gate can go green. P001 cannot
complete "all gates green" inside its one-board-file scope; this is a stop-and-escalate,
not a failure of execution.

1. **`vorma-build` Linux compile error (ticket `linux-waitid-id-moved`).**
   `crates/vorma-build/src/process_runner.rs:458` reuses a non-`Copy` `nix::sys::wait::Id`
   across loop iterations. This is a clear compile error on Linux (not a judgment call),
   and the fix is mechanical (hoist the `let id` into the loop body), but it is outside
   P001's allowed file set and touches `vorma-build` process-waiting code, which the
   `windows-child-exit-watcher` ticket suggests is sensitive territory. Maintainer ruling
   requested: may a follow-up packet apply the one-line hoist fix, or does the maintainer
   want to review the Linux exit-watcher design more broadly first?

2. **`vorma-tasks` trybuild UI drift (ticket `vorma-tasks-trybuild-compiler-drift`).** The
   pinned `.stderr` for `task_new_removed` predates rustc 1.96's E0599 wording change. The
   test still proves what it intends (`Task::new` is rejected); only wording drifted.
   Maintainer ruling requested: bless the new wording (one-line `.stderr` update), or take
   the opportunity to make the UI tests rustc-version-resilient?

3. **TS gate and e2e-smoke.** These require `pnpm install` and cannot run under the agent
   sandbox. Maintainer run needed on an unrestricted machine. This is the same step
   STATE.md previously called "not verified since the gap work landed"; it remains
   unverified after P001 because the sandbox constraint is not P001-specific.

A fourth, softer question: STATE.md's pre-P001 gates claim ("549 passed, 0 failed") was
recorded from a macOS run and did not reflect the Linux reality. The new STATE.md Gates
section records the Linux reality honestly; if the maintainer also wants a fresh macOS run
recorded for cross-platform parity, that is a separate ask (this agent is on Linux).

## Discovered out-of-scope work

- `docs/maintainer/tickets/linux-waitid-id-moved/__TICKET.md` — `vorma-build` Linux
  compile error (see escalation #1).
- `docs/maintainer/tickets/vorma-tasks-trybuild-compiler-drift/__TICKET.md` — trybuild UI
  stderr drift on rustc 1.96 (see escalation #2).

Both are blockers for the workspace gate on Linux/rustc-1.96 and should be resolved before
P002 (which needs `cargo bench -p vorma-tasks` to compile clean and the workspace to
build) and before P003/P004 (which exercise `vorma-build`).
