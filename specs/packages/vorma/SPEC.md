# vorma Specification

Status: Active  
Last Updated: 2026-02-09

## Scope

`vorma` is a thin wrapper/facade package over owner packages.

Owner package semantics are canonical in:

- `specs/packages/vormaruntime/SPEC.md` (`BR-*`)
- `specs/packages/vormaruntime/WIRE_CONTRACT_SPEC.md` (`WIRE-*`)
- `specs/packages/vormabuild/SPEC.md` (`BUILD-*`)
- `specs/packages/vormaclient/client/SPEC.md` (`FE-*`)

This package owns only wrapper-level contracts defined by `vorma.go`.

Current evidence note:

- Wrapper requirements are currently derived from `vorma.go` source.
- No legacy wrapper package tests outside `conformance/**` were found in this
  repo during this replay round.

## Requirement Catalog

### VORMA-API-001: Wrapper Constructor Delegation

Given `NewVormaApp(o)` from `vorma`  
When called with any valid `VormaAppConfig`  
Then behavior MUST be delegated to `vormaruntime.NewVormaApp(o)` without
introducing additional wrapper-side policy.

### VORMA-API-002: Wrapper Loader Registration Helper

Given `NewLoader(...)` from `vorma`  
When invoked  
Then it MUST register the generated task handler on
`app.LoadersRouter().NestedRouter` for the provided pattern and return the
registered handler.

### VORMA-API-003: Wrapper Action Registration Helper

Given `NewAction(...)` from `vorma`  
When invoked  
Then it MUST register the generated task handler on
`app.ActionsRouter().Router` for the provided method+pattern and return the
registered handler.

### VORMA-API-004: Re-Export Identity Surface

Given top-level re-exported vars/types in `vorma.go`  
When compiled and used by consumers  
Then those exports MUST remain alias/re-export contracts over their owner
symbols (not divergent wrapper implementations).

### VORMA-API-005: Embedded NPM Version Accessor

Given `Internal__GetCurrentNPMVersion()`  
When called  
Then it MUST parse the embedded root `package.json` and return the current npm
version string.
