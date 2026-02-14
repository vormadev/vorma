# AST-Based Route Registration Status

## Goal

Make `vorma.NewLoader`/`vorma.NewAction` non-registering at declaration time,
and run current registration internals only from build-time AST-discovered
registrar code.

## Status

- Date: 2026-02-14
- State: implemented for true no-op API + behind-the-scenes overlay registrar
  path, with current resiliency sprint completed
- Endpoint status: AST registration + dev `.go`-change rebuild regression fix is
  complete for this workstream.
- Consumer API impact: no call-site changes required
- Scope guardrails:
    - no `internal/site` edits
    - no `bootstrap` edits
    - framework-internal changes only

## Wave Audit Update (2026-02-14)

- Fixed Wave dev-server readiness probing to use explicit IPv4 loopback host
  (`127.0.0.1`) instead of `localhost` for:
    - app readiness (`resolveAppReadyURL`)
    - Vite readiness (`resolveViteReadyURL`)
- Why this mattered:
    - `localhost` could resolve to a different local listener than the process
      Wave just started, producing false-positive readiness and missed
      app-specific health probes.
- Added regression tests:
    - `TestResolveAppReadyURL_UsesIPv4LoopbackHost`
    - `TestResolveViteReadyURL_UsesIPv4LoopbackHost`
    - existing orchestration regressions now pass:
        - `TestBroadcastReload_WaitsForAppAndViteBeforeBroadcast`
        - `TestServerRun_ConfigRestartWaitsForAppBeforeReloadAndContinues`
- Watcher noise triage for
  `internal/site/backend/assets/markdown/docs/~/readme.md(.tmp)`:
    - classified as app/tooling interaction (not AST registration path):
        - `internal/site` places generated docs under `backend/assets` (private
          static tree watched by Wave defaults)
        - docsync writes `*.tmp` then renames, which naturally emits fsnotify
          create/remove events.

## Current Resiliency Sprint (4/4 Done)

1. Done: fail-fast validation for discovered registration expressions that
   reference function-local-only symbols not valid at package-init emission
   scope.
2. Done: canonical registration call identity now prefers `go/types` object
   identity (package path + object name), with existing AST/import-path
   fallback.
3. Done: discovery now respects compiled-file selection (build tags / file
   inclusion) and can still follow helper definitions in same-package compiled
   files outside matched roots.
4. Done: added end-to-end multi-package runtime registration regression test
   that verifies discovered registrations are present in the compiled runtime
   binary.

## What Changed

1. `NewLoader` and `NewAction` are now true registration no-ops.
2. Added explicit registration entry points used only by generated code:
    - `vorma.Internal__RegisterDiscoveredLoader`
    - `vorma.Internal__RegisterDiscoveredAction`
3. Extended backend AST discovery traversal to capture canonical framework call
   invocations (with bound args for wrapper call paths).
4. Added registrar source generation for discovered calls with explicit
   init-time registration calls.
5. Removed build-command pre-rerun overlay flow; build command now executes
   directly without internal self-rerun/marker behavior.
6. Wired Wave compile step to request framework-provided Go build overlay
   (`FrameworkPrepareGoBuildOverlay`) so runtime binaries are compiled with the
   same discovered registration overlay.
7. Wired framework hook execution callback (`FrameworkRunBuildHook`) so Wave can
   run framework build hooks directly via
   `go run -overlay=... ./<mainBuildEntry> --hook` (and `--dev` in development
   mode), without internal self-rerun marker flow.
8. Added build-tag-aware package file expansion for backend discovery:
    - root discovery still starts from `ServerRouteDefinitionPatterns`
    - helper resolution can include same-package compiled files selected by Go
      build rules
    - non-compiled/tag-excluded files are filtered out.
9. Added `go/types`-based canonical call identity resolution to reduce
   false-positive/false-negative risk from import-alias or shadowing shapes.
10. Added package-init-scope safety validation for discovered registration
    arguments (`app`, `handler`, `decorateCtx`) with fail-fast errors when local
    symbols are not re-emittable.
11. Added e2e multi-package runtime registration test coverage that builds a
    fixture app and asserts discovered loader handlers are registered in the
    compiled binary.
