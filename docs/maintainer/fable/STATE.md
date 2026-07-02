# Current State

Perishable snapshot. Update whenever a packet lands. Last updated: 2026-07-01 after
P002 executor pass (tasks perf restoration, Linux) — Linux (Ubuntu 26.04, rustc 1.96.0,
glibc 2.43). Prior update: P001 + P001b closed (Fable). All Rust gates green
(tests/clippy/fmt/loom); vorma-tasks Linux benches re-recorded post-restoration.

## Gates

Recorded 2026-07-01, P001 closeout. Everything is green except one e2e scenario, which
found a real Linux framework bug (flagged below, ticketed).

- `make rust-gate`: **GREEN — all steps.**
    - fmt: clean (workspace + fuzz manifests).
    - policy: `cargo audit` green with one allowed unsound warning (RUSTSEC-2026-0190,
      anyhow 1.0.102 — ticket `anyhow-rustsec-2026-0190-upgrade`); `cargo deny`
      advisories/bans/licenses/sources all ok. First policy run recorded on this machine.
    - clippy `--workspace --all-targets -- -D warnings`: clean. The board
      `needless_update` fix is now machine-verified (the P001 executor could only verify
      it textually while the workspace was compile-blocked).
    - tests: `cargo test --workspace --all-targets` + `--doc` — **548 tests + 1 doctest
      = 549 passed, 0 failed**, matching the prior macOS recording exactly.
    - loom: **7/7 models pass**.
    - build / doc (`-D warnings`) / bench-compile: clean.
    - client-wasm: builds via `wasm-opt` (binaryen 130); regenerated the tracked artifact
      `packages/vorma/core/client_wasm/vorma_client_wasm_bg.wasm` (81228 → 81070 bytes vs
      the macOS-built commit — reproducibility ticket
      `wasm-artifact-binaryen-version-pinning`).
    - package: all six crates package clean.
    - fuzz: both targets (`matcher_patterns`, `client_wasm_protocol`), 4096 runs each, no
      findings. Machine caveat: this box needs `RUSTFLAGS='-Clinker=/usr/bin/gcc'` for
      the fuzz step because miniconda's conda-forge `cc` shadows the system toolchain
      (ticket `linux-conda-cc-shadowing-fuzz-link`); every other step runs unmodified.
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
cargo-fuzz 0.13.2 via cargo install; `wasm-opt` 130 built from binaryen source
(statically linked) into `~/.local/bin`; Chrome for e2e = playwright chromium 148
fetched with `PLAYWRIGHT_HOST_PLATFORM_OVERRIDE=ubuntu24.04-x64` (playwright 1.59
predates Ubuntu 26.04) and symlinked into `~/.local/bin` as
chrome/chromium/google-chrome for Bombadil's auto-detection.

