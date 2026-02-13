# Locked Decisions And Architecture

## Locked Decisions

- Wave and Vorma stay separate products with clear boundaries.
- Wave is Go-first and must support zero-Node workflows.
- Vorma depends on Vite for rich frontend workflows.
- JSON is the sole authored Wave config surface.
- Frontend route-definition files use a strict static AST contract.
- Backend route discovery uses symbol-resolved static analysis.
- Backend route discovery must not rely on helper function naming conventions.
- Build/runtime determinism is enforced internally.
- No framework-enforced large-app package boundary model.

## Core Architecture Constraints

### Configuration Source Policy

- Application-authored Wave configuration is JSON only.
- Go config-as-code is not part of the app-facing configuration contract.
- Any vestigial config-as-code paths in Wave should be removed.

### Go Rebuild Reality

- Go source changes require compile + process restart.
- Dev architecture therefore requires:
    - process A: long-lived coordinator/devserver
    - process B: app server process that can restart

### Runtime/Build Boundary

- Runtime code must not pull build-only dependencies.
- Build-only behavior stays out of production binaries.
- Keep `runtime` and `build/tooling` responsibilities split.

### Fast Rebuild Path

- Route-definition and template edits must avoid full rebuilds when possible.
- Process A generates artifacts and notifies Process B reload endpoints.
- Process B reloads generated artifacts from disk.
- Full restart remains fallback for incompatible change classes.

### Wave Responsibilities

- Static asset hashing for Go-template-referenced assets.
- CSS pipeline for non-JS-entry CSS.
- Go-level rebuild/reload broadcast lifecycle.
- Dev orchestration and watcher control.

### Vorma Responsibilities

- Route model, typegen, and SSR app lifecycle.
- Integration hooks into Wave for framework-specific fast paths.
- Route conflict diagnostics and deterministic manifest generation.

## Backend Route Discovery Contract

### Will Match

- Direct canonical registration calls.
- Wrapper/helper calls whose forwarding can be proven statically.
- Compile-time method/pattern strings (including const concatenation).

### Will Not Match

- Runtime dispatch via reflection/interface function values.
- Runtime-computed method/pattern strings.
- Code paths that are not statically init-reachable.

### Failure Bias

- If static proof is ambiguous, fail unsupported rather than guessing.
- Prefer false negative over false positive.

## Diagnostics Contract

- `wave explain` and `wave doctor` must explain:
    - route conflict origin
    - rebuild trigger reason
    - planner decision path
    - config file trigger path
