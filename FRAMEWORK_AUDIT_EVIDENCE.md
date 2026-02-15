# Framework Audit Evidence (Append-Only)

## Usage

- Append-only log of concrete audit evidence for the current reset cycle.
- Reference entries from `FRAMEWORK_AUDIT.md` matrix cell completion decisions.

## Entry Format

- ID: `EV-YYYYMMDD-###`
- Package group
- Pass name
- Evidence
    - files reviewed
    - commands/tests run
    - findings/fixes or explicit none-found statement

---

## EV-20260214-001

- Package group: `vorma.go`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vorma.go`
        - `vorma_test.go`
        - `vormabuild/discovered_registration_runtime_test.go`
        - `vormabuild/backend_route_discovery.go` (discovered registration
          parse/validation)
        - `vormabuild/backend_route_registration_generation.go` (generated
          registration call emission)
    - Commands/tests run:
        - `sed -n '1,260p' vorma.go`
        - `sed -n '1,260p' vorma_test.go`
        - `rg -n "\\bNewLoader\\b|\\bNewAction\\b|Internal__RegisterDiscoveredLoader|Internal__RegisterDiscoveredAction"`
        - `sed -n '1,220p' vormabuild/discovered_registration_runtime_test.go`
        - `sed -n '880,1070p' vormabuild/backend_route_discovery.go`
        - `sed -n '470,560p' vormabuild/backend_route_registration_generation.go`
    - Findings/fixes:
        - Found `F-20260214-001`: `vorma.NewLoader`/`vorma.NewAction` accept
          runtime-registration-looking arguments but currently act as discovery
          markers while discovered registration is performed by generated `init`
          calls to `vorma.Internal__RegisterDiscoveredLoader` and
          `vorma.Internal__RegisterDiscoveredAction`.
        - No code change applied in this step because fixing this requires a
          user decision about API semantics and code generation strategy.

## EV-20260214-002

- Package group: `vorma.go`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vorma.go`
        - `kit/mux/mux.go` (middleware signature verification for wrapper)
        - `vormaruntime/get_root_handler.go` (`IsJSONRequest` and build header
          constant source)
    - Commands/tests run:
        - `rg -n "vorma\\.(MustGetPort|GetIsDev|SetModeToDev|VormaBuildIDHeaderKey|EnableThirdPartyRouter|IsJSONRequest|NewAppRoot|RunAppModuleRegistrationLifecycle)\\s*="`
        - `rg -n "func InjectTasksCtxMiddleware|InjectTasksCtxMiddleware\\(" kit/mux -S`
        - `rg -n "func IsJSONRequest\\(" vormaruntime/get_root_handler.go`
        - `gofmt -w vorma.go`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - `F-20260214-001` resolved by user decision on 2026-02-15: keep AST
          discovery + generated registration model, and reject runtime
          side-effect registration semantics.
        - Found and fixed `F-20260214-002`: public `vorma` root package
          re-exported multiple APIs as mutable `var` aliases, which allowed
          external reassignment of framework entry points and header key values.
        - Replaced mutable aliases in `vorma.go` with stable exports:
          `VormaBuildIDHeaderKey` as `const`, and function wrappers for
          `MustGetPort`, `GetIsDev`, `SetModeToDev`, `IsJSONRequest`,
          `EnableThirdPartyRouter`, `NewAppRoot`, and
          `RunAppModuleRegistrationLifecycle`.
        - Verified full required gates pass. `make tslint` reported existing
          warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` but no
          errors.

## EV-20260214-003

- Package group: `bootstrap/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `bootstrap/bootstrap.go`
        - `bootstrap/utils.go`
        - `bootstrap/ux_conformance_test.go`
        - `bootstrap/tmpls/backend_src_router_app_go_tmpl.txt`
        - `bootstrap/tmpls/backend_src_router_context_go_tmpl.txt`
        - `bootstrap/tmpls/backend_src_router_init_go_tmpl.txt`
        - `bootstrap/tmpls/backend_src_router_example_routes_go_tmpl.txt`
        - `bootstrap/tmpls/cmd_app_main_go_tmpl.txt`
        - `bootstrap/tmpls/cmd_build_main_go_tmpl.txt`
        - `bootstrap/tmpls/package_json_tmpl.txt`
        - `bootstrap/tmpls/vite_config_ts_tmpl.txt`
        - `bootstrap/tmpls/wave_config_json_tmpl.txt`
        - `bootstrap/tmpls/backend_wave_dev_go_str.txt`
        - `bootstrap/tmpls/backend_wave_prod_go_str.txt`
        - `bootstrap/tmpls/dockerfile_tmpl.txt`
        - `bootstrap/tmpls/vercel_json_tmpl.txt`
        - `bootstrap/tmpls/api_proxy_ts_str.txt`
    - Commands/tests run:
        - `rg --files bootstrap`
        - `rg -n "^func |^type |^const |^var " bootstrap/*.go`
        - `sed -n '1,260p' bootstrap/bootstrap.go`
        - `sed -n '260,520p' bootstrap/bootstrap.go`
        - `sed -n '1,220p' bootstrap/utils.go`
        - `sed -n '1,340p' bootstrap/ux_conformance_test.go`
        - `sed -n '1,220p' bootstrap/tmpls/backend_src_router_context_go_tmpl.txt`
        - `sed -n '1,220p' bootstrap/tmpls/backend_src_router_example_routes_go_tmpl.txt`
        - `sed -n '1,220p' bootstrap/tmpls/backend_src_router_app_go_tmpl.txt`
        - `sed -n '1,220p' bootstrap/tmpls/backend_src_router_init_go_tmpl.txt`
        - `sed -n '1,220p' bootstrap/tmpls/cmd_app_main_go_tmpl.txt`
        - `sed -n '1,240p' bootstrap/tmpls/cmd_build_main_go_tmpl.txt`
        - `sed -n '1,260p' bootstrap/tmpls/package_json_tmpl.txt`
        - `sed -n '1,260p' bootstrap/tmpls/vite_config_ts_tmpl.txt`
        - `sed -n '1,260p' bootstrap/tmpls/wave_config_json_tmpl.txt`
        - `sed -n '1,220p' bootstrap/tmpls/backend_wave_dev_go_str.txt`
        - `sed -n '1,220p' bootstrap/tmpls/backend_wave_prod_go_str.txt`
        - `sed -n '1,260p' bootstrap/tmpls/dockerfile_tmpl.txt`
        - `sed -n '1,240p' bootstrap/tmpls/vercel_json_tmpl.txt`
        - `sed -n '1,240p' bootstrap/tmpls/api_proxy_ts_str.txt`
    - Findings/fixes:
        - No additional Surface/API findings in `bootstrap/*` after this sweep.

## EV-20260214-004

