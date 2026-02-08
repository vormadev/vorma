# Vorma Versioning and Compatibility Specification

Status: Draft  
Last Updated: 2026-02-07  
Applies To: Versioning policy and compatibility guarantees for Go module, npm packages, and runtime surfaces

## 1. Why This Spec Exists

This document defines how Vorma versions are interpreted and what compatibility
commitments are made at each release stage.

It exists to:

- make upgrade expectations explicit,
- align Go and npm release channels,
- prevent accidental breakage in public surfaces.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Scope

This spec covers compatibility policy. Detailed artifact shape lives in release
and API surface specs.

## 3. Versioning Model

### VER-MODEL-001: Semantic Versioning Framework

Vorma version identifiers MUST follow semantic versioning structure:

`MAJOR.MINOR.PATCH[-PRERELEASE]`.

### VER-MODEL-002: Pre-Release Marker Semantics

Versions containing explicit pre-release marker (for example `-pre.*`) denote
non-final release channel.

### VER-MODEL-003: Channel Mapping

- Pre-release versions SHOULD publish to npm `pre` tag.
- Final versions SHOULD publish to npm default/latest channel.

### VER-MODEL-004: Cross-Channel Version Alignment

For an official release cycle, root npm package version and create package
version MUST remain aligned.

### VER-MODEL-005: Go Tag Version Form

Go releases MUST use `v`-prefixed semantic tag form (for example `v0.85.0`).

## 4. Pre-1.0 Compatibility Policy

### VER-PRE1-001: Current Stability Tier

While Vorma major version is `0`, breaking changes MAY occur between releases.

### VER-PRE1-002: Breaking Change Disclosure Requirement

Even in pre-1.0 phase, breaking changes SHOULD be explicitly documented in
release notes/changelog.

### VER-PRE1-003: Migration Guidance Requirement

When breaking changes are introduced in pre-1.0 releases, maintainers SHOULD
provide migration notes or examples for affected surfaces.

### VER-PRE1-004: Surprise-Break Minimization

Pre-1.0 policy does not permit arbitrary silent breakage; maintainers SHOULD
still prefer compatibility where feasible.

## 5. Compatibility Surface Classes

### VER-SURF-001: Primary Go Public Surface

Primary Go compatibility surface is top-level package import:

`github.com/vormadev/vorma`.

### VER-SURF-002: Primary npm Public Surface

Primary npm compatibility surface is package `vorma` with documented subpath
exports.

### VER-SURF-003: Create CLI Surface

Scaffolding compatibility surface is package `create-vorma` (CLI behavior and
minimum runtime prerequisites).

### VER-SURF-004: Wire Protocol Surface

Headers/query/payload contracts defined in wire/runtime specs are compatibility
surfaces and SHOULD remain stable unless explicitly versioned/deprecated.

### VER-SURF-005: Conformance ID Surface

Requirement IDs in accepted conformance specs are compatibility-sensitive for
traceability and SHOULD remain stable.

### VER-SURF-006: Internal/Underscore Surface

Surfaces intentionally prefixed as internal/underscore (for example names
starting with `__` or `Internal__`) MAY change faster and are not guaranteed as
stable consumer APIs unless explicitly promoted.

## 6. Runtime and Tooling Compatibility Floors

### VER-FLOOR-001: Go Toolchain Floor

Official support floor for building Vorma codebase MUST be Go 1.24 or higher
(consistent with module baseline).

### VER-FLOOR-002: Node Floor for Create CLI

`create-vorma` compatibility floor MUST remain explicit via engines field
(currently Node >= 22.11.0) unless intentionally revised.

### VER-FLOOR-003: Browser Runtime Baseline

Frontend runtime support target is modern evergreen browsers with ES2022-class
capabilities required by emitted runtime artifacts.

### VER-FLOOR-004: Legacy Browser Non-Goal

Legacy browsers below modern evergreen baseline are not covered by default
compatibility guarantees unless explicitly documented.

### VER-FLOOR-005: UI Variant Set

Supported UI variant compatibility set is:

- React
- Preact
- Solid

Release changes SHOULD preserve parity expectations across this set.

## 7. Change Classification and Version Impact

### VER-CHANGE-001: Patch-Class Change

Patch-class changes SHOULD be limited to bug fixes and non-breaking contract
clarifications.

### VER-CHANGE-002: Minor-Class Change

Minor-class changes MAY add new backward-compatible features, APIs, and
requirements.

### VER-CHANGE-003: Breaking-Class Change

Breaking changes SHOULD be reflected in version intent and release notes, even
in pre-1.0 stream.

### VER-CHANGE-004: Public API Removal Policy

Removal/rename of public exports SHOULD follow deprecation path when practical
rather than immediate deletion.

### VER-CHANGE-005: Contract Change Coordination

Behavioral contract changes MUST coordinate updates across:

- relevant conformance spec(s),
- testing traceability,
- release/distribution notes.

## 8. Compatibility Guarantees by Surface

### VER-GUAR-001: Go Top-Level API Stability Intent

Top-level Go package APIs SHOULD remain stable across routine releases, with
breaking changes explicitly communicated.

### VER-GUAR-002: npm Export Stability Intent

Documented npm subpath exports SHOULD remain stable and resolvable across
routine releases.

### VER-GUAR-003: Client Event Name Stability

Public client event names (`vorma:status`, `vorma:route-change`,
`vorma:build-id`, `vorma:location`) are compatibility-sensitive and SHOULD not
change without migration policy.

### VER-GUAR-004: Build Artifact Contract Stability

Build artifact schema/path contracts used by runtime (stage files, route
manifest semantics) are compatibility-sensitive and MUST be evolved carefully.

### VER-GUAR-005: Dev Endpoint Stability Intent

Core dev callback endpoints used by default workflow (`/__vorma/reload-routes`,
`/__vorma/reload-template`) SHOULD remain stable unless superseded with
migration path.

## 9. Compatibility Validation Process

### VER-VAL-001: Release Validation Inputs

Release readiness SHOULD validate:

- conformance test suite pass on changed surfaces,
- exported API surface integrity,
- version alignment between related packages,
- documented upgrade notes for breaking changes.

### VER-VAL-002: Spec Consistency Check

Release process SHOULD ensure compatibility-affecting changes are reflected in
relevant specs before final publish.

### VER-VAL-003: Upgrade Path Smoke Checks

For compatibility-sensitive releases, maintainers SHOULD run smoke checks for
common upgrade paths (Go import + npm subpath import).

## 10. Deprecation Policy

### VER-DEPR-001: Deprecation Annotation

Deprecated public surfaces SHOULD be explicitly annotated in docs/specs before
removal where feasible.

### VER-DEPR-002: Deprecation Window Guidance

Where practical, provide at least one release-cycle warning before removal of
widely used surfaces.

### VER-DEPR-003: Deprecation-to-Removal Traceability

When a deprecated surface is removed, release notes SHOULD reference the prior
deprecation notice.

## 11. Relation to Other Specs

- Release/distribution: `/Users/sjc/__code/river/specs/VORMA_RELEASE_DISTRIBUTION_SPEC.md`
- Public API map: `/Users/sjc/__code/river/specs/VORMA_PUBLIC_API_SURFACE_SPEC.md`
- Spec process: `/Users/sjc/__code/river/specs/VORMA_SPEC_PROCESS_RFC_SPEC.md`
- Checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`
