# Unified Plan — Architecture Finish + Go Bell-Ringer Audit

This is the single forward-looking tracker. It unifies three streams: (1) the
single-binary collapse + creative-pass items, (2) the remaining cleanup/punch-list items
from the first plan, and (3) the deep Go parity audit. Completed phases A–E and the C/D
records live in [CURRENT_PLAN.md](CURRENT_PLAN.md) (archive). Audit findings live in
[GO_PARITY_AUDIT.md](GO_PARITY_AUDIT.md) (ledger; this doc only sequences the work).

**The Go audit's stance (ratified):** Go is a bell-ringer, not gospel. The point of
reading it is to surface semantics we changed without noticing. Every difference gets one
of these verdicts: **PARITY** / **INTENTIONAL** (with the ruling) / **REGRESSION**
(restore) / **GO-BUG** (Go's behavior was itself wrong or accidental — fix properly in
Rust, document why). Correctness beats ancestry in both directions.

## Sequencing and rationale

The one non-obvious ordering call: **the Go audit is split around the binary collapse**
rather than done wholesale before or after it. The dev-loop half of the audit is a _design
input_ to the collapse (it reads exactly the semantics the collapse will rewrite — bells
must ring at design time). The runtime/TS half is audited _after_ the collapse, against
stable code, so we never verify code that is about to be deleted.

### Step 1 — Vite restart fix (small, confirmed regression, lands first)

Independent of binary count; removes a known dev-experience regression before bigger
surgery. Per GO_PARITY_AUDIT findings 1/1a/2:

