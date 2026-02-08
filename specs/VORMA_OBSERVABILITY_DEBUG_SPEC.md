# Vorma Observability and Debug Specification

Status: Draft  
Last Updated: 2026-02-07  
Applies To: Runtime/build/client observability surfaces (logs, events, debug hooks, reload diagnostics)

## 1. Why This Spec Exists

This document defines observable diagnostic contracts that operators and developers
rely on during development, CI, and production troubleshooting.

It exists to:

- preserve debuggability during large refactors,
- keep logging/event semantics stable enough for tooling and tests,
- define what Vorma exposes for health and state transitions.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Observability Surface Types

Vorma observability surfaces include:

- backend/build structured logs,
- client console diagnostics,
- browser window events,
- dev reload/invalidation endpoint responses,
- explicit debug hooks.

### 2.3 Out of Scope

Out of scope:

- external metrics backend schema,
- distributed tracing protocol standardization,
- product analytics instrumentation.

## 3. Requirement Catalog

## 3.1 Logger Provisioning and Namespacing

### OBS-LOG-001: Configurable Backend Logger

Given `NewVormaApp` is called with `VormaAppConfig.Logger`  
When app initializes  
Then runtime MUST use provided logger instance.

### OBS-LOG-002: Default Backend Logger Availability

Given no custom logger is provided  
When app initializes  
Then runtime MUST initialize a default logger suitable for human-readable diagnostics.

### OBS-LOG-003: Frontend Log Namespace

Given client runtime emits `logInfo` or `logError` output  
When console log is emitted  
Then message prefix MUST include stable namespace marker `Vorma:`.

### OBS-LOG-004: Stable Severity Classes

Runtime/backend log emission MUST preserve semantic severity classes:

- informational lifecycle logs,
- warning logs for degraded/non-ideal behavior,
- error logs for failed operations.

## 3.2 Backend Runtime Diagnostic Events

### OBS-BE-001: Initialization Success Log

Given `Init()` completes successfully  
When startup logging occurs  
Then runtime SHOULD log successful initialization including current build identifier.

### OBS-BE-002: Loader Error Diagnostics

Given loader execution fails  
When server handles failure  
Then runtime SHOULD log route pattern and server-side error context.

### OBS-BE-003: Generic-Message Warning

Given loader error is not typed `LoaderError`  
When generic client message is emitted  
Then runtime SHOULD log warning indicating generic client message fallback.

### OBS-BE-004: Template/Render Failure Logs

Given SSR head/script/template generation fails  
When request handling fails  
Then runtime SHOULD log error with failure stage context.

### OBS-BE-005: Dev Reload Failure Logs

Given `/__vorma/reload-routes` or `/__vorma/reload-template` operation fails  
When endpoint handling fails  
Then runtime SHOULD log error and return HTTP 500.

### OBS-BE-006: Dev Reload Success Logs

Given route/template reload from disk succeeds  
When reload operation completes  
Then runtime SHOULD log successful reload event.

## 3.3 Build Pipeline Diagnostic Events

### OBS-BUILD-001: Build Start/End Logs

Given Vorma build pipeline runs (dev or prod)  
When build begins and ends  
Then logs SHOULD include lifecycle markers for start and completion.

### OBS-BUILD-002: Build Duration Visibility

Given build/rebuild completes  
When completion log is emitted  
Then duration SHOULD be included for performance troubleshooting.

### OBS-BUILD-003: Build Identity Visibility

Given build/rebuild completes  
When completion log is emitted  
Then current build ID SHOULD be included.

### OBS-BUILD-004: Route Count Visibility

Given build/rebuild completes  
When completion log is emitted  
Then route count SHOULD be included.

### OBS-BUILD-005: Fast Route Rebuild Lifecycle Logs

Given dev fast route rebuild runs  
When operation starts and ends  
Then logs SHOULD include explicit fast-rebuild lifecycle markers.

### OBS-BUILD-006: Fallback-to-Restart Warning

Given route/template reload endpoint callback fails in dev flow  
When build pipeline falls back to restart behavior  
Then warning log SHOULD indicate fallback and underlying error.

### OBS-BUILD-007: Non-Fatal Static Dir Absence Warning

Given static public output directory is absent during cleanup  
When cleanup handles this case  
Then runtime SHOULD warn (not fail) with directory context.

## 3.4 Frontend Runtime Status and Event Bus

### OBS-FE-001: Status Event Contract

Given navigation/submission/revalidation state changes  
When state is published  
Then `vorma:status` MUST carry:

- `isNavigating`
- `isSubmitting`
- `isRevalidating`.

### OBS-FE-002: Route Change Event Contract

Given route rendering commits  
When event is published  
Then `vorma:route-change` MUST be dispatched, optionally carrying scroll restoration detail.

### OBS-FE-003: Build ID Event Contract

