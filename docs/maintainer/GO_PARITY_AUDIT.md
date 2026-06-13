# Go → Rust Parity Audit

Motivated by the confirmed Vite-restart regression (public-folder saves and app saves now
trigger full Vite restarts; the design intent — JIT hash RPC + retained stale outputs +
browser hard-reload, with Vite restarts reserved for true config changes — lived only in
the Go version and in memory).

**Why this audit exists in addition to REGRESSIONS_AUDIT.md:** the earlier audit compared
the current rewrite against `4b793aa0`, which was already mid-Rust. Anything lost during
the original Go→Rust pivot predates that baseline and was structurally invisible to it.
This audit reads the last full Go tree in this branch's history.

## Methodology

- Go reference: **`origin/refactor-2026-13`** (2026-05-24) — the final state of the modern
  Go rewrite, one week before the Rust pivot began (`851b5923` "wip", 2026-05-31, on
  `rust-init`). `main` is deliberately NOT used. NOTE: `rust-init`'s own first-parent
  history bottoms out at `be786db1` (v0.84.0), which still contains the long-deprecated
  `wave/` tree — that is a red herring; the modern Go work lived on the `refactor-2026-*`
  branches.
- Framework core: `internal/pkg/vormabuild/` (build/dev orchestration: browser_sync,
  live_state, supervisor, vite_plugin, run, prod, manifest), `internal/pkg/vormarun/`
  (runtime: core, handler, loader_payload, refresh_script, static),
  `internal/pkg/{cssbundle,fswatcher,staticproc, lockfile,viteutil,npm,mailbox}`,
  `kit/{matcher,mux,tasks,head, searchparams,tsgen,...}` — these map name-for-name onto
  the baseline Rust modules at `4b793aa0`.
- Each finding is classified: **PARITY** (same behavior), **INTENTIONAL** (changed on
  purpose, with the ruling), **REGRESSION** (silent loss), **GO-BUG** (Go's behavior was
  itself wrong/accidental — fix properly in Rust and document why), **NEEDS-RULING**
  (different; unclear if deliberate). Ratified stance: Go is a bell-ringer, not gospel —
  correctness beats ancestry in both directions. Work sequencing lives in
  [CURRENT_PLAN_2.md](CURRENT_PLAN_2.md); this file is the findings ledger.

## Findings

### 1. Vite restart semantics — REGRESSION (confirmed prior to this audit)

Go/baseline design: public-asset saves republish hashed outputs (retaining stale ones),
broadcast a browser `hard_reload`, and never touch Vite. JIT `hash` RPC keeps _new_
transforms fresh; retained stale outputs keep _cached_ transforms working. Vite restarts
(`/cfg-changed`) fire only on true config transitions.

Current Rust: `dev_build.rs` fires `notify_vite_plugin_config_changed()` (a full Vite
restart) on every filemap change (static fast path, ~line 637) and unconditionally on
every full generation activation (~line 587) — i.e. every public save and every app-code
save restarts Vite.

Fix (ratification pending): always update the RPC generation contract; notify only when
the `VitePluginConfig` payload itself changed. Add pins: no restart on filemap-only
update; restart when the view-module list changes. Record the invariant in REMINDERS.md.

### 1a. Vite restart semantics — REFINED against the true Go source

`refresh_go.go` + `vite_plugin.go` at `refactor-2026-13` settle the split:

- **Public/static path (`run_static_build`): NEVER touches Vite.** It broadcasts to the
  browser only: `hard_reload` when the filemap hash changed (early-return — subsumes
  everything else), else `update_critical_css` (with CSS payload), then
  `client_revalidate` when requested (early-return), else `hide_rebuilding_overlay`. The
  current Rust static fast path restarting Vite on filemap change is a **REGRESSION**,
  confirmed.
- **App-rebuild path (`refresh_go`): restarts Vite on EVERY non-initial rebuild**
  (`if !is_initial { send_vite_plugin_restart() }`). The current Rust full-generation path
  doing it unconditionally is therefore **PARITY WITH GO**, not a regression. Restricting
  it to "only when the `VitePluginConfig` payload changed" is an **improvement
  opportunity** (preserves Vite/HMR continuity on pure-backend edits), not a restoration.

### 2. Stale-output retention + dev freshness — RESOLVED (era TS plugin found and read)

