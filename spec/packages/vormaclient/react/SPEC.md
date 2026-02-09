# vormaclient/react Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `vormaclient/react`

## Scope

Package-owned React adapter contracts in `vormaclient/react/**`.

This package exposes React-specific typed helpers, link helpers, and root-outlet
adapter surface over owner runtime behavior in `vormaclient/client`.

Current evidence note:

- Requirements are currently source-backed in `vormaclient/react/**`.
- No active legacy tests outside `conformance/**` were found for this package in
  the repo.

## Requirements

- `VORMACLIENT-REACT-001` Entry-point export surface.
  `index.tsx` MUST export typed helper factories/types (`makeTypedAddClientLoader`,
  `makeTypedUseLoaderData`, `makeTypedUsePatternLoaderData`,
  `makeTypedUseRouterData`, `VormaRoute`, `VormaRouteProps`), link exports
  (`VormaLink`, `makeTypedLink`), and adapter exports (`VormaRootOutlet`,
  `useLocation`).
- `VORMACLIENT-REACT-002` Typed route alias surface.
  Helper-level `VormaRouteProps` and `VormaRoute` aliases MUST bind
  `VormaRoutePropsGeneric`/`VormaRouteGeneric` to `JSX.Element` while preserving
  app/pattern generic parameters.
- `VORMACLIENT-REACT-003` Typed loader/router helper contract.
  `makeTypedUseRouterData`, `makeTypedUseLoaderData`, and
  `makeTypedUsePatternLoaderData` MUST read from adapter snapshots and:
  return loader data by route index, resolve first exact matched pattern index,
  and return `undefined` when no match exists.
- `VORMACLIENT-REACT-004` Typed client-loader registration/access contract.
  `makeTypedAddClientLoader` MUST register pattern via
  `__registerClientLoaderPattern`, store loader in `patternToWaitFnMap`,
  optionally call `__runClientLoadersAfterHMRUpdate` in dev when
  `reRunOnModuleChange` is provided, and return hook accessor(s) that read
  client-loader data by explicit route index or matched-pattern lookup.
- `VORMACLIENT-REACT-005` Link anchor wiring contract.
  `VormaLink` MUST delegate navigation/prefetch wiring to
  `__makeFinalLinkProps`, expose `data-external`, wire helper-derived handlers
  (pointer-enter/focus/pointer-leave/blur/touch-cancel/click), and avoid
  forwarding Vorma-only control props (`prefetch`, `scrollToTop`, `replace`,
  `state`) as plain DOM attributes.
- `VORMACLIENT-REACT-006` Typed link destination-resolution contract.
  `makeTypedLink` MUST resolve loader destinations with `__resolvePath`, apply
  optional `search`/`hash` overrides on URL construction, merge `defaultProps`
  with call-site props, and pass resolved `href` plus `state` through `VormaLink`.
- `VORMACLIENT-REACT-007` Root-outlet/location adapter surface contract.
  Package MUST expose React root-outlet and `useLocation` surfaces. Rendering and
  route-change semantics are owned by `vormaclient/client` requirements
  (`FE-UI-*`) and this adapter surface MUST remain compatible with that owner
  contract.
- `VORMACLIENT-REACT-008` TypeScript JSX config contract.
  `tsconfig.json` MUST extend repo base config and set React JSX mode
  (`jsx: react-jsx`).
