# Public API Simplification Audit

Audit baseline:

- branch baseline for comparison: `main` at `0a94922d`
- active branch: `refactor-2026-7`
- UX/DX guardrail baseline: `4dc9191a`

Reviewed surfaces in this pass:

- Go public entrypoints: `vorma`, `wave`, `wave/tooling`, `vormabuild`,
  `vormaruntime`
- npm exports: `package.json` subpath exports
- client runtime/buildtime exports: `vormaclient/client/*`
- adapter exports: `vormaclient/react`, `vormaclient/preact`,
  `vormaclient/solid`
- bootstrap API and generated template shape: `bootstrap/` templates

## Findings

- `vorma/client` still exports many internals (`__*` symbols and mutable global
  handles) intended primarily for adapter wiring, not app code. Current result:
  app authors can accidentally depend on unstable internals. Simplification
  direction: split stable app surface and unstable adapter surface into distinct
  exports.
- Bootstrap route registration relies on package-init side effects
  (`var _ = NewLoader(...)` and `var _ = NewAction(...)`). Current result:
  registration correctness remains tied to import-time side effects, which
  conflicts with the long-term “no side-effect dependency” direction.
  Simplification direction: route discovery and registration should be explicit
  in framework internals while preserving one-backend-file authoring.
- Bootstrap still requires a context-wrapper layer (`context.go`) just to
  decorate loader/action request contexts. Current result: generated ceremony
  exists to bridge API shape mismatch. Simplification direction: introduce
  first-class context-decoration plumbing in public Go APIs so app code does not
  need wrapper boilerplate.
- Loader pattern ergonomics still expose explicit index-segment semantics
  (`"/_index"` and `loadersExplicitIndexSegment`) in generated app code and type
  helpers. Current result: authored patterns carry framework-internal index
  details. Simplification direction: make index behavior implicit in authoring
  and keep explicit segment handling internal.
- Bootstrap API helper generation (`frontend/src/vorma.app.tsx`) uses a mutable
  global request-init resolver. Current result: request behavior can be mutated
  globally after import. Simplification direction: move to an explicit immutable
  API-client creation path while keeping one-line default usage.
- Wave configuration contract remains JSON-only at the app-facing boundary. Do
  not introduce app-facing config-as-code inputs or provider payload plumbing.
- Public Go surface still exposes high-churn lower-level packages directly
  (`vormaruntime`, `vormabuild`) while the top-level `vorma` package already
  acts as a facade. Current result: users can bind to unstable internals by
  importing deep packages. Simplification direction: harden recommended surface
  around stable entry packages and clearly partition “unstable for framework
  internals.”

## Candidate API Simplification Plan

- Split client exports into a stable app surface (`vorma/client`) and an
  unstable adapter/internal surface (`vorma/client-internal`).
- Remove index-segment authoring requirements from generated apps while keeping
  deterministic internal matching.
- Replace bootstrap mutable request-init global with explicit API-client
  construction plus a concise default singleton.
- Keep JSON-only authored config contract enforced in future Wave API work; do
  not reintroduce app-facing config-as-code alternatives.
- Design and land a no-side-effect route registration model that keeps the
  one-backend-file and two-file full-stack authoring contract.

## Validation Requirements For Any Accepted Simplification

- Must preserve one-backend-file route addition ergonomics.
- Must preserve backend+frontend two-file full-stack route addition ergonomics.
- Must not require extra backend registration files.
- Must improve or preserve generated-app readability.
- Must update `BREAKING_CHANGES_LEDGER.md` if public behavior changes.
