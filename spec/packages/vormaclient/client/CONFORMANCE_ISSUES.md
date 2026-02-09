# vormaclient/client Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Type               | Affected Requirements                        | Summary                                                                                                                                                                                                                            | Status |
| -------- | ------------------ | -------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ------ |
| VCI-016  | impl-bug-candidate | FE-SKIP-005                                  | Skip param/splat guard appears to inspect innermost loader-bearing match rather than required outermost match.                                                                                                                     | open   |
| VCI-017  | impl-bug-candidate | FE-FETCH-005, FE-FETCH-006                   | Invalid highest-priority reload signal currently suppresses fallback evaluation of lower-priority redirect signals.                                                                                                                | open   |
| VCI-032  | impl-bug-candidate | FE-NAV-009                                   | Stale-origin revalidation short-circuit currently occurs after stale payload side effects can already run.                                                                                                                         | open   |
| VCI-033  | impl-bug-candidate | FE-LINK-013                                  | Prefetch stop path appears to compute lookup key without applying search/hash overrides used by prefetch start path.                                                                                                               | open   |
| VCI-036  | impl-bug-candidate | VORMACLIENT-PREACT-003, FE-UI-010, FE-UI-011 | Preact typed pattern-based helper memoization appears stale across route-change matched-pattern updates.                                                                                                                           | open   |
| VCI-037  | impl-bug-candidate | FE-UI-012                                    | UI adapters can get stuck when route-level component identity transitions from undefined to defined after initial render.                                                                                                          | open   |
| VCI-038  | impl-bug-candidate | FE-UI-013                                    | Preact terminal component-absent outlet branch inserts a synthetic wrapper node while React/Solid remain node-empty.                                                                                                               | open   |
| VCI-054  | impl-bug-candidate | FE-FETCH-012                                 | Route-data handling currently treats HTTP 304 as allowed but then fails due to unconditional JSON-required path.                                                                                                                   | open   |
| VCI-055  | impl-bug-candidate | FE-LINK-019                                  | Repeated prefetch start() calls can stack pending timers while stop() clears only the latest handle.                                                                                                                               | open   |
| VCI-075  | impl-bug-candidate | FE-CL-004                                    | Client-loader execution skips only the exact `outermostServerErrorIdx` route and still runs deeper loaders; spec requires skipping index `i` and all deeper routes when server outermost error index is `i`.                       | open   |
| VCI-077  | impl-bug-candidate | FE-FETCH-010, FE-FETCH-011                   | Build-id event path dispatches without persisting global `buildID`, and redirect responses can bypass build-id eventing before redirect handoff; spec requires storage update-before-dispatch and redirect-path dispatch ordering. | open   |
| VCI-078  | impl-bug-candidate | FE-ASSET-005, FE-ASSET-006                   | CSS apply path currently builds stylesheet href by raw `publicPathPrefix + bundle` concatenation instead of canonical public-href resolution, risking dev/prod base inconsistency and slash-boundary non-normalized URLs.          | open   |
| VCI-080  | impl-bug-candidate | FE-UI-001, FE-UI-002                         | Solid adapter assigns function-valued `activeErrorBoundary` via direct signal setter, which Solid treats as updater; this can execute/coerce component functions instead of storing them, breaking route-change state sync/parity. | open   |

## Source Validation Notes (`E2-R3`)

- `VCI-016`: `vormaclient/client/src/client.ts` finds `outermostLoaderIndex` by
  iterating from the tail (`for` loop descending in `findOutermostLoaderIndex`)
  and then applies parameter/splat guard using that index in
  `didOutermostParamsChange`.
- `VCI-017`: `vormaclient/client/src/redirects/redirects.ts` returns `null`
  immediately when `X-Vorma-Reload` is present but non-HTTP, which exits before
  lower-priority redirect parsing branches.
- `VCI-032`: `vormaclient/client/src/client.ts` performs module-map/CSS side
  effects in `processSuccessfulNavigation` before stale-origin revalidation
  guard exits.
- `VCI-033`: `vormaclient/client/src/links.ts` prefetch start applies
  `search`/`hash` overrides to `fullUrl`, but stop-path lookup key is recomputed
  from bare `relativeURL`.
- `VCI-036`: `vormaclient/preact/src/helpers.ts` pattern helper memoization is
  keyed only by `pattern`, and typed client-loader accessor memoization is keyed
  only by `props`; both omit `routerData` reactivity inputs.
- `VCI-037`: `vormaclient/react/src/react.tsx`,
  `vormaclient/preact/src/preact.tsx`, and `vormaclient/solid/src/solid.tsx` all
  gate identity refresh on existing `currentImportURL`, blocking
  undefined-to-defined identity transitions.
- `VCI-038`: `vormaclient/preact/src/preact.tsx` terminal component-absent
  branch renders `h("div", {})` instead of a node-empty branch.
- `VCI-054`: `vormaclient/client/src/client.ts` treats HTTP 304 as not-fatal but
  still throws on missing parsed JSON because JSON parse path is gated by
  `response.ok`.
- `VCI-055`: `vormaclient/client/src/links.ts` repeated `start()` calls can
  enqueue multiple timers before `prefetchStarted` flips; `stop()` clears only
  one tracked timer handle.
- `VCI-075`: `vormaclient/client/src/client_loaders.ts` skips only
  `i === outermostServerErrorIdx`; deeper client loaders still execute.
- `VCI-077`: `vormaclient/client/src/client.ts` dispatches build-id events
  without setting global `buildID`, and redirect return path exits before
  build-id dispatch ordering in redirect handoff flows.
- `VCI-078`: `vormaclient/client/src/asset_manager.ts` applies CSS href using
  raw `publicPathPrefix + bundle` concatenation.
- `VCI-080`: `vormaclient/solid/src/solid.tsx` assigns
  `setActiveErrorBoundary(ctx.get("activeErrorBoundary"))` directly; function
  values are treated by Solid setter semantics as updaters.

## Legacy-Test Sanity Notes (`E2-R3`)

- Redirect legacy tests cover valid signal priority and non-HTTP redirect ignore
  behavior, but do not currently cover the invalid-highest-signal fallback path;
  `VCI-017` remains source-backed.
- Asset/path legacy tests assert normalized stylesheet href outcomes under
  trailing-slash public prefixes; `VCI-078` remains tracked against current CSS
  apply path implementation.
- Prefetch legacy tests cover single-start cancellation and upgrade semantics,
  but do not currently cover repeated-start timer stacking; `VCI-055` remains
  source-backed.
- Existing legacy tests assert build-id event dispatch timing, but do not
  currently assert global build-id storage update-before-dispatch ordering;
  `VCI-077` remains source-backed.
- No legacy UI-adapter tests outside `conformance/**` currently exercise the
  adapter parity/reactivity edge cases behind `VCI-036`, `VCI-037`, `VCI-038`,
  and `VCI-080`; those remain source-backed.
