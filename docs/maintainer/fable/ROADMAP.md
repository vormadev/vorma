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
  closed); ~30 runtime-lifecycle/type-carrier items escalated as the F-17 policy ruling —
  awaiting maintainer decision (options + joint recommendation in
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
  handler-reaching row improved −6.4% to −26.9% with response bytes proven identical; e2e
  green over the optimized engine. See `packets/P004-request-path-review/REVIEW-part2.md`.

Phase B is **complete** (P003 and P004 accepted 2026-07-01).

## Phase C — Board completion

Packets authored 2026-07-01 (Fable). Dispatch order: P005 → P008 → P006 → P007 (P008 lands
its `From` conversions before P006 writes new task call sites, so the teaching code uses
the blessed `?` idiom from the start). Any large gaps P005's audit surfaces become P009+
packets authored on its report. Inputs: the census under `tickets/board-api-coverage/`,
P003's report and its post-ruling re-triage.

- `[x]` **P005 — Board census completion.** Closed 2026-07-01: full F1–F17 audit
  (`tickets/board-api-coverage/CENSUS_COMPLETION_P005.md`) — 12/17 as described, 3
  census-drift corrections, 0 missing; F9 prefetch composition and the F17 no-dead-surface
  clause landed (F3 was already landed pre-packet); two narrow gaps ruled at review (F-21
  → P008 rider; F-22 → revisit on real multi-attachment feature); stale lint ticket
  deleted. See `packets/P005-board-census-completion/REVIEW.md`.
- `[x]` **P006 — Board task-runtime teaching.** Closed 2026-07-02: session-pruning
  background worker owning its own `Tasks`/`ExecCtx`/`CancelToken` wired to graceful
  shutdown; mod-export scan with chunked `is_cancelled` cooperation and per-chunk
  `ExecCtx::child`; slow-task `TaskObserver` (+`Task::id`) shared across both runtimes.
  Census F-17 RESOLVED (exempt remainder: `TaskOverrides`, `Clock` family). +1 test (568).
  Review also ticketed two oxfmt markdown defects found in housekeeping
  (`oxfmt-markdown-corruption-and-nonconvergence`). See
  `packets/P006-board-task-runtime-teaching/REVIEW.md`.
- `[x]` **P008 — Ruled API additions: task-error exit conversions + test cookie
  continuation.** Closed 2026-07-02: concrete `From` impls landed with source chains
  preserved and the Cancelled conversion proven unreachable-on-live-paths (traced, pinned,
  doctrine-commented); `TestSession`/`TestResponseCookies` landed with the
  duplicate-Cookie-header trap caught in self-review and pinned; 12 board call sites
  converted to `?`; F-21 rider done in `public_api.rs`; both tickets deleted; +16 tests
  (567 total). See `packets/P008-api-ergonomics-exits-and-test-cookies/REVIEW.md`.
- `[x]` **P009 — Board multi-attachment submit.** Closed 2026-07-02: every multi-value
  `FormData` accessor landed with a distinct teaching role (incl. `fields_named` as
  shape-check vs `texts` as extraction); the `fields()` sweep closed a real missing
  body-length cap; F-21 document-shell call sites landed with a provably-escaping demo and
  a byte-for-byte body test; schema evolved (attachments own ids, story_tags); board tests
  20 → 28 (workspace 575); live-browser verified. Census F-21/F-22 LANDED. Follow-up
  ticket: `tsgen-drafter-oxfmt-idempotency`. See
  `packets/P009-board-multi-attachment-submit/REVIEW.md`.
- `[x]` **P007 — Board production build and serve, verified.** Closed 2026-07-02: full
  prod pipeline proven three times (build → honest prod boot → seven served-behavior
  checks → graceful worker shutdown, zero orphans). Found and fixed a real board defect
  (etag layered inner to response_body_timeout silently killed all ETags) and escalated
  the framework footgun with a minimal repro and ranked options (ticket
  `etag-response-body-timeout-ordering-footgun`, census F-23; maintainer ruling pending).
  See `packets/P007-board-prod-build/REVIEW.md`.

Phase C is **complete** (P005, P008, P006, P009, P007 accepted 2026-07-02). Board is
census-complete and prod-proven; workspace at 575 tests.

## Phase D — Release-quality sweeps

- `[x]` **P010 — Middleware composition helper (etag/timeout footgun).** Maintainer-ruled
  2026-07-02 (census F-23): verify the root cause against vendored tower-http source, land
  the composed pre-ordered public helper so the silent-zero-ETags ordering is
  unrepresentable, pin it, board adopts it. Upstream contribution deliberately deferred
  (ticket `tower-http-timeoutbody-size-hint-upstream`, low priority, nothing filed
  externally). Closed 2026-07-02: root cause re-verified against vendored sources;
  `response_body_timeout_with_etag` landed pair-scoped with five pins; board adopted with
  a corrected teaching comment; live prod verification passed; +5 tests (580). See
  `packets/P010-middleware-etag-composition-helper/REVIEW.md`.

Remaining Phase D packets (plan recorded 2026-07-02; Fable authors each on dispatch).
Shape: per-crate "release-quality pass" packets combining the public-API doc sweep
(rust-doc at the teaching bar + `deny(missing_docs)` as the durable enforcement) with the
thermo-nuclear review run in FINDINGS MODE — docs/tests/mechanical cleanups land directly;
structural code-judo findings are reported with analysis for Fable triage (each accepted
finding becomes its own granted packet), because the skill's ambition must not bypass the
frozen-semantics protocol on loom-verified, perf-tuned crates.

- `[x]` P011 vorma-matcher release-quality pass — closed 2026-07-02: 67 public items at
  the teaching bar, 17 doctests, intra-doc-link deny added; five structural findings held
  in ticket `matcher-release-quality-findings` for the batched Phase D-end surface-polish
  ruling. See `packets/P011-matcher-release-quality/REVIEW.md`.
- `[x]` P012 vorma-tasks release-quality pass — closed 2026-07-02: 60 public items to the
  bar, 15 doctests, one real coverage gap closed (parallel error tie-break race); four
  protocol-adjacent findings held in `tasks-release-quality-findings`; the vorma-build
  flake's signature captured (`AddrInUse` port collision — ticket advanced). See
  `packets/P012-tasks-release-quality/REVIEW.md`.
- `[x]` P013 vorma-contract + vorma-macros release-quality pass — closed 2026-07-02: both
  crates to the bar (+7 doctests, trybuild-verified derive contract); headline: a
  confirmed reproduced framework bug found and ticketed
  (`tsgen-shared-type-phase-name-collision` — shared input+output types fail app boot;
  maintainer design decision pending); one DRY consolidation landed; findings held for
  batch triage. See `packets/P013-contract-macros-release-quality/REVIEW.md`.
- `[x]` P014 vorma-build release-quality pass — closed 2026-07-02: 433 items to the bar
  (crate provably externally-unreachable except `run`, documented as such); the flake
  ROOT-CAUSED (TOCTOU in the test-port allocator, 64-thread reproduction) with the
  test-scoped retry fix ruled and granted on the ticket; findings held in
  `build-release-quality-findings`. See `packets/P014-build-release-quality/REVIEW.md`.
- `[x]` P015 vorma + vorma-client-wasm release-quality pass — closed 2026-07-02: full
  app-facing surface to the bar (doctests 2→11), client-wasm ABI contract documented;
  lock_effects consolidation landed under P004's byte-identity bar (7-response
  capture/diff, zero delta); findings: the app! state-path footgun (fix authorized →
  P020), dead DocumentBuildIdentity surface (held for batch triage), flake test-name
  captured (ruling scope amended). See `packets/P015-vorma-release-quality/REVIEW.md`.
