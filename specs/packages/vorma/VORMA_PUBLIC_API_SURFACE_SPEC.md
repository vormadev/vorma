# Vorma Public API Surface Specification

Status: Draft  
Last Updated: 2026-02-09  
Applies To: Public Go and npm API surfaces exposed by Vorma

## 1. Why This Spec Exists

This document defines Vorma's public API map so that:

- refactors can preserve behavior without reading implementation internals,
- API-surface changes are visible early,
- conformance tests can be generated from explicit surface contracts.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST** and **MUST NOT** are normative.

### 2.2 Scope

This spec catalogs API surface and API-conformance checks. Detailed runtime
semantics are defined in runtime/build/wire/frontend specs.

## 3. API Classes

### API-CLASS-001: Tier Definitions

Publicly exposed surfaces MUST be classified as one of:

- **Tier A (core public API)**: documented consumer-facing surfaces that are
  primary conformance targets.
- **Tier B (interop API)**: exposed surfaces whose behavior is delegated to
  kit/wave packages and governed by dedicated interop specs.
- **Tier C (internal-exported API)**: exported symbols used primarily by
  framework internals/tooling, included for inventory completeness.

### API-CLASS-002: Internal Marker Semantics

Identifiers prefixed with `Internal__` (Go) or `__` (TypeScript) MUST be
classified as Tier C.

### API-CLASS-003: Promotion Rule

A Tier C API MUST NOT be treated as Tier A unless explicitly promoted in this
spec.

### API-CLASS-004: Interop Ownership Rule

Tier B APIs MUST identify the owning behavior spec (typically kit interop) when
behavior is delegated to non-Vorma packages.

## 4. Canonical Inventory Sources

### API-SOURCE-001: Go Source of Truth

Primary source of truth for top-level Go API is:

- `vorma.go`

### API-SOURCE-002: npm Source of Truth

Primary source of truth for npm subpath exports is:

- `package.json` (`exports` field)

### API-SOURCE-003: TypeScript Entry Sources

Symbol-level TypeScript export inventories are derived from:

- `vormaclient/client/index.ts`
- `vormaclient/react/index.tsx`
- `vormaclient/preact/index.tsx`
- `vormaclient/solid/index.tsx`
- `vormaclient/vite/vite.ts`

### API-SOURCE-004: Inventory Update Rule

When exported surface changes, this spec MUST be updated in the same change
set.

## 5. Go Public API (`github.com/vormadev/vorma`)

### 5.1 Namespace Contract

### API-GO-001: Primary Go Import Surface

Tier A Go API surface is the package:

- `github.com/vormadev/vorma`

### API-GO-002: Public Constructor Functions

The package MUST expose these constructor helpers:

- `NewVormaApp`
- `NewLoader`
- `NewAction`

### API-GO-003: Public Variables/Re-exports

The package MUST expose these variables:

- `MustGetPort`
- `GetIsDev`
- `SetModeToDev`
- `IsJSONRequest`
- `VormaBuildIDHeaderKey`
- `EnableThirdPartyRouter`

### API-GO-004: Public Type Aliases

The package MUST expose these type aliases:

- `Vorma`
- `HeadEls`
- `AdHocType`
- `VormaAppConfig`
- `LoadersRouter`
- `LoaderReqData`
- `ActionsRouter`
- `ActionReqData`
- `None`
- `Action`
- `Loader`
- `LoaderFunc`
- `ActionFunc`
- `LoadersRouterOptions`
- `ActionsRouterOptions`
- `FormData`
- `LoaderError`

### API-GO-005: Internal Export Marker

`Internal__GetCurrentNPMVersion` is Tier C and exists for internal/tooling use.

### API-GO-006: Alias Delegation Contract

For aliased/re-exported symbols, externally observable behavior MUST remain
compatible with underlying owner packages (for example `mux`, `wave`,
`vormaruntime`).

### API-GO-007: Router Interop Contract

`NewLoader` and `NewAction` MUST preserve registration semantics compatible with
the nested routing/task-registration model defined in interop specs.

### API-GO-008: Constructor Signature Contract

The root Go constructors MUST keep the following callable signatures:

- `NewVormaApp(o VormaAppConfig) *Vorma`
- `NewLoader[O any, CtxPtr ~*Ctx, Ctx any](app *Vorma, p string, f func(CtxPtr) (O, error), decorateCtx func(*LoaderReqData) CtxPtr) *Loader[O]`
- `NewAction[I any, O any, CtxPtr ~*Ctx, Ctx any](app *Vorma, m string, p string, f func(CtxPtr) (O, error), decorateCtx func(*mux.ReqData[I]) CtxPtr) *Action[I, O]`

