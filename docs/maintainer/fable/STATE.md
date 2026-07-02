# Current State

Perishable snapshot. Update whenever a packet lands. Last updated: 2026-07-01 after P002
executor pass (tasks perf restoration, Linux) — Linux (Ubuntu 26.04, rustc 1.96.0, glibc
2.43). Prior update: P001 + P001b closed (Fable). All Rust gates green
(tests/clippy/fmt/loom); vorma-tasks Linux benches re-recorded post-restoration.

## Gates

Recorded 2026-07-01, P001 closeout; re-verified in full at the P003 review (same day:
fmt/clippy clean, workspace tests green twice at **551** — the +2 are P003's board tests —
loom 7/7, full `make ts-gate` green with the maintainer docs now oxfmt-normalized, full
`make e2e-smoke` aggregate exit 0). One-time observation: a single `vorma-build` lib test
failure under full parallel load in one executor run, never reproduced across six
subsequent full/isolated runs — ticket `vorma-build-lib-test-flake-under-load`. Original
P001-closeout detail follows.

- `make rust-gate`: **GREEN — all steps.**
    - fmt: clean (workspace + fuzz manifests).
    - policy: `cargo audit` green with one allowed unsound warning (RUSTSEC-2026-0190,
      anyhow 1.0.102 — ticket `anyhow-rustsec-2026-0190-upgrade`); `cargo deny`
      advisories/bans/licenses/sources all ok. First policy run recorded on this machine.
    - clippy `--workspace --all-targets -- -D warnings`: clean. The board
      `needless_update` fix is now machine-verified (the P001 executor could only verify
      it textually while the workspace was compile-blocked).
    - tests: `cargo test --workspace --all-targets` + `--doc` — **548 tests + 1 doctest =
      549 passed, 0 failed**, matching the prior macOS recording exactly.
    - loom: **7/7 models pass**.
    - build / doc (`-D warnings`) / bench-compile: clean.
    - client-wasm: builds via `wasm-opt` (binaryen 130); regenerated the tracked artifact
      `packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm` (81228 → 81070 bytes vs
      the macOS-built commit — reproducibility ticket
      `wasm-artifact-binaryen-version-pinning`).
    - package: all six crates package clean.
    - fuzz: both targets (`matcher_patterns`, `client_wasm_protocol`), 4096 runs each, no
      findings. Machine caveat: this box needs `RUSTFLAGS='-Clinker=/usr/bin/gcc'` for the
      fuzz step because miniconda's conda-forge `cc` shadows the system toolchain (ticket
      `linux-conda-cc-shadowing-fuzz-link`); every other step runs unmodified.
- `make ts-gate`: **GREEN** — pnpm install; oxfmt (write mode, normalized the tree);
  oxlint exit 0 with 10 pre-existing type-aware warnings (ticket
  `ts-lint-react-redundant-type-warnings`); tsgo typecheck green for all 8 projects;
  vitest **851/851 tests across 42 files**.
- `make e2e-smoke`: **GREEN — full aggregate, exit 0** (after the P001b fix; the initial
  P001 run found test-dev-changes deterministically red on Linux). test-prod green for
  react, preact, and solid (full Bombadil browser property suites against Chrome 148
  headless); test-dev (react) green; test-dev-changes (react) green including the
  previously-failing `server Rust rebuild observed` step. Root cause was inotify
  access-event feedback (see P001b review and the LEARNINGS gotcha); fixed by the
  event-kind gate in `crates/vorma-build/src/dev_watcher.rs`.

Machine provisioning performed for this recording (user-local, reversible): wasm32
target + minimal nightly toolchain via rustup; cargo-audit 0.22.2 / cargo-deny 0.19.9 /
cargo-fuzz 0.13.2 via cargo install; `wasm-opt` 130 built from binaryen source (statically
linked) into `~/.local/bin`; Chrome for e2e = playwright chromium 148 fetched with
`PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu24.04-x64` (playwright 1.59 predates Ubuntu
26.04) and symlinked into `~/.local/bin` as chrome/chromium/google-chrome for Bombadil's
auto-detection.

Bench recordings are per-machine and additive under `docs/maintainer/bench-results/` (see
LEARNINGS). Machines on record: `m3-max` (Apple M3 Max — the mac; the Go bars were
recorded there) and `sjc-z390-aorus-pro-wifi` (Intel i9-9900K — the Linux box, the
standing perf-work machine per maintainer ruling 2026-07-01).

## Benchmarks — vorma-matcher (recorded, healthy)

Mac recording (`bench-results/vorma-matcher/m3-max.bench.results.txt`): beats the Go
baselines on every row.

