# P012 — vorma-tasks release-quality pass (docs + thermo-nuclear findings)

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md` (the tasks doctrine and the loom/wait-notify sections are load-bearing),
`STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and the review bar:
`docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`. Model packet: P011's
INSTRUCTIONS/REPORT/REVIEW (the matcher pass — same shape, tasks-specific constraints
below).

## Context

Same Phase D shape as P011: rust-doc is the user documentation; the crate is sovereign,
post-review (P002 restored perf; loom verifies the wait/notify protocol; the parallel
contract and cache-policy semantics are maintainer-ratified). This pass documents to the
bar and audits against the thermo-nuclear standard in FINDINGS MODE.

## Scope

1. **Doc sweep.** Every public item at the teaching bar. The tasks doctrine belongs in the
   public docs where users meet it, user-facing: the three cache policies and their exact
   retention semantics (memoized retains non-cancellation errors; extended_cache never
   retains errors; cancelled-run successes are discarded; cycle detection errors instead
   of deadlocking), the single-flight coalescing story, true spawned parallelism and when
   ParallelBatch earns its spawn cost, cooperative cancellation, the observer surface, and
   the `task!` static-node model (capture-free bodies, Copy handles).
   `#![deny(missing_docs)]` + `#![deny(rustdoc::broken_intra_doc_links)]` if not already
   present; doctests where they genuinely teach (doctests that construct runtimes must be
   cheap and deterministic — no timing races; tests-never-cheat applies to doctests too).
2. **Thermo-nuclear review, FINDINGS MODE.** Directly landable: docs,
   coverage-strengthening tests, zero-behavior private mechanical cleanups. Everything
   else is an analyzed finding — especially anything near the slot wait/notify protocol,
   the fingerprint path (keyed SipHash is a DoS-surface decision, frozen), the poll-once
   fast path, or ParallelBatch internals (P002's A/B history binds: one plausible refactor
   already measured slower and was reverted — read that report before proposing it again).
   The memo-hit hot path has a standing headroom ticket (`tasks-memo-hit-hot-path`) —
   findings touching it should reference, not duplicate.
3. **Checklist verdict** item by item with evidence.

## Hard constraints

- Observable semantics FROZEN. **Any touched line in this crate requires `make loom-tasks`
  green**, and nothing may add paths the loom build cannot see; concurrency-protocol code
  should simply not be touched by this packet's granted class.
- No public API changes; no bench recordings (direct unrecorded runs only if any code is
  touched; revert on regression — the P002 baselines are not yours to move).
- Tests never cheat; scoped fmt writes only; no git actions; no network installs;
  unexpectedly dirty unowned files are escalations, never cleanup.

## Definition of done

- deny attributes landed and green; `RUSTDOCFLAGS="-D warnings" cargo doc` clean; doctests
  pass.
- Findings report + checklist verdict.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check,
  `make loom-tasks` (mandatory-green, not just untouched), direct unrecorded tasks bench
  run showing no regression if code was touched.
- REPORT.md per template (or in-message on guardrail).