Given client detects build ID change from response headers  
When event is published  
Then `vorma:build-id` MUST include `{ oldID, newID }`.

### OBS-FE-004: Location Event Contract

Given history key changes  
When location event is published  
Then `vorma:location` MUST be dispatched.

### OBS-FE-005: Listener Cleanup Contract

Given a listener adder API returns cleanup function  
When cleanup is called  
Then associated event listener MUST be removed.

### OBS-FE-006: Status Debounce/Dedup Semantics

Given rapid state transitions  
When status emission occurs  
Then runtime MUST debounce and avoid duplicate consecutive status payloads.

## 3.5 Debug and Developer Hooks

### OBS-DBG-001: Dev Revalidate Hook Exposure

Given client runtime runs in dev mode  
When HMR init completes  
Then `window.__waveRevalidate` MUST be exposed as a debug/manual revalidation hook.

### OBS-DBG-002: HMR Pattern Refresh Logging

Given dev HMR JS update affects currently matched pattern  
When client triggers post-update loader rerun  
Then runtime SHOULD log refresh intent including pattern identifier.

### OBS-DBG-003: History Fallback Error Diagnostics

Given browser-history POP navigation fails client-side  
When runtime falls back to hard reload  
Then runtime SHOULD log error describing failure and fallback action.

### OBS-DBG-004: Redirect Limit Diagnostic

Given redirect chain exceeds configured max  
When client aborts chain  
Then runtime SHOULD log explicit redirect-limit diagnostic.

## 3.6 Dev Endpoint Diagnostics

### OBS-ENDPOINT-001: Route Reload Endpoint Success Signature

Given `GET /__vorma/reload-routes` succeeds  
When endpoint responds  
Then response MUST be HTTP 200 with body `ok`.

### OBS-ENDPOINT-002: Template Reload Endpoint Success Signature

Given `GET /__vorma/reload-template` succeeds  
When endpoint responds  
Then response MUST be HTTP 200 with body `ok`.

### OBS-ENDPOINT-003: Reload Endpoint Error Signature

Given reload endpoint fails operation  
When endpoint responds  
Then response MUST be HTTP 500 with error text body.

### OBS-ENDPOINT-004: Vite Filemap Invalidate Endpoint Signature

Given `POST /__vorma_invalidate_filemap` is invoked in dev Vite plugin flow  
When endpoint responds  
Then response MUST be HTTP 200 with `ok`, and full-reload signal SHOULD be broadcast.

## 3.7 Data Hygiene in Diagnostics

### OBS-HYGIENE-001: No Raw Request Body Logging by Default

Default Vorma diagnostics SHOULD avoid logging full request bodies or other high-risk payload data.

### OBS-HYGIENE-002: Client Error Logs Should Avoid Secret Material

Client runtime diagnostic logs SHOULD avoid printing sensitive tokens/cookies/auth secrets.

### OBS-HYGIENE-003: Build/Route Logs Should Prefer Metadata

Build/runtime logs SHOULD favor structured metadata (IDs, durations, counts, route patterns) over dumping full payload objects.

## 3.8 Compatibility and Change Policy

### OBS-COMPAT-001: Event Name Stability

Public event names (`vorma:status`, `vorma:route-change`, `vorma:build-id`, `vorma:location`) are compatibility-sensitive and MUST NOT change without versioned migration policy.

### OBS-COMPAT-002: Payload Shape Stability

Event payload fields documented in this spec are compatibility-sensitive and MUST NOT be removed/renamed without migration/deprecation plan.

### OBS-COMPAT-003: Log Message Text Flexibility

Exact human-readable log strings MAY evolve; however, semantic presence of key lifecycle/error diagnostics MUST be preserved.

### OBS-COMPAT-004: Prefix/Namespace Stability

Namespace markers used by default logger channels (`Vorma:` client prefix, backend logger identity) SHOULD remain stable enough for log filtering tools.

## 4. Conformance Test Guidance

Observability conformance tests SHOULD include:

1. event name and payload shape assertions for all public client events,
2. listener cleanup verification,
3. status debounce/dedup behavior assertions,
4. endpoint response signature checks for reload/invalidation endpoints,
5. smoke checks for key lifecycle logging points (init/build/rebuild/reload/error).

## 5. Relation to Other Specs

- Backend runtime contracts: `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- Wire contracts: `/Users/sjc/__code/river/specs/VORMA_WIRE_CONTRACT_SPEC.md`
- Build/dev contracts: `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Frontend runtime contracts: `/Users/sjc/__code/river/specs/VORMA_FRONTEND_RUNTIME_SPEC.md`
- Security model: `/Users/sjc/__code/river/specs/VORMA_SECURITY_MODEL_SPEC.md`
- Testing strategy: `/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md`
- Roadmap checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`