Bench recordings are per-machine and additive under `docs/maintainer/bench-results/`
(see LEARNINGS). Machines on record: `m3-max` (Apple M3 Max — the mac; the Go bars were
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

| Row                                            | Linux (i9-9900K)              |
| ---------------------------------------------- | ----------------------------- |
| parse_segments                                 | 20.37                         |
| flat static / dynamic / splat                  | 18.07 / 184.4 / 106.5         |
| scale small / medium / large                   | 75.8 / 107.4 / 107.2          |
| worst-case deep                                | 177.5                         |
| nested static / dynamic / deep / splat / mixed | 117 / 355.5 / 420.5 / 319.9 / 300.8 |

## Benchmarks — vorma-tasks (recorded, RESTORED — P002)

P002 eliminated the three diagnosed regression causes on the Linux machine (keyed-SipHash
fingerprints replacing blake3 on the resolve hot path; the restored poll-once fast path).
Every Linux row improved except `high_contention` (structural, spawn-bound in the harness).
The mac table below (`bench-results/vorma-tasks/m3-max.bench.results.txt`) still shows the
pre-restoration regression's shape — pre-gap is the last known-good same-machine recording,
and the maintainer bar (beat Go, not match it) was defined on that machine. **The mac-side
"beat Go" verification is pending the maintainer's next mac recording
(`BENCH_MACHINE_ID=m3-max make bench-tasks`); the mac table below is NOT yet re-recorded
post-P002 and remains the regressed shape for reference only.**

| Row                        | Go         | Pre-gap (target) | Staged now |
| -------------------------- | ---------- | ---------------- | ---------- |
| single_task                | 214.1      | 144.4            | 258.8      |
| parallel_independent_tasks | 2,405      | 589.5            | 10,901     |
| high_contention            | 5,922      | 13,193           | 15,298     |
| task_with_dependencies     | 331.4      | 263.4            | 502.5      |
| allocations                | 271.5      | 175.3            | 293.3      |
| parallel_scaling tasks-1   | 288.7      | 215.5            | 406.6      |
| parallel_scaling tasks-2   | 1,779      | 427.9            | 10,117     |
| parallel_scaling tasks-5   | 3,543      | 974.3            | 16,230     |
| parallel_scaling tasks-10  | 8,473      | 1,842            | 24,172     |
| parallel_scaling tasks-20  | 20,280     | 3,641            | 50,871     |
| parallel_scaling tasks-50  | 49,112     | 8,731            | 118,455    |
| context_cancellation       | 10,995,394 | 8,226            | 9,295      |
| repeated_task_calls        | 17.03      | 23.3             | 95.35      |

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

| Row                        | Before (regressed) | After (restored) | Delta   |
| -------------------------- | ------------------ | ---------------- | ------- |
| single_task                | 404.2              | 313.5            | −22.4%  |
| parallel_independent_tasks | 9,100              | 7,942            | −12.7%  |
| high_contention            | 11,136             | 11,520           | +3.4%   |
| task_with_dependencies     | 776.6              | 596.4            | −23.2%  |
| allocations                | 451.7              | 348.7            | −22.8%  |
| parallel_scaling tasks-1   | 661.0              | 525.3            | −20.5%  |
| parallel_scaling tasks-2   | 7,598              | 6,821            | −10.2%  |
| parallel_scaling tasks-5   | 12,216             | 10,647           | −12.8%  |
| parallel_scaling tasks-10  | 16,623             | 15,332           | −7.8%   |
| parallel_scaling tasks-20  | 27,175             | 23,453           | −13.7%  |
| parallel_scaling tasks-50  | 56,437             | 48,884           | −13.4%  |
| context_cancellation       | 6,771              | 5,031            | −25.7%  |
| repeated_task_calls        | 126.0              | 75.87            | −39.8%  |

`high_contention` is the one row that did not improve: it is dominated by ten
`tokio::spawn` calls in the benchmark harness itself (Go-goroutine fan-out mirror), so its
+3.4% is within run-to-run variance (±4% observed across the session), not a regression.
`context_cancellation` improves as a side effect of the poll-once change and already wins
by design (preemptive cancellation vs Go running the body to completion).

## Open flags

- **Commit caveat (delete this flag once committed):** a harness checkpoint staged the
  tree mid-P001b, leaving a stale `dev_watcher.rs` in the git INDEX (contains a removed
  probe test). The working tree is the source of truth and is verified clean — stage
  from the working tree (`git add -A`) before committing; do not commit the current
  index as-is.
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
- Tasks first-principles review: **P002 restoration landed on Linux** (blake3→keyed-SipHash
  and the poll-once fast path recorded; every Linux row improved except the spawn-bound
  `high_contention`). Remaining: the maintainer's mac-side "beat Go" re-recording
  (`BENCH_MACHINE_ID=m3-max make bench-tasks`) is the acceptance check for the absolute bar,
  which is defined and verified on the mac. Everything else about the crate — API shape,
  loom verification, test suite — landed well.
- Router/request-path review: **not started** (P004; ticket exists at
  `docs/maintainer/tickets/router-request-path-review/`).
- Board: census features and 100% API coverage audit open (P003 and phase C).
