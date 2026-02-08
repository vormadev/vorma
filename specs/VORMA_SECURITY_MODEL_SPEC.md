# Vorma Security Model Specification

Status: Draft  
Last Updated: 2026-02-07  
Applies To: Vorma backend/runtime, wire protocol, build artifacts, and frontend runtime security boundaries

## 1. Why This Spec Exists

This document defines security contracts that Vorma MUST preserve during refactors.

It exists to:

- make security-relevant behavior explicit at contract level,
- prevent accidental widening of trust boundaries,
- support black-box security conformance testing.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 What Is In Scope

This spec covers Vorma-visible security behavior for:

- HTML/SSR bootstrap boundaries,
- head element and dangerous HTML channels,
- request input parsing and validation edges,
- redirect safety and navigation hardening,
- client/server error disclosure boundaries,
- build artifact integrity signals and stale-build handling.

### 2.3 What Is Out of Scope

Out of scope:

- full standalone specs for kit packages,
- application-specific authentication/authorization policy,
- infrastructure/network perimeter controls.

## 3. Security Model Assumptions

### SEC-ASSUME-001: Application Controls Business Auth

Vorma does not define business authorization policy; applications are responsible for route access control and identity enforcement.

### SEC-ASSUME-002: Trusted Build Inputs Requirement

Vorma security guarantees assume trusted source/build inputs. Compromised source/templates/assets are out-of-scope for runtime hardening guarantees.

### SEC-ASSUME-003: Browser Platform Trust

Client-side guarantees assume standard browser security model semantics for same-origin policy, history API, and DOM event behavior.

## 4. Requirement Catalog

## 4.1 Trust Boundaries and Data Classes

### SEC-TRUST-001: Explicit Dangerous HTML Channels

Vorma-visible dangerous HTML channels are limited to explicit fields:

- `dangerousInnerHTML` in head/title/script element payloads,
- trusted template output channels (`template.HTML`) under server control.

Security-sensitive refactors MUST NOT silently introduce new implicit raw-HTML channels.

### SEC-TRUST-002: Safe Attribute Channel Separation

Element serialization channels MUST remain distinct:

- escaped attribute/value channel (`attributes`),
- explicitly trusted raw-value channel (`attributesKnownSafe`).

### SEC-TRUST-003: Explicit Trust Escalation

Moving data from escaped channel to trusted raw channel MUST be treated as explicit trust escalation and SHOULD require code review scrutiny.

### SEC-TRUST-004: Stable Global Symbol Namespace

Client bootstrap state MUST be scoped to `globalThis[Symbol.for("__vorma_internal__")]` and MUST NOT be copied into additional global namespaces by default.

## 4.2 SSR Bootstrap and CSP-Related Contracts

### SEC-SSR-001: CSP Hash Signal Availability

Given SSR bootstrap script is generated  
When server prepares root template data  
Then `VormaSSRScriptSha256Hash` MUST be available to template layer.

### SEC-SSR-002: CSP Policy Application Responsibility

Vorma runtime provides CSP hash material but does not, by itself, guarantee enforcement of a specific CSP header policy; host application/infrastructure remains responsible for final CSP policy emission.

### SEC-SSR-003: Bootstrap Serialization Safety

Given server renders bootstrap script values  
When values are embedded into script payload  
Then value serialization MUST preserve syntactic safety for JSON-compatible payloads and avoid implicit string concatenation of untrusted raw HTML.

### SEC-SSR-004: Route Manifest URL Scoping

Given bootstrap includes `routeManifestURL`  
When generated  
Then URL MUST resolve under Vorma public path prefix (same deployment scope), not arbitrary external origins by default.

### SEC-SSR-005: Root Template Error Handling

Given template data generation or template execution fails  
When serving request  
Then runtime MUST fail with server error response and MUST NOT emit partially rendered insecure document fragments.

## 4.3 Head Element and HTML Safety Contracts

### SEC-HEAD-001: Null/Undefined Known-Safe Attribute Rejection

Given client head reconciliation receives `attributesKnownSafe` containing null/undefined value  
When applying head updates  
Then runtime MUST fail loudly instead of silently producing malformed attributes.

### SEC-HEAD-002: Marker-Scoped Head Mutation

Given head updates run on client  
When mutating DOM  
Then modifications MUST be scoped to Vorma marker ranges (`meta`/`rest` blocks), leaving unrelated head nodes intact.

### SEC-HEAD-003: Head Dedupe and Reconciliation

Given repeated head updates  
When applying new block sets  
Then runtime MUST dedupe and reorder deterministically to prevent unbounded duplicate injection.