| Row                                            | Go (old repo)               | Recorded now                         |
| ---------------------------------------------- | --------------------------- | ------------------------------------ |
| parse_segments                                 | 42.5                        | 15.24                                |
| flat static / dynamic / splat                  | 19.9 / 222 / 107            | 7.23 / 123.1 / 68.5                  |
| scale small / medium / large                   | 92 / 124 / 126              | 42.5 / 71.7 / 72.5                   |
| worst-case deep                                | 218                         | 123.2                                |
| nested static / dynamic / deep / splat / mixed | 166 / 657 / 745 / 590 / 536 | 69.8 / 198.8 / 266.6 / 205.9 / 193.6 |

(ns per op throughout.)

Linux recording (`bench-results/vorma-matcher/sjc-z390-aorus-pro-wifi.bench.results.txt`,
2026-07-01, same code as the mac recording — context baseline for P004-era work):

| Row                                            | Linux (i9-9900K)                    |
| ---------------------------------------------- | ----------------------------------- |
| parse_segments                                 | 20.37                               |
| flat static / dynamic / splat                  | 18.07 / 184.4 / 106.5               |
| scale small / medium / large                   | 75.8 / 107.4 / 107.2                |
| worst-case deep                                | 177.5                               |
| nested static / dynamic / deep / splat / mixed | 117 / 355.5 / 420.5 / 319.9 / 300.8 |

## Benchmarks — vorma-tasks (recorded, RESTORED — P002)

P002 eliminated the three diagnosed regression causes (keyed-SipHash fingerprints
replacing blake3 on the resolve hot path; the restored poll-once fast path; ParallelBatch
overhead attributed to the accepted spawn floor). The mac verification recording landed
2026-07-01 (`bench-results/vorma-tasks/m3-max.bench.results.txt`, maintainer-run,
committed as `7be09a88`) — the maintainer has said mac re-recordings are not routine, so
that recording is the standing mac reference.

| Row                        | Go         | Pre-gap | Pre-P002 staged | Mac now (post-P002) |
| -------------------------- | ---------- | ------- | --------------- | ------------------- |
| single_task                | 214.1      | 144.4   | 258.8           | 167.9               |
| parallel_independent_tasks | 2,405      | 589.5   | 10,901          | 12,677              |
| high_contention            | 5,922      | 13,193  | 15,298          | 16,474              |
| task_with_dependencies     | 331.4      | 263.4   | 502.5           | 321.1               |
| allocations                | 271.5      | 175.3   | 293.3           | 200.6               |
| parallel_scaling tasks-1   | 288.7      | 215.5   | 406.6           | 275.8               |
| parallel_scaling tasks-2   | 1,779      | 427.9   | 10,117          | 10,098              |
| parallel_scaling tasks-5   | 3,543      | 974.3   | 16,230          | 13,448              |
| parallel_scaling tasks-10  | 8,473      | 1,842   | 24,172          | 21,064              |
| parallel_scaling tasks-20  | 20,280     | 3,641   | 50,871          | 38,303              |
| parallel_scaling tasks-50  | 49,112     | 8,731   | 118,455         | 65,540              |
| context_cancellation       | 10,995,394 | 8,226   | 9,295           | 8,351               |
| repeated_task_calls        | 17.03      | 23.3    | 95.35           | 36.89               |

**Verdict vs the Go bar (mac, post-P002):** every non-spawning row beats Go —
`single_task` −22%, `task_with_dependencies` −3%, `allocations` −26%,
`parallel_scaling tasks-1` −4.5%, `context_cancellation` ~1300× (by design). Two families
do not:

- **The multi-task parallel family** (parallel_independent, tasks-2..50, plus the
  already-accepted `high_contention`) loses on spawn cost: goroutine spawn is ~300ns,
  tokio spawn ~2-5µs, and the ratified true-spawned-parallelism design pays it per
  sibling. Pre-gap "beat Go" on these rows only because that code was not actually
  parallel. **Maintainer-ratified 2026-07-01: the structural explanation is accepted for
  the whole parallel family** — these rows are exempt from the Go column; same-machine
  regressions against our own baselines still count (doctrine recorded in LEARNINGS,
  performance conventions). The mac parallel absolutes carry one-casual-run noise
  (parallel_independent read above the old staged value while tasks-20/50 improved
  25-45%); treat ballpark only.
- **`repeated_task_calls`** (36.9 vs Go 17.0): the pure memo-hit row; no structural excuse
  — even pre-gap (23.3) never beat Go here. Real headroom; ticket
  `tasks-memo-hit-hot-path`.