`NewLoader` and `NewAction` MUST return the registered task-handler instance.

### API-GO-009: Alias Shape Contract

The following alias shapes are normative:

- `Action[I, O]` aliases `mux.TaskHandler[I, O]`.
- `Loader[O]` aliases `mux.TaskHandler[None, O]`.
- `LoaderFunc[Ctx, O]` is `func(*Ctx) (O, error)`.
- `ActionFunc[Ctx, I, O]` is `func(*Ctx) (O, error)` (input is conveyed through
  decorated context, not as a direct function argument).

### API-GO-010: Internal Tooling Export Signature

`Internal__GetCurrentNPMVersion` MUST remain zero-arg and return `string`.

### API-GO-011: `Vorma` Alias Method Surface Inventory

Because root package type `Vorma` aliases `vormaruntime.Vorma`, the following
exported methods are part of the Go surface contract and MUST remain
intentionally managed (add/remove/rename is API drift requiring spec update):

- lifecycle/router: `Init`, `InitWithDefaultRouter`, `Loaders`, `Actions`,
  `ServeStatic`,
- handler helpers: `GetLoadersHandler`, `GetActionsHandler`,
- reload helpers: `ReloadRoutesFromDisk`, `ReloadTemplateFromDisk`,
- route/build helpers: `IsCurrentBuildJSONRequest`, `GetCurrentBuildID`,
  `GetBuildID`, `GetIsDevMode`, `SetIsDev`,
- router/state accessors: `ServerAddr`, `LoadersRouter`, `ActionsRouter`,
  `GetPathsSnapshot`, `RegisterPatternIfNeeded`,
- artifact/template accessors: `GetClientEntryOut`, `GetClientEntryDeps`,
  `GetDepToCSSBundleMap`, `GetRootTemplate`, `GetRouteManifestFile`,
- TS-generation accessors: `GetAdHocTypes`, `GetExtraTSCode`,
- lock-scoped access APIs: `WithLock`, `WithRLock`.

### API-GO-012: `Loaders`/`Actions` Wrapper Method Surface

The wrapper types returned by `app.Loaders()` / `app.Actions()` MUST expose
their current callable helper surface:

- `Loaders`: `HandlerMountPattern`, `Handler`,
- `Actions`: `HandlerMountPattern`, `Handler`, `SupportedMethods`.

### API-GO-013: Method-Surface Drift Rule

If any method listed in `API-GO-011` or `API-GO-012` is removed, renamed, or
semantically re-tiered, the change MUST be treated as explicit API-surface
change and reflected in this spec and traceability artifacts.

### API-GO-014: Mutable Return Ownership Rule

For public accessors that expose collection-shaped data (maps/slices), returned
values MUST be treated as read-only snapshots from caller perspective.

Surface contract intent:

- caller mutation of returned collections MUST NOT mutate runtime-authoritative
  internal state.

This applies to accessors in `API-GO-011` and `API-GO-012` that return
collection-shaped values (for example maps/slices).

This includes both:

- direct snapshot getters (for example `GetPathsSnapshot`,
  `GetClientEntryDeps`, `GetDepToCSSBundleMap`, `GetAdHocTypes()`), and
- collection-returning methods reachable through router accessors exposed by
  `LoadersRouter()` / `ActionsRouter()` (for example promoted
  `AllRoutes()` accessors), in addition to wrapper helper accessors like
  `Actions().SupportedMethods()`.

### API-GO-015: Embedded Wave Method Surface Inventory

Because `vormaruntime.Vorma` embeds `*wave.Wave`, the following inherited
methods are part of `vorma.Vorma` callable surface and MUST be treated as
intentional API (add/remove/rename is surface drift requiring explicit spec
update):

- logging/config/runtime mode: `Logger`, `RawConfigJSON`,
  `AddFrameworkWatchPatterns`, `AddIgnoredPatterns`, `SetPublicFileMapOutDir`,
  `GetIsDev`, `MustGetPort`, `SetModeToDev`, `GetParsedConfig`,
- filesystem/public-asset helpers: `GetBaseFS`, `GetPublicFS`, `GetPrivateFS`,
  `MustGetPublicFS`, `MustGetPrivateFS`, `GetPublicFileMap`, `GetPublicURL`,
  `IsPublicAsset`, `GetServeStaticHandler`, `MustGetServeStaticHandler`,
  `FaviconRedirect`,