CORRECTION: the era TS package IS in the Go tree, at `internal/pkg/npm/vorma/`
(vite/vite.ts, core/create_client_core.ts, the tsx adapters, kit). An earlier pass wrongly
declared it absent after checking only for a root `packages/` dir and one symbol spelling
(`vormaPublicUrl`; the era spelled it `vormaPublicURL`).

Facts, now all verified in the era source:

- The era plugin transforms in dev AND build (no mode gate): `vormaPublicURL()` in JS and
  `url(@public/...)` in CSS are statically rewritten via the JIT `hash` endpoint — same as
  today.
- `staticproc.Reconcile` deletes stale outputs; `vormarun` serves the public dir as a
  plain `fs.Sub(static_fs, "public")` file server (with `immutable` cache headers). No
  hashed-name fallback resolution.
- Therefore the Go version had a **GO-BUG** here (reclassified; an earlier draft of this
  finding called it a "knowing tradeoff" with zero evidence of intent — nothing in the Go
  source documents it): after a public asset's content changed in dev, any module whose
  Vite transform was already cached kept the old hashed URL → 404 (broken asset) until
  that module was next edited or Vite restarted for another reason. Go's blunt
  restart-on-every-Go-rebuild partially masked it by accident. Server-rendered references
  (document head, `PublicURL()`, critical CSS) always resolved fresh, so the bug was
  confined to module-baked references — narrow trigger (asset-only edit loops), but
  visibly broken when hit. Current Rust's retention converts the same window from
  visibly-broken (404) to silently-stale (old content) — a worse failure class.

The correct fix (Go-parity-with-the-bug and restart-per-save are both rejected as end
states — the former is a known bug, the latter is the regression):

- No Vite restart on public-asset saves.
- At transform time the plugin records asset→module edges (it already intercepts every
  public-URL resolution to make the `hash` RPC call, so the edge set is complete by
  construction; the CSS/postcss half needs the owning file id threaded through — design
  detail, not a blocker).
- On filemap change the build sends the changed source paths to a lightweight invalidate
  control message (instead of `/cfg-changed`); the plugin invalidates exactly the affected
  modules via `moduleGraph.invalidateModule`. Fresh on next request, no restart, no 404
  window, no stale window.
- The app-rebuild path's restart becomes conditional on the `VitePluginConfig` payload
  actually changing (GO-BUG fix as well — Go restarted on every Go rebuild).

### 3. Config-transition handling — NEEDS-CHECK in current Rust

Go `refresh_go` diffs the serialized config each rebuild. On change: full teardown
(release dev lock, stop Vite, recursive re-bootstrap), and on `dist_dir` change it
`RemoveAll`s the old `.vorma` directory. Verify the current Rust dev loop handles config
transitions equivalently (esp. stale `.vorma` cleanup on dist_dir change, lock
re-acquisition, Vite restart).

### 4. Go dev rebuild pipeline shape — recorded for comparison

`refresh_go` ordering: broadcast rebuilding overlay → ensure dirs → acquire PID lock
(first time) → CONCURRENTLY (stop app server || compile app server) → read live state from
build-entry subprocess → config diff/transition → restart fswatcher with new patterns →
write .gitignore → static build → wait for compile → write manifest BEFORE app start
(runtime init needs APIMountRoot; manifest rewritten later with Vite port) → start app
server → send Vite restart (non-initial) → start Vite server if not running. Cancellation
via build_ctx checks between stages. Current Rust uses epochs/candidates instead —
semantic parity spot-checks worth doing: manifest-before-app-start ordering, watcher
restart per generation, gitignore write timing (all believed ported; verify in passing).

### 5. Misc confirmations

- Go-era ChangeType wire strings identical to current (`show_rebuilding_ overlay`,
  `hide_rebuilding_overlay`, `show_build_error`, `hard_reload`, `update_critical_css`,
  `revalidate_client`). PARITY.
- Go vite cfg JSON used `RouteModules` (PascalCase); current uses `view_modules`
  (snake_case) — INTENTIONAL (Phase C ruling; view/resource naming shift).
- Go `prehashed_dirs`/`SrcNamePrehashed` passthrough feature: dropped — INTENTIONAL
  (explicit earlier ruling).
- Go hashed via sha256+base32 w/ mtime-keyed LRU cache; pub_fm change detected by sorted
  sha256 of the map. Current Rust hashes content per publish (with its own caching).
  Mechanism differs; behavior parity presumed — low risk.