### SEC-HEAD-004: Missing Marker Safe No-Op

Given expected marker comments are missing  
When head update executes  
Then runtime MUST no-op rather than mutating arbitrary document.head regions.

### SEC-HEAD-005: Trusted Raw HTML Is Explicit

Given head/title payload contains `dangerousInnerHTML`  
When rendered  
Then runtime MUST treat it as explicitly trusted content boundary (no implicit escaping guarantees at that boundary).

### SEC-HEAD-006: Title Decode Behavior

Given title uses `dangerousInnerHTML` payload  
When applied client-side  
Then runtime MUST decode entities via controlled DOM decoding flow before assignment to `document.title`.

## 4.4 Error Disclosure and Confidentiality

### SEC-ERR-001: LoaderError Client/Server Split

Given loader returns typed `LoaderError{Client, Server}`  
When response is produced  
Then client-visible error MUST use `Client` message and server logs MUST use `Server` error.

### SEC-ERR-002: Generic Error Message for Untyped Errors

Given loader returns non-typed error  
When response is produced  
Then client-visible message MUST remain generic and MUST NOT expose raw server error text.

### SEC-ERR-003: Internal Error Logging

Given loader/action/runtime internal errors occur  
When handled  
Then server SHOULD log diagnostic details for operators while preserving client disclosure constraints.

### SEC-ERR-004: Error Cutoff Boundaries

Given outermost server error occurs in nested loader chain  
When route-data is emitted  
Then payload MUST be truncated at outermost failing index to avoid leaking deeper data from unsafe partial execution context.

### SEC-ERR-005: JSON/HTML Failure Safety

Given JSON marshalling or HTML rendering fails  
When serving request  
Then runtime MUST emit internal error response and MUST NOT emit partially formed success payloads.

## 4.5 Redirect and Navigation Security

### SEC-REDIR-001: Client Redirect Handshake Opt-In

Given client sends navigation/action fetches  
When redirect semantics are needed  
Then request MUST include `X-Accepts-Client-Redirect: 1` to indicate support for client-redirect protocol.

### SEC-REDIR-002: Redirect Signal Precedence

Given multiple redirect indicators are present  
When client resolves redirect behavior  
Then precedence MUST remain:

1. `X-Vorma-Reload`,
2. native fetch `response.redirected`,
3. `X-Client-Redirect`.

### SEC-REDIR-003: Non-HTTP Redirect Rejection

Given redirect target is non-HTTP URL scheme  
When client evaluates redirect  
Then redirect MUST be ignored.

### SEC-REDIR-004: Internal vs External Redirect Strategy

Given redirect target is internal HTTP URL  
When redirect is followed  
Then default strategy MUST be soft client navigation unless explicitly forced hard reload.

Given redirect target is external HTTP URL  
When redirect is followed  
Then strategy MUST be hard browser redirect.

### SEC-REDIR-005: Forced Reload Build Binding

Given stale-build or forced reload redirect (`X-Vorma-Reload`)  
When client builds redirect URL for internal target  
Then query key `vorma_reload` SHOULD carry latest build identifier.

### SEC-REDIR-006: Redirect Loop Bound

Given redirect chain is cyclic or excessive  
When redirect count reaches configured max threshold  
Then client MUST stop redirect recursion and surface failure signal.

### SEC-REDIR-007: Redirect Cleanup Safety

Given redirect effectuation begins  
When navigation state transitions occur  
Then active redirect/revalidation entries MUST be cleaned up to avoid stale loading state or inconsistent navigation state.

## 4.6 Input Parsing and Validation Security

### SEC-INPUT-001: GET Input Source Contract

Given action method is GET  
When input parsing occurs  
Then input MUST be sourced from URL search params via validation layer.

### SEC-INPUT-002: Non-GET JSON Parsing Contract

Given action method is supported non-GET and content type is not form content type  
When input parsing occurs  
Then JSON body parsing/validation MUST be applied.

### SEC-INPUT-003: Form Content-Type Handling

Given content type is `application/x-www-form-urlencoded` or `multipart/form-data`  
When action parser runs  
Then parser MUST NOT blindly JSON-decode body.

### SEC-INPUT-004: Unsupported Method Rejection

Given request method is outside supported action method set  
When parser executes  
Then request MUST be rejected.

### SEC-INPUT-005: Validation Error Mapping

Given validation/parsing fails for action input  
When response is emitted  
Then HTTP 400 contract MUST be preserved.

### SEC-INPUT-006: No Silent Input Coercion Expansion

Security-sensitive refactors MUST NOT silently broaden accepted input shapes/methods beyond documented parser contracts without spec update.

