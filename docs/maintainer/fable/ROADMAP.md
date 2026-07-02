# Roadmap to Release

Sequenced plan, owned by Fable. Executors take packets from `packets/` in order unless the
maintainer redirects. A packet is done when its `REPORT.md` exists, its definition of done
is met, and Fable's `REVIEW.md` says accepted.

Status legend: `[ ]` not started · `[>]` in progress · `[R]` awaiting review · `[x]`
accepted.

## Phase A — Stabilize the baseline

Goal: every gate green, the tasks-crate performance bar restored, the recorded state
honest. Nothing else proceeds on a red or dishonest baseline.

- `[x]` **P001 — Green all gates and record the baseline.** Closed 2026-07-01: all gates
  green on Linux and recorded in STATE.md (rust-gate all steps, ts-gate, e2e-smoke
  aggregate exit 0; 549 workspace tests matching the macOS-era count). See
  `packets/P001-green-all-gates/REVIEW.md` for the full run and rulings record.
- `[x]` **P001b — Fix: Linux dev loop never rebuilds on source change.** Closed
  2026-07-01: root cause was inotify access-event feedback (compiler reads classified as
  source changes, cancel-restarting rebuilds forever); fixed with the event-kind gate in
  `dev_watcher.rs`, pinned red-before/green-after. See
  `packets/P001b-linux-dev-rebuild-fix/REVIEW.md`. Discovered follow-up ticketed:
  `dev-watch-excludes-not-pruned-from-os-watch`.
- `[x]` **P002 — Tasks perf restoration.** Closed 2026-07-01: blake3 replaced with keyed
  SipHash on the resolve hot path, poll-once fast path restored, ParallelBatch overhead
  attributed by measurement (residual = accepted spawn floor; the one plausible reduction
  A/B-measured slower and reverted), fingerprint hasher hardened. Every Linux row improved
  except the spawn-bound `high_contention` (−10% to −40%; `repeated_task_calls` 126.0 →
  75.87 ns). See `packets/P002-tasks-perf-restoration/REVIEW.md`. Mac verification
  recorded 2026-07-01: all non-spawning rows beat Go; verdict, the parallel-family
  structural exemption (maintainer-ratified 2026-07-01), and the memo-hit headroom ticket
  are in STATE.md's tasks benchmark section.

Phase A is **complete** (P001, P001b, P002 accepted; gates green; baselines recorded and
honest).

## Phase B — Finish the standing reviews

- `[x]` **P003 — Board 100% API coverage audit.** Closed 2026-07-01: ~200-item inventory
  crossed against board with citations
  (`tickets/board-api-coverage/INVENTORY_VS_BOARD_P003.md`); 12 items newly covered
  (single_flight task, ViewExit::with_source, raw request accessors, append_header, the
  testing quartet); the extended_cache "orphan" was already covered (stale premise,
  closed); ~30 runtime-lifecycle/type-carrier items escalated as the F-17 policy ruling
  — awaiting maintainer decision (options + joint recommendation in
  `packets/P003-board-api-coverage/REVIEW.md`). Census F-16/17/18 appended. Follow-up
  tickets: `vorma-build-lib-test-flake-under-load`.
- `[>]` **P004 — Request-path performance review.** The vorma engine review: measure from
  the moment a request becomes owned by vorma to the moment vorma returns its result
  (adapter excluded), find matcher-review-class costs, fix within frozen semantics, add
  `bench-engine` recording. Consumes ticket `router-request-path-review`. Part 1
  (fixture + baseline recording) accepted 2026-07-01
  (`packets/P004-request-path-review/REVIEW-part1.md`; Sonnet 5 executor) — headline:
  handler-reaching rows cost 24–71µs vs 2–4µs for non-handler rows, so the weight is
  decode/execute/finalize, not matching. Part 2 (Fable's ANALYSIS.md plan: six ranked
  mechanical steps) closed 2026-07-01: four steps kept with faithful A/Bs, two proven
  unimplementable within frozen surfaces (design tickets filed:
  `engine-view-output-ownership`, `contract-borrowed-element-construction`). Every
  handler-reaching row improved −6.4% to −26.9% with response bytes proven identical;
  e2e green over the optimized engine. See
  `packets/P004-request-path-review/REVIEW-part2.md`.

Phase B is **complete** (P003 and P004 accepted 2026-07-01).

## Phase C — Board completion

Remaining board census features and production build. Packets to be authored by Fable when
Phase B closes (inputs: the census under `tickets/board-api-coverage/`, P003's report and
its post-ruling re-triage). Expected shape: one packet per census feature cluster, a
task-teaching packet for the F-17-ruled rows (background worker with its own
`Tasks`/`ExecCtx`/`CancelToken`; cooperative cancellation in a task body; `TaskObserver`
slow-task telemetry), then a prod-build packet.

## Phase D — Release-quality sweeps

To be authored as Phase C closes:

- Doc comments for every public API, per the documentation strategy in `AGENTS.md`
  (rust-doc and jsdoc are the primary user documentation). One packet per crate, plus one
  for the TS package.
- `ARCHITECTURE.md` accuracy pass against the shipped code.
- Per-crate compliance pass against
  `docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`.
- Packaging dry-runs (`make rust-package`, TS publish dry-run) and dependency policy
  review (`cargo deny`, supply-chain inventory).

## Phase E — Endgame

- create-vorma rewrite (maintainer ruling: this comes last).
- vorma.dev docs site per the strategy in `AGENTS.md` (a Vorma app embedding the generated
  API reference and the Board example).
- Release.

## Standing inputs

- `STATE.md` — current gates, numbers, and flags. Packets cite it as the baseline.
- `LEARNINGS.md` — doctrine executors must not violate.
- `docs/maintainer/tickets/` — the inbox. New discoveries file tickets; the roadmap pulls
  tickets into packets, never the reverse.
