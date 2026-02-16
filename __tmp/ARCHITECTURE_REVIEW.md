# Vorma/Wave Architecture Proposals

This document is intentionally de-duplicated and proposal-only.

## Proposal 1: Curate Vorma Public API Surface

Problem: `vorma.Vorma` is a type alias to internal runtime state
(`vorma.go:17`), and internal `Vorma` embeds `*wave.Wave`
(`internal/vormaruntime/vorma_core.go:27`). This leaks a very large, unstable
method set into the public API.

Solution: Replace alias-based exposure with an explicit public facade type that
exposes only intended stable APIs. Keep internal runtime types internal.

## Proposal 2: Remove Public Methods That Depend on Internal-Only Types

Problem: Publicly visible methods like `WithLock`/`WithRLock` require callback
types from `internal/vormaruntime`, which external consumers cannot import
(`internal/vormaruntime/vorma_core.go:146`,
`internal/vormaruntime/vorma_core.go:154`).

Solution: Remove these from the public app-facing API surface. If lock-scoped
helpers are still needed, expose a separate public-safe lock API that does not
reference internal package types.

## Proposal 3: Make Route Declaration API Semantics Explicit

Problem: `NewLoader`/`NewAction` currently look like runtime
constructors/registrars but actually act as declaration markers for discovery
(`vorma.go:53`, `vorma.go:64`). The semantic mismatch makes callsites harder to
reason about.

Solution: Keep top-level generic declaration APIs (not receiver methods) and
rename them to declaration semantics:

- `vorma.DeclareLoader(...)`
- `vorma.DeclareAction(...)`

Keep `app` as an explicit argument so discovery can bind the correct app
expression in generated registrations.

For application ergonomics, generated starter wrappers should remain concise:

- app-local wrapper `Loader(...)` -> calls `vorma.DeclareLoader(...)`
- app-local wrapper `Action(...)` -> calls `vorma.DeclareAction(...)`

## Proposal 4: Move Generated-Only Registration Entrypoints off Main Public API

Problem: `Internal__RegisterDiscoveredLoader` and
`Internal__RegisterDiscoveredAction` are exported from the root public package
(`vorma.go:76`, `vorma.go:89`) even though they are generated-code plumbing.

Solution: Move generated-only entrypoints to a dedicated generated-support
package/path intended for build output use, not general app authoring.

## Proposal 5: Keep AST Discovery, but Make Its Output First-Class and Observable

Problem: The current registration path relies heavily on generated `init()` side
effects (`vormabuild/backend_route_registration_generation.go:477`), which is
effective but can be opaque during debugging.

Solution: Preserve automatic AST discovery and side-effect-free ergonomics (no
manual package import burden), and add explicit generated artifacts first:

- Registration manifest/metadata artifact.
- Explain/doctor style diagnostics can be added later (deferred for now).

## Proposal 6: Fail Route Discovery Issues Loudly by Default

Problem: Unresolved/dynamic route module expressions are currently
warning-and-skip in some paths (`vormabuild/route_parsing_pipeline.go:241`,
`vormabuild/route_parsing_pipeline.go:247`). This can produce confusing partial
route graphs.

Solution: Switch to fail-fast defaults for unresolved declarations. Provide
explicit opt-in APIs for intentionally dynamic route declarations.

## Proposal 7: Fix Root Element ID Contract Mismatch End-to-End

Problem: Server runtime supports configurable `ClientRootElementID`
(`internal/vormaruntime/types.go:47`), but client `getRootEl()` is hardcoded to
`"vorma-root"` (`typescript/vorma/client/src/client.ts:102`).

Solution: Use one shared bootstrap contract for root element ID and ensure
client runtime resolves root element ID from that contract instead of hardcoded
constants.

## Proposal 8: Make Mode/Port/Cache Policy Instance-Scoped

Problem: Wave mode/port resolution and dev/prod cache behavior currently depend
on process-global environment state (`wave/env.go:19`,
`wave/cache_internal.go:19`, `wave/cache_internal.go:61`).

Solution: Move mode and cache policy to instance-scoped state on
`*Wave`/`*Vorma`. Treat environment variables as startup defaults only.

## Proposal 9: Replace Panic-First Core APIs with Error-Returning APIs + OrPanic Variants

Problem: Core creation/init flows panic on configuration/runtime errors
(`wave/wave.go:71`, `internal/vormaruntime/glue.go:136`,
`internal/vormaruntime/vorma_init.go:15`).

Solution: Adopt error-returning primary APIs and provide convenience OrPanic
variants:

- `wave.New(...) (*Wave, error)` + `wave.NewOrPanic(...)`
- `vorma.NewVormaApp(...) (*Vorma, error)` + `vorma.NewVormaAppOrPanic(...)`
- `(*Vorma).Init() error` + `(*Vorma).InitOrPanic()`

Expected default in starter templates: use OrPanic variants at package scope,
where inline error handling is not practical.

## Proposal 10: Use Correct HTTP Semantics for Dev Reload Mutation Endpoints

Problem: Dev reload endpoints mutate runtime state but use GET
(`internal/vormaruntime/get_root_handler.go:99`,
`vormabuild/reload_endpoint.go:95`).

Solution: Switch these endpoints to POST and update build/dev caller paths
accordingly.

## Proposal 11: Separate Runtime-Safe Wave API from Build/Framework Mutation API