## 4.7 Build ID, Staleness, and Freshness Safety

### SEC-FRESH-001: Build ID Header on Runtime Responses

Given loaders/actions responses are emitted  
When client consumes them  
Then `X-Vorma-Build-Id` MUST be present to support freshness checks.

### SEC-FRESH-002: Stale JSON Reload Signal

Given loaders JSON request build token is stale  
When backend responds  
Then backend MUST emit `X-Vorma-Reload` and MUST NOT return normal route-data payload for stale token.

### SEC-FRESH-003: Client Build ID Update Event

Given response build id differs from client global build id  
When processed  
Then client MUST update build id and dispatch build-id event.

### SEC-FRESH-004: Revalidation Deployment Tag Propagation

Given deployment skew protection ID is present in client global  
When revalidation request is made  
Then deployment ID MUST be propagated (`dpl` query) to support deployment-consistency checks.

### SEC-FRESH-005: Hard Reload Token Cleanup

Given initialization sees `vorma_reload` query param  
When client starts  
Then param MUST be removed from visible URL to reduce long-lived leakage of transient control token.

## 4.8 Asset and Build Artifact Integrity Signals

### SEC-ASSET-001: Build Artifact Name Hardening

Given Vorma build artifacts are emitted  
When naming route manifests/Vite outputs  
Then Vorma-specific filename prefixes MUST remain deterministic and collision-resistant at namespace level.

### SEC-ASSET-002: Build ID Determinism Inputs

Given production build ID is computed  
When hash inputs are assembled  
Then inputs MUST include template content hash, stage-two path metadata hash, and public output summary hash.

### SEC-ASSET-003: Module/CSS Dedup on Client

Given repeated dependency/bundle references  
When preloading/applying assets  
Then client MUST dedupe to avoid unbounded repeated injection.

### SEC-ASSET-004: Dev Invalidation Endpoint Scope

Given dev invalidation endpoint is called  
When filemap invalidation occurs  
Then behavior MUST stay scoped to dev tooling semantics and MUST NOT become a production mutation channel.

## 4.9 Dependency Interop Security Constraints

### SEC-INTEROP-001: Security-Relevant Kit Behaviors Are Normative at Boundary

Where Vorma inherits security-relevant behavior from kit dependencies (`response`, `validate`, `matcher`, `headels`), Vorma-visible boundary behavior MUST remain compatible with Vorma conformance specs.

### SEC-INTEROP-002: Upstream Behavior Drift Review Requirement

Given upstream kit behavior changes in security-sensitive areas  
When integrating update  
Then Vorma security and conformance specs MUST be reviewed and updated if boundary behavior changes.

## 4.10 Operational Hardening Recommendations (Non-Blocking Guidance)

### SEC-OPS-001: CSP Header Enforcement Recommendation

Deployments SHOULD enforce a CSP policy that incorporates `VormaSSRScriptSha256Hash` for SSR bootstrap script allowances.

### SEC-OPS-002: Secure Headers Recommendation

Deployments SHOULD apply secure HTTP header middleware (for example HSTS, frame protections, content-type sniff protections) at app/infrastructure layer.

### SEC-OPS-003: Redirect Allowlist Recommendation

Applications SHOULD constrain business-level redirect targets via explicit allowlist policy where user-controlled redirect inputs exist.

### SEC-OPS-004: Observability for Security Events

Applications SHOULD instrument alerting/metrics for repeated redirect-limit hits, validation-failure spikes, and reload endpoint failures.

## 5. Conformance Test Guidance

Security conformance tests SHOULD include:

1. loader error disclosure split tests (`LoaderError` vs generic errors),
2. redirect precedence and non-HTTP redirect rejection tests,
3. action parsing/validation boundary tests (GET, JSON, form, unsupported methods),
4. stale-build reload signaling tests,
5. head-element boundary tests for marker scoping, dedupe, and known-safe attribute validation failures,
6. SSR hash signal presence tests (`VormaSSRScriptSha256Hash` in template data).

## 6. Relation to Other Specs

- Backend runtime contracts: `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- Wire protocol contracts: `/Users/sjc/__code/river/specs/VORMA_WIRE_CONTRACT_SPEC.md`
- Build/dev contracts: `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Frontend runtime contracts: `/Users/sjc/__code/river/specs/VORMA_FRONTEND_RUNTIME_SPEC.md`
- Kit interop map: `/Users/sjc/__code/river/specs/VORMA_KIT_INTEROP_SPEC.md`
- Testing strategy: `/Users/sjc/__code/river/specs/VORMA_TESTING_STRATEGY_SPEC.md`
- Roadmap checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`
