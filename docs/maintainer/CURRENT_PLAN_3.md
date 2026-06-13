# CURRENT_PLAN_3 — Fresh first-principles review (post-CURRENT_PLAN_2)

Written after completing every CURRENT_PLAN_2 step and the CURRENT_PLAN.md leftovers. This
is a from-scratch look at the framework as it now stands: "if we were writing this today,
would it look like this?" Each finding has a disposition. Companion docs:
[ARCHITECTURE.md](ARCHITECTURE.md) (the system map this review was conducted against),
[GO_PARITY_AUDIT.md](GO_PARITY_AUDIT.md) (findings ledger),
[CURRENT_PLAN_2.md](CURRENT_PLAN_2.md) (what was just executed, with rulings).

## Verdict in one paragraph

The architecture now matches what a from-scratch design would choose. The single-binary
collapse removed the one structural wart we kept apologizing for (two app-linked binaries
with a combined-build dance); the Vite freshness model is now strictly better than both
the Go era (staleness/404 window) and the pre-fix Rust state (restart-per-save); the dev
loop is event-driven end to end with per-generation classification; the build↔runtime
boundary is a named, documented surface; and the contract crate cleanly owns every shared
truth. Remaining items below are deliberate stubs, parked rulings, or optional hardening —
none are structural.

## Open items (carried forward, with dispositions)

1. **Windows child-exit watcher — stub, RULED: wait.** Unix is covered (linux
   `waitid`+WNOWAIT, macOS kqueue NOTE_EXIT). User ruling (2026-06-11): leave the
   documented stub until Windows is a supported dev target with a way to actually test;
   landing untested `windows-sys` code is worse than the degraded next-request failure
   mode.
2. **`runtime_*` module prefix — RESOLVED: keep (user ruling 2026-06-11).** The prefix
   does the namespacing work, the layer docs exist, and the modules are crate-private;
   folding into a `runtime/` directory would be pure import churn. Closed permanently.
3. **CI gate job — DEFERRED by user ruling.** `make gate` is the gate.
4. **E2E `.dist` staleness footgun — RESOLVED (stronger than proposed).** `e2e` (and the
   new `e2e-smoke`) now depend on `ts-build` in the Makefile, so any e2e invocation
   rebuilds the package dist first — the class of failure is gone rather than merely
   detected.
5. **Live-state protocol version field — RESOLVED.** `LIVE_BUILD_STATE_PROTOCOL`
   (currently 1) rides every emission; `from_json_bytes` rejects any other version with a
   precise `ProtocolMismatch` error naming both versions and the remedy. Emitters
   predating the field deserialize as protocol 0 and hit the same precise error (pinned),
   so even old binaries fail clearly rather than as a serde parse failure.
6. **User-facing documentation — gated behind the public-API scrutiny pass (user ruling
   2026-06-11).** Before writing docs, every public API (Rust declaration surface,
   `vorma_build::run`, the TS package exports, the vite plugin, create-vorma) gets a "do
   we love this" review so docs are written once against a surface we believe in. Findings
   live in the "Public API scrutiny" section below as they are produced.

## Tooling review (Makefile / xtask / tests / configs) — done on request

A from-scratch pass over the repo tooling, same standard as the framework review. Verdict:
the structure was already right — make targets as the verb surface, a tiny tested `xtask`
for the things make does badly (per-step gate logs, fuzz-corpus copying, the publish
dance), co-located TS tests, per-crate Rust integration dirs, minimal configs (deny.toml,
one-rule custom oxlint plugin with a fixer, one-line rustfmt). Defects found and fixed:

- **`e2e` now depends on `ts-build`** (user-ratified), plus a new `e2e-smoke` target
  (prod + react dev + react dev-changes) for a fast full-pipeline pass. Costs one
  redundant dist build inside `make gate` (gate's steps are separate make invocations);
  correctness over seconds.
- **`rust-package` was missing `vorma-contract`** — the crate is publishable and `vorma`
  depends on it, so the gate never exercised its packaging and a real release would have
  failed mid-sequence. RELEASE_INSTRUCTIONS.md had the same gap (publish order now:
  matcher, tasks, macros, contract, vorma, vorma-build). Packaging verified.
- **No `.PHONY` declarations existed** — any file/dir named like a target (`gate`, `e2e`,
  …) would have silently broken make. All targets declared.
- **`ts-gate` listed `rust-build-client-wasm` redundantly** (already reached through
  `ts-typecheck` → `ts-build`); dropped with a comment.
- **`XTASK_CMD_BASE` used `--manifest-path`** although xtask is a workspace member; now
  `cargo run -p xtask`.
- Added a `clean` aggregate (bombadil + gate logs + dists + package-gate target dir) and a
  Makefile header documenting the gate philosophy — including the deliberate choice that
  gates run formatters in WRITE mode (normalize, then verify; lint/type/test stay
  check-only).

Reviewed and deliberately kept as-is: the xtask gate's Make→cargo→Make sandwich (per-step
logs + cargo env scrubbing are exactly what make alone does badly); hand-rolled TOML
version parse in ts_publish (40 tested lines beats a dependency in a zero-dep tool);
bombadil's poll loops (a test harness observing external processes is not the framework
dev loop — the no-polling invariant doesn't apply); the mutate-and-restore `dev_marker.rs`
probe fixture; nested .gitignores (verified complete via `git check-ignore`);
`multiple-versions = "allow"` in deny.toml (pragmatic for an app-framework workspace).

