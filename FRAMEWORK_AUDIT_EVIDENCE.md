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
