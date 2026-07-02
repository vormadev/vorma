# vorma-tasks memo-hit hot path: repeated_task_calls has never beaten Go

Standing perf headroom identified at the P002 mac verification (2026-07-01).

## Facts

`repeated_task_calls` is the pure ExecCtx memo-hit row: the task has already run in this
execution context, so a resolve is fingerprint → slot lookup → return the retained output.
No task body, no `run_in_exec_ctx`.

| Recording                                   | ns/op |
| ------------------------------------------- | ----- |
| Go (M3 Max, old repo)                       | 17.03 |
| Pre-gap Rust (M3 Max, last known-good)      | 23.3  |
| Post-P002 Rust (M3 Max, 2026-07-01)         | 36.9  |
| Post-P002 Rust (Linux i9-9900K, 2026-07-01) | 75.87 |

P002 recovered the blake3 regression on this row (95.4 → 36.9 on the mac, −61%), but the
row has NEVER beaten Go — even the pre-gap best (23.3) lost to Go's 17.0. Every other
non-spawning row now beats Go, so this is the one remaining hot-path gap with no
structural excuse.

## Where the time plausibly goes (start from P002's attribution data)

Costs on the memo-hit path to measure and attack, roughly in suspected order:

- Keyed SipHash fingerprint of the input (`fingerprint_for`, ~10-15ns on small inputs) —
  required for DoS resistance (see LEARNINGS: task inputs are attacker-influenceable); any
  change must keep a keyed, flooding-resistant hash.
- Fingerprint map lookup + full typed-key equality verification (`Slot::matches`
  downcast + input compare).
- Output retention plumbing (Arc clones, slot state checks, lock/entry discipline in the
  store).
- Observer/clock gating (should already be presence-gated per the performance conventions;
  verify it truly is on this path).
- Cancellation-token check(s) on the resolve path.

## Constraints

- Observable semantics and public API frozen (memo retention rules, error retention,
  single-flight coalescing all stay exactly as pinned by the test suite).
- The fingerprint hash stays keyed and flooding-resistant (LEARNINGS doctrine).
- Loom models stay green if any slot-protocol-adjacent code is touched
  (`make loom-tasks`).
- Measurement per the benchmark-attribution doctrine: one mechanical change per measured
  step, same-machine before/after, recorded only via `make bench-tasks`.

## Target

Linux same-machine improvement with attribution, sized so the mac row lands ≤ the pre-gap
23.3 (parity-class with Go); stretch: beat Go's 17.0. The maintainer has said mac
re-recordings are not routine — treat the committed 2026-07-01 mac recording as the
reference point and rely on Linux ratios for verification.