- Go dev lock: PID lock at `.vorma/dev.lock`, acquired once on first bootstrap. Current
  Rust: paranoid ProcessLock at `dist/.vorma/build.lock` (restored during the regressions
  audit). Naming/scope differ slightly — verify the "second dev instance exits cleanly" UX
  survived.

### 6. Tranche A — dev-loop semantics (run.go / supervisor.go / watcher)

- **Event coalescing + supersede — PARITY.** Go: keyed mailbox, claim-all, go-implicated
  supersedes static, revalidate flag merged. Rust: `drain_pending_dev_loop_action` merges
  queued changes; intent precedence in `classify_dev_rebuild_work` gives the same
  supersede.
- **In-flight build cancellation — PARITY-PLUS.** Go cancelled `build_ctx` on any
  go-implicated event. Rust cancels only for app-server-class changes and queue-merges
  static/revalidate changes without killing the in-flight build.
- **Browser port model — INTENTIONAL improvement.** Go served the browser directly from a
  stable preferred port (8080) with stop-then-start downtime per rebuild and no proxy.
  Rust's dev mux (stable port, readiness probe, atomic backend switch) is zero-downtime by
  design.
- **Ready endpoints — PARITY** (`/.vorma/healthz`, Vite `/@vite/client`).
- **Critical-CSS imports — REGRESSION, FIXED.** Go rebuilt `css_files_to_watch` from
  bundle imports every build and classified events against it. Rust tracked
  `CriticalCssBundle.imports` but consumed it nowhere (the getter had even been
  cfg(test)-gated as apparently dead — the missing consumer was the tell). Fix: the dev
  watcher's classification plan is now swappable per generation
  (`DevFileWatchPlanHandle.replace`; OS watch roots stay session-static — same reach as
  Go, which also only watched configured roots),
  `BuildDevWatchPlan:: extended_with_critical_css_imports` feeds the bundle's canonical
  import paths in with `CriticalCssInput` intent, and the import set is plumbed through
  every published candidate into `StartedDevGeneration`. Pins: plan extension; swap
  reclassification through the callback's exact lock path; `@import` partial surfacing in
  the started generation across re-bundles.
- **dist_dir transition cleanup — REGRESSION, FIXED (with a Go-divergence on purpose).**
  Go `RemoveAll`'d the old `.vorma` immediately during refresh (safe only because Go had
  already stopped the app server). Rust keeps the previous generation serving until
  activation commits, so eager deletion would break zero-downtime and failed-build
  rollback. Fix: `reacquire_if_output_layout_moved` returns the orphaned output dir; the
  dev loop deletes it only after the moved-layout generation commits.
- **Child unexpected-exit monitoring — REGRESSION, FIXED (in the step-3 collapse).** Go
  panicked dev when the app server or Vite exited unexpectedly. Now: exit watchers observe
  children without reaping (linux `waitid`+WNOWAIT, macOS kqueue NOTE_EXIT), a
  terminate-intent flag suppresses intentional kills, registration follows app-server
  replacements across generations, and the dev loop fails the session with a named
  `ChildProcessExited` error. Windows: watcher is an explicit stub pending a
  SYNCHRONIZE-handle wait.
- **Bootstrap note for the collapse:** Go pinned `build_entry` from the caller file
  deliberately NOT as config ("changing it requires a manual restart") — same philosophy
  as the session-bootstrap + mismatch-guard ruling.
- `fswatcher/watch_plan.go` deep pattern-semantics comparison: deferred to tranche B
  (current dev_watcher was rewritten this branch with its own ruled semantics incl. escape
  support; spot-check only).

### 7. Tranche B — runtime + TS parity census

- **Runtime manifest — PARITY, field-for-field.** VormaVersion, the three Dev\_\* fields
  (omitted when empty both sides), PublicStaticBasePath, APIMountRoot, UIVariant,
  PublicFilepaths/PublicFilemap, CriticalCSS, SearchSchemas, ClientEntry, ClientCoreAssets
  (optional), per-view client modules, and the client-build-id-is-hash-of-manifest
  semantics all map 1:1. One rename: `RootHTMLTemplateHash` → `root_document_shell_hash`
  (same role, document-shell provenance).
