# Vorma Poisonous Test Manual Triage (2026-02-27)

This is a manual triage pass over the Phase 1 mapping output. It separates true
policy violations from false-positive heuristics.

## Confirmed Poisonous (Rewrite or Delete)

These tests conflict with current rewrite rules (backend-contract trust,
browser-only runtime, no frontend fallback/panic for backend-owned contracts).

### A) Non-browser runtime resiliency assertions

- `typescript/vorma/client/src/tests/unit/events_platform.test.ts:10`
    - `dispatchStatusEvent does not throw when window is unavailable`
- `typescript/vorma/client/src/tests/unit/events_platform.test.ts:25`
    - `addStatusListener returns a safe cleanup when window is unavailable`

Reason:

- Runtime is browser-only. These tests enforce dead SSR/Node guard behavior.

### B) Backend payload validation/fallback assertions

- `typescript/vorma/client/src/tests/contracts/client.error_and_edge.contract.test.ts:99`
    - `treats empty JSON response as failed and then recovers`
- `typescript/vorma/client/src/tests/contracts/client.error_and_edge.contract.test.ts:280`
    - `rejects client-loader serverDataPromise with AbortError when required server loader payload is missing`
- `typescript/vorma/client/src/tests/contracts/client.prefetch.contract.test.ts:767`
    - `rejects serverDataPromise with AbortError when server omits a prestarted matched pattern`

- `typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:215`
    - `throws when payload root is not an object`
- `typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:223`
    - `throws when a required route-data key is missing`
- `typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:241`
    - `throws when top-level required array fields are not arrays`
- `typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:251`
    - `throws when hasRootData is not a boolean`
- `typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:261`
    - `throws when params is not an object`
- `typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:271`
    - `throws when optional clientLoadersData is present and not an array`
- `typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:310`
    - `deep-freezes decoded payload to prevent post-decode mutation`
- `typescript/vorma/client/src/tests/unit/fetch_route_data_server_internal.test.ts:484`
    - `throws and aborts when route-data JSON violates top-level contract at ingress`

- `typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts:3373`
    - `fails fast when server JSON omits required importURLs`
- `typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts:3409`
    - `uses build-id fallback in server route-data request URLs`
- `typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts:3451`
    - `falls back to build-id 1 when loader server-data response header is missing`
- `typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts:3587`
    - `aborts waiting client loaders when server JSON omits required matched loader arrays`
- `typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts:3682`
    - `defaults omitted deps and cssBundles to empty arrays in production`

- `typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts:392`
    - `returns null when client-loader server data is missing pattern match`
- `typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts:403`
    - `returns null when root data is required but unavailable`
- `typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts:734`
    - `supports setupClientLoaders with missing snapshots and empty maps`
- `typescript/vorma/client/src/tests/unit/render_runtime_internal.test.ts:750`
    - `passes unavailable server data to client loaders when required server payload is missing`

Reason:

- These tests enshrine frontend behavior for backend contract violations
  (fallbacks, panics, coercions, deep-freeze safety machinery, or synthetic
  AbortError mapping for missing backend-owned fields).

## Likely Keep (Not Poisonous)

- Network or transport failure handling:
    - non-OK responses, rejected fetch promises, redirect handoff behavior.
- Status/event continuity and stale outcome ownership tests.
- Storage/write failure handling on user/browser surfaces:
    - malformed `sessionStorage` data,
    - blocked storage APIs.

Reason:

- These do not enforce distrust of backend-owned shape contracts; they cover
  real user/browser runtime failure modes.

## Needs Explicit Sign-Off Before Editing

- `typescript/vorma/client/src/tests/unit/navigation_runtime_internal.test.ts:3892`
    - `throws explicit error when server route-data request returns 304`

Decision needed:

- Keep explicit loud failure for impossible protocol status, or remove all
  frontend special handling and rely fully on backend invariants.

## Next Execution Step

- Convert all confirmed poisonous items into rewrite/delete tasks in the Phase 4
  migration checklist.
- Do not change any ambiguous semantic item until user confirms intent.