- path/location helpers: `GetPublicPathPrefix`, `GetDistDir`,
  `GetPublicStaticDir`, `GetPrivateStaticDir`, `GetConfigFile`,
  `GetViteManifestLocation`, `GetViteOutDir`, `GetStaticPrivateOutDir`,
  `GetStaticPublicOutDir`,
- css/filemap/refresh helpers: `GetCriticalCSS`,
  `GetCriticalCSSStyleElement`, `GetCriticalCSSStyleElementSha256Hash`,
  `GetCriticalCSSElementID`, `GetStyleSheetURL`, `GetStyleSheetLinkElement`,
  `GetStyleSheetElementID`, `GetPublicFileMapURL`, `GetPublicFileMapElements`,
  `GetPublicFileMapScriptSha256Hash`, `GetRefreshScript`,
  `GetRefreshScriptSha256Hash`.

Shadowing note:

- `vormaruntime.Vorma.ServeStatic()` shadows embedded
  `wave.Wave.ServeStatic(immutable bool)`, so the promoted callable surface on
  `vorma.Vorma` includes only the Vorma wrapper signature listed under
  `API-GO-011`.

### API-GO-016: Embedded Wave Behavioral Ownership Contract

Detailed behavior for embedded Wave method groups MUST remain compatible with
their owning behavioral specs:

- Wave runtime-serving/public-asset behavior:
  `specs/packages/wave/WAVE_RUNTIME_SERVING_SPEC.md`,
- Wave build/dev/control-plane behavior:
  `specs/packages/wave/WAVE_BUILD_DEV_CONFORMANCE_SPEC.md`,
- dependency package spec index:
  `specs/packages/PACKAGE_INDEX.md`,
- Vorma integration boundary expectations:
  `specs/packages/vorma/VORMA_KIT_INTEROP_SPEC.md`.

## 6. npm Public API (`vorma`)

### 6.1 Package-Level Export Model

### API-NPM-001: Subpath-Only Public Model

Public npm access is defined through subpath exports map, not ad-hoc deep
imports.

### API-NPM-002: No Root Import Guarantee

Because no `"."` export is declared, consumers MUST NOT rely on `import "vorma"`
as an in-scope public API contract for conformance.

### API-NPM-003: Exported Subpath Inventory

Current public subpath exports are:

- `vorma/client`
- `vorma/react`
- `vorma/solid`
- `vorma/preact`
- `vorma/vite`
- `vorma/kit/converters`
- `vorma/kit/cookies`
- `vorma/kit/csrf`
- `vorma/kit/debounce`
- `vorma/kit/fmt`
- `vorma/kit/json`
- `vorma/kit/listeners`
- `vorma/kit/matcher/register`
- `vorma/kit/matcher/find-best`
- `vorma/kit/matcher/find-nested`
- `vorma/kit/theme`
- `vorma/kit/url`

### API-NPM-004: Deep Import Restriction

Paths not declared in `exports` MUST NOT be considered supported public API.

### 6.2 `vorma/client` Symbol Inventory

### API-CLIENT-001: Tier A Runtime Functions

`vorma/client` MUST expose these Tier A runtime functions:

- `initClient`
- `vormaNavigate`
- `revalidate`
- `submit`
- `getBuildID`
- `getHistoryInstance`
- `getLocation`
- `getRootEl`
- `getStatus`
- `route`
- `buildMutationURL`
- `buildQueryURL`
- `resolveBody`
- `makeTypedNavigate`
- `defaultErrorBoundary`
- `setupGlobalLoadingIndicator`
- `revalidateOnWindowFocus`
- `addBuildIDListener`
- `addLocationListener`
- `addRouteChangeListener`
- `addStatusListener`
- `getRouterData`

### API-CLIENT-002: Tier C Internal-Exported Functions

`vorma/client` exposes these Tier C internal functions:

- `__registerClientLoaderPattern`
- `__runClientLoadersAfterHMRUpdate`
- `__getPrefetchHandlers`
- `__makeLinkOnClickFn`
- `__applyScrollState`
- `__makeFinalLinkProps`
- `__resolvePath`

### API-CLIENT-003: Tier C Internal-Exported State

`vorma/client` exposes this Tier C internal state handle:

- `__vormaClientGlobal`

### API-CLIENT-004: Public Type Exports

`vorma/client` MUST expose these public types:

