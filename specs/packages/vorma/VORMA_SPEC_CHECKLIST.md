# Vorma Spec Checklist

Status: Active  
Last Updated: 2026-02-09

## Boundary Rule

- `VORMA_*` specs define Vorma user-visible boundaries only.
- Wave/Kit/Lab internals are referenced via owner-package specs.
- Vorma specs do not duplicate owner-package requirement text.

## Core Spec Coverage

- [x] `VORMA_DOMAIN_MODEL_TERMINOLOGY_SPEC.md`
- [x] `VORMA_SPEC_PROCESS_RFC_SPEC.md`
- [x] `VORMA_PUBLIC_API_SURFACE_SPEC.md`
- [x] `VORMA_BACKEND_RUNTIME_SPEC.md`
- [x] `VORMA_WIRE_CONTRACT_SPEC.md`
- [x] `VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- [x] `VORMA_FRONTEND_RUNTIME_SPEC.md`
- [x] `VORMA_KIT_INTEROP_SPEC.md`
- [x] `VORMA_SECURITY_MODEL_SPEC.md`
- [x] `VORMA_PERFORMANCE_MODEL_SPEC.md`
- [x] `VORMA_OBSERVABILITY_DEBUG_SPEC.md`
- [x] `VORMA_TESTING_STRATEGY_SPEC.md`

## Vorma-Owned Tracking Artifacts

- [x] `specs/packages/vorma/VORMA_TRACEABILITY_MATRIX.md`
- [x] `specs/packages/vorma/VORMA_CONFORMANCE_ISSUES.md`
- [x] `specs/packages/vorma/VORMA_NORMATIVE_INTENT_LEDGER.md`
- [x] `specs/packages/vorma/VORMA_FULL_AUDIT_TRACKER.md`

## Dependency Reference Artifacts

- [x] `specs/packages/PACKAGE_INDEX.md`
- [x] `specs/packages/wave/WAVE_BUILD_DEV_CONFORMANCE_SPEC.md`
- [x] `specs/packages/kit/mux/SPEC.md`
- [x] `specs/packages/kit/matcher/SPEC.md`
- [x] `specs/packages/kit/response/SPEC.md`
- [x] `specs/packages/kit/validate/SPEC.md`
- [x] `specs/packages/kit/headels/SPEC.md`
- [x] `specs/packages/lab/tsgen/SPEC.md`
- [x] `specs/packages/lab/viteutil/SPEC.md`
- [x] `specs/packages/vormaclient/client/SPEC.md`

## Current Work

- [ ] Complete full from-scratch structural pass across all Vorma artifacts.
- [ ] Complete full from-scratch boundary pass across all Vorma artifacts.
- [ ] Complete full from-scratch semantic pass across all Vorma artifacts.
- [ ] Record two consecutive no-gap full rounds in `VORMA_NORMATIVE_INTENT_LEDGER.md`.
