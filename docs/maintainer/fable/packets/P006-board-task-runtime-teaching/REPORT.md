# P006 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02) was blocked from writing
this file by the recurring harness report-file guardrail and returned the full content in
its final message; Fable placed it (HTML transport escaping undone).

## What changed

- `examples/board/src/maintenance.rs` (new) — the app's own task runtime, outside any
  request: `PRUNE_EXPIRED_SESSIONS` (`task!`, real DB work: delete `sessions` rows past a
  retention window), `run_session_pruner` (constructs its own `Tasks<vorma::Error>` via
  `Tasks::new`, opens a fresh `ExecCtx` per sweep via `Tasks::exec_ctx`, a
  `tokio::time::interval` loop racing a `CancelToken` via `select!`), `SlowTaskObserver` +
  `slow_task_observer()` (a `TaskObserver` logging task name + `Task::id()` + duration for
  runs crossing a threshold).
- `examples/board/src/repo.rs` — `blocking_task` became `pub(crate)` so `maintenance`'s
  task body can reuse the same join-error mapping instead of duplicating it (`blocking`
  stayed private). New: `ModExportRow`, `ModExportDigest`, `MOD_EXPORT_SCAN` (`task!`) — a
  moderator-triggered bulk scan over every story, chunked, checking `ctx.is_cancelled()`
  once per chunk and bailing with `complete: false` if so, each chunk's comment-count
  reads running through `ctx.child()`.
- `examples/board/src/resources.rs` — new `MOD_EXPORT` resource (`POST /api/mod/export`),
  reusing the existing `/api/mod/*` middleware scope (`mod_gate()`), no middleware change
  needed.
- `examples/board/src/lib.rs` — registered `pub mod maintenance;` and
  `resources::MOD_EXPORT` in the app's resource list; `app_config()` unwound one line into
  a new public `db_path()` fn so the server binary can open its own `Arc<Db>` and hand the
  identical handle to both the framework's config and the worker, without changing
  `app_config()`'s own behavior or its callers.
- `examples/board/src/bin/server.rs` — `serve()` now opens `Db` directly, builds
  `TasksOptions` with the shared slow-task observer wired into `.observer`, spawns
  `maintenance::run_session_pruner` alongside axum, and races the worker's
  `CancelToken.cancel()` into the same point `shutdown_signal()` already resolves, then
  joins the worker's `JoinHandle` after `axum::serve` returns.
- `examples/board/src/client/views/mod.view.tsx` — an "Export audit" button
  (`useApiMutation` against `/api/mod/export`) and a result panel reading
  `export.data.complete`/`.rows`, teaching the cancellation-partial-result shape on the
  client side too.
- `examples/board/src/client/vorma.gen.ts` — regenerated via board's own build binary
  (adds `ModExportDigest`/`ModExportRow`).
- `examples/board/tests/app.rs` — new test `mod_export_is_gated_and_digests_every_story`
  (middleware-gate assertion + happy-path digest content assertion: 2 stories, one
  commented, digest has 2 rows with correct titles/comment counts, `complete: true`).
- `examples/board/Cargo.toml` — added the `time` feature to the existing `tokio`
  dependency (not a new dependency; see Decisions).
- `docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md` — appended an "F-17
  RESOLVED (P006, 2026-07-02)" entry documenting the three landed rows with citations and
  the still-exempt surface (`TaskOverrides`/`Clock` family).

No changes to `vorma`, `vorma-tasks`, or any other framework/sovereign-crate source.

## Decisions made

- **Chose session pruning as the worker's job, not an invented counter.** `sessions` has a
  `created_at` column and no expiry mechanism anywhere in the app; a background sweep
  deleting stale rows is something this app would actually want in production. Considered
  and rejected: a stats-refresh snapshot (the `single_flight` `LIVE_SITE_STATS` task
  already teaches live aggregation).
- **The mod export task runs through the framework's own per-request `ExecCtx`, not a
  second standalone runtime.** An HTTP-triggered action legitimately uses the framework's
  runtime, exactly like every other resource in the file; only row 1 (the background
  worker) genuinely needs its own `Tasks::new`, because nothing there is a request. This
  reading is consistent with the census's original escalation table
  (`Tasks::{new,exec_ctx}`/`CancelToken` were the second-runtime items;
  `ExecCtx::{is_cancelled,child}` were flagged separately).
