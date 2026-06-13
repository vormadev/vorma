# Fable Session Handoff Notes

These notes preserve go-forward context distilled from the prior Fable conversations.
They are not a changelog. They record decisions, traps, and active workstreams that future
maintainers should not need to rediscover from raw transcripts.

This was the initial high-level handoff. The exhaustive post-audit notes are split across
the focused `FABLE_*_NOTES.md` files and the coverage ledger; when details conflict or
more precision is needed, use those focused files.

`FABLE_FABLE1_COMMIT_NOTES.md` is the committed-code checkpoint for
`fable-1` (`eff2c2f8d6edeb0f98c62af7cd9c46c183175844`). Use it when a
transcript-era intention must be reconciled against the actual tree that landed. It
intentionally excludes post-commit staged or working-tree changes.

## Operating Agreements

- Durable project memory belongs in repository docs or tests that the maintainer can
  review in a normal diff. Hidden tool memory, hidden preference files, or out-of-repo
  summaries are not acceptable unless the maintainer explicitly asks for them.
- When the maintainer says "stop", asks a direct question, or rejects a tool call, stop
  immediately. Do not continue tool calls, edits, or adjacent work before answering.
- Do not reinterpret a request into a nearby task. If the maintainer asks for an
  explanation, answer first; do not start "fixing" unless the request actually asks for a
  fix.
- Do not declare work complete until the verification appropriate to the touched behavior
  has actually run. Runtime, client, dev-loop, or routing changes usually need e2e proof,
  not only unit tests, typecheck, or lint.
- Do not present vague open questions. Explain the concrete API/code shape, take a
  position, recommend a path, and say exactly what ruling is needed.
- Do not manufacture process work. The repository prefers executable guarantees:
  tests, golden fixtures, compile-fail diagnostics, and explicit failure messages over
  prose that can rot.
- Generated artifacts must have one writer. If a formatter rewrites a generated golden,
  either the generator must emit formatter-stable bytes or the exact generated file must
  be formatter-ignored. Do not leave two tools fighting over byte output.
- Generated TypeScript emitted into user apps must be deterministic, clean, and correct
  across the full settings combination space. Its style must not depend on this repo's
  formatter configuration.
- For TypeScript, top-level exports are not the whole public surface. Option bags, client
  object members, `apiClient` members, adapter props, and nested helper types are API too.

## Core Architecture Facts

- The rewrite's high-level architecture was judged sound: canonical graph, immutable
  execution plans, transactionally committed build generations, generated/golden-pinned
  contracts, and the decomposed client core are the right center of gravity.
- The early regression audit existed because a structurally good rewrite still dropped
  behavior. Important restored classes included output locking, generated `.gitignore`
  writes, symlink defenses, process-group behavior, watch-root escaping, public-asset byte
  caching, dev CSP gating, Vercel manifest fallback, critical-CSS fast paths, child-exit
  monitoring, and no-restart Vite freshness for public assets.
- Go-era code is a bell-ringer, not gospel. Differences get classified as parity,
  intentional, regression, or Go bug. Correctness beats ancestry in both directions.
- The old Go public-asset freshness model had a real bug: cached module transforms could
  keep stale hashed public URLs. The Rust end state fixes this properly with asset to
  module edges and targeted Vite invalidation rather than restarting Vite on every public
  save.
- The single-binary collapse is the settled design. The app server binary answers
  live-state under an env key and then serves normally without a second app-linked
  build-entry binary. The per-app build entry remains only a thin orchestrator shim
  calling `vorma_build::run(app_config)`.
- `BuildOptions` died by ruling. The server target is single-sourced from app config, read
  in-process at session start, with a mid-session mismatch guard requiring restart.
- Windows child-exit monitoring remains intentionally deferred. Unix/macOS unexpected
  child exits are watched; Windows has a documented stub until Windows dev support can be
  tested properly.
- `runtime_*` module names are intentionally kept. The prefix performs useful namespace
  work for crate-private modules, and moving to a `runtime/` directory was ruled import
  churn.
- Some maintainer docs and generated inventories predate later Board/matcher work. In
  particular, anything still speaking as if `api_base` or `api_mount_root` exists on the
  public config surface is stale. Resources now declare full URL patterns in the same
  observable URL space as views.

## API Design Rulings

- Semantic coherence and unrepresentability beat "easy to learn." If sugar is needed to
  make a normal API feel usable, that is evidence the normal API needs redesign.