12. Fixed init-scope validator false positives for symbols declared inside
    self-contained inline expressions (for example wrapper-bound function
    literal params/locals); only true out-of-expression function-local captures
    are now rejected.
13. Added discovery traversal support for init-reachable immediately invoked
    function literal wrappers (`func(...) { ... }(...)`) including parameter
    binding for compile-time pattern resolution.
14. Added fail-fast validation during registrar import collection for
    unqualified external symbols in discovered expressions (dot-import style),
    when resolvable via `go/types`.

## Current Contract

- Route declaration execution is no longer the registration mechanism for
  framework APIs.
- Registration is emitted from build-discovered call data into temporary overlay
  source inputs.
- Consumer source trees are not written with registrar files.
- Runtime Go compilation now also consumes overlay inputs via framework callback
  so loader/action handlers exist in compiled app binaries.
- Framework hook execution uses direct callback command invocation with overlay
  and no internal self-rerun marker flow.

## Overlay Cache Fingerprint Scope

- Fingerprint currently includes:
    - matched server-route root files
    - all non-test `.go` file bytes in each discovered route package directory
      (same-package helper files included).
- Fingerprint currently does not hash full transitive import dependency graphs.
- This is intentional for perf: route discovery/generation depends on the route
  declaration packages and same-package helper code, while normal Go rebuild
  invalidation already handles transitive dependency recompilation.
- Regression proof now exists:
    - dependency-only handler behavior changes (outside discovered route package
      dirs) still produce updated runtime handler output under cached discovery
      artifacts.

## Dev Rebuild Regression Fix

- Root cause identified for "first `make dev` works, then `.go` save breaks
  `vorma.gen/index.ts`":
    - `.go` watch hook path used `RunCombinedDevBuildHookCommands` command
      chaining and bypassed `FrameworkRunBuildHook` callback.
    - That skipped overlay-backed discovered route registrar injection during
      rebuild hook execution, so runtime handlers were missing in TS generation.
- Fix implemented:
    - watch hook planning/execution now resolves a hook execution plan that uses
      `FrameworkRunBuildHook` when configured, instead of relying solely on
      framework hook command strings for combined dev hooks.
- Regression test added for this exact path:
    - combined dev hook execution on concurrent stage now verifies:
        - user dev build hook command runs
        - framework build hook runner callback runs
        - framework command-string fallback is not used when callback exists.

## Why Generated Registrar Exists

Given the current constraints:

1. `vorma.NewLoader`/`vorma.NewAction` must be true no-op for registration.
2. Consumers should not have to rewrite route declarations or maintain a
   separate manual route spec list.

Then something still has to execute real registration calls at runtime with
actual handler/decorator function values. AST discovery gives locations and
expressions, but does not itself execute registration. Generated registrar code
is the mechanism that bridges AST discovery to runtime execution without
consumer call-site changes.

## How Handler Reachability Works

The system does not serialize or copy handler implementations. Instead:

1. AST discovery finds canonical `vorma.NewLoader` / `vorma.NewAction` calls
   (including some local wrapper-call argument binding paths).
2. It captures the original call expressions for `app`, `handler`,
   `decorateCtx`, and action `method`/`pattern`.
3. It renders a registrar source file in the same Go package that calls
   `vorma.Internal__RegisterDiscoveredLoader` /
   `vorma.Internal__RegisterDiscoveredAction` with those expressions.
4. That registrar source is fed to the Go compiler via a temporary `-overlay`
   map (not written into consumer trees).
5. Because the generated source is compiled as part of the same package, normal
   Go type-checking resolves user-defined types, dependencies, and generics for
   handlers exactly as if they were referenced in handwritten code.

In short: we are not “grabbing internals”; we are compiling additional package
source that references the same symbols.

## Visibility Clarification

User-visible generated registrar files in app packages are not required for
AST-based registration and are no longer used in the current implementation.

- Fundamental requirement:
    - some executable registration artifact must exist so discovered routes are
      actually registered with concrete handler function values.
- Current implementation:
    - transient overlay-backed registrar sources consumed by Go compilation.
    - framework hook execution is direct build-hook command invocation with
      overlay and no rerun plumbing.

## Tradeoffs And Concerns

Compared to the old side-effect/import model:

- Solved: consumers no longer need explicit import wiring solely to trigger
  runtime registration side effects.
- New cost: route declarations now must be statically discoverable and safely
  re-emittable into generated package-init registration code.

1. Dynamic method/path values are intentionally unsupported for AST registration
   discovery. Method/path must be compile-time resolvable strings.
2. Overlay temp artifact lifecycle must stay robust; failures in temp-file
   writes/cleanup can fail build startup and need strong error handling.
3. Helper call-graph traversal is intentionally bounded (local package
   identifier-call chains), not arbitrary cross-package dynamic analysis.

## Additional Review: Other Behavior Classes

1. Supported and now covered by regression tests:
    - direct inline loader/action function literals at package scope
    - wrapper-bound inline handler/decorate literals with expression-local
      params/locals
    - init-reachable IIFE wrappers with bound params.
2. Still intentionally unsupported:
    - runtime-dynamic method/path values
    - helper indirections requiring arbitrary cross-package call-graph
      traversal.
3. Hardened with explicit regression coverage:
    - generated-import alias collisions across files fail fast with explicit
      errors during registrar generation.
4. Partially hardened:
    - unqualified external symbols in discovered expressions now fail fast when
      `go/types` identity is available, instead of surfacing later as generated
      compile failures.

### Concrete Examples

1. Init-scope re-emission failure:
    - If route declaration happens inside a function and uses local vars:
      `decorate := func(...) ...; vorma.NewLoader(app, "/x", h, decorate)`
    - Discovery now fails fast before generation with an explicit
      `function-local symbol` error for the offending argument.
    - Full example:

        ```go
        func registerUsers() {
        	localDecorate := func(rd *vorma.LoaderReqData) *LoaderCtx {
        		return &LoaderCtx{LoaderReqData: rd}
        	}
        	_ = vorma.NewLoader(App, "/users", usersLoader, localDecorate)
        }

        var _ = registerUsers()
        ```

        This now fails early with an init-scope validation error.

2. False-negative from non-constant method/pattern:
    - Current discovery requires compile-time strings for `NewAction` method and
      pattern; something like `method := strings.ToUpper("post")` will not
      resolve as compile-time constant, so registration discovery fails.
3. Unqualified external-symbol expression class:
    - Dot-import-style unqualified external symbols in discovered expressions
      are now fail-fast validated (when resolvable through `go/types`) to avoid
      deferred generated-compile failures.

### Most Likely Normal-User Failure Case

Most likely failure for a normal user is using non-constant route method/pattern
values where discovery currently requires compile-time string resolution.

Example risk pattern:

- `method := strings.ToUpper("post")`
- `path := "/" + section + "/list"` where `section` is not a `const`

In those cases discovery may fail to resolve method/pattern and either skip or
error, even though the user’s intent is reasonable.

Scope rule clarification:

- It is not strictly "routes must be declared at package top level."
- The real constraint is: discovered registration expressions must be valid when
  emitted into package `init` scope.
- Top-level declarations satisfy this by default.
- Function-scoped route declarations can still work only when all referenced
  symbols/expressions resolve cleanly at package init scope (no local-only
  symbols that disappear outside function scope).

## App Helper Support

App builders can define their own helper wrappers across many packages.

Supported multi-package model:

- Route declarations can live in many packages, as long as those files are
  included by `ServerRouteDefinitionPatterns`.
- Those packages must still be part of the compiled app build graph so their
  generated registrar init code executes at runtime.

Safe helper pattern:

1. Keep declaration entrypoints at package scope (`var _ = ...` or `init()`).
2. Keep helper wrappers as package-level functions in discovered files/packages.
3. Ensure helper call chains eventually resolve to canonical
   `vorma.NewLoader`/`vorma.NewAction` (or canonical mux register calls).
4. Keep method/path compile-time-resolvable (const/literal/const concat).
5. Avoid function-local-only symbols in expressions used for registration args.

Current limitations:

- Discovery follows local package function-call chains (identifier calls) with
  arg binding, but does not do full cross-package arbitrary helper call-graph
  resolution.
- Method receivers and function-literal call graphs are not traversal roots for
  helper resolution in the current implementation.

Likely helper pitfall and fix:

- Pitfall:

    ```go
    func RegisterUserRoutes() {
    	decorateWithUserDeps := func(rd *vorma.LoaderReqData) *LoaderCtx {
    		return &LoaderCtx{LoaderReqData: rd, UserRepo: userRepo}
    	}
    	_ = vorma.NewLoader(App, "/users", loadUsers, decorateWithUserDeps)
    }

    var _ = RegisterUserRoutes()
    ```

    `decorateWithUserDeps` is function-local, so generated package-init
    registrar code cannot reference it.

- Preferred:

    ```go
    func decorateWithUserDeps(rd *vorma.LoaderReqData) *LoaderCtx {
    	return &LoaderCtx{LoaderReqData: rd, UserRepo: userRepo}
    }

    var _ = vorma.NewLoader(App, "/users", loadUsers, decorateWithUserDeps)
    ```

## Impossible Vs Reorganizable

Currently impossible (by design/implementation limits):

1. Runtime-dynamic route keys (`method`/`path`) that are not compile-time
   resolvable strings.
2. Route-registration patterns hidden inside function literals / local-only
   closure scopes that cannot be referenced from package init scope.
3. Some advanced helper indirections (for example selector-based non-canonical
   cross-package call graph traversal) that the current resolver does not
   follow.

Usually reorganizable to work:

1. Splitting routes across many packages.
2. Custom helper layers/wrappers.
3. Shared decoration/context assembly logic.

Reorganization rule: keep discovered registration expressions representable at
package init scope and keep canonical registration visibility in discovered
files/packages.

## Goal-Level Limits (Not Syntax-Level)

Goals currently not achievable with this AST model:

1. Runtime route topology from runtime data:
    - Example: fetch routes from DB/config service/feature flags at startup and
      mount different paths/methods per environment or tenant.
2. Late-bound plugin route loading after compile:
    - Example: drop-in plugin binaries/modules discovered at runtime that add
      routes without being in the compile/discovery graph.
3. Truly dynamic API surface per request/session:
    - Example: user role/session/tenant determines what routes "exist" at all,
      not just what handlers return.

Goals that remain achievable:

1. Large multi-package route architecture in app source.
2. Layered helper/wrapper abstractions around loader/action declarations.
3. Shared contextual decoration/injection logic, if symbols remain usable from
   package-init-emittable expressions.

## Function-Local Scope Limitation: Goal Impact

Considering only the function-local-scope constraint:

- Mostly this does **not** block product goals; it mainly constrains
  organization/encapsulation style.
- The goal it does block is: keep route registration logic entirely inside
  function-local/private closure scope while still having those local symbols be
  the direct registration arguments.

Concrete blocked goal pattern:

1. "My route handlers/decorators should be private local closures over local
   deps in `RegisterXRoutes(...)`, with no package-scope symbols."

That goal conflicts with package-init-emitted registration because those local
symbols do not exist at package init scope.

## Dev-Server Perf And Parallelism Notes

- Common framework build-hook execution now runs in one process via
  `FrameworkRunBuildHook` callback
  (`go run -overlay=... ./<mainBuildEntry> --hook ...`) without overlay-rerun
  marker plumbing.
- Runtime app compilation prepares overlay before `go build` so compiled
  binaries include discovered registrations.
- Build command internal self-rerun
  (`--__vorma_internal_registrars_synchronized`) has been removed.

Current parallelism:

1. Dev server initial/restart path runs hook/file build and Go compile in
   parallel when `Core.SequentialGoBuild` is false.
2. Event pipeline runs build phase and concurrent hooks in parallel.
3. Build phase itself parallelizes private static processing and CSS work.
4. User and framework build hooks remain intentionally sequential (user first,
   framework second) due dependency ordering.

Recent perf improvement:

- Removed build-command overlay self-rerun flow.
- Removed internal marker-flow from framework hook callback command path.
- Added package-level hint pre-scan to skip expensive discovery initialization
  when no registration primitives are present.

## Work Log

- 2026-02-14: Marked queue-based deferred-registration approach invalid.
- 2026-02-14: Removed queue-based runtime plumbing.
- 2026-02-14: Implemented no-op `NewLoader`/`NewAction` and explicit discovered
  registration helpers.