- Package group: `vormabuild/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormabuild/vorma_build.go`
        - `vormabuild/build_environment.go`
        - `vormabuild/configschema.go`
        - `vormabuild/route_parsing.go`
        - `vormabuild/route_parsing_pipeline.go`
        - `vormabuild/route_registry_build.go`
        - `vormabuild/vorma_gen_ts.go`
        - `vormabuild/vorma_gen_ts_metadata.go`
        - `vormabuild/vorma_gen_ts_vite_and_write.go`
    - Commands/tests run:
        - `rg --files vormabuild`
        - `for f in vormabuild/*.go; do if [[ $f != *_test.go ]]; then rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" "$f"; fi; done`
        - `rg -n "vormabuild\\.(VormaSchema|RouteCall|UnresolvedRouteCall|TSGenInput|WriteGeneratedTS|Build)\\b" -S --glob '!vormabuild/**'`
        - `sed -n '1,240p' vormabuild/vorma_build.go`
        - `sed -n '1,220p' vormabuild/route_parsing.go`
        - `sed -n '1,240p' vormabuild/vorma_gen_ts.go`
        - `sed -n '1,280p' vormabuild/vorma_gen_ts_vite_and_write.go`
        - `sed -n '1,220p' vormabuild/configschema.go`
        - `gofmt -w vormabuild/build_environment.go vormabuild/configschema.go vormabuild/route_parsing.go vormabuild/route_parsing_pipeline.go vormabuild/route_parsing_pipeline_test.go vormabuild/route_parsing_test.go vormabuild/route_registry_build.go vormabuild/vorma_gen_ts.go vormabuild/vorma_gen_ts_error_paths_test.go vormabuild/vorma_gen_ts_metadata.go vormabuild/vorma_gen_ts_test.go vormabuild/vorma_gen_ts_vite_and_write.go`
        - `go test ./vormabuild`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260214-003`: `vormabuild` exposed several
          internal-only symbols as public API without external callsites in this
          repository (`VormaSchema`, `RouteCall`, `UnresolvedRouteCall`,
          `TSGenInput`, `WriteGeneratedTS`), which widened the unstable API
          surface unnecessarily.
        - Tightened API boundary by unexporting those symbols and updating
          internal references/tests:
            - `VormaSchema` -> `vormaSchema`
            - `RouteCall` -> `routeCall`
            - `UnresolvedRouteCall` -> `unresolvedRouteCall`
            - `TSGenInput` -> `tsGenInput`
            - `WriteGeneratedTS` -> `writeGeneratedTS`
        - Post-change exported API check confirms only
          `vormabuild.Build(*vormaruntime.Vorma)` remains exported from non-test
          files.
        - Required gates pass. `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` with no
          errors.

## EV-20260214-005

- Package group: `vormaruntime/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaruntime/types.go`
        - `vormaruntime/paths.go`
        - `vormaruntime/get_root_handler.go`
        - `vormaruntime/vorma_core.go`
    - Commands/tests run:
        - `for f in vormaruntime/*.go; do if [[ $f != *_test.go ]]; then rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" "$f"; fi; done`
        - `for f in vormaruntime/*.go; do if [[ $f != *_test.go ]]; then rg -n "^var [A-Z]" "$f"; fi; done`
        - `sed -n '1,220p' vormaruntime/types.go`
        - `sed -n '1,220p' vormaruntime/paths.go`
        - `rg -n "\\bUIVariants\\.(React|Preact|Solid)\\b"`
        - `rg -n "\\bVormaPaths\\b"`
        - `gofmt -w vormaruntime/get_root_handler.go vormaruntime/glue_test.go vormaruntime/paths.go vormaruntime/paths_test.go vormaruntime/test_helpers_test.go vormaruntime/types.go vormaruntime/vorma_init_test.go vormabuild/backend_route_discovery_test.go vormabuild/build_artifacts_test.go vormabuild/test_helpers_test.go vormabuild/vorma_gen_ts_error_paths_test.go vormabuild/vorma_gen_ts_test.go vormabuild/vorma_gen_ts_vite_and_write.go`
        - `go test ./vormaruntime ./vormabuild`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260214-004`: `vormaruntime` exported mutable
          global variables (`UIVariants` and `VormaPaths`) that widened public
          mutation surface for core runtime semantics.
        - Replaced mutable exports with immutable APIs:
            - `UIVariants` replaced by constants: `UIVariantReact`,
              `UIVariantPreact`, `UIVariantSolid`.
            - `VormaPaths` replaced by functions:
              `VormaPathsStageOneJSONPath()`, `VormaPathsStageTwoJSONPath()`.
        - Updated all internal and cross-package references/tests accordingly.
        - Required gates pass. `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` with no
          errors.

## EV-20260214-006

- Package group: `wave/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `wave/wave.go`
        - `wave/env.go`
        - `wave/runtime_framework.go`
        - `wave/types.go`
        - `wave/types_methods.go`
        - `wave/parse.go`
    - Commands/tests run:
        - `rg --files wave`
        - `for f in wave/*.go; do if [[ $f != *_test.go ]]; then rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" "$f"; fi; done`
        - `for f in wave/*.go; do if [[ $f != *_test.go ]]; then rg -n "^var [A-Z]" "$f"; fi; done`
        - `sed -n '1,220p' wave/wave.go`
        - `sed -n '1,220p' wave/env.go`
        - `sed -n '1,220p' wave/runtime_framework.go`
        - `sed -n '1,260p' wave/types_methods.go`
        - `sed -n '1,180p' wave/types.go`
        - `sed -n '1,260p' wave/parse.go`
        - `rg -n "\\bRelPaths\\.[A-Za-z]+\\("`
    - Findings/fixes:
        - No additional Surface/API findings in `wave/*` after this sweep.

## EV-20260214-007

- Package group: `wave/tooling/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `wave/tooling/builder.go`
        - `wave/tooling/cli.go`
        - `wave/tooling/devserver.go`
        - `wave/tooling/devserver_config.go`
        - `wave/tooling/events_hook_executor.go`
        - `wave/tooling/watcher.go`
        - `wave/tooling/watcher_debouncer.go`
        - `wave/tooling/watcher_dirs.go`
        - `wave/tooling/watcher_matching.go`
        - `wave/tooling/watcher_setup.go`
    - Commands/tests run:
        - `for f in wave/tooling/*.go; do if [[ $f != *_test.go ]]; then rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" "$f"; fi; done`
        - `rg -n "\\b(NewWatcher|Watcher|NewDebouncer|Debouncer|CLIOptions|ParseCLIOptions|BuildWaveWithHookFromArgs|BuildWaveWithHookOptions|BuildWaveWithHook|BuildWave|SetupDistDir|ValidateConfig|ErrLockHeld)\\b" --glob '!wave/tooling/**'`
        - `sed -n '1,220p' wave/tooling/builder.go`
        - `sed -n '1,220p' wave/tooling/cli.go`
        - `sed -n '1,260p' wave/tooling/watcher.go`
        - `sed -n '1,220p' wave/tooling/watcher_debouncer.go`
        - `sed -n '1,200p' wave/tooling/devserver.go`
        - `gofmt -w wave/tooling/*.go`
        - `go test ./wave/...`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260214-005`: `wave/tooling` exposed internal
          devserver implementation types as public API (`Watcher`, `NewWatcher`,
          `Debouncer`, `NewDebouncer`) with no external repository callsites.
        - Tightened API boundary by unexporting those symbols to
          `watcher`/`newWatcher` and `debouncer`/`newDebouncer`, and updated
          internal references/tests.
        - Corrected one mechanical rename regression (`fsnotify.newWatcher` ->
          `fsnotify.NewWatcher`) and re-verified.
        - Required gates pass. `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` with no
          errors.

## EV-20260215-001

- Package group: `vormaclient/client/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaclient/client/index.ts`
        - `vormaclient/client/internal.ts`
        - `vormaclient/client/buildtime.ts`
        - `vormaclient/client/src/client.ts`
        - `vormaclient/react/src/helpers.ts`
        - `vormaclient/react/src/link.tsx`
        - `vormaclient/react/src/react.tsx`
        - `vormaclient/preact/src/helpers.ts`
        - `vormaclient/preact/src/link.tsx`
        - `vormaclient/preact/src/preact.tsx`
        - `vormaclient/solid/src/helpers.ts`
        - `vormaclient/solid/src/link.tsx`
        - `vormaclient/solid/src/solid.tsx`
        - `vormaclient/client/dist_tests/npm_dist_client_runtime.test.ts`
    - Commands/tests run:
        - `cat vormaclient/client/index.ts`
        - `cat vormaclient/client/internal.ts`
        - `cat vormaclient/client/buildtime.ts`
        - `sed -n '1,340p' vormaclient/client/src/client.ts`
        - `rg -n "^export (type |interface |const |function |class |let |var |\\{)" vormaclient/client/src -g'*.ts' -g'*.tsx'`
        - `rg -n "from \\\"vorma/client/__internal\\\"" vormaclient/react vormaclient/preact vormaclient/solid vormaclient/client/dist_tests -g'*.ts' -g'*.tsx'`
        - `rg -n "\\b(runClientLoadersAfterHMRUpdate|registerClientLoaderPattern|getClientRuntimeRenderState|setClientLoaderWaitFn|RouteOutletBranchState)\\b" vormaclient/react vormaclient/preact vormaclient/solid vormaclient/client/dist_tests --glob '!**/node_modules/**'`
        - `pnpm prettier --write vormaclient/client/internal.ts`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260215-001`: `vorma/client/__internal` exported
          several low-level runtime hooks/types with no in-repo consumers
          (`runClientLoadersAfterHMRUpdate`, `registerClientLoaderPattern`,
          `getClientRuntimeRenderState`, `setClientLoaderWaitFn`, and
          `RouteOutletBranchState`), which widened the unstable boundary beyond
          what adapters and dist tests use.
        - Tightened `vormaclient/client/internal.ts` by removing those exports
          while preserving all symbols consumed by `vormaclient/react`,
          `vormaclient/preact`, `vormaclient/solid`, and dist tests.
        - Required gates pass. `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` with no
          errors.

## EV-20260215-002

- Package group: `vormaclient/react/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaclient/react/index.tsx`
        - `vormaclient/react/src/helpers.ts`
        - `vormaclient/react/src/react.tsx`
        - `vormaclient/react/src/link.tsx`
        - `npm_dist/vormaclient/react/index.d.ts`
    - Commands/tests run:
        - `rg --files vormaclient/react`
        - `sed -n '1,260p' vormaclient/react/index.tsx`
        - `sed -n '1,320p' vormaclient/react/src/helpers.ts`
        - `sed -n '1,340p' vormaclient/react/src/react.tsx`
        - `sed -n '1,300p' vormaclient/react/src/link.tsx`
        - `rg -n "^export (type |interface |const |function |class |let |var |\\{)" vormaclient/react -g'*.ts' -g'*.tsx'`
        - `sed -n '1,260p' npm_dist/vormaclient/react/index.d.ts`
        - `rg -n "from \\\"vorma/react\\\"|import\\(\\\"vorma/react\\\"\\)" --glob '!node_modules/**'`
    - Findings/fixes:
        - No additional Surface/API findings in `vormaclient/react/*` after this
          sweep.

## EV-20260215-003

- Package group: `vormaclient/preact/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaclient/preact/index.tsx`
        - `vormaclient/preact/src/helpers.ts`
        - `vormaclient/preact/src/preact.tsx`
        - `vormaclient/preact/src/link.tsx`
        - `vormaclient/preact/src/preact.test.tsx`
        - `npm_dist/vormaclient/preact/index.d.ts`
    - Commands/tests run:
        - `rg --files vormaclient/preact`
        - `sed -n '1,260p' vormaclient/preact/index.tsx`
        - `sed -n '1,320p' vormaclient/preact/src/helpers.ts`
        - `sed -n '1,380p' vormaclient/preact/src/preact.tsx`
        - `sed -n '1,300p' vormaclient/preact/src/link.tsx`
        - `sed -n '1,320p' vormaclient/preact/src/preact.test.tsx`
        - `sed -n '1,260p' npm_dist/vormaclient/preact/index.d.ts`
        - `rg -n "^export (type |interface |const |function |class |let |var |\\{)" vormaclient/preact -g'*.ts' -g'*.tsx'`
        - `rg -n "from \\\"vorma/preact\\\"|import\\(\\\"vorma/preact\\\"\\)|\\blocation\\.value\\b" --glob '!node_modules/**'`
        - `pnpm prettier --write vormaclient/preact/src/preact.tsx`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260215-002`: `vorma/preact` publicly exported
          `location` as a writable `Signal`, allowing consumer writes to mutate
          router-managed location state directly.
        - Tightened surface by keeping internal mutable
          `locationState = signal(...)` private and exporting read-only
          `location = computed(() => locationState.value)`, preserving read
          semantics while closing external mutation.
        - Required gates pass. `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` with no
          errors.

## EV-20260215-004

- Package group: `vormaclient/solid/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaclient/solid/index.tsx`
        - `vormaclient/solid/src/helpers.ts`
        - `vormaclient/solid/src/solid.tsx`
        - `vormaclient/solid/src/link.tsx`
        - `npm_dist/vormaclient/solid/index.d.ts`
    - Commands/tests run:
        - `rg --files vormaclient/solid`
        - `sed -n '1,260p' vormaclient/solid/index.tsx`
        - `sed -n '1,340p' vormaclient/solid/src/helpers.ts`
        - `sed -n '1,520p' vormaclient/solid/src/solid.tsx`
        - `sed -n '1,320p' vormaclient/solid/src/link.tsx`
        - `rg -n "^export (type |interface |const |function |class |let |var |\\{)" vormaclient/solid -g'*.ts' -g'*.tsx'`
        - `sed -n '1,260p' npm_dist/vormaclient/solid/index.d.ts`
        - `rg -n "from \\\"vorma/solid\\\"|import\\(\\\"vorma/solid\\\"\\)" --glob '!node_modules/**'`
    - Findings/fixes:
        - No additional Surface/API findings in `vormaclient/solid/*` after this
          sweep.

## EV-20260215-005

- Package group: `vormaclient/vite/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaclient/vite/vite.ts`
        - `vormaclient/vite/vite.test.ts`
        - `npm_dist/vormaclient/vite/vite.d.ts`
        - `bootstrap/tmpls/vite_config_ts_tmpl.txt`
        - `internal/site/vite.config.ts`
    - Commands/tests run:
        - `rg --files vormaclient/vite`
        - `sed -n '1,460p' vormaclient/vite/vite.ts`
        - `sed -n '1,360p' vormaclient/vite/vite.test.ts`
        - `rg -n "^export (type |interface |const |function |class |let |var |\\{)" vormaclient/vite -g'*.ts' -g'*.tsx'`
        - `sed -n '1,260p' npm_dist/vormaclient/vite/vite.d.ts`
        - `rg -n "from \\\"vorma/vite\\\"|import\\(\\\"vorma/vite\\\"\\)" --glob '!node_modules/**'`
        - `pnpm prettier --write vormaclient/vite/vite.ts`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260215-003`: public `vorma/vite` plugin factory
          `vormaVitePlugin` returned `any`, which erased the public type
          contract for consumers.
        - Tightened API typing by importing Vite `Plugin` and changing the
          signature to
          `export default function vormaVitePlugin(config: VormaVitePluginConfig): Plugin`.
        - Required gates pass. `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` with no
          errors.

## EV-20260215-006

- Package group: `vormaclient/create/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaclient/create/package.json`
        - `vormaclient/create/main.ts`
        - `vormaclient/create/runtime_helpers.ts`
        - `vormaclient/create/runtime_helpers.test.ts`
        - `vormaclient/create/tsconfig.json`
    - Commands/tests run:
        - `rg --files vormaclient/create`
        - `sed -n '1,260p' vormaclient/create/package.json`
        - `sed -n '1,420p' vormaclient/create/main.ts`
        - `sed -n '1,260p' vormaclient/create/runtime_helpers.ts`
        - `sed -n '1,280p' vormaclient/create/runtime_helpers.test.ts`
        - `cat vormaclient/create/tsconfig.json`
        - `rg -n "^export (type |interface |const |function |class |let |var |\\{)" vormaclient/create -g'*.ts' -g'*.tsx'`
        - `pnpm prettier --write vormaclient/create/package.json`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260215-004`: `create-vorma` package had no
          `exports` map, which made all published `dist/*` modules importable by
          default even though this package is intended as a CLI entrypoint.
        - Tightened boundary by adding `"exports": { ".": "./dist/main.js" }` in
          `vormaclient/create/package.json`, restricting package imports to the
          CLI entry module.
        - Required gates pass. `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` with no
          errors.

## EV-20260215-007

- Package group: `kit/bytesutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/bytesutil/bytesutil.go`
        - `kit/bytesutil/bytesutil_test.go`
        - `kit/bytesutil/README.md`
    - Commands/tests run:
        - `rg --files kit/bytesutil`
        - `sed -n '1,260p' kit/bytesutil/bytesutil.go`
        - `sed -n '1,300p' kit/bytesutil/bytesutil_test.go`
        - `sed -n '1,220p' kit/bytesutil/README.md`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/bytesutil/*.go`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/bytesutil` after this
          sweep.

## EV-20260215-008

- Package group: `kit/colorlog`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/colorlog/colorlog.go`
        - `kit/colorlog/colorlog_test.go`
        - `kit/colorlog/README.md`
    - Commands/tests run:
        - `rg --files kit/colorlog`
        - `sed -n '1,260p' kit/colorlog/colorlog.go`
        - `sed -n '1,260p' kit/colorlog/colorlog_test.go`
        - `sed -n '1,220p' kit/colorlog/README.md`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/colorlog/*.go`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/colorlog` after this sweep.

## EV-20260215-009

- Package group: `kit/contextutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/contextutil/contextutil.go`
        - `kit/contextutil/contextutil_test.go`
        - `kit/contextutil/README.md`
    - Commands/tests run:
        - `rg --files kit/contextutil`
        - `sed -n '1,260p' kit/contextutil/contextutil.go`
        - `sed -n '1,320p' kit/contextutil/contextutil_test.go`
        - `sed -n '1,220p' kit/contextutil/README.md`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/contextutil/*.go`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/contextutil` after this
          sweep.

## EV-20260215-010

- Package group: `kit/cookies`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/cookies/cookies.go`
        - `kit/cookies/cookies_test.go`
        - `kit/cookies/README.md`
    - Commands/tests run:
        - `rg --files kit/cookies`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/cookies/*.go`
        - `sed -n '1,320p' kit/cookies/cookies.go`
        - `sed -n '260,520p' kit/cookies/cookies.go`
        - `sed -n '1,340p' kit/cookies/cookies_test.go`
        - `sed -n '1,260p' kit/cookies/README.md`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/cookies` after this sweep.

## EV-20260215-011

- Package group: `cross-cutting prior-round reassessment`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaclient/client/index.ts`
        - `vormaclient/client/internal.ts`
        - `vormaclient/client/src/core/extras.ts`
        - `vormaclient/client/src/core/render_runtime.ts`
        - `vormaclient/create/package.json`
        - `vormaclient/create/main.ts`
        - `vormaclient/create/runtime_helpers.ts`
        - `vormabuild/route_parsing.go`
        - `vormabuild/route_parsing_pipeline.go`
        - `vormabuild/vorma_gen_ts.go`
        - `vormabuild/vorma_gen_ts_vite_and_write.go`
        - `wave/tooling/watcher.go`
        - `wave/tooling/watcher_debouncer.go`
        - `wave/tooling/devserver.go`
        - `wave/tooling/devserver_config.go`
        - `wave/tooling/events_hook_executor.go`
    - Commands/tests run:
        - `git show 5fa540e9 -- vormabuild/route_parsing.go vormabuild/route_parsing_pipeline.go vormabuild/vorma_gen_ts.go vormabuild/vorma_gen_ts_vite_and_write.go`
        - `git show 5fa540e9 -- wave/tooling/watcher.go wave/tooling/watcher_debouncer.go wave/tooling/devserver.go wave/tooling/devserver_config.go wave/tooling/events_hook_executor.go wave/tooling/events.go`
        - `sed -n '1,220p' vormaclient/client/index.ts`
        - `sed -n '1,240p' vormaclient/client/internal.ts`
        - `sed -n '1,280p' vormaclient/client/src/core/extras.ts`
        - `sed -n '1,340p' vormaclient/client/src/core/render_runtime.ts`
        - `sed -n '1,260p' vormaclient/create/package.json`
        - `sed -n '1,320p' vormaclient/create/main.ts`
        - `sed -n '1,260p' vormaclient/create/runtime_helpers.ts`
        - `for f in vormabuild/*.go; do if [[ $f != *_test.go ]]; then rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" "$f"; fi; done`
        - `for f in wave/tooling/*.go; do if [[ $f != *_test.go ]]; then rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" "$f"; fi; done`
    - Findings/fixes:
        - Corrected boundary placement from `F-20260215-001`: app-facing
          `runClientLoadersAfterHMRUpdate` and `registerClientLoaderPattern` are
          now exported from `vorma/client` (`vormaclient/client/index.ts`) and
          are no longer exported from `vorma/client/__internal`.
        - First-principles reassessment keeps prior `vormabuild` unexports
          (`VormaSchema`, `RouteCall`, `UnresolvedRouteCall`, `TSGenInput`,
          `WriteGeneratedTS`) as internal-only because they are implementation
          details with no stable app-developer extension contract.
        - First-principles reassessment keeps prior `wave/tooling` unexports
          (`Watcher`, `NewWatcher`, `Debouncer`, `NewDebouncer`) as internal
          devserver implementation details; public app-level API remains
          `RunDev`, builder APIs, and CLI helpers.
        - First-principles reassessment keeps the `create-vorma` package
          `exports` boundary (`"." -> "./dist/main.js"`) as a CLI-only surface.
        - No additional prior-round boundary reversions were required beyond the
          `vorma/client` relocation above.

## EV-20260215-012

- Package group: `kit/cryptoutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/cryptoutil/cryptoutil.go`
        - `kit/cryptoutil/cryptoutil_test.go`
        - `kit/cryptoutil/README.md`
    - Commands/tests run:
        - `rg --files kit/cryptoutil`
        - `sed -n '1,340p' kit/cryptoutil/cryptoutil.go`
        - `sed -n '1,360p' kit/cryptoutil/cryptoutil_test.go`
        - `sed -n '1,260p' kit/cryptoutil/README.md`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/cryptoutil/cryptoutil.go`
        - `gofmt -w kit/cryptoutil/cryptoutil.go`
        - `go test ./kit/cryptoutil`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260215-005`: `ToAEADFuncXChaCha20Poly1305` and
          `ToAEADFuncAESGCM` were publicly mutable globals, allowing external
          reassignment of cryptographic constructor behavior.
        - Tightened API by exporting these as stable functions instead of
          mutable vars while preserving call-site compatibility with
          `EncryptSymmetricGeneric`/`DecryptSymmetricGeneric`.
        - Updated `kit/cryptoutil/README.md` API coverage to match the
          function-based exports.
        - Required gates pass. `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` with no
          errors.

## EV-20260215-013

- Package group: `vormaclient/client/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaclient/client/index.ts`
        - `vormaclient/client/internal.ts`
        - `vormaclient/react/src/helpers.ts`
        - `vormaclient/preact/src/helpers.ts`
        - `vormaclient/solid/src/helpers.ts`
    - Commands/tests run:
        - `sed -n '1,80p' vormaclient/client/index.ts`
        - `sed -n '1,80p' vormaclient/client/internal.ts`
        - `rg -n "registerClientLoaderForAdapter" vormaclient/react vormaclient/preact vormaclient/solid -g'*.ts' -g'*.tsx'`
        - `pnpm prettier --write vormaclient/client/index.ts FRAMEWORK_AUDIT.md FRAMEWORK_AUDIT_EVIDENCE.md`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Product-boundary decision confirmed: custom client-loader abstraction
          support is not an app-level goal.
        - Removed `runClientLoadersAfterHMRUpdate` and
          `registerClientLoaderPattern` from `vorma/client` public exports in
          `vormaclient/client/index.ts`; adapters continue using
          `registerClientLoaderForAdapter` from `vorma/client/__internal`.
    - This supersedes the public-export placement portion of `EV-20260215-011`.

## EV-20260215-014

- Package group: `kit/csrf`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/csrf/csrf.go`
        - `kit/csrf/csrf_test.go`
        - `kit/csrf/README.md`
    - Commands/tests run:
        - `rg --files kit/csrf`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/csrf/*.go`
        - `sed -n '1,260p' kit/csrf/csrf.go`
        - `sed -n '1,260p' kit/csrf/csrf_test.go`
        - `sed -n '260,620p' kit/csrf/csrf_test.go`
        - `sed -n '1,260p' kit/csrf/README.md`
        - `rg -n "CycleTokenWithProxy|CycleTokenWithWriter|Middleware|NewProtector|ProtectorConfig|AllowedOrigins|HeaderName|CookieName" kit/csrf/csrf_test.go`
        - `go test ./kit/csrf`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/csrf` after this sweep.

## EV-20260215-015

- Package group: `kit/envutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/envutil/envutil.go`
        - `kit/envutil/envutil_test.go`
        - `kit/envutil/README.md`
    - Commands/tests run:
        - `rg --files kit/envutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/envutil/*.go`
        - `sed -n '1,260p' kit/envutil/envutil.go`
        - `sed -n '1,260p' kit/envutil/envutil_test.go`
        - `sed -n '1,240p' kit/envutil/README.md`
        - `rg -n "\benvutil\.(GetStr|GetInt|GetBool)\b|github.com/vormadev/vorma/kit/envutil" --glob '!**/node_modules/**'`
        - `go test ./kit/envutil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/envutil` after this sweep.

## EV-20260215-016

- Package group: `kit/executil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/executil/executil.go`
        - `kit/executil/executil_test.go`
        - `kit/executil/README.md`
    - Commands/tests run:
        - `rg --files kit/executil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/executil/*.go`
        - `sed -n '1,260p' kit/executil/executil.go`
        - `sed -n '260,520p' kit/executil/executil.go`
        - `sed -n '1,320p' kit/executil/executil_test.go`
        - `sed -n '1,260p' kit/executil/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/executil|\bexecutil\.(MakeCmdRunner|RunCmd|RunCmdCapture|RunShell|RunShellWithContext|GetExecutableDir|ErrCommandExecutionTimedOut|ErrCommandExecutionCanceled)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/executil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/executil` after this sweep.

## EV-20260215-017

- Package group: `kit/fsutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/fsutil/fsutil.go`
        - `kit/fsutil/fsutil_test.go`
        - `kit/fsutil/README.md`
    - Commands/tests run:
        - `rg --files kit/fsutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/fsutil/*.go`
        - `sed -n '1,300p' kit/fsutil/fsutil.go`
        - `sed -n '300,620p' kit/fsutil/fsutil.go`
        - `sed -n '1,320p' kit/fsutil/fsutil_test.go`
        - `sed -n '320,700p' kit/fsutil/fsutil_test.go`
        - `sed -n '1,280p' kit/fsutil/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/fsutil|\bfsutil\.(EnsureDir|EnsureDirs|GetCallerDir|CopyDir|CopyFile|CopyFiles|FromGobInto|FromGob|MustSub|MustReadFile)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/fsutil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/fsutil` after this sweep.

## EV-20260215-018

- Package group: `kit/genericsutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/genericsutil/genericsutil.go`
        - `kit/genericsutil/genericsutil_test.go`
        - `kit/genericsutil/README.md`
    - Commands/tests run:
        - `rg --files kit/genericsutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/genericsutil/*.go`
        - `sed -n '1,320p' kit/genericsutil/genericsutil.go`
        - `sed -n '1,360p' kit/genericsutil/genericsutil_test.go`
        - `sed -n '1,280p' kit/genericsutil/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/genericsutil|\bgenericsutil\.(AnyZeroHelper|None|ZeroHelper|Zero|AssertOrZero|OrDefault|IsNone)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/genericsutil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/genericsutil` after this
          sweep.

## EV-20260215-019

- Package group: `kit/grace`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/grace/grace.go`
        - `kit/grace/grace_test.go`
        - `kit/grace/README.md`
    - Commands/tests run:
        - `rg --files kit/grace`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/grace/*.go`
        - `sed -n '1,320p' kit/grace/grace.go`
        - `sed -n '1,380p' kit/grace/grace_test.go`
        - `sed -n '1,260p' kit/grace/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/grace|\bgrace\.(OrchestrateOptions|Orchestrate|TerminateProcess)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/grace`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/grace` after this sweep.

## EV-20260215-020

- Package group: `kit/headels`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/headels/headblocks.go`
        - `kit/headels/headblocks_test.go`
        - `kit/headels/README.md`
    - Commands/tests run:
        - `rg --files kit/headels`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/headels/*.go`
        - `sed -n '1,360p' kit/headels/headblocks.go`
        - `sed -n '360,760p' kit/headels/headblocks.go`
        - `sed -n '1,360p' kit/headels/headblocks_test.go`
        - `sed -n '360,760p' kit/headels/headblocks_test.go`
        - `sed -n '1,260p' kit/headels/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/headels|\bheadels\.(NewInstance|FromRaw|New|HeadEls|Instance|Tag|Attr|BooleanAttribute|InnerHTML|TextContent|SelfClosing|SortedAndPreEscapedHeadEls)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/headels`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/headels` after this sweep.

## EV-20260215-021

- Package group: `kit/htmlutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/htmlutil/htmlutil.go`
        - `kit/htmlutil/htmlutil_test.go`
        - `kit/htmlutil/README.md`
    - Commands/tests run:
        - `rg --files kit/htmlutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/htmlutil/*.go`
        - `sed -n '1,360p' kit/htmlutil/htmlutil.go`
        - `sed -n '360,760p' kit/htmlutil/htmlutil.go`
        - `sed -n '1,400p' kit/htmlutil/htmlutil_test.go`
        - `sed -n '400,820p' kit/htmlutil/htmlutil_test.go`
        - `sed -n '1,280p' kit/htmlutil/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/htmlutil|\bhtmlutil\.(Element|AddNonce|ComputeContentSha256|EscapeIntoTrusted|RenderElement|RenderElementToBuilder|RenderModuleScriptToBuilder|SetSha256Integrity)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/htmlutil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/htmlutil` after this sweep.

## EV-20260215-022

- Package group: `kit/id`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/id/id.go`
        - `kit/id/id_test.go`
        - `kit/id/README.md`
    - Commands/tests run:
        - `rg --files kit/id`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/id/*.go`
        - `sed -n '1,320p' kit/id/id.go`
        - `sed -n '1,380p' kit/id/id_test.go`
        - `sed -n '1,260p' kit/id/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/id|\bid\.(New|NewMulti)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/id`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/id` after this sweep.

## EV-20260215-023

- Package group: `kit/ioutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/ioutil/ioutil.go`
        - `kit/ioutil/ioutil_test.go`
        - `kit/ioutil/README.md`
    - Commands/tests run:
        - `rg --files kit/ioutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/ioutil/*.go`
        - `sed -n '1,360p' kit/ioutil/ioutil.go`
        - `sed -n '1,420p' kit/ioutil/ioutil_test.go`
        - `sed -n '1,280p' kit/ioutil/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/ioutil|\bioutil\.(ReadLimited|ErrReadLimitExceeded|OneKB|OneMB|OneGB)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/ioutil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/ioutil` after this sweep.

## EV-20260215-024

- Package group: `kit/jsonutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/jsonutil/jsonutil.go`
        - `kit/jsonutil/jsonutil_test.go`
        - `kit/jsonutil/README.md`
    - Commands/tests run:
        - `rg --files kit/jsonutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/jsonutil/*.go`
        - `sed -n '1,360p' kit/jsonutil/jsonutil.go`
        - `sed -n '1,420p' kit/jsonutil/jsonutil_test.go`
        - `sed -n '1,280p' kit/jsonutil/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/jsonutil|\bjsonutil\.(JSONString|Serialize|Parse)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/jsonutil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/jsonutil` after this sweep.

## EV-20260215-025

- Package group: `kit/keyset`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/keyset/keyset.go`
        - `kit/keyset/keyset_test.go`
        - `kit/keyset/README.md`
    - Commands/tests run:
        - `rg --files kit/keyset`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/keyset/*.go`
        - `sed -n '1,360p' kit/keyset/keyset.go`
        - `sed -n '360,760p' kit/keyset/keyset.go`
        - `sed -n '1,420p' kit/keyset/keyset_test.go`
        - `sed -n '420,860p' kit/keyset/keyset_test.go`
        - `sed -n '1,320p' kit/keyset/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/keyset|\bkeyset\.(RootSecret|RootSecrets|UnwrappedKeyset|Keyset|FromUnwrapped|Attempt|LoadRootKeyset|RootSecretsToRootKeyset|LoadRootSecrets|AppKeysetConfig|AppKeyset|MustAppKeyset)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/keyset`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/keyset` after this sweep.

## EV-20260215-026

- Package group: `kit/lazyget`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/lazyget/lazyget.go`
        - `kit/lazyget/lazyget_test.go`
        - `kit/lazyget/README.md`
    - Commands/tests run:
        - `rg --files kit/lazyget`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/lazyget/*.go`
        - `sed -n '1,280p' kit/lazyget/lazyget.go`
        - `sed -n '1,340p' kit/lazyget/lazyget_test.go`
        - `sed -n '1,260p' kit/lazyget/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/lazyget|\blazyget\.(Cache|New)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/lazyget`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/lazyget` after this sweep.

## EV-20260215-027

- Package group: `kit/lru`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/lru/lru.go`
        - `kit/lru/lru_test.go`
        - `kit/lru/README.md`
    - Commands/tests run:
        - `rg --files kit/lru`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/lru/*.go`
        - `sed -n '1,360p' kit/lru/lru.go`
        - `sed -n '360,760p' kit/lru/lru.go`
        - `sed -n '1,420p' kit/lru/lru_test.go`
        - `sed -n '420,860p' kit/lru/lru_test.go`
        - `sed -n '1,320p' kit/lru/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/lru|\blru\.(Cache|NewCache|NewCacheWithTTL)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/lru`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/lru` after this sweep.

## EV-20260215-028

- Package group: `kit/matcher`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/matcher/matcher.go`
        - `kit/matcher/register.go`
        - `kit/matcher/find_best_match.go`
        - `kit/matcher/find_nested_matches.go`
        - `kit/matcher/parse_segments.go`
        - `kit/matcher/find_best_match_test.go`
        - `kit/matcher/find_nested_matches_test.go`
        - `kit/matcher/parse_segments_test.go`
        - `kit/matcher/README.md`
    - Commands/tests run:
        - `rg --files kit/matcher`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/matcher/*.go`
        - `sed -n '1,360p' kit/matcher/matcher.go`
        - `sed -n '360,760p' kit/matcher/matcher.go`
        - `sed -n '1,320p' kit/matcher/register.go`
        - `sed -n '1,320p' kit/matcher/find_best_match.go`
        - `sed -n '1,360p' kit/matcher/find_nested_matches.go`
        - `sed -n '1,220p' kit/matcher/parse_segments.go`
        - `sed -n '1,360p' kit/matcher/find_best_match_test.go`
        - `sed -n '1,360p' kit/matcher/find_nested_matches_test.go`
        - `sed -n '1,260p' kit/matcher/parse_segments_test.go`
        - `sed -n '1,320p' kit/matcher/README.md`
        - `rg -n "github.com/vormadev/vorma/kit/matcher|\bmatcher\.(Matcher|Options|RegisteredPattern|BestMatch|FindNestedMatchesResults|ParseSegments|JoinPatterns|HasLeadingSlash|HasTrailingSlash|EnsureLeadingSlash|EnsureTrailingSlash|EnsureLeadingAndTrailingSlash|StripLeadingSlash|StripTrailingSlash|New)\b" --glob '!**/node_modules/**'`
        - `go test ./kit/matcher`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/matcher` after this sweep.

## EV-20260215-029

- Package group: `kit/middleware`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/middleware/middleware.go`
        - `kit/middleware/README.md`
    - Commands/tests run:
        - `rg --files kit/middleware`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/middleware/*.go`
        - `sed -n '1,360p' kit/middleware/middleware.go`
        - `sed -n '1,320p' kit/middleware/README.md`
        - `go test ./kit/middleware`
        - `rg -n "github.com/vormadev/vorma/kit/middleware|\bmiddleware\.(Middleware|ToHandlerMiddleware)\b" --glob '!**/node_modules/**'`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/middleware` after this
          sweep.

## EV-20260215-030

- Package group: `kit/middleware/etag`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/middleware/etag/etag.go`
        - `kit/middleware/etag/etag_test.go`
        - `kit/middleware/etag/README.md`
    - Commands/tests run:
        - `rg --files kit/middleware/etag`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/middleware/etag/*.go`
        - `sed -n '1,340p' kit/middleware/etag/etag.go`
        - `sed -n '1,380p' kit/middleware/etag/etag_test.go`
        - `sed -n '1,280p' kit/middleware/etag/README.md`
        - `go test ./kit/middleware/etag`
        - `rg -n "github.com/vormadev/vorma/kit/middleware/etag|\betag\.(Config|Auto)\b" --glob '!**/node_modules/**'`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/middleware/etag` after this
          sweep.

## EV-20260215-031

- Package group: `kit/middleware/healthcheck`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/middleware/healthcheck/healthcheck.go`
        - `kit/middleware/healthcheck/healthcheck_test.go`
        - `kit/middleware/healthcheck/README.md`
    - Commands/tests run:
        - `rg --files kit/middleware/healthcheck`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/middleware/healthcheck/*.go`
        - `sed -n '1,260p' kit/middleware/healthcheck/healthcheck.go`
        - `sed -n '1,320p' kit/middleware/healthcheck/healthcheck_test.go`
        - `sed -n '1,260p' kit/middleware/healthcheck/README.md`
        - `gofmt -w kit/middleware/healthcheck/healthcheck.go`
        - `pnpm prettier --write kit/middleware/healthcheck/README.md`
        - `go test ./kit/middleware/healthcheck`
        - `rg -n "healthcheck\.Healthz\b" --glob '!**/node_modules/**'`
    - Findings/fixes:
        - Found and fixed `F-20260215-006`: `Healthz` was exported as a mutable
          package variable, allowing external reassignment of the default
          healthcheck middleware entrypoint.
        - Replaced `var Healthz = OK("/healthz")` with a stable exported
          function `Healthz(next http.Handler) http.Handler` that preserves the
          same call style (`healthcheck.Healthz(next)`) without mutable global
          state.
        - Updated `kit/middleware/healthcheck/README.md` API reference to match
          the function export.

## EV-20260215-032

- Package group: `kit/middleware/robotstxt`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/middleware/robotstxt/robotstxt.go`
        - `kit/middleware/robotstxt/robotstxt_test.go`
        - `kit/middleware/robotstxt/README.md`
    - Commands/tests run:
        - `rg --files kit/middleware/robotstxt`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/middleware/robotstxt/*.go`
        - `sed -n '1,280p' kit/middleware/robotstxt/robotstxt.go`
        - `sed -n '1,320p' kit/middleware/robotstxt/robotstxt_test.go`
        - `sed -n '1,260p' kit/middleware/robotstxt/README.md`
        - `gofmt -w kit/middleware/robotstxt/robotstxt.go`
        - `pnpm prettier --write kit/middleware/robotstxt/README.md`
        - `go test ./kit/middleware/robotstxt`
        - `rg -n "robotstxt\.(Allow|Disallow)\b" --glob '!**/node_modules/**'`
    - Findings/fixes:
        - Found and fixed `F-20260215-007`: `Allow` and `Disallow` were exported
          as mutable package variables, allowing external reassignment of
          default robots.txt middleware entrypoints.
        - Replaced those mutable vars with stable exported functions
          (`Allow(next http.Handler) http.Handler` and
          `Disallow(next http.Handler) http.Handler`) that preserve existing
          call style while eliminating mutable global state.
        - Updated `kit/middleware/robotstxt/README.md` API reference to match
          the function exports.

## EV-20260215-033

- Package group: `kit/middleware/secureheaders`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/middleware/secureheaders/secureheaders.go`
        - `kit/middleware/secureheaders/secureheaders_test.go`
        - `kit/middleware/secureheaders/README.md`
    - Commands/tests run:
        - `rg --files kit/middleware/secureheaders`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/middleware/secureheaders/*.go`
        - `sed -n '1,340p' kit/middleware/secureheaders/secureheaders.go`
        - `sed -n '1,360p' kit/middleware/secureheaders/secureheaders_test.go`
        - `sed -n '1,300p' kit/middleware/secureheaders/README.md`
        - `go test ./kit/middleware/secureheaders`
        - `rg -n "github.com/vormadev/vorma/kit/middleware/secureheaders|\bsecureheaders\.Middleware\b" --glob '!**/node_modules/**'`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/middleware/secureheaders`
          after this sweep.

## EV-20260215-034

- Package group: `kit/modulegraph`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/modulegraph/module_graph.go`
        - `kit/modulegraph/module_graph_test.go`
    - Commands/tests run:
        - `rg --files kit/modulegraph`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/modulegraph/*.go`
        - `sed -n '1,360p' kit/modulegraph/module_graph.go`
        - `sed -n '360,760p' kit/modulegraph/module_graph.go`
        - `sed -n '1,420p' kit/modulegraph/module_graph_test.go`
        - `sed -n '420,900p' kit/modulegraph/module_graph_test.go`
        - `go test ./kit/modulegraph`
        - `rg -n "github.com/vormadev/vorma/kit/modulegraph|\bmodulegraph\.(Module|RegistrationPhase|ResolveDeterministicModuleRegistrationOrder|RunDeterministicModuleRegistrationLifecycle|RegistrationPhaseRegisterLoaders|RegistrationPhaseRegisterActions|RegistrationPhaseRegisterRoutes|RegistrationPhaseFinalize)\b" --glob '!**/node_modules/**'`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/modulegraph` after this
          sweep.

## EV-20260215-035

- Package group: `kit/mux`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/mux/mux.go`
        - `kit/mux/README.md`
    - Commands/tests run:
        - `rg -n "HandlerNeedsTasksCtxImplReflectType|handlerNeedsTasksCtxImplReflectType" kit/mux/mux.go`
        - `gofmt -w kit/mux/mux.go`
        - `pnpm prettier --write kit/mux/README.md`
        - `go test ./kit/mux`
    - Findings/fixes:
        - Found and fixed `F-20260215-008`: `kit/mux` exported a mutable
          package-level reflect cache value
          (`HandlerNeedsTasksCtxImplReflectType`) that had no app-level use case
          and unnecessarily expanded public mutation surface.
        - Unexported the cache value to `handlerNeedsTasksCtxImplReflectType`
          and updated internal call sites.
        - Removed corresponding public API mention from `kit/mux/README.md`.

## EV-20260215-036

- Package group: `kit/modulegraph`, `vormaruntime/*`, `vorma.go`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vormaruntime/app_root.go`
        - `vormaruntime/app_root_test.go`
        - `vormaruntime/internal/modulegraph/module_graph.go`
        - `vormaruntime/internal/modulegraph/module_graph_test.go`
        - `vorma.go`
    - Commands/tests run:
        - `rg -n "modulegraph|NewAppRoot|RunAppModuleRegistrationLifecycle|AppModule" --glob '!FRAMEWORK_AUDIT*.md'`
        - `rm -f vormaruntime/internal/modulegraph/module_graph.go vormaruntime/internal/modulegraph/module_graph_test.go`
        - `rmdir vormaruntime/internal/modulegraph`
        - `rm -f vormaruntime/app_root.go vormaruntime/app_root_test.go`
        - `gofmt -w vorma.go`
        - `go test ./...`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Found and fixed `F-20260215-009`: `modulegraph` and `AppRoot` /
          `AppModule` formed a separate runtime module-registration API that was
          not part of AST-discovered route registration architecture and had no
          non-test in-repo runtime consumers.
        - User requested deletion experiment and approved removal if gates pass.
        - Removed vestigial surface and implementation:
            - Deleted `vormaruntime/internal/modulegraph/*`.
            - Deleted `vormaruntime/app_root.go` and
              `vormaruntime/app_root_test.go`.
            - Removed public aliases/wrappers from `vorma.go`: `AppRoot`,
              `AppModule`, `AppModuleRegistrationHook`, `NewAppRoot`,
              `RunAppModuleRegistrationLifecycle`.
        - Initial `go test ./...` after deleting implementation failed only due
          `vormaruntime/app_root_test.go` referencing removed API.
        - After deleting that vestigial test file, full gates pass
          (`make gotest`, `make tstest`, `make tscheck`, `make tslint`).
        - `make tslint` reports existing warnings in
          `internal/site/frontend/src/components/rendered-markdown.tsx` and no
          errors.

## EV-20260215-037

- Package group: `kit/netutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/netutil/netutil.go`
        - `kit/netutil/netutil_test.go`
        - `kit/netutil/README.md`
    - Commands/tests run:
        - `rg --files kit/netutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/netutil/*.go`
        - `sed -n '1,320p' kit/netutil/netutil.go`
        - `sed -n '1,340p' kit/netutil/netutil_test.go`
        - `sed -n '1,260p' kit/netutil/README.md`
        - `go test ./kit/netutil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/netutil` after this sweep.

## EV-20260215-038

- Package group: `kit/reflectutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/reflectutil/reflectutil.go`
        - `kit/reflectutil/reflectutil_test.go`
        - `kit/reflectutil/README.md`
    - Commands/tests run:
        - `rg --files kit/reflectutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/reflectutil/*.go`
        - `sed -n '1,320p' kit/reflectutil/reflectutil.go`
        - `sed -n '1,320p' kit/reflectutil/reflectutil_test.go`
        - `sed -n '1,260p' kit/reflectutil/README.md`
        - `go test ./kit/reflectutil`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/reflectutil` after this
          sweep.

## EV-20260215-039

- Package group: `kit/response`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/response/response.go`
        - `kit/response/response_test.go`
        - `kit/response/proxy.go`
        - `kit/response/proxy_test.go`
        - `kit/response/README.md`
    - Commands/tests run:
        - `rg --files kit/response`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/response/*.go`
        - `sed -n '1,360p' kit/response/response.go`
        - `sed -n '1,360p' kit/response/response_test.go`
        - `sed -n '1,380p' kit/response/proxy.go`
        - `sed -n '1,260p' kit/response/README.md`
        - `gofmt -w kit/response/proxy.go`
        - `go test ./kit/response`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/response` after this sweep.
        - Corrected an internal stale comment in `kit/response/proxy.go`
          referencing `GetHeadElements()` to `GetHeadEls()` for API-name
          accuracy.

## EV-20260215-040

- Package group: `kit/securebytes`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/securebytes/securebytes.go`
        - `kit/securebytes/securebytes_test.go`
        - `kit/securebytes/README.md`
    - Commands/tests run:
        - `rg --files kit/securebytes`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/securebytes/*.go`
        - `sed -n '1,320p' kit/securebytes/securebytes.go`
        - `sed -n '1,360p' kit/securebytes/securebytes_test.go`
        - `sed -n '1,260p' kit/securebytes/README.md`
        - `go test ./kit/securebytes`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/securebytes` after this
          sweep.

## EV-20260215-041

- Package group: `kit/securestring`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/securestring/securestring.go`
        - `kit/securestring/securestring_test.go`
        - `kit/securestring/README.md`
    - Commands/tests run:
        - `rg --files kit/securestring`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/securestring/*.go`
        - `sed -n '1,320p' kit/securestring/securestring.go`
        - `sed -n '1,360p' kit/securestring/securestring_test.go`
        - `sed -n '1,260p' kit/securestring/README.md`
        - `go test ./kit/securestring`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/securestring` after this
          sweep.

## EV-20260215-042

- Package group: `kit/set`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/set/set.go`
        - `kit/set/set_test.go`
        - `kit/set/README.md`
    - Commands/tests run:
        - `rg --files kit/set`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/set/*.go`
        - `sed -n '1,320p' kit/set/set.go`
        - `sed -n '1,340p' kit/set/set_test.go`
        - `sed -n '1,260p' kit/set/README.md`
        - `go test ./kit/set`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/set` after this sweep.

## EV-20260215-043

- Package group: `kit/tasks`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/tasks/tasks.go`
        - `kit/tasks/tasks_test.go`
        - `kit/tasks/tasks_bench_test.go`
        - `kit/tasks/README.md`
    - Commands/tests run:
        - `rg --files kit/tasks`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/tasks/*.go`
        - `sed -n '1,340p' kit/tasks/tasks.go`
        - `sed -n '1,360p' kit/tasks/tasks_test.go`
        - `sed -n '1,260p' kit/tasks/README.md`
        - `go test ./kit/tasks`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/tasks` after this sweep.

## EV-20260215-044

- Package group: `kit/theme`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/theme/theme.go`
        - `kit/theme/theme_test.go`
        - `kit/theme/README.md`
        - `internal/site/backend/src/router/app.go`
    - Commands/tests run:
        - `rg --files kit/theme`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/theme/*.go`
        - `sed -n '1,320p' kit/theme/theme.go`
        - `sed -n '1,340p' kit/theme/theme_test.go`
        - `sed -n '1,260p' kit/theme/README.md`
        - `rg -n "theme\\.SystemThemeScript(Sha256Hash)?\\b|SystemThemeScriptSha256Hash|SystemThemeScript\\b" --glob '!FRAMEWORK_AUDIT*.md'`
        - `gofmt -w kit/theme/theme.go internal/site/backend/src/router/app.go`
        - `pnpm prettier --write kit/theme/README.md`
        - `go test ./kit/theme`
        - `go test ./backend/src/router` (from `internal/site`)
    - Findings/fixes:
        - Found and fixed `F-20260215-010`: `kit/theme` exported mutable package
          variables (`SystemThemeScript`, `SystemThemeScriptSha256Hash`) that
          could be reassigned by consumers.
        - Replaced those exports with stable functions:
          `SystemThemeScript() template.HTML` and
          `SystemThemeScriptSha256Hash() string`, backed by unexported cached
          values.
        - Updated call sites in `internal/site/backend/src/router/app.go` and
          API docs in `kit/theme/README.md`.
        - `kit/theme` tests pass after change.

## EV-20260215-045

- Package group: `kit/validate`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `kit/validate/validate.go`
        - `kit/validate/error_collector.go`
        - `kit/validate/rules.go`
        - `kit/validate/search_params.go`
        - `kit/validate/validate_test.go`
        - `kit/validate/error_collector_test.go`
        - `kit/validate/rules_test.go`
        - `kit/validate/search_params_test.go`
        - `kit/validate/README.md`
    - Commands/tests run:
        - `rg --files kit/validate`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" kit/validate/*.go`
        - `sed -n '1,360p' kit/validate/validate.go`
        - `sed -n '1,360p' kit/validate/validate_test.go`
        - `sed -n '1,320p' kit/validate/README.md`
        - `go test ./kit/validate`
    - Findings/fixes:
        - No additional Surface/API findings in `kit/validate` after this sweep.

## EV-20260215-046

- Package group: `lab/bumper`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/bumper/bumper.go`
        - `internal/scripts/bumper/main.go`
    - Commands/tests run:
        - `rg --files lab/bumper`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/bumper/*.go`
        - `sed -n '1,340p' lab/bumper/bumper.go`
        - `rg -n "lab/bumper|bumper\\.Run\\(" --glob '!FRAMEWORK_AUDIT*.md'`
        - `go test ./lab/bumper`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/bumper` after this sweep.

## EV-20260215-047

- Package group: `lab/cliutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/cliutil/cliutil.go`
    - Commands/tests run:
        - `rg --files lab/cliutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/cliutil/*.go`
        - `sed -n '1,360p' lab/cliutil/cliutil.go`
        - `go test ./lab/cliutil`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/cliutil` after this sweep.

## EV-20260215-048

- Package group: `lab/errutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/errutil/errutil.go`
    - Commands/tests run:
        - `rg --files lab/errutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/errutil/*.go`
        - `sed -n '1,320p' lab/errutil/errutil.go`
        - `go test ./lab/errutil`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/errutil` after this sweep.

## EV-20260215-049

- Package group: `lab/esbuildutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/esbuildutil/esbuildutil.go`
        - `wave/tooling/css_build_execution.go`
    - Commands/tests run:
        - `rg --files lab/esbuildutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/esbuildutil/*.go`
        - `sed -n '1,360p' lab/esbuildutil/esbuildutil.go`
        - `rg -n "KindDymanicImport|KindDynamicImport|CollectErrors|UnmarshalOutput|FindAllDependencies|FindRelativeEntrypointPath|ESBuildMetafileSubset" --glob '!FRAMEWORK_AUDIT*.md'`
        - `gofmt -w lab/esbuildutil/esbuildutil.go`
        - `go test ./lab/esbuildutil ./wave/tooling`
    - Findings/fixes:
        - Found and fixed `F-20260215-011`: exported constant was misspelled as
          `KindDymanicImport`, creating a typoed public API name.
        - Renamed to `KindDynamicImport` and updated internal usage.

## EV-20260215-050

- Package group: `lab/fsmarkdown`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/fsmarkdown/fsmarkdown.go`
    - Commands/tests run:
        - `rg --files lab/fsmarkdown`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/fsmarkdown/*.go`
        - `sed -n '1,360p' lab/fsmarkdown/fsmarkdown.go`
        - `sed -n '360,760p' lab/fsmarkdown/fsmarkdown.go`
        - `go test ./lab/fsmarkdown`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/fsmarkdown` after this
          sweep.

## EV-20260215-051

- Package group: `lab/jsonschema`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/jsonschema/jsonschema.go`
    - Commands/tests run:
        - `rg --files lab/jsonschema`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/jsonschema/*.go`
        - `sed -n '1,360p' lab/jsonschema/jsonschema.go`
        - `go test ./lab/jsonschema`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/jsonschema` after this
          sweep.

## EV-20260215-052

- Package group: `lab/mailutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/mailutil/mailutil.go`
        - `lab/mailutil/mailutil_test.go`
    - Commands/tests run:
        - `rg --files lab/mailutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/mailutil/*.go`
        - `sed -n '1,360p' lab/mailutil/mailutil.go`
        - `sed -n '1,360p' lab/mailutil/mailutil_test.go`
        - `go test ./lab/mailutil`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/mailutil` after this sweep.

## EV-20260215-053

- Package group: `lab/parseutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/parseutil/parseutil.go`
    - Commands/tests run:
        - `rg --files lab/parseutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/parseutil/*.go`
        - `sed -n '1,360p' lab/parseutil/parseutil.go`
        - `go test ./lab/parseutil`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/parseutil` after this
          sweep.

## EV-20260215-054

- Package group: `lab/repoconcat`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/repoconcat/repoconcat.go`
        - `lab/repoconcat/repoconcat_test.go`
    - Commands/tests run:
        - `rg --files lab/repoconcat`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/repoconcat/*.go`
        - `sed -n '1,360p' lab/repoconcat/repoconcat.go`
        - `sed -n '1,360p' lab/repoconcat/repoconcat_test.go`
        - `go test ./lab/repoconcat`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/repoconcat` after this
          sweep.

## EV-20260215-055

- Package group: `lab/rpc`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/rpc/rpc.go`
        - `lab/rpc/rpc_test.go`
    - Commands/tests run:
        - `rg --files lab/rpc`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/rpc/*.go`
        - `sed -n '1,360p' lab/rpc/rpc.go`
        - `sed -n '1,360p' lab/rpc/rpc_test.go`
        - `go test ./lab/rpc`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/rpc` after this sweep.

## EV-20260215-056

- Package group: `lab/sqlutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/sqlutil/sqlutil.go`
    - Commands/tests run:
        - `rg --files lab/sqlutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/sqlutil/*.go`
        - `sed -n '1,360p' lab/sqlutil/sqlutil.go`
        - `go test ./lab/sqlutil`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/sqlutil` after this sweep.

## EV-20260215-057

- Package group: `lab/stringsutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/stringsutil/stringsutil.go`
        - `lab/stringsutil/collect_lines.go`
    - Commands/tests run:
        - `rg --files lab/stringsutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/stringsutil/*.go`
        - `sed -n '1,360p' lab/stringsutil/stringsutil.go`
        - `sed -n '1,240p' lab/stringsutil/collect_lines.go`
        - `go test ./lab/stringsutil`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/stringsutil` after this
          sweep.

## EV-20260215-058

- Package group: `lab/timer`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/timer/timer.go`
    - Commands/tests run:
        - `rg --files lab/timer`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/timer/*.go`
        - `sed -n '1,320p' lab/timer/timer.go`
        - `go test ./lab/timer`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/timer` after this sweep.

## EV-20260215-059

- Package group: `lab/tsgen`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/tsgen/generate_ts_content.go`
        - `lab/tsgen/to_file.go`
        - `lab/tsgen/statements.go`
        - `lab/tsgen/generate_ts_content_test.go`
    - Commands/tests run:
        - `rg --files lab/tsgen`
        - `for f in lab/tsgen/*.go; do if [[ $f != *_test.go ]]; then rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" "$f"; fi; done`
        - `for f in lab/tsgen/*.go; do if [[ $f != *_test.go ]]; then rg -n "^var [A-Z]" "$f"; fi; done`
        - `sed -n '1,360p' lab/tsgen/generate_ts_content.go`
        - `sed -n '1,360p' lab/tsgen/to_file.go`
        - `sed -n '1,260p' lab/tsgen/statements.go`
        - `go test ./lab/tsgen`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/tsgen` after this sweep.

## EV-20260215-060

- Package group: `lab/tsgen/tsgencore`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/tsgen/tsgencore/tsgencore.go`
        - `lab/tsgen/tsgencore/tsgencore_test.go`
    - Commands/tests run:
        - `rg --files lab/tsgen/tsgencore`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/tsgen/tsgencore/*.go`
        - `rg -n "^var [A-Z]" lab/tsgen/tsgencore/*.go`
        - `sed -n '1,420p' lab/tsgen/tsgencore/tsgencore.go`
        - `sed -n '420,840p' lab/tsgen/tsgencore/tsgencore.go`
        - `sed -n '1,240p' lab/tsgen/tsgencore/tsgencore_test.go`
        - `go test ./lab/tsgen/tsgencore`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/tsgen/tsgencore` after this
          sweep.

## EV-20260215-061

- Package group: `lab/vitecmd`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/vitecmd/cmd.go`
        - `lab/vitecmd/cmd_test.go`
    - Commands/tests run:
        - `rg --files lab/vitecmd`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/vitecmd/*.go`
        - `sed -n '1,420p' lab/vitecmd/cmd.go`
        - `sed -n '1,420p' lab/vitecmd/cmd_test.go`
        - `rg -n "\\bvitecmd\\.Log\\b|\\bLog\\.(Info|Warn|Error)\\(" --glob '!FRAMEWORK_AUDIT*.md'`
        - `gofmt -w lab/vitecmd/cmd.go`
        - `go test ./lab/vitecmd`
    - Findings/fixes:
        - Found and fixed `F-20260215-012`: `lab/vitecmd` exported mutable
          logger variable `Log` without an app-facing use case.
        - Unexported logger variable (`Log` -> `log`) and updated internal call
          sites.

## EV-20260215-062

- Package group: `lab/viteutil`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/viteutil/viteutil.go`
        - `lab/vitecmd/cmd.go`
        - `vormaruntime/get_root_handler.go`
        - `vormabuild/vite_manifest_paths.go`
        - `vormabuild/vite_paths_stage_two.go`
    - Commands/tests run:
        - `rg --files lab/viteutil`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/viteutil/*.go`
        - `sed -n '1,360p' lab/viteutil/viteutil.go`
        - `rg -n "\\bviteutil\\.(Manifest|ManifestChunk|ReadManifest|FindAllDependencies|FindRelativeEntrypointPath|Variant|VariantReact|VariantOther|ToDevScriptsOptions|ToDevScripts|PortEnvName|InitPort|GetVitePortStr)\\b" --glob '!FRAMEWORK_AUDIT*.md'`
        - `rg -n "InitPort\\(|PortEnvName|VariantReact|VariantOther" lab/viteutil lab/vitecmd vormaruntime vormabuild --glob '*.go'`
        - `gofmt -w lab/viteutil/viteutil.go`
        - `go test ./lab/viteutil ./lab/vitecmd`
        - `go test ./vormaruntime ./vormabuild`
    - Findings/fixes:
        - Found and fixed `F-20260215-013`: `InitPort(defaultPort int)` ignored
          its `defaultPort` argument, creating a misleading app-facing API
          contract.
        - Updated `InitPort` to pass caller-provided `defaultPort` to
          `netutil.GetFreePort`.

## EV-20260215-063

- Package group: `lab/xyz`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `lab/xyz/xyz.go`
    - Commands/tests run:
        - `rg --files lab/xyz`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" lab/xyz/*.go`
        - `sed -n '1,320p' lab/xyz/xyz.go`
        - `rg -n "\\bxyz\\.MakeEmojiDataURL\\b|\\bMakeEmojiDataURL\\(" --glob '!FRAMEWORK_AUDIT*.md'`
        - `go test ./lab/xyz`
    - Findings/fixes:
        - No additional Surface/API findings in `lab/xyz` after this sweep.

## EV-20260215-064

- Package group: `internal/scripts/buildts`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `internal/scripts/buildts/main.go`
    - Commands/tests run:
        - `rg --files internal/scripts/buildts`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" internal/scripts/buildts/*.go`
        - `sed -n '1,340p' internal/scripts/buildts/main.go`
        - `sed -n '320,760p' internal/scripts/buildts/main.go`
        - `go test ./internal/scripts/buildts`
    - Findings/fixes:
        - No additional Surface/API findings in `internal/scripts/buildts` after
          this sweep.

## EV-20260215-065

- Package group: `internal/scripts/bumper`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `internal/scripts/bumper/main.go`
    - Commands/tests run:
        - `rg --files internal/scripts/bumper`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" internal/scripts/bumper/*.go`
        - `sed -n '1,320p' internal/scripts/bumper/main.go`
        - `go test ./internal/scripts/bumper`
    - Findings/fixes:
        - No additional Surface/API findings in `internal/scripts/bumper` after
          this sweep.

## EV-20260215-066

- Package group: `internal/scripts/npm_bumper`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `internal/scripts/npm_bumper/main.go`
    - Commands/tests run:
        - `rg --files internal/scripts/npm_bumper`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" internal/scripts/npm_bumper/*.go`
        - `sed -n '1,320p' internal/scripts/npm_bumper/main.go`
        - `go test ./internal/scripts/npm_bumper`
    - Findings/fixes:
        - No additional Surface/API findings in `internal/scripts/npm_bumper`
          after this sweep.

## EV-20260215-067

- Package group: `internal/scripts/sum`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `internal/scripts/sum/main.go`
    - Commands/tests run:
        - `rg --files internal/scripts/sum`
        - `rg -n "^func [A-Z]|^type [A-Z]|^const [A-Z]|^var [A-Z]" internal/scripts/sum/*.go`
        - `sed -n '1,320p' internal/scripts/sum/main.go`
        - `go test ./internal/scripts/sum`
    - Findings/fixes:
        - No additional Surface/API findings in `internal/scripts/sum` after
          this sweep.

## EV-20260215-068

- Package group: `vorma.go`
- Pass name: Correctness/Fragility
- Evidence
    - Files reviewed:
        - `vorma.go`
    - Commands/tests run:
        - `sed -n '1,360p' vorma.go`
        - `rg -n "^type |^func |^const |^var " vorma.go`
        - `go test ./...`
    - Findings/fixes:
        - No additional Correctness/Fragility findings in `vorma.go` after this
          sweep.

## EV-20260215-069

- Package group: `bootstrap/*`
- Pass name: Correctness/Fragility
- Evidence
    - Files reviewed:
        - `bootstrap/bootstrap.go`
        - `bootstrap/utils.go`
    - Commands/tests run:
        - `rg --files bootstrap`
        - `rg -n "^func |^type |^const |^var " bootstrap/*.go`
        - `sed -n '1,360p' bootstrap/bootstrap.go`
        - `sed -n '360,520p' bootstrap/bootstrap.go`
        - `sed -n '1,220p' bootstrap/utils.go`
        - `go test ./bootstrap`
    - Findings/fixes:
        - No additional Correctness/Fragility findings in `bootstrap/*` after
          this sweep.

## EV-20260215-070

- Package group: `vormabuild/*`
- Pass name: Correctness/Fragility
- Evidence
    - Files reviewed:
        - `vormabuild/build_inner.go`
        - `vormabuild/route_sync.go`
        - `vormabuild/rollback_transaction.go`
        - `vormabuild/build_watch.go`
        - `vormabuild/reload_endpoint.go`
        - `vormabuild/rebuild_routes.go`
        - `vormabuild/build_environment.go`
        - `vormabuild/vorma_build.go`
        - `vormabuild/vite_paths_stage_two.go`
        - `vormabuild/route_parsing_pipeline.go`
        - `vormabuild/backend_route_discovery.go`
        - `vormabuild/backend_route_registration_generation.go`
        - `vormabuild/route_registry_build.go`
        - `vormabuild/runtime_state_snapshot.go`
        - `vormabuild/build_artifact_fs.go`
        - `vormabuild/atomic_file_write.go`
        - `vormabuild/resource_runner.go`
        - `vormabuild/build_cli.go`
        - `vormabuild/vite_cmd.go`
        - `vormabuild/vite_manifest_paths.go`
    - Commands/tests run:
        - `rg --files vormabuild`
        - `rg -n "^func |^type |^const |^var " vormabuild/*.go`
        - `ls -la vormabuild`
        - `sed -n '1,260p' vormabuild/build_inner.go`
        - `sed -n '1,240p' vormabuild/route_sync.go`
        - `sed -n '1,220p' vormabuild/rollback_transaction.go`
        - `sed -n '1,320p' vormabuild/build_watch.go`
        - `sed -n '1,280p' vormabuild/reload_endpoint.go`
        - `sed -n '1,260p' vormabuild/rebuild_routes.go`
        - `sed -n '1,280p' vormabuild/build_environment.go`
        - `sed -n '1,260p' vormabuild/vorma_build.go`
        - `sed -n '1,260p' vormabuild/vite_paths_stage_two.go`
        - `sed -n '1,260p' vormabuild/route_parsing_pipeline.go`
        - `sed -n '260,520p' vormabuild/route_parsing_pipeline.go`
        - `sed -n '1,260p' vormabuild/backend_route_discovery.go`
        - `rg -n "ensureGoTypesInfoInitialized|packageContainsRouteRegistrationHints|discoverRouteRegistrationsInRootFiles|discoverRouteRegistrationsInFunctionDeclaration|discoverCanonicalRouteRegistrationCallsInNode|goTypes" vormabuild/backend_route_discovery.go`
        - `sed -n '240,500p' vormabuild/backend_route_discovery.go`
        - `sed -n '560,920p' vormabuild/backend_route_discovery.go`
        - `sed -n '920,1320p' vormabuild/backend_route_discovery.go`
        - `sed -n '1,240p' vormabuild/backend_route_registration_generation.go`
        - `sed -n '240,520p' vormabuild/backend_route_registration_generation.go`
        - `sed -n '520,780p' vormabuild/backend_route_registration_generation.go`
        - `sed -n '1,260p' vormabuild/route_registry_build.go`
        - `sed -n '1,240p' vormabuild/runtime_state_snapshot.go`
        - `sed -n '1,220p' vormabuild/build_artifact_fs.go`
        - `sed -n '1,260p' vormabuild/atomic_file_write.go`
        - `sed -n '1,220p' vormabuild/resource_runner.go`
        - `sed -n '1,320p' vormabuild/build_cli.go`
        - `sed -n '1,280p' vormabuild/vite_cmd.go`
        - `sed -n '1,260p' vormabuild/vite_manifest_paths.go`
        - `go test ./vormabuild -count=1`
    - Findings/fixes:
        - No additional Correctness/Fragility findings in `vormabuild/*` after
          this sweep.

## EV-20260215-071

- Package group: `vormaruntime/*`, `vormabuild/*` callsites
- Pass name: Correctness/Fragility
- Evidence
    - Files reviewed:
        - `vormaruntime/vorma_core.go`
        - `vormaruntime/runtime_state.go`
        - `vormaruntime/route_registry.go`
        - `vormaruntime/route_reload.go`
        - `vormaruntime/glue.go`
        - `vormaruntime/gmpd.go`
        - `vormabuild/build_inner.go`
        - `vormabuild/runtime_state_snapshot.go`
        - `vormabuild/route_registry_build.go`
    - Commands/tests run:
        - `rg --files vormaruntime`
        - `rg -n "^func |^type |^const |^var " vormaruntime/*.go`
        - `ls -la vormaruntime`
        - `sed -n '1,260p' vormaruntime/runtime_state.go`
        - `sed -n '1,260p' vormaruntime/route_registry.go`
        - `sed -n '1,260p' vormaruntime/route_reload.go`
        - `sed -n '1,260p' vormaruntime/glue.go`
        - `sed -n '260,420p' vormaruntime/glue.go`
        - `sed -n '1,260p' vormaruntime/vorma_core.go`
        - `sed -n '1,260p' vormaruntime/gmpd.go`
        - `sed -n '260,560p' vormaruntime/gmpd.go`
        - `sed -n '560,760p' vormaruntime/gmpd.go`
        - `rg -n "WithRLock\\(func\\(" --glob '!**/node_modules/**'`
        - `gofmt -w vormaruntime/vorma_core.go vormabuild/build_inner.go vormabuild/runtime_state_snapshot.go vormabuild/build_artifacts_test.go vormabuild/route_registry_build.go vormaruntime/vorma_core_test.go vormaruntime/gmpd_cache_test.go`
        - `go test ./vormaruntime ./vormabuild`
    - Findings/fixes:
        - Found and fixed `F-20260215-014`: `WithRLock` exposed `*LockedVorma`,
          which includes mutating setters and therefore allowed write operations
          under a read lock.
        - Added `ReadLockedVorma` and changed `WithRLock` to expose read-only
          state access.
        - Updated dependent callsites/signatures to consume read-only access
          where required (`vormabuild/runtime_state_snapshot.go`,
          `vormabuild/route_registry_build.go`, and tests).

## EV-20260215-072

- Package group: `wave/*`
- Pass name: Correctness/Fragility
- Evidence
    - Files reviewed:
        - `wave/wave.go`
        - `wave/runtime_framework.go`
        - `wave/parse.go`
        - `wave/cache_internal.go`
        - `wave/runtime_assets.go`
        - `wave/filemap.go`
        - `wave/css.go`
    - Commands/tests run:
        - `rg --files wave | head -n 200`
        - `rg -n "^func |^type |^const |^var " wave/*.go`
        - `ls -la wave`
        - `sed -n '1,260p' wave/wave.go`
        - `sed -n '1,260p' wave/runtime_framework.go`
        - `sed -n '1,260p' wave/parse.go`
        - `sed -n '1,220p' wave/cache_internal.go`
        - `sed -n '1,220p' wave/runtime_assets.go`
        - `sed -n '1,220p' wave/filemap.go`
        - `sed -n '1,220p' wave/css.go`
        - `go test ./wave -count=1`
    - Findings/fixes:
        - No additional Correctness/Fragility findings in `wave/*` after this
          sweep.

## EV-20260215-073

- Package group: `internal/vormaruntime/*`, `vorma.go`, `vormabuild/*`,
  `wave/*`, `wave/tooling/*`
- Pass name: Surface/API
- Evidence
    - Files reviewed:
        - `vorma.go`
        - `internal/vormaruntime/*`
        - `vormabuild/vorma_build.go`
        - `vormabuild/build_environment.go`
        - `vormabuild/build_watch.go`
        - `wave/runtime_framework.go`
        - `wave/parsed_config_clone.go`
        - `wave/tooling/builder.go`
    - Commands/tests run:
        - `mv vormaruntime internal/vormaruntime`
        - `rg -l 'github.com/vormadev/vorma/vormaruntime' | xargs perl -0pi -e 's#github.com/vormadev/vorma/vormaruntime#github.com/vormadev/vorma/internal/vormaruntime#g'`
        - `rg -n 'Internal__GetParsedConfigMutableReference|Internal__GetMutableConfigReference'`
        - `gofmt -w vorma.go wave/*.go wave/tooling/*.go vormabuild/*.go internal/vormaruntime/*.go`
        - `go test ./... -count=1`
        - `make gotest`
        - `make tstest`
        - `make tscheck`
        - `make tslint`
    - Findings/fixes:
        - Internalized `vormaruntime` to root `internal` boundary
          (`internal/vormaruntime`) so external consumers cannot import runtime
          internals directly.
        - Updated `vorma.go` and `vormabuild/*` imports to consume
          `internal/vormaruntime`.
        - Removed mutable parsed-config escape hatches from public API:
            - removed `(*wave.Wave).Internal__GetParsedConfigMutableReference`
            - removed `(*tooling.Builder).Internal__GetMutableConfigReference`
        - Added explicit Wave framework-facing APIs for buildtime wiring without
          exposing live mutable config pointers:
            - `RegisterFrameworkSchemaSection`
            - `SetFrameworkDevBuildHookCommand`
            - `SetFrameworkProdBuildHookCommand`
            - `SetFrameworkRunBuildHookRunner`
            - `SetFrameworkPrepareGoBuildOverlay`
            - `GetBuildtimeParsedConfig`
        - Refactored `vormabuild` build-environment wiring to operate on
          buildtime config snapshots (`configureBuildEnvironment*`) instead of
          mutating Wave internals through a raw pointer.
        - Updated dependent tests and e2e fixture code to use explicit buildtime
          APIs; resolved a transient `vormabuild` test regression where a
          refactored test accidentally exercised real dev-server startup.