Caveats on the pre-gap column: it predates the `task!` static-node redesign, the
`ParallelBatch` API, and the restoration of spawned parallelism, so parallel rows are not
apples-to-apples (spawn cost is real and accepted); single-run and repeated rows are
directly comparable. `high_contention` is dominated by ten `tokio::spawn` calls in the
benchmark harness itself (mirroring Go's goroutine fan-out) and is the one row with an
accepted structural explanation. `context_cancellation` wins by design (preemptive
cancellation vs Go running the body to completion).

Regression causes (diagnosed and resolved by P002): (1) blake3 fingerprinting on the
per-resolve hot path — replaced with keyed SipHash (std `RandomState`); (2) the removed
poll-once fast path (every run paid cancellation-subscription setup, including a boxed
parent wait on child tokens) — restored. (3) The "unattributed per-sibling overhead in
`ParallelBatch` beyond expected spawn cost" was measured to be almost entirely cause 2
(the per-sibling subscription setup), now recovered; the residual is the irreducible
`tokio::spawn` floor (~2.4µs/task on this box, maintainer-accepted) plus sub-100ns
structural plumbing that a direct A/B proved is not safely reducible (see P002 REPORT).

**Linux — P002 before/after** (same machine `i9-9900K`, ns/op). "Before" is the P002
working baseline (regressed state, 2026-07-01); "after" is the post-restoration recording
(`bench-results/vorma-tasks/sjc-z390-aorus-pro-wifi.bench.results.txt`, 2026-07-01). All
P002 measurement is same-machine before/after; cross-machine comparison to the mac table
above is meaningless in both directions.

| Row                        | Before (regressed) | After (restored) | Delta  |
| -------------------------- | ------------------ | ---------------- | ------ |
| single_task                | 404.2              | 313.5            | −22.4% |
| parallel_independent_tasks | 9,100              | 7,942            | −12.7% |
| high_contention            | 11,136             | 11,520           | +3.4%  |
| task_with_dependencies     | 776.6              | 596.4            | −23.2% |
| allocations                | 451.7              | 348.7            | −22.8% |
| parallel_scaling tasks-1   | 661.0              | 525.3            | −20.5% |
| parallel_scaling tasks-2   | 7,598              | 6,821            | −10.2% |
| parallel_scaling tasks-5   | 12,216             | 10,647           | −12.8% |
| parallel_scaling tasks-10  | 16,623             | 15,332           | −7.8%  |
| parallel_scaling tasks-20  | 27,175             | 23,453           | −13.7% |
| parallel_scaling tasks-50  | 56,437             | 48,884           | −13.4% |
| context_cancellation       | 6,771              | 5,031            | −25.7% |
| repeated_task_calls        | 126.0              | 75.87            | −39.8% |

`high_contention` is the one row that did not improve: it is dominated by ten
`tokio::spawn` calls in the benchmark harness itself (Go-goroutine fan-out mirror), so its
+3.4% is within run-to-run variance (±4% observed across the session), not a regression.
`context_cancellation` improves as a side effect of the poll-once change and already wins
by design (preemptive cancellation vs Go running the body to completion).

## Benchmarks — vorma engine (recorded, OPTIMIZED — P004 Part 2)

Owned-request-to-response benchmarks for the `crates/vorma` runtime engine: an
already-collected `http::Request<Bytes>` in, its finished `http::Response<Bytes>` out,
through the same committed-runtime-pipeline call
(`CommittedRuntimeService::handle_request`) `vorma::testing::TestApp` exposes — no socket,
no adapter, no body-collection/limit machinery (that lives one layer up, in the tower
`Service::call` wrapper, which is adapter-facing and out of scope). There are no Go-era
baselines for this surface (the engine postdates the Go implementation entirely); Part 1's
recording was the initial baseline, Part 2 optimized against it on the same machine.

Linux recording (`bench-results/vorma/sjc-z390-aorus-pro-wifi.bench.results.txt`,
2026-07-01, i9-9900K, idle machine, post-P004-Part-2):

| Row                                        | Part 1 baseline | Part 2 (now) | Delta  |
| ------------------------------------------ | --------------- | ------------ | ------ |
| static_view_render                         | 48,798          | 43,891       | −10.1% |
| nested_dynamic_view_chain_4_deep           | 70,348          | 55,750       | −20.8% |
| json_resource_small_input_output           | 25,051          | 20,388       | −18.6% |
| resource_body_binary_resource              | 24,100          | 18,104       | −24.9% |
| request_through_middleware_chain_2         | 44,903          | 39,961       | −11.0% |
| request_through_middleware_chain_0_control | 39,500          | 31,798       | −19.5% |
| not_found_catch_all_view                   | 45,061          | 42,161       | −6.4%  |
| not_found_bare_no_catch_all                | 2,163           | 2,197        | +1.6%  |
| head_request_to_resource                   | 24,518          | 17,917       | −26.9% |
| method_not_allowed                         | 4,048           | 3,866        | −4.5%  |

Every handler-reaching row improved; the two rows that never execute a handler
(`not_found_bare_no_catch_all`, `method_not_allowed`) sit within run-to-run noise (±1-5%,
consistent with the noise band observed throughout P004), confirming the wins are
attributable to the specific mechanical changes rather than systemic drift.

Fixture (`crates/vorma/benches/engine.rs`): one in-memory `TestApp` (no build artifacts on
disk, the same harness `vorma::testing` exposes to app tests) registering a static view, a
4-level nested dynamic view chain (`/`, `/shelves/:shelf_id`,
`/shelves/:shelf_id/boards/:board_id`,
`/shelves/:shelf_id/boards/:board_id/cards/:card_id`), a JSON mutation resource, a
`ResourceBody` binary resource (4KiB payload), and a root catch-all view (`/*`). Every
route-reaching row runs an identical two-unscoped-middleware chain except the dedicated
control row, which reuses the exact same route and paths with zero middlewares registered,
isolating the middleware-chain cost as a true diff rather than an inference. A second app
variant omits the catch-all to isolate the bare-404 classifier-miss cost. Every row
rotates two realistic paths, mirroring the matcher benchmark suite's shape.

**P004 Part 2 changes (all response-byte-identical, verified by direct capture-and-diff
against every row's pre-change response; see REPORT-part2.md):**

- **Snapshot-precomputed view-response fragments (kept).** The critical-CSS `<style>`
  element, the constant root-div markup, the prod module `<script>` tag, and a per-URL
  CSS-bundle `<link>` markup cache are all pure functions of the committed
  `RuntimeManifest` alone — never of the request-varying `document` a
  `RuntimeDocumentProvider` may supply, and confirmed distinct in the framework's own
  contract (Board's document builder embeds a genuinely per-request `data-request-path`
  attribute) — so they render once at `RuntimeSnapshot::compile()` instead of once per
  request. Biggest single win: `nested_dynamic_view_chain_4_deep` (4 CSS-bundle links per
  request) and the other view/HTML rows.
- **Per-invocation clone-tree collapse (kept, modest).** `HandlerInvocation`'s fields
  moved directly into `HandlerInput` inside the spawned closure instead of being cloned a
  second time on top of the clone already made to enter the closure. Real but small and
  row-dependent (routes with dynamic params show it; static-only routes, with nothing to
  save, do not) — `Params`/`SplatValues` were already cheap for these payload sizes, as
  flagged going in.
- **Single-invocation phase fast path (kept, largest win).** When a phase has exactly one
  invocation (the overwhelmingly common shape for resource routes: one middleware phase of
  N, one handler phase of exactly 1), the `JoinSet` spawn + `join_next_with_id`
  coordination — built for arbitrating N≥2 completion order — is skipped in favor of
  polling the handler's future inline; commit logic is shared with the N≥2 path via an
  extracted helper so both apply identical rules. The N≥2 parallel contract is untouched
  (never reachable through this branch). Drove the −19% to −27% wins on every resource
  row.
- **SSR payload escape fused into serialization (kept).** `ssr_payload_json` now
  serializes through a `serde_json` formatter that escapes `&`/`<`/`>`/U+2028/U+2029
  inline as each string fragment is written, instead of serializing to a plain string and
  then copying the whole thing again through a second escaping pass. Verified against the
  reference char-by-char algorithm on adversarial UTF-8 inputs (multi-byte scripts, emoji,
  consecutive escape targets) before landing. Drove most of the remaining win on view/HTML
  rows.
- **Taking the projection's `Value`s instead of cloning them (considered, not
  implemented).** `project_view_payload` takes `report: &RouteExecutionReport` by shared
  reference and both callers read `report` again after the call, so moving
  `HandlerCommit`s out is not available without changing frozen public signatures; storing
  `Arc<Value>` internally in `HandlerOutput` cannot avoid the clone either; when accessed
  only through `&Report`, an `Arc` can never be uniquely owned at the point it would need
  to be unwrapped, so it always falls back to a deep clone — the only way to realize the
  win is to change the public wire type `ViewPayload::views_data`'s field type, out of
  scope here. The bench's payloads (17-50 bytes of JSON) are provably too small for the
  clone to register above measurement noise either way. Ticket-worthy if a real app
  profile ever shows this mattering.
- **Rendering head elements without per-call attribute clones (considered, not
  implemented).** `DocumentElementContract`'s builder API is owned-by-design and used
  pervasively across `vorma-contract`; the only lower-level rendering entry point
  (`render_element`) requires it. Redesigning that public contract type for borrowed
  construction is a broader change than this packet's scope, and — like the projection
  clone above — the actual per-request payloads here (a title, at most one or two meta
  elements) are too small to show a measurable win on this instrument.

First-read of where time goes, updated: the resource rows are now the cheapest by a wider
margin (17.9-20.4µs) thanks to the single-invocation fast path; the view/HTML rows
(31.8-55.8µs) still cost more, now primarily attributable to the SSR payload
serialize-and-escape pass and per-node view execution/effects-merging rather than the
formerly-dominant precomputable-fragment rebuilding. The 4-deep nested chain (55.8µs)
remains the single most expensive row and still scales worse than 4× a single view,
consistent with per-node decode/execute/effects-merge cost stacking across depth — the
clone-tree and projection-`Value` costs that scale with view count are the likely residual
there, the latter unrealized per the note above.

## Open flags

- **Uncommitted-work hazards (standing, two distinct mechanisms observed 2026-07-01):**
  (1) the agent harness checkpoints have staged stale content into the git INDEX (observed
  mid-P001b and again mid-P003; harmless if the index is rebuilt from the working tree —
  `git add -A` — before committing). (2) A P003 executor, cleaning up after
  `oxfmt --write .` reflowed all markdown, bulk-restored 13 "unexpectedly dirty"
  Fable-owned docs to HEAD content — destroying legitimate uncommitted orchestrator edits
  those files carried (disclosed in its report; Fable re-applied the records the same day;
  an earlier harness-revert theory of this loss was wrong). Mitigations: commit accepted
  work promptly; executors must never restore or revert files they did not author — an
  unexpectedly dirty file is an escalation, not cleanup (now in Fable's dispatch template;
  candidate line for AGENTS.md, maintainer's call).
- Notes example deleted per the board-is-canonical policy; its unique coverage (a
  TTL'd/extended-cache task exercised through an app, its request-level test suite) has no
  recorded home. Folded into the board coverage packet (P003).
- `FingerprintHasher` field-order brittleness (`vorma-tasks/src/key.rs`): **CLOSED by
  P002.** `KeyFingerprint` now implements `Hash` manually (writes only its precomputed
  `hash` u64), so the passthrough hasher is structurally correct rather than dependent on
  derived field order; the fingerprint itself is a keyed SipHash mixing task id + input.
- Lock-poisoning recovery (`unwrap_or_else(|poison| poison.into_inner())`) in the task
  store is deliberate and test-covered; not a flag, recorded so nobody "fixes" it.
- The census lives at `docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md`
  (moved there during the gap-work docs cleanup).

## Where work stands

- Matcher first-principles review: **complete** (semantics corrected and pinned, benches
  beat Go, recorded).
- Tasks first-principles review: **complete** (P002 accepted; mac verification recorded
  2026-07-01; parallel-family structural exemption maintainer-ratified 2026-07-01 —
  doctrine in LEARNINGS). Every non-spawning row beats Go. One standing loose end: the
  memo-hit hot path has a headroom ticket (`tasks-memo-hit-hot-path` — the one row that
  has never beaten Go). Everything else — API shape, loom verification, test suite —
  landed well.
- Router/request-path review: **Part 1 (baseline) and Part 2 (optimization) executor work
  complete, pending Fable review** (P004; ticket exists at
  `docs/maintainer/tickets/router-request-path-review/`). Part 2 kept 4 of ANALYSIS.md's 6
  ranked steps (the fragment precomputation, the clone-tree collapse, the
  single-invocation fast path, the fused SSR escape); 2 were considered and not
  implemented with reasoning recorded (the projection `Value` clone and per-element
  attribute clones — both provably unmeasurable at this bench's payload sizes without
  touching a frozen public signature). Every handler-reaching row improved −6% to −27%;
  response bytes verified byte-identical before/after by direct capture-and-diff.
- Board: **P003 coverage audit complete** (12 items newly covered; inventory artifact in
  the `board-api-coverage` ticket dir). Pending: the F-17 policy ruling (does the "100% in
  board" rule get the ratified "realistic app flow" reading? — options and recommendation
  in P003's REVIEW.md); census features remain Phase C work.
