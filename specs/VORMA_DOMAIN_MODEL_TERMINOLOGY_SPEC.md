# Vorma Domain Model and Terminology Specification

Status: Draft  
Last Updated: 2026-02-07  
Applies To: Shared vocabulary and conceptual model across Vorma runtime/build/frontend specs

## 1. Why This Spec Exists

This spec defines the canonical domain language for Vorma.

It exists to:

- keep all other specs aligned on term meaning,
- reduce ambiguity during large refactors,
- provide stable conceptual boundaries independent of implementation details.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Role of This Spec

This is a vocabulary/model spec. It does not replace feature-level
requirements from backend/wire/build/frontend specs.

### 2.3 Canonical Language Rule

Terms defined here are canonical and SHOULD be used consistently in all Vorma
specs, tests, and release notes.

## 3. Core Domain Model

## 3.1 Product-Layer Model

### TERM-CORE-001: Wave vs Vorma Layer Separation

- **Wave**: Go-centric build/dev toolkit (watching, static asset processing,
  process orchestration, reload transport).
- **Vorma**: full-stack framework built on Wave, adding route model, typed
  data loading/actions, SSR bootstrap, and client runtime contracts.

### TERM-CORE-002: Vorma App

A **Vorma App** is a configured framework instance with:

- Wave integration,
- Vorma config block,
- loaders router,
- actions router,
- generated build artifacts,
- optional UI adapter bindings.

### TERM-CORE-003: App Configuration Domains

Vorma configuration is conceptually split into:

- runtime config (router behavior, template path, action mount root, etc.),
- build config (entries, route definition file, TS output dir, UI variant),
- Wave/tooling config (watch patterns, hooks, static dirs).

## 3.2 Process and Lifecycle Model

### TERM-PROC-001: Process A / Process B Model

In development:

- **Process A**: dev orchestration process (watch/build coordination),
- **Process B**: running application server process.

This distinction is normative for understanding fast-reload and callback flows.

### TERM-PROC-002: Build Hook Invocation

A **build hook invocation** is the framework-specific subprocess execution that
runs Vorma build-inner logic during Wave-driven build lifecycle.

### TERM-PROC-003: Full Rebuild vs Fast Route Rebuild

- **Full Rebuild**: normal rebuild path including broad build pipeline stages.
- **Fast Route Rebuild**: dev-only optimized path for route-definition changes
  where artifact regeneration can occur without full app recompilation.

### TERM-PROC-004: Reload Endpoint Call

A **reload endpoint call** is a Process A -> Process B HTTP callback used to
trigger in-process state/template reload from already-written artifacts.

## 3.3 Routing and Handler Model

### TERM-ROUTE-001: Loaders Router

The **loaders router** is nested-route oriented and resolves route data for
page/document and JSON route-data flows.

### TERM-ROUTE-002: Actions Router

The **actions router** is method-aware request handling surface for query and
mutation operations under configured mount root.

### TERM-ROUTE-003: Route Pattern

A **route pattern** is the canonical string key used to register and match
route handlers/modules (including dynamic/splat/index segment semantics).

### TERM-ROUTE-004: Matched Pattern Chain

A **matched pattern chain** is ordered outermost->innermost route pattern list
for a resolved request/navigation.

### TERM-ROUTE-005: Params and Splat Values

- **Params**: named dynamic-segment bindings for current match.
- **Splat values**: trailing wildcard segment values (ordered list).

## 3.4 Data Loading and Mutation Model

### TERM-DATA-001: Loader

A **loader** is a route-associated data function evaluated in nested-route
resolution context.

### TERM-DATA-002: Action

An **action** is a method-bound request handler for query/mutation-like
operations under actions router.

### TERM-DATA-003: Route Data

**Route data** is the structured payload representing resolved routing/data/
module metadata for a navigation target.

### TERM-DATA-004: JSON Route-Data Mode

A request in **JSON route-data mode** is a loaders request negotiated via
`vorma_json` query key for structured JSON response instead of HTML document.

### TERM-DATA-005: Current vs Stale Build JSON Request

- **Current build JSON request**: `vorma_json` token equals current server
  build id.
- **Stale build JSON request**: token differs, requiring reload signal semantics.