- Different semantics require different types. This produced `ViewExit` and `HttpExit`;
  views are framework-owned rendering segments, not HTTP documents.
- Nothing reaches the client unless explicitly marked client-facing. Bare server-side
  errors must not leak internal messages.
- Views have no HTTP status surface. Resource and middleware outcomes are real HTTP
  outcomes.
- Redirects ride the exit channel. Handlers should not set response state and then
  fabricate success data after a redirect.
- Resource errors use the JSON envelope `{"error": text}`. Success stays bare JSON.
- `ResourceKind` is a client-codegen/revalidation classification, not a dispatch fact.
  Dispatch and URL conflict reasoning are based on HTTP method.
- Names must answer what the caller puts there. Do not remove semantic words merely to
  avoid field/type stutter. `server_config` can be correct if the value is a config, not a
  server.
- Within a name, canonical abbreviations are good when they do not remove semantics:
  `err`, `msg`, `dir`, `src`, `cmd`. Rust `config` should not become `cfg`, because
  `#[cfg]` owns that spelling.
- `AppConfig` remains an exhaustive struct literal pre-1.0. Revisit `#[non_exhaustive]`
  around a 1.0 stability story.
- `kit/*` is not framework surface for the pressure-test initiative. It may be consumed
  like any third-party library only when that is the realistic app story; it should not be
  treated as required Vorma framework API.
- `vorma/__internal` answers its own question: the double-underscore prefix means
  internal, unstable, undocumented adapter plumbing. Anything intended for real public
  use must be promoted out under a real name.

## Matcher And Routing Lessons

- The matcher's deepest doctrine is specificity. Overlap is normal; a conflict is not
  "two patterns can match the same path", it is an unresolved tie under the same ordering
  the matcher uses at runtime.
- `vorma-matcher` now has typed `FlatMatcher` and `NestedMatcher` surfaces rather than a
  public dual-mode matcher. The builder remains the shared grammar authority.
- `find_overlap` is the lower-level evidence API. It is a free function over either
  matcher type via a sealed trait and returns an `Overlap` witness with an example path and
  the winning patterns. It must stay DRY with the matcher ordering logic.
- API mount root was deleted as a framework concept. Resources and views now share one URL
  space; the framework relies on resources-first dispatch and specificity-aware validation
  rather than a separate mount namespace.
- GET/HEAD resource validation must only reject ties that cannot be resolved by
  specificity. It must not reject coherent cases like a specific GET resource under a
  broader splat view.
- A `/*` catch-all is the branded-404 idiom. It is not a custom not-found slot trial; it is
  the working matcher-backed answer.
- The matcher ordering now has one public home:
  `vorma_matcher::compare_specificity(a, b) -> Ordering`. The runtime matching order,
  cross-table validation, and resource/view dispatch must use that order rather than
  reimplementing scores.
- The specificity order is: segment score, then leftmost differing segment rank, then
  splat-last loses, then longer wins. Do not summarize this as "static beats dynamic" and
  then invent local exceptions.
- The API-mount deletion relies on two facts at once: GET/HEAD resources and views share a
  URL space, and dispatch compares the best resource claim against the best view claim
  with the same specificity order. `ResourceKind` is irrelevant to this validation.
- Static assets are not route patterns. The public static base remains a reserved prefix
  for manifest/public-file serving, not part of route specificity.
- The catch-all correction was proven red-first: a prefix hit is not a match. With views
  `"/foo"` and `"/*"`, request `"/foo/bar"` should be claimed by the catch-all unless a
  covering child such as `"/foo/:id"` also exists.
- Dirty-path semantics were tightened as corrections from Go: at most one trailing slash
  is tolerated as noise, empty path segments never match anything, params/splat values
  never contain empty strings, and root catch-all matches root with empty splat values.
- The nested `params.is_empty()` gate was unreachable and deleted. Treat similar defensive
  code in matcher internals with suspicion: prefer proving and deleting dead branches over
  preserving "just in case" paths.
- Dynamic index routes without an explicitly registered dynamic parent now match their
  parent path. The Go behavior requiring the parent for dynamic index but not static index
  was corrected as an asymmetry, not preserved as precedent.
- Matcher performance was brought above Go by making pattern storage/results cheap and
  avoiding avoidable allocation. The public surface must stay std/Vorma-owned; fast hash
  maps and storage strategy are internal implementation details.
- The wasm binding is the same matcher implementation compiled to wasm. Do not build a
  separate wasm conformance corpus as if it were an independent implementation like the
  old Go-vs-TS world.