- **Live-state protocol — INTENTIONAL redesign.** Go shipped projections
  (TSResult/TSModules/SearchSchemas computed in the entry process); Rust ships
  declarations (the canonical graph + root document hash source) and compiles all
  projections build-side. Cleaner ownership; no capability lost (search schemas ride the
  route type contracts).
- **Middleware census — PARITY/INTENTIONAL, strictly richer.** Go `kit/middleware`:
  bodylimit → `request_body_limit`, etag → `etag()` (plus strong/max-body/skip builders),
  secureheaders → `secure_headers`. healthcheck/robotstxt → app-space one-liners (example
  app demonstrates; dev readiness `/.vorma/healthz` stayed runtime-owned). Go had NO csrf
  middleware — nothing vanished. Rust adds request_id, sensitive_headers, panic_recovery,
  body/handler timeouts, compression. Go's chain helper → tower `ServiceBuilder` idiom.
- **Generated TS — PARITY at section granularity.** Both emit one generated file: Go
  (routes section + core types + static/public-filemap section) ↔ Rust
  (`render_typescript_contracts`: client seed + public-url setup + type defs + view/api
  contracts). Byte-level differs by design; the Rust generator is golden-tested.
- **Era npm core surface — fully accounted, no silent drops.** Era-only exports map to
  renames (ToAPIClient→ToApiClient casing family,
  ToRouteComponentProps→ToViewComponentProps), redesigns (ToLoaderInput/Output →
  ToClientLoaderArgs/ClientLoaderServerState; build_action_url superseded by
  create_typed_api_client/make_link_props), or deliberate internalization (make_route_id;
  resolve_outlet_slot → create_outlet_slot_resolver factory, still tested + OutletSlot
  type still public).
- **Dev refresh protocol/cssbundle semantics** — covered previously (refresh-protocol
  parity sweep in REGRESSIONS_AUDIT; css imports classification fixed in finding 6).
- **E2E proof:** with the rebuilt package dist, `test-dev -variant react` passes
  end-to-end on the collapsed single-binary pipeline (browser scenarios: nested, counter,
  latency). NOTE: the e2e consumes `packages/vorma/.dist` as-built — it had gone stale
  (pre-snake_case) without anyone noticing, which is why the first run failed in the
  plugin's `config` hook; `make gate` rebuilds it (`ts-build` precedes e2e), so this is a
  local-sequencing footgun, not a CI gap.

## Remaining to audit (next session continues here)

Priority order:

1. ~~Era TS plugin~~ FOUND at `internal/pkg/npm/vorma/` and read for Finding 2. Remaining
   value there: era `core/create_client_core.ts` and adapters as a TS-side parity
   reference (client core, HMR wiring, api_client) — compare against current
   `packages/vorma/core`.
2. `vormabuild/run.go` + `supervisor.go` (422+297): event classification → which path
   (refresh_go vs static vs css vs revalidate), debounce semantics, overlay show/hide
   pairing, build-error broadcast lifecycle (`show_build_error` flow), app-server
   readiness gating before reload broadcasts (`wait_for_app`-style semantics, if any),
   kill/restart sequencing.
3. `vormarun/` runtime: `handler.go` (443) + `core.go` (304) + `loader_payload.go` +
   `static.go` — redirect/skew/JSON-nav protocol parity, asset serving headers/caching.
   Specifically census the Go middleware surface
   (`kit/middleware/{etag,secureheaders,robotstxt, healthcheck,bodylimit}`, `kit/csrf`)
   against the current Rust runtime: which were runtime defaults vs opt-in, and which
   survived the port. NOTE: prior runtime verification (BACKEND_RUNTIME.md sweep, engine
   matrix pins, wire goldens) compared against the Rust baseline and spec docs — NOT
   against the Go runtime; this item closes that gap.
4. `fswatcher/watch_plan.go` — pattern semantics vs current dev_watcher (escape support
   was re-added; verify intent classification parity).
5. `cssbundle.go` vs current critical-CSS bundling (URL resolution rules matched in the
   static-build read; verify watch-file list semantics — Go re-derives
   `css_files_to_watch` from bundle imports each build and the watcher consumes it;
   current Rust equivalent?).
6. `live_state.go` + `manifest.go` (both crates' manifests) — field-level parity vs
   intentional changes.
7. `ts_gen.go`/`ts_modules.go` vs current tsgen/typescript_contracts — output parity
   beyond the golden-pinned wire/plugin contracts.