## 3.5 SSR and HTML Model

### TERM-SSR-001: Root Template

The **root template** is server-side HTML skeleton receiving Vorma-specific
template data keys for head/bootstrap/body scripts.

### TERM-SSR-002: SSR Bootstrap Script

The **SSR bootstrap script** initializes client global state under the Vorma
symbol namespace and carries route/build context.

### TERM-SSR-003: Head Element Blocks

Head output is logically partitioned into:

- title,
- meta block,
- rest block,

with explicit markers for client reconciliation boundaries.

## 3.6 Client Runtime Model

### TERM-FE-001: Client Global

The **client global** is runtime state object at:

`globalThis[Symbol.for("__vorma_internal__")]`.

### TERM-FE-002: Navigation Entry

A **navigation entry** is an in-flight navigation control object with
navigation type, intent, phase, target/origin URLs, and abort/promise state.

### TERM-FE-003: Navigation Types

Canonical navigation types:

- `userNavigation`
- `browserHistory`
- `revalidation`
- `redirect`
- `prefetch`
- `action`

### TERM-FE-004: Navigation Intent

Canonical intents:

- `none` (prefetch-only),
- `navigate` (commit URL/render),
- `revalidate` (refresh-if-still-current context).

### TERM-FE-005: Status State

Client status state is triad:

- `isNavigating`
- `isSubmitting`
- `isRevalidating`.

### TERM-FE-006: Route Change Commit

A **route change commit** is the point where route-data update has been applied
and `vorma:route-change` is dispatched.

## 3.7 Artifact and Build Metadata Model

### TERM-ART-001: Stage-1 Paths File

**Stage-1 paths file**: pre-Vite route/build artifact metadata snapshot.

### TERM-ART-002: Stage-2 Paths File

**Stage-2 paths file**: post-Vite metadata including output/dependency
resolution for runtime consumption.

### TERM-ART-003: Route Manifest

**Route manifest**: public JSON map pattern -> server-loader presence flag,
used for client skip-optimization decisions.

### TERM-ART-004: Build ID

**Build ID**: version token representing runtime/build artifact freshness.

## 3.8 Wire/Protocol Model

### TERM-WIRE-001: Build ID Header

`X-Vorma-Build-Id` is canonical response header conveying server build id.

### TERM-WIRE-002: Reload Header

`X-Vorma-Reload` is canonical stale-build reload signal header.

### TERM-WIRE-003: Client Redirect Header

`X-Client-Redirect` is canonical server-to-client redirect intent header.

### TERM-WIRE-004: Redirect Handshake Header

`X-Accepts-Client-Redirect` is canonical client capability signal for redirect
protocol handling.

## 3.9 Error and Safety Model

### TERM-ERR-001: LoaderError

**LoaderError** is typed server/client error split model with client-safe
message and server diagnostic error.

### TERM-ERR-002: Outermost Error Index

**Outermost error index** is the first failing index in matched route chain and
is used for truncation and boundary rendering.

### TERM-ERR-003: Effective Error

**Effective error** in frontend is selected outermost between server-side and
client-loader errors.

## 4. Naming and Documentation Rules

### TERM-DOC-001: Use Canonical Terms in Specs

New/updated Vorma specs SHOULD prefer canonical terms from this document over
ad-hoc aliases.

### TERM-DOC-002: First-Use Expansion Rule

When using abbreviated jargon (for example SSR, HMR, TTI), docs SHOULD provide
first-use expansion in section scope.

### TERM-DOC-003: Conflict Resolution Rule

If another spec uses conflicting term definitions, this spec is authoritative
unless explicitly superseded by approved update.

## 5. Conformance Guidance

Conformance checks for terminology alignment SHOULD include:

1. requirement ID references using canonical term labels,
2. release note wording consistency for routing/build/runtime concepts,
3. test naming consistency with canonical navigation/build/error terms.

## 6. Relation to Other Specs

- Backend runtime: `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- Wire contract: `/Users/sjc/__code/river/specs/VORMA_WIRE_CONTRACT_SPEC.md`
- Build/dev: `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Frontend runtime: `/Users/sjc/__code/river/specs/VORMA_FRONTEND_RUNTIME_SPEC.md`
- Checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`
