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
        - This supersedes the public-export placement portion of
          `EV-20260215-011`.