## Middleware And Loader Corrections

- `MiddlewareCtx::matched_pattern()` was the wrong abstraction. The real fix was scoped
  middleware declarations, not exposing a request-time route-pattern accessor for scoping
  logic.
- Middlewares live in one `middlewares![...]` array. Declarations can carry optional
  plural filters: `patterns` and `methods`. Omitted means unrestricted; provided fields
  restrict; multiple provided fields AND together.
- Scoped middleware patterns are URL patterns with no hidden rewriting. If a resource URL
  starts with `/api/...`, the middleware scope must say `/api/...`. The reverted
  mount-stripping "fix" was worse because it made one pattern denote two URL spaces.
- Each middleware with `patterns` gets its own matcher; the scope match is a boolean
  run/skip decision. Scope-pattern captures do not become `ctx.params()` or
  `ctx.splat_values()`, which remain the request route's facts.
- Vorma-owned middlewares are still task-aware. Middleware ctx exposes `exec_ctx()`,
  middlewares run in parallel before handlers, and the route handler shares the same
  request task scope. This is the preload-current-user idiom and the reason Vorma
  middlewares exist instead of only Tower layers.
- Go's typed-output task middleware species was considered incidental to Go's task
  chassis. Rust's preferred idiom is sharing a typed `Task` between middleware and
  handler rather than threading middleware output channels.
- Client loaders previously lost typed own-server-data in the public type. The intended
  type surface is that a loader for pattern `P` can access its own view data directly and
  typed, not by fishing through `matches` and casting.

## Client Core Lessons

- The original `create_client_core.ts` monolith already had an elegant base-fact model; the
  right move was to finish that architecture, not split files for taxonomy.
- The decomposed client units are the scheduler, work projection, redirects, wire payload,
  client loaders, route modules, submissions, history position, head, and css. The
  remaining core is a composition root over navigation, fetch transactions, publish, boot,
  and public API.
- Bundle measurement must use real Vite production builds of consumer apps after
  `make ts-build`; npm package size is not the relevant measure.
- `workIndicator` is intentionally shaped for nprogress-like integrations. Vorma API work
  is already included even when orchestrated by react-query; `track(promise)` is for
  non-Vorma async work the app wants reflected in the global indicator.
- App-level react-query integration should use typed wrapper hooks such as `useApiQuery`
  and `useApiMutation`, with Vorma typed args as the call-site interface and key/fn wiring
  in one place. `apiClient.toIdentityArray(args)` is the library-neutral query key
  primitive.

## Pressure-Test App Direction

- The old expanded Notes workout proved breadth but was still contrived. It is being
  replaced by an HN-shaped SQLite app, Vorma Board, whose job is to scrutinize the API as a
  demanding real app developer would.
- The pressure-test app is not the goal; it is the rig. If building it exposes a framework
  simplification, stop and improve the framework rather than forcing the app through a bad
  API.
- The app uses a real SQLite store with a small repo layer and Vorma Tasks at the
  domain-operation level. Do not build a generic task keyed by raw SQL strings.
- The app intentionally integrates normal ecosystem libraries: nprogress, react-query,
  jotai, and `kit/theme` local helpers. The purpose is to prove Vorma cooperates with
  common lifecycles without making those libraries framework requirements.
- `kit/theme` remains a generic TS kit. Rust helper convenience can exist, but TS kit must
  not point at or depend on Rust/Vorma. The TS kit source remains the conceptual source of
  truth.
- Theme boot snippets must do what the runtime initializer does, including storage
  write-backs when runtime getters read resolved values from storage. Do not write a
  "simpler" snippet without first reading the initializer it replaces.
- Do not name the value returned by `initTheme()` as `theme`; that name lies. Destructure
  the returned helpers or use a name that describes the helper bundle honestly.
- Board already surfaced important framework improvements: scoped middlewares, removal of
  the API mount, typed API wrapper design, `workIndicator`/nprogress wiring,
  react-query cooperation, jotai theme reactivity, matcher overlap/specificity fixes, and
  task API pressure. More such detours are the point, not schedule slippage.
- `useApiMutation` in Board should demonstrate the app-owned wrapper idiom: endpoint
  identity at hook creation, varying params/input at mutate time. Queries should use full
  args at the hook because args are the cache identity.
- Manual `revalidate()` after Vorma mutations is usually wrong. Vorma mutations
  auto-revalidate route data by default; react-query should own only its derived caches
  unless a mutation explicitly opts out.