- [x] Static/public path: no Vite restart; targeted invalidation landed —
      `VITE_PLUGIN_ASSETS_CHANGED_PATH` control message carries changed source keys; the
      plugin records asset→module edges at transform time (JS via the transform hook, CSS
      via the postcss plugin's file id) and invalidates exactly the referencing modules
      via `moduleGraph`.
- [x] App-rebuild path: `notify_vite_plugin` dispatches cfg-changed only when the
      `VitePluginConfig` payload changed, else assets-changed only when the filemap
      changed, else nothing; failure rollback re-syncs with the same (symmetric)
      changed-key set.
- [x] Pins: filemap-only static update → assets-changed only; app rebuild with reused
      outputs → no notification at all; view-module-list change → cfg-changed only;
      notify-failure rollback preserved; plugin endpoint pins token-gating, 400 on bad
      payloads, no-op on unreferenced assets, slash-normalized keys, exact-module
      invalidation.
- [x] REMINDERS.md invariant recorded ("Vite dev-server lifecycle").

### Step 2 — Go audit, tranche A: dev-loop semantics (design inputs)

Read before designing the collapse; verdicts to the ledger:

- [x] `run.go` + `supervisor.go` read; verdicts in ledger finding 6: coalescing/cancel
      PARITY(+), port model INTENTIONAL improvement, ready endpoints PARITY. Two
      regressions found and FIXED this step: critical-CSS import watching (swappable
      classification plan + per-generation import plumbing + 3 pins) and dist_dir-move
      stale output cleanup (commit-time orphan deletion — deliberately later than Go's
      eager delete, which is unsafe under zero-downtime). One regression CONFIRMED and
      assigned to step 3: child unexpected-exit monitoring (collapse rewrites process
      supervision).
- [x] Config-transition handling: lock follows moves (already present); stale-output
      cleanup fixed above; full config-transition teardown semantics fold into the
      collapse's session-bootstrap guard.
- [~] `fswatcher/watch_plan.go` deep comparison deferred to tranche B (dev_watcher was
  rewritten with ruled semantics this branch).
- [x] Pipeline-ordering spot-checks: manifest-before-app-start and gitignore timing
      verified present; watcher-per-generation parity now exceeded via the swappable
      classification plan.

### Step 3 — The single-binary collapse (ratified)

The app-server binary answers live-state itself (env-keyed; the runtime already
half-implements this). The build-entry binary dies. Design notes from the creative pass,
refined: the two-PROCESS model survives (orchestrator

- app processes); it is the two-BINARY model that dies.

* [x] Live-state protocol moved to `vorma_contract::live_state` (struct, parsing, env
      keys, error envelope); the async constructor
      (`live_build_state_from_app_build_contract`) lives in `vorma`; `App::from_config`
      emits-and-exits under the env key on a dedicated thread+runtime (safe inside the
      app's tokio main) — every app server is now the live-state emitter.
* [x] `app_binaries_build.rs` deleted; `app_server_build.rs` builds ONE binary per
      iteration (streaming artifact parse kept for executable discovery; the scoped-thread
      live-state overlap, dual-target config, and identical-target rejection are gone;
      attribution tests ported).
* [x] REMINDERS parallelism contract replaced with the single-binary truth.
* [x] Dev rebuild pipeline: one cargo invocation → live-state from the server executable →
      same executable reused as the prebuilt dev server. Production never used the
      combined build (in-process contract + Vite); unchanged.
* [x] Mismatch guard: a mid-session `ServerBuildTarget` change errors with a restart
      instruction instead of silently building the old target.
* [ ] Child unexpected-exit monitor (assigned regression from tranche A)
* [x] **RULING (settled): no CLI — ever.** The entry stays a per-app binary and keeps
      taking the app config: `vorma_build::run(app_config)` with `BuildOptions` deleted.
      The server target is single-sourced from the app config (`ServerBuildTarget`,
      already declared there); the orchestrator reads it in-process at session start and
      gets fresh graphs from the live-state child per iteration. Guard: if live-state
      reports a different server target than the session bootstrapped with, error and
      instruct a dev-server restart. Rationale: the entry being app-linked costs one link
      per SESSION START (the dep tree is compiled for the server anyway); the
      per-iteration loop builds only the server binary either way — and this keeps exactly
      one source of truth for the target, vs the shim duplicating it as constants.
      Provenance: this is the proven Go bootstrap pattern (`refresh_go.go` initial run
      reads the linked config in-process via `to_cfg(rs.__c)` — fresh by construction at
      launch — then live-state subprocess reads thereafter). The collapse's only delta
      from Go is which binary plays the per-iteration child: Go re-ran the build-entry
      binary; we re-run the server binary, which is rebuilt anyway.
* [x] User-space migration: `run(app_config)` (options parameter gone; `BuildOptions`
      deleted); examples/minimal + framework-tests entries shrunk; public_api pins
      updated. The per-app entry binary remains — thin, never the live-state emitter.
* [x] Projection-triple fold evaluated — RULING: keep all three. The bundle (graph
      projections), plan (validated paths/targets/watch shape, fallible compile), and
      prepared inputs (expensive rendered outputs) have distinct lifecycles and failure
      modes; folding bundle+plan would push path validation into the projection layer for
      no capability gain across ~40 call sites. The collapse did not change their
      relationship.
* [ ] Update tests/pins (public_api, process lifecycle, e2e)

### Step 4 — Go audit, tranche B: runtime + TS — COMPLETE (ledger finding 7)

- [x] `vormarun/handler.go` + `core.go` + `loader_payload.go` + `static.go`:
      redirect/skew/JSON-nav protocol parity, asset-serving headers/caching
- [x] Middleware census: Go shipped
      `kit/middleware/{etag,secureheaders,     robotstxt,healthcheck,bodylimit}` +
      `kit/csrf` — which were defaults vs opt-in, and which survived the port (prior
      runtime verification compared against Rust baseline + spec docs only; this closes
      that gap)
- [x] `live_state.go` + both `manifest.go`s: field-level parity vs intentional changes
- [x] `cssbundle.go`: css watch-file list semantics (Go re-derived `css_files_to_watch`
      from bundle imports each build)
- [x] `ts_gen.go`/`ts_modules.go` vs tsgen/typescript_contracts output
- [x] Era TS package (`internal/pkg/npm/vorma/`): compare `core/create_client_core.ts` +
      adapters + `api_client` against the current decomposed client core (HMR wiring,
      submit semantics, redirects)

### Step 5 — Layering punch-list residue

COMPLETED notes: `vorma::build_interface` is the named, documented build↔runtime contract
(assets/contracts/config/env/facade/route*input/ runtime groups + AppBuildContract + the
build-state constructors); `__private` shrank to the macro ABI only (the exact paths
`vorma-macros` emits into app crates: Params, type_resolver, search_schema_resolver,
InputError, PathParams, run_static_resource, run_static_view) and is now doc(hidden).
TypeDef naming unified on the short names — `contracts::` already says "contract", so
TypeDefContract/FieldDefContract/ RawTypePartContract became TypeDef/FieldDef/RawTsPart
and tsgen's aliases became plain re-exports (serde wire unchanged: type names are not
serialized). Optional prose-driven items: none taken (nothing earned a change beyond the
two above). Parked runtime*\* rename stays parked for a user ruling — recorded in
CURRENT_PLAN_3.

- [x] Promote the build↔runtime interface out of `vorma::__private` into a named,
      documented surface (smaller after the collapse — live-state emission will have
      moved; what remains is assets/facade/route_input/ runtime/erased-handler machinery)
- [x] Unify `tsgen::TypeDef` vs `contracts::TypeDefContract` naming (one concept, one
      name)
- [ ] Optional, prose-driven: single context access model; name the def → contract →
      prepared head pipeline as stages
- [ ] `runtime_*` rename ruling from CURRENT_PLAN.md Phase D (parked)

### Step 6 — Enforcement and docs (Phase F close)

COMPLETED notes: five compile-fail pins live in `crates/vorma/tests/compile_fail/` (TsGen:
generic, tuple struct, union, serde(flatten); view declaration: field-order diagnostic)
via trybuild in the `vorma` crate (no dev-dep cycle; `::vorma` paths resolve naturally).
Architecture prose: docs/maintainer/ARCHITECTURE.md (post-collapse, written as review —
its findings feed CURRENT_PLAN_3). CI gate job stays DEFERRED by user ruling. Plus the
CURRENT_PLAN.md leftover landed: the 4,932-line create_client_core.test.ts decomposed into
7 themed integration files (shared helpers promoted into \_test_helpers.ts), which exposed
and fixed one latent order-dependent test (prefetch prestart asserted synchronously across
the async WASM-matcher boundary — proven pre-existing by -t isolation against the
monolith).

- [x] trybuild compile-fail tests for `vorma-macros`
- [x] Architecture prose (two-process chapter written post-collapse; treat the writing as
      the final review — awkward prose = punch-list item)
- [~] CI `make gate` job — deferred by ruling (tooling overhaul later)

## Rescinded / revised items

- **"Ship the public filemap to the plugin once; delete the `hash` RPC" — RESCINDED.** It
  was premised on restart-driven freshness, which turned out to be the regression itself.
  Under the restored model the JIT `hash` endpoint is load-bearing (it is what keeps
  newly-transformed modules fresh without restarts). See GO_PARITY_AUDIT finding 2.
- The creative-pass write-up said "the two-process model… I'd rebuild identically" —
  clarified above: the process model stays, the binary model goes.

## Constraints audited and kept (from the creative pass, ratified context)

Vite ownership of frontend dev; the WASM matcher (parity over hand-mirrored matching); the
injected refresh script staying separate from the client core (must work when app JS is
broken); the recompute-the-world dev generation loop (boring and correct beats
incremental).