- 2026-02-14: Implemented discovered registrar generation and build rerun hook.
- 2026-02-14: Added/updated tests for:
    - overlay-backed registrar generation output and no consumer-tree file
      writes
    - runtime helper behavior for no-op vs explicit registration
    - build CLI rerun behavior and internal marker flag parsing
- 2026-02-14: Clarified explicit tradeoff comparison versus side-effect import
  registration model.
- 2026-02-14: Replaced on-disk consumer registrar generation with temporary
  Go-overlay-backed registrar inputs.
- 2026-02-14: Updated build CLI rerun path to pass `-overlay=` and cleanup
  temporary overlay artifacts after rerun.
- 2026-02-14: Added framework-to-Wave compile integration so `go build` uses
  discovered registration overlay, fixing missing runtime loader/action
  registration in compiled apps.
- 2026-02-14: Added regression tests for compile-path overlay argument wiring
  and callback lifecycle (prepare + cleanup).
- 2026-02-14: Added framework build-hook runner callback integration so hook
  execution uses direct overlay+marker command and avoids extra rerun.
- 2026-02-14: Added tests verifying framework build-hook runner wiring,
  preservation, argument construction, and overlay cleanup lifecycle.
- 2026-02-14: Added init-scope fail-fast validation for discovered registration
  arguments that reference function-local-only symbols.
- 2026-02-14: Added `go/types`-identity canonical registration call matching
  (with fallback), reducing alias/shadowing misclassification risk.
- 2026-02-14: Added build-tag-aware compiled-file inclusion/filtering for
  backend discovery and helper traversal.
- 2026-02-14: Added multi-package runtime e2e regression test that verifies
  discovered registrations exist in compiled app binaries.
- 2026-02-14: Fixed false-positive init-scope validation on wrapper-bound inline
  function literals (`handler`/`decorateCtx` and action equivalents) and added
  regression tests covering those patterns.
- 2026-02-14: Added support for init-reachable IIFE route wrappers and bound
  parameter propagation for discovery.
- 2026-02-14: Added fail-fast validation for unqualified external symbols in
  discovered expressions (dot-import style) during registrar import collection.
- 2026-02-14: Added regression coverage for cross-file conflicting import alias
  detection during registrar generation.
- 2026-02-14: Switched go/types import resolution to export-data importer for
  lower discovery latency.
- 2026-02-14: Removed build CLI internal overlay-rerun marker flow and its test
  surface.
- 2026-02-14: Simplified framework build-hook callback command path to direct
  `go run -overlay=... ./<mainBuildEntry> --hook` invocation with no internal
  marker flow.
- 2026-02-14: Added package-level route-registration hint pre-scan in discovery
  initialization.

## Test Results

- `go test ./vormabuild -count=1` passes.
- `go test ./vormabuild -run 'TestParseBackendLoaderPatterns|TestPrepareDiscoveredRouteRegistrarOverlay|TestBuildRuntimeRegistration_E2EMultiPackageDiscoveredRoutes|TestConfigureBuildEnvironment_' -count=1`
  passes.
- `go test ./vormabuild -run TestParseBackendLoaderPatterns -count=1` passes
  with added inline-literal and IIFE discovery regression coverage.
- `go test ./vormabuild -run TestPrepareDiscoveredRouteRegistrarOverlay -count=1`
  passes with added unqualified-external-symbol fail-fast regression coverage.
- `go test ./vormabuild -run 'TestPrepareDiscoveredRouteRegistrarOverlay|TestParseBackendLoaderPatterns' -count=1`
  passes with added alias-collision and inline/IIFE coverage.
- `go test ./vormabuild -run 'TestParseBuildCommandOptions|TestRunBuildCommand|TestConfigureBuildEnvironment_|TestParseBackendLoaderPatterns|TestPrepareDiscoveredRouteRegistrarOverlay' -count=1`
  passes after removal of build CLI rerun/marker flow.
- `go test ./wave/tooling -run 'TestRunHooks_|TestBuildGoBuildCommand_|TestBuilderCompileGoOnly_' -count=1`
  passes.
- `go test ./... -count=1` still fails in pre-existing `wave/tooling` tests
  unrelated to this route-registration work.