- `[x]` P016 TS package release-quality pass — closed 2026-07-02: 176/176 public exports
  documented (13 entry points; adapters divergence-only incl. the caught Preact
  signal-return semantics); enforcement landed (zero-dep compiler-API checker in ts-gate,
  break-proven); generated-types verdict negative → ticket
  `tsgen-doc-comments-not-carried-to-generated-ts`; findings held in
  `ts-package-release-quality-findings`. See
  `packets/P016-ts-package-release-quality/REVIEW.md`.
- `[x]` P017 ARCHITECTURE.md accuracy pass — closed 2026-07-02: claim-by-claim audit with
  citations; two real drifts corrected (the watch-root "config-transition error" guard
  claim described a nonexistent mechanism — honest behavior now documented and the gap
  recorded on the dev-watch ticket; stale test-suite list), two facts added
  (`vorma::tasks` re-export, P004 fast path) plus the Fable-ruled P019 dedup clause.
  One-line `vite_plugin_contract.rs` rustdoc fix queued behind P018. See
  `packets/P017-architecture-accuracy/REVIEW.md`.
- `[>]` P018 packaging + supply-chain dry-runs — dispatched 2026-07-02 in parallel with
  P017 (disjoint: docs-only vs build commands).
- `[x]` P020 app! macro state-path fix — closed 2026-07-02: anchor-alias landed, all three
  previously-failing forms pinned red-to-green (591 tests), existing callers proven
  unchanged; one rustdoc-synthetic-main edge case root-caused with an isolated rustc repro
  and fixed at the doctest. Finding 1 RESOLVED. See
  `packets/P020-app-macro-state-path-fix/REVIEW.md`.
- `[>]` P019 TsGen shared-type dedup — the ruled name-collision fix (maintainer-approved
  2026-07-02: one exported TS type per name on structural phase agreement; teaching error
  on genuine divergence). Dispatched 2026-07-02 in parallel with P014 (disjoint crates).
  Includes the P013 coverage rider (serde(transparent), default-path).

Original phase description:

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
