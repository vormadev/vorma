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
