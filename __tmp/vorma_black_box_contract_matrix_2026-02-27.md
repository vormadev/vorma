# Vorma Canonical Black-Box Contract Matrix (Draft, 2026-02-27)

This matrix defines public behaviors in observable terms only.

## Global Rules

- Backend-owned route JSON contract is trusted.
- Browser runtime is browser-only.
- Contract tests must observe only public API behavior, DOM effects, events,
  history, and network calls.
- Contract tests must not assert runtime internals, reducer/event-plan
  internals, singleton shapes, helper names, or internal map state.
- UI adapter behavior validation remains dist-only via `npm_dist` imports.

## 1) Navigation

Public surfaces:

- `vormaNavigate`, `Link`/`makeFinalLinkProps` click behavior.

Observable inputs:

- target href (same-document noop/hash-change/cross-document),
- replace/push options,
- overlapping navigation starts,
- redirect responses.

Observable outputs:

- `window.location`/history updates,
- route-change/build-id/location/status events,
- title/head/component commit results,
- stale completion suppression (only winning operation commits).

Must not test:

- internal operation IDs,
- internal command/reducer transitions,
- internal lane maps.

## 2) Revalidation

Public surfaces:

- `revalidate`,
- focus-triggered revalidation APIs.

Observable inputs:

- explicit `revalidate` calls,
- focus events with stale-window policy.

Observable outputs:

- status transitions (`isRevalidating`),
- whether fetch/commit occurs or is skipped,
- stale suppression when superseded by newer work.

Must not test:

- revalidation lane internals,
- queue slot internals,
- reducer checkpoint internals.

## 3) Submissions

Public surfaces:

- `submit` and submit result behavior.

Observable inputs:

- method/body combinations,
- redirect/non-redirect responses,
- overlapping/deduped submissions.

Observable outputs:

- `SubmitResult` shape,
- status transitions (`isSubmitting`),
- redirects/navigation handoff behavior,
- optional auto-revalidation behavior.

Must not test:

- internal dedupe map entry identity,
- internal submission ownership bookkeeping.

## 4) Client Loaders (Parallelism + Ownership + Commit)

Public surfaces:

- route-scoped loader readers,
- global loader readers,
- navigation/prefetch interactions with client loader waits.

Observable inputs:

- registered loader waits,
- overlapping prefetch/navigation/revalidation sequences,
- resolve order permutations.

Observable outputs:

- parallel start behavior (independent waits can begin before prior settle),
- final committed loader data reflects authoritative operation only,
- stale completion has no visible commit side effects,
- no unhandled rejection leaks.

Must not test:

- internal wait map shape,
- internal promise registry topology.

## 5) Route-Props Scoped Readers/Hooks

Public surfaces:

- `useLoaderData(routeProps)`,
- `useClientLoaderData(routeProps)`,
- `useRouterData(routeProps)` where applicable.

Observable inputs:

- transition windows with overlapping operations,
- stable route scope vs changing route scope.

Observable outputs:

- scoped reads stay tied to current authoritative route scope per render tick,
- no stale-scope bleed-through in committed UI state.

Must not test:

- internal token objects,
- internal adapter host bookkeeping details.

## 6) Global Readers/Selectors

Public surfaces:

- `getRouterData`, `getStatus`, event listener APIs.

Observable inputs:

- navigation/submit/revalidate lifecycle operations.

Observable outputs:

- synchronous reads reflect current committed snapshot semantics,
- event order is observable and contractually stable where documented,
- no duplicate status event emission for identical status.

Must not test:

- internal snapshot canonicalization strategy.

## 7) Link Behavior (Click + Prefetch Intent)

Public surfaces:

- link click handling and prefetch handlers.

Observable inputs:

- primary vs modified clicks,
- self vs non-self targets,
- same-document classifications,
- prefetch intent timing/start-stop/cancel patterns.

Observable outputs:

- preventDefault decisions,
- whether fetch/navigation occurs,
- prefetch dedupe behavior by navigation target semantics,
- click callbacks ordering (consumer hooks + framework hooks).

Must not test:

- private helper branching details.

## 8) Head/Title/CSS/Module Preload Side Effects

Public surfaces:

- navigation commit visible DOM/head effects.

Observable inputs:

- server payload title/head/css/deps for authoritative operation.

Observable outputs:

- title updates,
- head section reconciliation,
- stylesheet and module preload insertion behavior,
- stale operation side effects not committed.

Must not test:

- internal staging/canonicalization implementation details.

## Poisonous Assertions Excluded By Matrix

- Tests requiring frontend fallback behavior for backend-owned contract
  violations.
- Tests requiring frontend panic/assert behavior for backend-owned contract
  violations.
- Tests requiring non-browser runtime guards for browser-only runtime paths.

## Ambiguous Items Requiring User Sign-Off Before Contract Edits

- Explicit behavior on impossible protocol statuses (example: route-data
  response `304` in a JSON fetch path).