- `SubmitOptions`
- `RouteChangeEvent`
- `StatusEvent`
- `VormaLinkPropsBase`
- `ParamsForPattern`
- `UseRouterDataFunction`
- `VormaRouteGeneric`
- `ExtractApp`
- `PermissivePatternBasedProps`
- `VormaAppBase`
- `VormaAppConfig`
- `VormaLoaderOutput`
- `VormaLoaderPattern`
- `VormaMutationInput`
- `VormaMutationOutput`
- `VormaMutationPattern`
- `VormaMutationProps`
- `VormaQueryInput`
- `VormaQueryOutput`
- `VormaQueryPattern`
- `VormaQueryProps`
- `VormaRoutePropsGeneric`
- `ClientLoaderAwaitedServerData`

### API-CLIENT-005: Path-Resolution Helper Contract

`__resolvePath` MUST:

- substitute dynamic params using configured dynamic rune (`actionsDynamicRune`
  for `query`/`mutation`, `loadersDynamicRune` for `loader`),
- substitute splat segment using configured splat rune (`actionsSplatRune` for
  `query`/`mutation`, `loadersSplatRune` for `loader`),
- for `loader` resolution only, strip trailing explicit index segment
  `/<loadersExplicitIndexSegment>` (falling back to `/` when stripping empties
  path),
- return unresolved segments unchanged when no replacement input is supplied.

### API-CLIENT-006: URL Builder Helper Contract

`buildQueryURL` and `buildMutationURL` MUST:

- prefix resolved path with `actionsRouterMountRoot` after trailing-slash strip,
- resolve against current document origin,
- return absolute `URL` objects.

`buildQueryURL` query-input rule:

- when `props.input` is present, URL search MUST be set from
  `serializeToSearchParams(props.input)`.

### API-CLIENT-007: Request Body Helper Contract

`resolveBody` MUST pass through input unchanged for:

- `null`/`undefined`,
- `string`,
- `Blob`,
- `FormData`,
- `URLSearchParams`,
- `ReadableStream`,
- `ArrayBuffer`,
- `ArrayBuffer` views.

For all other non-null objects, `resolveBody` MUST return `JSON.stringify(input)`.

### API-CLIENT-008: Typed Navigate Helper Contract

`makeTypedNavigate` MUST:

- resolve loader target path through loader-pattern resolver using `pattern`,
  `params`, and `splatValues`,
- delegate navigation to `vormaNavigate` with `replace`, `scrollToTop`,
  `search`, `hash`, and `state` forwarded unchanged,
- return/await `vormaNavigate` promise (`Promise<void>` contract).

### 6.3 UI Adapter Subpaths

### API-UI-001: Shared Adapter Export Parity

`vorma/react`, `vorma/preact`, and `vorma/solid` MUST each export:

- `makeTypedAddClientLoader`
- `makeTypedUseLoaderData`
- `makeTypedUsePatternLoaderData`
- `makeTypedUseRouterData`
- `VormaLink`
- `makeTypedLink`
- `VormaRootOutlet`
- `VormaRoute` (type)
- `VormaRouteProps` (type)

### API-UI-002: Location API Variant

The location hook/signal export is framework-specific:

- `vorma/react`: `useLocation`
- `vorma/preact`: `location`
- `vorma/solid`: `location`

### API-UI-003: Typed Data Helper Reactive Shape Contract

Typed helper factories MUST preserve framework-native data access shape:

- `makeTypedUseRouterData`
- `makeTypedUseLoaderData`
- `makeTypedUsePatternLoaderData`

Shape requirements:

- React/Preact helpers MUST return direct values (hook-read value shape).
- Solid helpers MUST return accessors (`() => value`) for reactive reads.

### API-UI-004: `makeTypedAddClientLoader` Registration and Access Contract

`makeTypedAddClientLoader` MUST:

- register provided pattern through client pattern-registration API on add,
- install loader function for that pattern into runtime client-loader map,
- support optional dev HMR rerun registration when `reRunOnModuleChange` is
  provided,
- keep registration failures non-fatal to caller (error logging is optional),
- invoke loader callbacks with `params`, `splatValues`, `signal`, and
  `serverDataPromise` payload contract compatible with
  `specs/packages/vorma/VORMA_FRONTEND_RUNTIME_SPEC.md`
  (`FE-CL-003`, `FE-CL-010`, `FE-CL-011`),
- return accessor overloads:
  - with route props: read by route index,
  - without route props: read by matched pattern and return `undefined` when
    pattern is not currently matched.

### API-UI-005: Typed Link URL Composition Contract

`makeTypedLink` MUST:

- resolve destination path via loader-pattern resolver (`pattern` + `params` +
  `splatValues`),
