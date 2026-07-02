# vorma-build lib test flake under full-workspace parallel load

Observed once during P003 verification (2026-07-01, Linux i9-9900K), recorded here so the
signal is not lost. A flaky test is a real issue per repo policy — this ticket exists to
catch and fix it, not to excuse it.

## What was observed

During a full `cargo test --workspace --all-targets` run, the vorma-build lib unit suite
reported one failure:

```
     Running unittests src/lib.rs (target/debug/deps/vorma_build-...)
test result: FAILED. 133 passed; 1 failed; 0 ignored; 0 measured; 0 filtered out; finished in 5.61s
```

The failing test's NAME was not captured (the observing agent's grep passed only suite
summary lines — a lesson recorded below). Reproduction attempts:
`cargo test -p vorma-build --lib` in isolation passed 134/134 four times; two other full
workspace runs in the same session passed clean. The one failure occurred only under full
parallel `--all-targets` load, which concurrently ran the board SQLite integration tests
and the vorma-tasks bench binary in test mode — heavy process and filesystem concurrency.

## Plausible candidate class (unproven)

The vorma-build lib suite contains timing-sensitive tests that use REAL notify watchers on
temp directory trees (`src/dev_watcher.rs` tests, including the P001b pin
`real_watcher_ignores_reads_but_sees_writes`, which asserts a write-event arrives within a
bounded deadline via the test-only `recv_relevant_within`). Under heavy machine load,
inotify delivery latency could exceed a test's receive budget. This is the most likely
class; it is NOT confirmed — do not "fix" anything without first catching the actual
failing test.

## Task

1. Catch it: run the full workspace suite under load in a loop with full output captured
   (no summary-only greps — capture `test <name> ... FAILED` lines and the stdout blocks).
   A dozen runs under parallel load on a busy machine should reproduce if the candidate
   class is right; widen the net if not.
2. Once named: diagnose properly. If it is a bounded-deadline watcher test, the fix must
   NOT be a blind timeout inflation or tolerance hack (tests-never-cheat policy) —
   understand the delivery path and make the test robust by construction (e.g.
   deterministic readiness signaling) while keeping it a real end-to-end notify test.
3. Re-run the loop to demonstrate the flake is gone.

## Verification

- The full workspace suite passes repeatedly (10+ consecutive runs) under parallel load
  with zero intermittent failures.
- The fixed test still fails loudly when its condition is genuinely broken (no weakening).
