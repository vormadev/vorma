# P002 Review

Reviewer: Fable (orchestrator). Date: 2026-07-01. Executor: Opus subagent dispatched by
Fable under maintainer authorization (Linux machine, per the 2026-07-01 re-base ruling).

## Verdict

**Accepted. Packet closed** (with one standing maintainer follow-up: the mac-side "beat
Go" re-recording, which is process, not code). The three diagnosed causes are resolved or
honestly attributed, the measurement discipline was exemplary, semantics and API are
untouched, and every gate is green — all re-verified independently by Fable.

## Independent verification performed (Fable)

- **Diff audit vs HEAD:** exactly `key.rs`, `task.rs`, `Cargo.toml` (blake3 dropped from
  the crate manifest — grep confirms zero references). `parallel.rs` is byte-identical to
  HEAD (0 diff lines) — the reverted step-3 refactor left no residue. No scratch files in
  the tree.
- **key.rs:** process-global `LazyLock<RandomState>` (keyed SipHash-1-3), `TaskId` mixed
  before the input hash, manual `Hash` on `KeyFingerprint` writing only the precomputed
  u64 — the passthrough `FingerprintHasher` is now structurally correct and the stale
  field-order comment is gone. Matches the packet spec line for line and satisfies the
  LEARNINGS DoS-surface doctrine.
- **task.rs:** the poll-once fast path is the LEARNINGS-ratified technique verbatim — pin,
  one noop-waker poll, `Ready` short-circuits the select, `Pending` falls into the same
  pinned future's select. The in-code semantics argument is correct: a body that finishes
  on its first poll never suspended, so the before-run cancellation check and the
  post-outcome handling cover every observable case. The three named cancellation pins
  and all loom models pass untouched.
- **Gates re-run by Fable:** workspace tests all-targets + doc — zero non-ok summary
  lines; fmt clean; clippy `-D warnings` clean; loom 7/7.
- **Recording:** the per-machine file matches the REPORT table row for row and is a pure
  `make bench-tasks` redirect (header + 13 rows).
- **Reproducibility spot-check:** an independent bench run (direct, not touching the
  recording) reproduced every row within a few percent — headline
  `repeated_task_calls` 75.07 vs recorded 75.87. The two structurally noisy rows behave
  exactly as documented (`high_contention` spawn-bound in the harness;
  `context_cancellation` timer-race-shaped, still far below its baseline).
- **STATE.md updates:** accurate — before/after table, cause-resolution note,
  fingerprint-hasher flag closed, mac verification flagged.

## Findings

No issues found.

Worth recording as review commentary (not defects): the step-3 handling is the packet
protocol working at its best — the plausible single-box refactor was implemented,
A/B-measured with a faithful harness, found 5–9% slower at scale, reverted, and the
earlier misleading micro-bench was explained. The attribution table (spawn floor
~2.4µs/task, resolve work ~0.6µs, sub-100ns plumbing) is durable knowledge for future
ParallelBatch work.

## Standing follow-up (maintainer)

The absolute bar — beat Go on every row except the two accepted structural rows — is
defined and verified on the mac. Whenever convenient, on the mac:
`BENCH_MACHINE_ID=m3-max make bench-tasks`, then compare against the Go column in
STATE.md's mac table. The Linux deltas (−10% to −40% on the affected rows) are what that
verification will be inheriting.