- apply optional `search` and `hash` overrides onto resolved URL,
- emit final `href` as absolute URL based on current document origin,
- preserve link `state` as client-navigation state payload.

### API-UI-006: Typed Link Default Prop Merge Contract

When `makeTypedLink` is created with `defaultProps` and link is invoked with
per-call props, final link props MUST be shallow-merged with per-call props
taking precedence over defaults.

### API-UI-007: `VormaLink` Anchor Wiring Contract

`VormaLink` render contract MUST:

- derive final behavior from shared link-props helper,
- expose helper-produced external marker through `data-external` attribute,
- wire helper-provided handlers for `onPointerEnter`, `onFocus`,
  `onPointerLeave`, `onBlur`, `onTouchCancel`, and `onClick`,
- avoid forwarding Vorma-only control props (`prefetch`, `scrollToTop`,
  `replace`, `state`) as DOM attributes.

### API-UI-008: `VormaLink` Prefetch Mode Gate Contract

`VormaLink` prefetch behavior MUST be explicit-mode gated:

- `prefetch="intent"` MUST enable helper prefetch lifecycle wiring,
- absent/non-`"intent"` prefetch value MUST disable prefetch lifecycle and
  retain click-only navigation wiring.

### API-UI-009: Cross-Adapter Alignment Default Rule

For `vorma/react`, `vorma/preact`, and `vorma/solid`, public API shape and
observable behavior MUST align by default.

Allowed divergence rule:

- differences are allowed only where lower-level framework component/reactivity
  models require adapter-specific behavior,
- each allowed divergence MUST be explicitly documented in this spec and/or
  `specs/packages/vorma/VORMA_FRONTEND_RUNTIME_SPEC.md`,
- silent adapter drift MUST NOT be treated as acceptable.

### 6.4 `vorma/vite` Surface

### API-VITE-001: Default Export Contract

`vorma/vite` MUST export a default plugin factory function.

### API-VITE-002: Plugin Config Shape Contract

The plugin factory config contract MUST include:

- `rollupInput`
- `publicPathPrefix`
- `staticPublicAssetMap`
- `buildtimePublicURLFuncName`
- `filemapJSONPath`
- `ignoredPatterns`
- `dedupeList`

### API-VITE-003: Dev Invalidation Endpoint Contract

Plugin behavior MUST include support for the dev invalidation endpoint
`/__vorma_invalidate_filemap` (runtime behavior specified in build/dev specs).

### 6.5 `vorma/kit/*` Subpath Contract

### API-KIT-001: Kit Subpath Exposure Scope

`vorma/kit/*` subpaths are Tier B interop surfaces: path-level compatibility is
public, while detailed behavior is defined by kit package contracts.

### API-KIT-002: Behavioral Ownership

Behavioral changes in kit-exported subpaths MUST be assessed for Vorma impact
using the kit interop spec and reflected in affected Vorma behavior specs.

## 7. API Conformance Checks

### API-TEST-001: Go Compile-Time Surface Check

Conformance suite MUST include compile-time checks that import
`github.com/vormadev/vorma` and reference Tier A symbols.

### API-TEST-002: npm Subpath Resolution Check

Conformance suite MUST verify each documented subpath resolves from package
artifacts under test.

### API-TEST-003: Type Export Presence Check

Type-level smoke tests MUST verify key TypeScript exports for `vorma/client`
and UI adapters remain available.

### API-TEST-004: API Diff Gate

API-surface validation MUST include diff checks between baseline and candidate
for Tier A and Tier B surfaces.

### API-TEST-005: UI Adapter Parity Gate

API-surface validation MUST include adapter parity checks for
`vorma/react`, `vorma/preact`, and `vorma/solid` so shared exports and behavior
contracts stay aligned except for explicitly documented adapter-specific
differences.

## 8. Relation to Other Specs

- Checklist: `specs/packages/vorma/VORMA_SPEC_CHECKLIST.md`
- Backend runtime:
  `specs/packages/vorma/VORMA_BACKEND_RUNTIME_SPEC.md`
- Frontend runtime:
  `specs/packages/vorma/VORMA_FRONTEND_RUNTIME_SPEC.md`
- Build/dev:
  `specs/packages/vorma/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Wire contract:
  `specs/packages/vorma/VORMA_WIRE_CONTRACT_SPEC.md`
- Kit interop:
  `specs/packages/vorma/VORMA_KIT_INTEROP_SPEC.md`
- Spec process:
  `specs/packages/vorma/VORMA_SPEC_PROCESS_RFC_SPEC.md`
