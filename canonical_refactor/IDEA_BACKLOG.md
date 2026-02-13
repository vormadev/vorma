# Public API Simplification Audit

Audit baseline:

- branch baseline for comparison: `main` at `0a94922d`
- active branch: `refactor-2026-7`
- UX/DX guardrail baseline: `4dc9191a`

Reviewed surfaces in this pass:

- Go public entrypoints: `vorma`, `vormabuild`, `vormaruntime`
- npm exports: `package.json` subpath exports
- client runtime/buildtime exports: `vormaclient/client/*`
- adapter exports: `vormaclient/react`, `vormaclient/preact`,
  `vormaclient/solid`
- bootstrap API and generated template shape: `bootstrap/` templates

## Findings

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
  need wrapper boilerplate, if possible with the rules of generics in Go (Go
  does not have generic methods).
- Bootstrap API helper generation (`frontend/src/vorma.app.tsx`) uses a mutable
  global request-init resolver. Current result: request behavior can be mutated
  globally after import. Simplification direction: move to an explicit immutable
  API-client creation path while keeping one-line default usage.
- Public Go surface still exposes high-churn lower-level packages directly
  (`vormaruntime`, `vormabuild`) while the top-level `vorma` package already
  acts as a facade. Current result: users can bind to unstable internals by
  importing deep packages. Simplification direction: harden recommended surface
  around stable entry packages and clearly partition “unstable for framework
  internals.”

## Candidate API Simplification Plan

- Split client exports into a stable app surface (`vorma/client`) and an
  unstable adapter/internal surface (`vorma/client-internal`).
- Replace bootstrap mutable request-init global with explicit API-client
  construction plus a concise default singleton.
- Design and land a no-side-effect route registration model that keeps the
  one-backend-file and two-file full-stack authoring contract, using bulletproof
  AST parsing for registration calls.

## Validation Requirements For Any Accepted Simplification

- Must preserve one-backend-file route addition ergonomics.
- Must preserve backend+frontend two-file full-stack route addition ergonomics.
- Must not require extra backend registration files.
- Must improve or preserve generated-app readability.
- Must update `BREAKING_CHANGES_LEDGER.md` if public behavior changes.

## More ideas:

```json
[
	{
		"id": "BRK-009",
		"area": "Client export surface",
		"futureBreakingChange": "Split stable app exports from internal/adapter exports so app code cannot depend on __* internals by default",
		"status": "not-yet-accepted",
		"notes": "See API_SIMPLIFICATION_AUDIT.md"
	},
	{
		"id": "BRK-010",
		"area": "Route authoring internals",
		"futureBreakingChange": "Remove side-effect registration dependency while preserving one-backend-file and two-file full-stack authoring ergonomics",
		"status": "not-yet-accepted",
		"notes": "See LOCKED_DECISIONS_AND_ARCHITECTURE.md"
	},
	{
		"id": "BRK-011",
		"area": "Loader index authoring",
		"futureBreakingChange": "Remove explicit index-segment authoring (/_index) from default app-facing route patterns",
		"status": "not-yet-accepted",
		"notes": "Pending explicit acceptance"
	},
	{
		"id": "BRK-013",
		"area": "Bootstrap API client helper",
		"futureBreakingChange": "Replace global mutable setAPIRequestInitResolver(...) path with explicit immutable API-client creation",
		"status": "not-yet-accepted",
		"notes": "Must keep concise default usage"
	}
]
```

# Idea Backlog

Purpose: hold candidate ideas and zany experiments without polluting active
refactor tracks.

## Backlog Rules

- This file is not the active tracks document.
- Active tracks live in `ACTIVE_EXECUTION_PLAN.md`.
- Keep entries as candidates until explicitly promoted or rejected.

## Active Candidates

```json
[
	{
		"id": "P03",
		"proposal": "Strict frontend route DSL + strict manifest generation",
		"direction": "keep",
		"acceptanceGate": "Unresolved/dynamic frontend route definitions fail fast with clear diagnostics"
	}
]
```

## Parked For Revisit

```json
[
	{
		"id": "P06",
		"proposal": "Typed immutable context extension pipeline",
		"revisitCondition": "Revisit only if simpler than current typed context decorators"
	},
	{
		"id": "P08",
		"proposal": "Explicit app client instance (createAppClient)",
		"revisitCondition": "Revisit only if default usage is at least as simple as global helper path"
	},
	{
		"id": "P10",
		"proposal": "Remove all implicit singleton behavior",
		"revisitCondition": "Revisit only with zero regression to default-path ergonomics"
	},
	{
		"id": "P12",
		"proposal": "Explicit transport/serialization codec contracts",
		"revisitCondition": "Revisit with concrete user demand and low-ceremony shape"
	},
	{
		"id": "P13",
		"proposal": "Stable plugin API as kernel boundary",
		"revisitCondition": "Revisit after core boundaries settle"
	}
]
```

## Rejected Unless Reopened

- Route IDs as primary default authoring identity.
- Unified default primitive for loader/query/mutation.
- Any non-JSON app-authored config path.
- Default-required module/feature composition manifest.
- Helper-name-only backend route discovery.
- Any default model requiring extra backend registration file edits per route.
- Framework-enforced application package-organization model.

## Zany Ideas Inbox

- Keep experimental ideas here until they have concrete constraints and
  acceptance gates.
- Promote ideas only if they preserve the UX contract in
  `RULES_AND_GUARDRAILS.md`.