## Public API scrutiny pass (pre-docs, user-ratified 2026-06-11)

Walked the full user surface the way a user meets it: examples/minimal (declarations,
config, document, server main, build shim), the ctx/error/ head/document APIs behind it,
`vorma_build::run`, and the TS journey (entry → createVormaClient →
defineView/useViewData/Link/apiClient).

Landed immediately (no semantics changed):

- **Head/document element builders now take `impl Into<HtmlElementDef>` items**
  (`meta`/`link`/`script`/`style`/`add`). Callers pass raw attribute markers —
  `head.link([head.rel("preload"), head.href(url), head.r#as("font"), …])` — instead of
  binding temp lets and calling `.into()` on every part. The example's font-preload block
  shrank from eleven lines of plumbing to one expression; internal helpers and pins
  updated workspace-wide.

Judged GOOD as-is (recorded so they stay decided):

- `vorma_build`'s public surface is one function, `run(app_config) -> Result<(), String>`;
  the stringly error is correct for a main()-shim whose only consumer prints and exits.
- The ctx read surface (`input`/`state`/`param`/`params`/`splat_values`/
  `request`/`public_url`/`head`/`response`) is uniform across View/Resource/Middleware
  ctxs and reads exactly as it should.
- `view!`/`resource!` field-order strictness stays: it buys uniform declarations across
  codebases and the diagnostics are compile-fail pinned.
- `vorma::app!(pub mod app for crate::AppState)` + the `app::` alias module is the right
  amount of magic.
- TS naming is a deliberate two-register convention: the user-facing adapter surface is
  camelCase (`createVormaClient`, `defineView`, `useViewData`, `apiClient.mutateOrThrow`,
  `vormaPublicUrl`) matching ecosystem idiom; core internals consumed by adapters are
  snake_case. CORRECTED during the full pass: there is NO root `"vorma"` export at all —
  the adapter contract is exported solely as `vorma/__internal` (honestly named); users
  enter via `vorma/{react,preact,solid,vite}` and `vorma/kit/*`.
- `PathConfig { public_static_base, api_base }` public names diverge from internal
  manifest names (`api_mount_root` etc.) — public brevity wins; no churn.
- Minor notes, no action: `js_package_manager_base_cmd` is a space-separated command
  string (document split semantics when docs land); `Error::runtime` is the sole
  message-error constructor and its name leans setup-flavored but adding a second
  constructor would create two ways to say the same thing.

Rulings received and executed (2026-06-11):

- **AppConfig stays an exhaustive struct literal** (user ruling). It is self-documenting,
  create-vorma scaffolds it, and pre-1.0 field additions are loud honest compile errors.
  Revisit `#[non_exhaustive]` at 1.0.
- **Handler rejection: NO sugar; the names were the bug** (user ruling: "anytime we are
  tempted to add sugar, it means our normal API sucks" — and the reviewer's own confusion
  proved the API under-communicated). The two statements are two deliberate CHANNELS, not
  duplication: one sets what the client receives, the other records the server-side
  reason. Renamed so every call site says which channel it feeds:
    - `ResponseHandle::set_error_status(status, text)` →
      **`set_client_error(status, client_text)`**, with docs spelling out the
      terminal-effects interaction (your status/text wins over the generic 500 when the
      handler then returns `Err`).
    - `vorma::Error::runtime(...)` → **`vorma::Error::msg(...)`** ("runtime" falsely
      suggested a category; it is a message error used for setup AND handler records),
      with docs stating the server-side channel.
    - The example's CREATE_NOTE rejection now feeds DIFFERENT strings to the two channels
      with a comment — the identical strings in the old example were themselves part of
      the confusion.