- In the `fable-1` committed tree, Board's attachment download is not a successful
  non-JSON response pressure test. The handler sets `content-disposition` and returns
  `Ok(())`; the public typed resource path serializes `()` as JSON rather than exposing
  the stored bytes. See `FABLE_FABLE1_COMMIT_NOTES.md` before designing the raw-body
  resource surface.
- In the same committed tree, Board's frontend is incomplete. The server graph declares
  eight client files that are absent: `/submit`, `/u/:username`, `/u/:username/comments`,
  `/docs`, `/docs/*`, `/search`, `/mod`, and `/mod/diagnostics`. Treat Board's request
  tests as server/API evidence, not proof that the full browser/Vite app builds.

## Active Tasks-Crate Workstream

- The latest Fable session ended during a `vorma-tasks` first-principles review, not at a
  clean initiative boundary.
- A lost-wakeup race in coalescing wait loops was found and fixed. Loom models were added
  to prove the old ordering deadlocks and the new ordering does not.
- The maintainer approved correctness proofs via loom when concurrency correctness is
  load-bearing. Rare scheduler windows must be proved correct or exposed by tests; they
  are not acceptable without proof.
- A serious unapproved semantic change happened: `run_parallel` was changed from spawned
  Tokio-task parallelism to single-poller `FuturesUnordered` concurrency. This must be
  restored to spawned parallelism before treating the tasks review as complete. The
  general-purpose `vorma-tasks` crate must support CPU-bound parallel work, build systems,
  CLIs, and daemons, not only IO-bound HTTP requests.
- The single-task fast path in `run_parallel` is fine to keep. It avoids spawn overhead for
  a batch of one and has no semantic content.
- The `Task::new(Duration::ZERO, ...)` sentinel was ruled semantically wrong. The intended
  API direction is a constructor split:
  `Task::new(f)` for execution-context-lifetime memoization,
  `Task::with_extended_cache(ttl, f)` for cache beyond the execution context, and
  `Task::singleflight(f)` for concurrent coalescing without completed-result retention.
- Do not call the extended-cache constructor `shared`; both local and extended caches are
  shared. The semantic distinction is cache extent.
- Go's coalesce-only/singleflight mode is a valid use case for long-lived contexts. Do not
  dismiss it as a request-rendering footgun.
- Task-store fingerprints were changed from SipHash to FxHash without surfacing the
  security tradeoff. That should be reverted to SipHash for shared caches keyed by
  attacker-influenced input. The speed cost is tiny relative to the DoS-resistance value.
- `mimalloc` already landed as a default-on `vorma` feature, plus direct declarations for
  standalone benchmark binaries that do not link the `vorma` crate. The library API must
  not expose allocator-related types.
- Benchmark output itself must be the scannable, recordable artifact. The desired shape is
  Go-like: machine header plus one line per benchmark. Results files should be directly
  replaceable from benchmark stdout, not hand-curated summaries.
- `loom-tasks` is wired into `rust-gate` already. That was an unasked expansion beyond the
  standalone `make loom-tasks` target, but it is now part of the current gate shape.
- Remaining tasks-work package from the transcript: restore spawned `run_parallel`,
  re-measure honestly after the restore, add the three constructors and call-site
  migration, model singleflight in loom, revert task fingerprints to SipHash, record the
  behavioral-change inventory in maintainer docs, then continue to the router/request-path
  review.

## Known Stale Or Incomplete Records

- `api_inventory_rust.txt` was generated after the API campaign but before later
  API-mount deletion and tasks/matcher changes. Regenerate it before relying on it for a
  fresh API pass.
- `CURRENT_PLAN_3.md` accurately closes the pre-Board architecture campaign but does not
  include the later Board-driven matcher/API-mount/tasks work. Treat
  `PRESSURE_TEST_CENSUS.md` plus this handoff as later context.
- If docs mention `PathConfig { public_static_base, api_base }`, `api_mount_root`,
  `api_base`, or mount-relative generated resource URLs as current architecture, update
  them before writing user-facing docs.

## Verification Culture

- `make gate` is the pre-release gate, not a per-push CI job by ruling.
- e2e depends on fresh `ts-build`; stale `.dist` caused a real false failure during the
  collapse work and was eliminated by Makefile dependencies.
- `pnpm install` must be escalated immediately under this environment's sandbox rules.
- For Node/oxfmt issues in old transcripts: one Claude session inherited Node 20 through
  a stale PATH, while the user's refreshed environment later reported Node 26. Do not infer
  user environment problems from an agent shell without checking.