- **`.child()` exercised via a real task call, not a bare no-op scope.** Each chunk's
  `ctx.child()` runs `COMMENTS_FOR_STORY.run(&chunk_ctx, ...)` — reusing the existing task
  and giving `.child()` genuine memoization/cancellation-scoping purpose.
- **`repo::blocking_task` made `pub(crate)` instead of duplicating its join-error mapping
  in `maintenance.rs`** (Stay-DRY; `repo.rs`'s own doc comment describes it as the one
  blocking-database primitive every task shares).
- **`tokio`'s `time` feature added explicitly, not left implicit.**
  `tokio::time::interval` compiled without it (feature-unified transitively through
  axum/hyper-util/tower-http), but relying on an undeclared transitive feature is fragile.
  Verified `Cargo.lock` byte-identical before/after (zero new resolution, purely manifest
  honesty). Read as a feature flag on an already-present direct dependency, not a new
  dependency requiring escalation; flagged for the maintainer in case that read should be
  confirmed as standing guidance.
- **`MOD_EXPORT`'s handler discards the `require_user` result** (auth as a pure gate check
  vs identity-stamping) — teaches a real variation rather than binding an unused value.
- **No board test exercises the `complete: false` (cancelled) path.** Considered and
  rejected: racing wall-clock timing (tests-never-cheat) or reaching into framework
  internals (framework-semantic behavior belongs in framework-owned suites). `TestApp` has
  no supported way to cancel a request mid-flight from outside; the cancellation mechanism
  itself is loom-verified and exhaustively tested in `vorma-tasks`' own suite, which is
  the correct owner. The happy-path test proves the resource is wired and returns the
  right shape.
- **Shared one `SlowTaskObserver` instance across both `Tasks` runtimes** — the more
  honest statement of "one telemetry sink watches the whole process."
- **Did not touch `docs/maintainer/board-example/README.md`** — its content already
  matches the 2026-07-01/07-02 rulings (prior work, nothing for this packet to add).

## Gate results

All run from repo root after all edits (`sjc-z390-aorus-pro-wifi`, i9-9900K, rustc
1.96.0):

- `cargo test --workspace --all-targets` — 31 test-result blocks, all ok, 0 FAILED; **568
  tests passed** (board `tests/app.rs`: 21 incl. the new mod_export test).
- `cargo test --workspace --doc` — 2 passed, 0 failed.
- `cargo clippy --workspace --all-targets -- -D warnings` — clean.
- `cargo fmt --all -- --check` — clean, no diff.
- `make loom-tasks` — 7/7 models pass; vorma-tasks source untouched (untouched-green).
- `pnpm exec tsgo -p examples/board` — clean. `oxlint examples/board` — clean.
- `pnpm exec vitest run` — 851/851 across 42 files, matching the STATE.md baseline.
- `RUSTDOCFLAGS="-D warnings" cargo doc --workspace --no-deps` — clean (one broken
  intra-doc link found and fixed mid-session).
- `cargo bench --workspace --no-run` — all targets compile.

Fmt-write scoping (F-18 compliance): `oxfmt --write` was run scoped to exactly three files
(`mod.view.tsx`, `vorma.gen.ts`, `Cargo.toml`); never unscoped. Pre-existing,
not-introduced note: `PRESSURE_TEST_CENSUS.md` fails `oxfmt --check` (the documented F-18
drift; my appended entry is hand-wrapped to the surrounding convention; running a write
would reformat ~540 lines I do not own — left per the dirty-unowned-files-are-escalations
rule).

## Benchmarks

Not applicable — app-code only; no benchmark row affected; bench compile clean.

## Escalations / open questions

None requiring a maintainer ruling. Flagged for visibility: the tokio `time` feature-flag
read (above) — confirm as standing guidance if agreeable.

## Discovered out-of-scope work

The pre-existing F-18 oxfmt drift on `PRESSURE_TEST_CENSUS.md`, already tracked; no new
tickets.