Problem: `*Wave` mixes runtime-serving concerns and build/framework mutation
methods in one surface (`wave/runtime_framework.go:14`,
`wave/runtime_framework.go:55`, `wave/runtime_framework.go:157`).

Solution: Split API surfaces by lifecycle intent:

- Runtime-safe app-serving surface.
- Build/dev/framework extension surface (tooling/bridge package or clearly
  namespaced methods).

## Proposal 12: Make Adapter-Shared API Explicitly Unstable and Purpose-Built

Problem: `./client/__internal` is exported and used by adapters
(`package.json:34`), but the naming does not clearly encode intended audience
and compatibility policy.

Solution: Rename to a purpose-specific, policy-explicit path (for example
`client/unstable-adapter-core`) and document that it is intended for adapter
authors and can change between prerelease/minor versions.

## Proposal 13: Add API Surface Governance to Prevent Accidental Contract Growth

Problem: Public contract growth is easy to introduce accidentally in both Go and
TypeScript exports.

Solution: Add automated API surface snapshot checks for:

- Go public symbols.
- npm export map and `.d.ts` package surfaces.

## Proposal 14: Add First-Class Discovery/Overlay Debug Tooling

Problem: Overlay generation/replacement and registration flow can be difficult
to inspect in CI/dev failures
(`vormabuild/backend_route_registration_generation.go:329`,
`vormabuild/build_environment.go:109`).

Solution: Add a dedicated diagnostics path (for example `vorma doctor` and/or
build `--explain`) that prints:

- discovered registration declarations,
- generated replacement targets,
- unresolved/skipped declarations with source locations,
- final registration graph summary.

Status: deferred for now while the local Vorma CLI shape is intentionally set
aside.

## Bootstrap Alignment Proposals

## Proposal 15: Keep Bootstrap Templates in Lockstep with API Evolution

Problem: Bootstrap templates currently hardcode APIs that are targeted by
earlier proposals (for example `wave.New`, `vorma.NewVormaApp`,
`vorma.NewLoader`, `vorma.NewAction`) across:

- `bootstrap/tmpls/backend_wave_dev_go_str.txt:14`
- `bootstrap/tmpls/backend_wave_prod_go_str.txt:15`
- `bootstrap/tmpls/backend_src_router_app_go_tmpl.txt:10`
- `bootstrap/tmpls/backend_src_router_context_go_tmpl.txt:20`
- `bootstrap/tmpls/backend_src_router_context_go_tmpl.txt:28`

If core API shape changes without synchronized bootstrap updates, new apps will
start from stale patterns.

Solution: Treat bootstrap as a first-class compatibility surface:

- Update templates in the same PRs as core API changes.
- Expand bootstrap conformance tests to assert new API usage patterns.
- Fail CI when template APIs drift from intended public guidance.

## Proposal 16: Keep Starter Route Boilerplate Minimal and Intentional

Problem: Starter apps include context/wrapper boilerplate before route
declarations:

- `bootstrap/tmpls/backend_src_router_context_go_tmpl.txt:5`
- `bootstrap/tmpls/backend_src_router_context_go_tmpl.txt:16`
- `bootstrap/tmpls/backend_src_router_context_go_tmpl.txt:23`

Most of this boilerplate is currently structural to preserve strong typing and
discovery-friendly top-level declarations.

Solution: Keep the current pattern largely intact, with targeted ergonomic
improvements only:

- Keep concise app-local `Loader(...)` / `Action(...)` wrappers.
- Avoid large internalization attempts that hide important typed context
  boundaries.
- Revisit only if a clearly better typed abstraction emerges.

## Proposal 17: Align Bootstrap Defaults with Error-First Core APIs

Problem: Bootstrap defaults currently follow panic-first constructor usage:

- `bootstrap/tmpls/backend_wave_dev_go_str.txt:14`
- `bootstrap/tmpls/backend_wave_prod_go_str.txt:15`
- `bootstrap/tmpls/backend_src_router_app_go_tmpl.txt:10`

This will conflict with the proposed move to error-returning primaries plus
OrPanic variants.

Solution: Once error-first APIs land, update bootstrap defaults to use the
recommended entrypoints (likely OrPanic variants at package scope, with explicit
error handling in process entrypoints where appropriate).

## Proposal 18: Make Root Mount Contract Explicit in Starter Defaults

Problem: Starter templates already render a configurable root placeholder in
HTML (`bootstrap/tmpls/backend_static_entry_go_html_str.txt:10`) and call client
`getRootEl()` in entrypoints
(`bootstrap/tmpls/frontend_entry_tsx_react_tmpl.txt:2`). Given the known root-ID
mismatch risk, starter defaults should make this contract explicit and
verifiable.

Solution: Standardize on passing app config to root element resolution directly
in starter entrypoints:

- `getRootEl(vormaAppConfig)`

and ensure generated `vormaAppConfig` includes root-element ID metadata so
server template config and client mount lookup remain synchronized.

## Proposal 19: Future Starter Diagnostics Script for Discovery/Registration Visibility

Problem: Starter `package.json` currently includes only `dev` and `build`
(`bootstrap/tmpls/package_json_tmpl.txt:4`). As route discovery/generation grows
more sophisticated, the default app lacks a built-in diagnostics path.

Solution: Defer this until a local Vorma CLI shape is adopted. Once that exists,
add a standard diagnostics script (for example `doctor` or `build --explain`) so
new apps can inspect discovery results and registration state without custom
setup.
