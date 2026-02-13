# Framework Ideal End-State Plan (Sub-1.0 Hard-Break Track)

- Last updated: 2026-02-12
- Status: in progress (UX rollback landed)
- Scope: Wave + Vorma + bootstrap + reference apps

## Direction Correction (Locked)

The previous `src/modules` + `AppModule` default app shape regressed app-builder
UX. That direction is no longer the target default.

Locked correction:

- No mandatory per-route multi-file backend registration boilerplate.
- No mandatory `src/modules` package shape in scaffolded apps.
- No side-effect route registration (`init`, underscore imports) for
  correctness.
- No requirement to edit a second backend file to add one backend route.

## UX/DX Non-Regression Gate (Hard Line)

This roadmap has a hard constraint: break APIs if needed, but do not degrade
app-builder UX/DX.

Reference baseline for route authoring UX:

- Commit `4dc9191a` (`v0.83.0`) is the canonical baseline for "original" simple
  route authoring (top-level `NewLoader`/`NewAction` declarations in router
  files, no extra per-route registration ceremony).
- Commit `7471ca2c` marks the split-file/manual-registration transition and is
  treated as historical context, not the UX floor.

Required pass/fail criteria for any framework/bootstrap/reference-app change:

- Adding one backend route must not require touching a second backend file.
- Adding one full-stack route must not require touching a third file solely for
  backend registration wiring.
- Runtime/build determinism must be implemented internally (sorting, checks,
  generated artifacts) and must not add user ceremony.
- Wave/Vorma defaults must keep one obvious happy path per task and must not
  introduce duplicate ways to do the same common operation.

## UX Contract (Locked)

- Backend-only route addition: one backend file edit.
- Full-stack route addition: two file edits (backend route + frontend route
  file), not three.
- Deterministic codegen outputs are achieved by explicit sorting in tooling, not
  by forcing runtime registration ceremony.
- One composition source of truth should exist at app/feature level, not
  per-route.
- Users keep full low-level freedom (headers, transport, request context,
  loader/action context extension).

## Completed Since Last Revision

- Bootstrap defaults rolled back to top-level route declarations:
    - no `registration.go`
    - no `registerAllRoutes()`
    - build path uses `router.App` directly
- `internal/site` router migrated to the same top-level route declaration shape.
- Added bootstrap UX conformance tests that fail on regression to split
  registration wrapper files.
- Added backend Go AST route discovery in `vormabuild`:
    - discovers loader patterns from top-level `var _ = NewLoader(...)`
      declarations
    - merges backend-only loader patterns into route sync without extra user
      registration ceremony
- Removed default app-facing Wave dev/prod split files:
    - default path is now `var Wave = wavecfg.NewWave(config.WaveConfig)`
    - no `wave_options_dev.go` / `wave_options_prod_*.go` required in defaults

## End-State Architecture

### 1) Route Authoring (Small Apps)

Default scaffold stays simple and explicit in `backend/src/router`:

- `app.go`: app singleton + head/template defaults
- `context.go`: typed ctx wrappers and `NewLoader`/`NewAction` helpers
- `init.go`: HTTP middleware wiring
- `routes_*.go`: self-contained top-level route declarations

Key requirement:

- Route logic is defined once at top-level (`var _ = NewLoader(...)`,
  `var _ = NewAction(...)`), with no extra wrapper registration layer.

### 2) Route Authoring (Large Apps)

Large apps scale by feature packages, while keeping one composition root:

- Feature packages export `RegisterRoutes(app *vorma.Vorma)`.
- Root router/composition file calls feature registrars explicitly.
- Feature packages do not import root router package.
- Shared contracts/services live in explicit shared packages.

This keeps package boundaries clear without adding per-route ceremony.

### 3) Single Composition Manifest (Runtime + Build)

One composition manifest is the source of truth for feature/module inclusion.

Each feature descriptor may declare:

- `ID`
- `DependsOn`
- `RegisterBackend(app *vorma.Vorma)`
- `ClientRouteDefinitionPatterns()`

Build and runtime both consume this same manifest.

Key requirement:

- No duplicate backend registration declarations in separate runtime/build
  files.

### 4) Deterministic and Safe Registration

- Detect duplicate/overlapping backend pattern conflicts as hard errors.
- Keep user authoring explicit and low-ceremony.
- Keep build/runtime determinism enforced internally by tooling (sorting, AST
  discovery, stable artifact generation), not by extra route-registration files.
- Keep generated files/manifests sorted and stable.
- Keep diagnostics source-oriented (point to concrete files/functions).

### 5) Typed Context and Escape Hatches

- Keep typed ctx decorators centralized once per app.
- Keep direct access to low-level request/response APIs.
- Keep client-level request middleware/interceptors for auth/header/transport
  needs.

## What Remains Good From Existing Work

- Config-as-code foundation (`wave.ConfigSource`, `wavecfg.Document`, provider
  protocol)
- Provider dependency-driven reload plumbing
- Wave runtime/tooling hardening and expanded tests
- Template-data configurability and endpoint configurability improvements

These stay, but route authoring UX constraints above take precedence.

## Work Plan (Updated)

### Phase A: UX Regression Rollback and Baseline Restore

- Completed:
- Removed mandatory `src/modules` routing pattern from bootstrap defaults.
- Restored terse top-level route declarations in scaffolds and `internal/site`.
- Enforced one-route backend change = one backend file edit in default shape.

### Phase B: Single Composition Manifest (Feature-Level)

- Add feature-level manifest descriptor consumed by both build and runtime.
- Keep feature inclusion declaration in one place.
- Keep per-route authoring self-contained and low-ceremony.

### Phase C: Generated Registration Surface

- Generate deterministic backend registration glue from discovered route
  registration funcs.
- Keep generated glue as the mechanism for determinism, not manual boilerplate.
- Keep duplicate/conflict checks strict and readable.

### Phase D: Large-App Boundary Enforcement

- Conventions and checks for package boundaries (feature-to-root
  directionality).
- Cycle detection and contract diagnostics focused on feature manifest level.

### Phase E: Tooling and Conformance

- Add/expand tests that enforce UX contract directly:
- one backend edit for backend-only route additions in scaffold flow
- stable generated artifacts with sorted output
- no side-effect registration dependency for correctness

## Immediate Next Steps

1. Decide whether to keep `AppModule`/`AppRoot` as advanced opt-in APIs or
   remove them entirely from public surface to avoid default-path confusion.
2. Design a single feature-level composition manifest that does not reintroduce
   per-route registration ceremony.
3. Extend UX conformance tests beyond bootstrap templates to assert route
   editing ergonomics in generated app fixtures.