## Fresh observations that need no action (recorded so they stay decided)

- **`create_client_core.ts` at ~1,700 lines** is the assembler over the decomposed modules
  (scheduler, work projection, redirects, wire payload, loaders, route modules,
  submissions, history). An assembler this size is the honest shape of "one function that
  boots a browser runtime"; the integration suite is now split by theme. Revisit only if
  it grows new responsibilities.
- **Two small control planes** (build→plugin ctrl server; plugin→build RPC) instead of one
  duplex channel: symmetric, tiny, and token-gated. A WebSocket would save a port at the
  cost of framing/reconnect machinery. Keep.
- **Dev mux as a sibling proxy** (vs embedding Vite middleware-mode): impossible to embed
  a Node Vite into a Rust host; the proxy is the correct topology, and it is what buys
  zero-downtime swaps.
- **`live_state_emit` propagates emission-thread panics via `join().expect`**: loud,
  build-time-only, correct.
- **Per-rebuild `DevFileWatchPlan` recompilation** is O(entries) and buys per-generation
  classification correctness. Fine.
- **`extern crate self as vorma` in `vorma-contract`** remains the one genuinely exotic
  trick in the codebase; it is documented at the site and pinned by the macro-ABI tests.
  Keep (the alternative is a three-crate macro-path dance with no user benefit).
- **Zero TODO/FIXME markers in production code** across all crates and the TS package —
  confirmed by sweep this session.

## What changed underneath this review (context for future readers)

Everything in CURRENT_PLAN_2 steps 1–6 plus the CURRENT_PLAN.md leftovers, most notably:
targeted Vite invalidation with asset→module edges; the Go bell-ringer audit (tranches
A+B) with two regressions found and fixed beyond the Vite one (critical-CSS import
watching via the swappable classification plan; dist_dir-move orphan cleanup at commit
time) and one fixed inside the collapse (child-exit monitoring); the single-binary
collapse itself (live-state moved into the runtime behind `vorma_contract::live_state`,
`run(app_config)` as the whole public build surface, mismatch guard);
`vorma::build_interface`; TypeDef naming unification; trybuild diagnostic pins;
ARCHITECTURE.md; and the 7-way integration-suite split. Verified state at time of writing:
Rust workspace 471/0 with clippy `-D warnings` and fmt clean; TS 850/850 with tsgo and
oxlint silent; e2e green post-collapse (test-prod all variants; test-dev and
test-dev-changes for react, including rebuild, critical-CSS/HMR, and view-refresh
scenarios).

## API campaign closeout (2026-06-11)

Everything in API_DESIGN.md's execution order through stage 4+ is landed and verified
(workspace 485/0; TS 851/851; clippy -D warnings, fmt, tsgo, oxlint clean; example prod
build green; e2e-smoke green on every cut):

- Error/exit protocol (`ViewExit`/`HttpExit`, JSON error envelope, TS envelope parsing,
  terminal redefinition, redirects on the exit channel).
- Config reshape (`path_config` dissolved; `ServerTarget`; canonical field order + path
  rule).
- Raw idents; dead statusText surface removed; F4 adjudicated.
- Example expanded to the full-surface "Vorma Notes" workout (examples/notes; tests are 12
  request-level `vorma::testing::TestApp` cases — F2 resolved; TestApp grew `request()`
  builder).
- Scoped middlewares (declaration `patterns`/`methods` filters, per-middleware flat
  matchers, request-time selection; live-state protocol v2;
  `MiddlewareCtx::matched_pattern()` retired).
- clientLoader "regression" RETRACTED — bad example code, healthy API (see API_DESIGN.md
  for the full retraction and process lesson).

`make gate` DEFERRED by maintainer ruling: the next initiative will likely move APIs
again, so the full gate runs after it. Gate-speed profiling is queued as a separate task.

## Next initiative: realistic pressure-test app (replaces Notes)

Ratified direction: an HN-shaped community app with a real SQLite store (rusqlite behind
Tasks — DB reads as deduped task runs, middleware preloads `current_user` shared with
handlers), passwordless demo auth, and a census-first method: every user-reachable Rust
and TS FRAMEWORK API maps to a named feature BEFORE code (`vorma/kit/*` is explicitly out
of scope — utility packages with their own someday design pass; the app must not import
them); anything that cannot find a realistic home goes to a scrutinize-or-cut list for
maintainer adjudication. See docs/maintainer/PRESSURE_TEST_CENSUS.md. Docs follow, using
the app as their running example; create-vorma rewrite stays last.
