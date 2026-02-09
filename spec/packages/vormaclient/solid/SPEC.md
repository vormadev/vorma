# vormaclient/solid Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `vormaclient/solid`

## Scope

Package-owned Solid adapter contracts in `vormaclient/solid/**`.

This package exposes Solid-specific typed helpers, link helpers, and root-outlet
adapter surface over owner runtime behavior in `vormaclient/client`.

Current evidence note:

- Requirements are currently source-backed in `vormaclient/solid/**`.
- No active legacy tests outside `conformance/**` were found for this package in
  the repo.

## Requirements

- `VORMACLIENT-SOLID-001` Entry-point export surface. `index.tsx` MUST export
  typed helper factories/types (`makeTypedAddClientLoader`,
  `makeTypedUseLoaderData`, `makeTypedUsePatternLoaderData`,
  `makeTypedUseRouterData`, `VormaRoute`, `VormaRouteProps`), link exports
  (`VormaLink`, `makeTypedLink`), and adapter exports (`VormaRootOutlet`,
  `location`).
- `VORMACLIENT-SOLID-002` Typed route alias surface. Helper-level
  `VormaRouteProps` and `VormaRoute` aliases MUST bind
  `VormaRoutePropsGeneric`/`VormaRouteGeneric` to `JSX.Element` while preserving
  app/pattern generic parameters.
- `VORMACLIENT-SOLID-003` Typed loader/router helper contract.
  `makeTypedUseRouterData`, `makeTypedUseLoaderData`, and
  `makeTypedUsePatternLoaderData` MUST read from adapter snapshots and: return
  loader data by route index, resolve first exact matched pattern index, and
  return `undefined` when no match exists.
- `VORMACLIENT-SOLID-004` Typed client-loader registration/access contract.
  `makeTypedAddClientLoader` MUST register pattern via
  `__registerClientLoaderPattern`, store loader in `patternToWaitFnMap`,
  optionally call `__runClientLoadersAfterHMRUpdate` in dev when
  `reRunOnModuleChange` is provided, and return accessor(s) that read
  client-loader data by explicit route index or matched-pattern lookup.
- `VORMACLIENT-SOLID-005` Link anchor wiring contract. `VormaLink` MUST delegate
  navigation/prefetch wiring to `__makeFinalLinkProps`, expose `data-external`,
  wire helper-derived handlers
  (pointer-enter/focus/pointer-leave/blur/touch-cancel/click), and avoid
  forwarding Vorma-only control props (`prefetch`, `scrollToTop`, `replace`,
  `state`) as plain DOM attributes.
- `VORMACLIENT-SOLID-006` Typed link destination-resolution contract.
  `makeTypedLink` MUST resolve loader destinations with `__resolvePath`, apply
  optional `search`/`hash` overrides on URL construction, merge `defaultProps`
  with call-site props, and pass resolved `href` plus `state` through
  `VormaLink`.
- `VORMACLIENT-SOLID-007` Root-outlet/location adapter surface contract. Package
  MUST expose Solid root-outlet and location signal surfaces. Rendering and
  route-change semantics are owned by `vormaclient/client` requirements
  (`FE-UI-*`) and this adapter surface MUST remain compatible with that owner
  contract.
- `VORMACLIENT-SOLID-008` TypeScript JSX config contract. `tsconfig.json` MUST
  extend repo base config and set Solid JSX settings (`jsx: preserve`,
  `jsxImportSource: solid-js`).
