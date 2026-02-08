# Vorma Testing Strategy Specification

Status: Draft  
Last Updated: 2026-02-07  
Applies To: All test suites validating Vorma runtime/build/wire/frontend behavior

## 1. Why This Spec Exists

This document defines how Vorma tests are derived from specs, organized, and
validated.

It exists to:

- make large refactors safe without reading implementation internals,
- ensure tests validate externally visible contracts rather than current code shape,
- create a uniform test strategy across Go runtime/build, wire, and TypeScript client layers.

## 2. Conformance Boundaries

### 2.1 Normative Terms

The terms **MUST**, **MUST NOT**, **SHOULD**, and **MAY** are normative.

### 2.2 Scope of This Spec

This is a **strategy and governance spec**, not a feature behavior spec.

Feature behavior remains defined in:

- `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- `/Users/sjc/__code/river/specs/VORMA_WIRE_CONTRACT_SPEC.md`
- `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- `/Users/sjc/__code/river/specs/VORMA_FRONTEND_RUNTIME_SPEC.md`
- `/Users/sjc/__code/river/specs/VORMA_KIT_INTEROP_SPEC.md` (supporting dependency map)

### 2.3 What Test Conformance Is Measured Against

Test conformance is measured by:

- requirement ID traceability coverage,
- black-box behavior validation quality,
- anti-mirroring compliance,
- deterministic and reliable CI operation.

## 3. Requirement Inventory and Traceability Model

### TEST-MAP-001: Requirement IDs Are the Source of Truth

Given Vorma conformance specs define requirement IDs (`BR-*`, `WIRE-*`, `BUILD-*`, `FE-*`)  
When test plans are created or updated  
Then those IDs MUST be treated as the authoritative behavior inventory.

### TEST-MAP-002: Every Normative Requirement Needs Test Coverage

Given a normative requirement ID in a conformance spec  
When test coverage is evaluated  
Then that ID MUST map to at least one active automated test case.

### TEST-MAP-003: Traceability Matrix Must Be Maintained

Given conformance requirements and test cases  
When repository metadata is updated  
Then a machine-readable or structured matrix MUST map:

- `Requirement ID`
- `Test file(s)`
- `Test case id/title`
- `Status` (`implemented`, `partial`, `missing`, `deprecated`).

### TEST-MAP-004: Missing Coverage Must Be Explicit

Given a requirement has no test yet  
When matrix is reviewed  
Then it MUST be explicitly marked `missing` with owner/priority note.

### TEST-MAP-005: Behavior Change Requires Spec/Test Delta Pairing

Given a PR changes externally visible behavior  
When merged  
Then it MUST include:

- spec requirement updates, and
- corresponding test traceability updates.

Behavior-changing code changes without spec/test mapping updates SHOULD be treated as non-conformant.

### TEST-MAP-006: Non-Normative Behavior Must Not Become Gate Criteria

Given behavior is not represented by a normative requirement ID  
When deciding release-blocking tests  
Then it MUST NOT be promoted to release-gate status without spec update.

## 3.1 Concrete Traceability Matrix Format

### TEST-MAP-007: Canonical Matrix Artifact Path

The canonical traceability artifact MUST be maintained at:

`/Users/sjc/__code/river/specs/VORMA_TRACEABILITY_MATRIX.md`

### TEST-MAP-008: Required Matrix Columns

Each matrix row MUST include these columns:

- `Requirement ID`
- `Scenario ID(s)`
- `Suite Family`
- `Suite Name`
- `Fixture Type`
- `Test File(s)`
- `Pass Criteria`
- `Status`
- `Owner`

### TEST-MAP-009: Row Granularity

Matrix rows MUST be keyed by one concrete requirement ID (no combined
multi-requirement row keys).

### TEST-MAP-010: Allowed Suite Family Values

`Suite Family` MUST be one of:

- `backend-runtime`
- `wire-contract`
- `build-dev`
- `frontend-runtime`
- `interop-regression`

### TEST-MAP-011: Allowed Fixture Type Values

`Fixture Type` SHOULD use one of:

- `go-fixture-app-http`
- `wire-http-fixture`
- `cli-fs-sandbox`
- `browser-dom-harness`
- `cross-layer-integration`

### TEST-MAP-012: Pass Criteria Format

`Pass Criteria` MUST be a one-sentence black-box assertion summary in the form:

`Observable contract for <Requirement ID> passes under mapped scenario(s).`

### TEST-MAP-013: Scenario Link Rule

`Scenario ID(s)` MUST reference scenario identifiers from conformance specs when
available (for example `BRC-*`, `WRC-*`, `BDC-*`, `FEC-*`).

### TEST-MAP-014: Matrix Status Semantics

`Status` MUST be one of:

- `implemented`
- `partial`
- `missing`
- `deprecated`
- `blocked`

### TEST-MAP-015: CI Gate Query

Release gating for changed behavior MUST query the matrix by changed
requirement IDs and fail when any changed ID remains `missing` or `blocked`
without explicit approved exception.

## 4. Anti-Mirroring and Black-Box Rules

### TEST-BB-001: Public Surface Only

Conformance tests MUST assert behavior through public/observable surfaces only:

- public Go APIs/handlers,
- CLI behavior,
- HTTP I/O,
- filesystem artifacts,
- browser/DOM/events,
- exported TS client APIs.

### TEST-BB-002: No Private Coupling

Conformance tests MUST NOT require:

- private package symbols,
- internal struct fields,
- lock ordering details,
- file-internal helper functions.

### TEST-BB-003: Prefer Contract Assertions Over Implementation Assertions

Given multiple ways to verify behavior  
When writing tests  
Then assertions SHOULD target protocol/contract outcomes (status/headers/schema/events/artifacts), not call graphs or incidental intermediate state.

### TEST-BB-004: Avoid Snapshotting Unstable Incidental Output

Given output includes unstable incidental details (timing noise, map iteration order, absolute temp paths)  
When writing assertions  
Then tests MUST normalize or ignore unstable fields.

### TEST-BB-005: Reject “Cheating by Mirroring”

Given implementation currently behaves a certain way but spec does not guarantee it  
When authoring tests  
Then tests MUST NOT encode that accidental behavior as required.

## 5. Test Taxonomy and Required Suite Families

## 5.1 Portfolio-Level Suite Families

### TEST-SUITE-001: Required Families

Vorma conformance portfolio MUST include at least these families:

- Backend runtime conformance (Go + HTTP black-box)
- Wire protocol conformance (HTTP-level)
- Build/dev conformance (CLI + artifacts + callbacks)
- Frontend runtime conformance (TS runtime + browser semantics)
- Interop regression coverage for Vorma-visible kit contracts

### TEST-SUITE-002: Family Ownership

Each suite family MUST have clear ownership and CI execution path.

### TEST-SUITE-003: Cross-Family Overlap Is Allowed but Intent Must Be Clear

Given a requirement is validated by multiple families  
When traceability is recorded  
Then one test SHOULD be designated primary, others secondary.

## 5.2 Backend Runtime Conformance Family

### TEST-BR-001: Harness Form

Backend runtime conformance tests MUST run against constructed app fixtures and observable HTTP/public API outcomes.

### TEST-BR-002: Loader and Action Fixture Coverage

Backend conformance suite MUST include fixtures covering:

- nested loader matching,
- action method/input parsing,
- loader error and redirect behaviors,
- SSR template contract.

### TEST-BR-003: Reload Endpoint Coverage

Backend conformance suite MUST include positive and failure-path tests for:

- `/__vorma/reload-routes`
- `/__vorma/reload-template`.

### TEST-BR-004: Concurrency Safety Coverage

Backend suite SHOULD include race/stress scenarios for concurrent requests and reload operations.

### TEST-BR-005: Current Gap Obligation

Given current repository state has no direct `_test.go` files in `vormaruntime` or `vormabuild` (as of 2026-02-07)  
When adopting this strategy  
Then introducing dedicated Vorma-level Go conformance harness tests is REQUIRED backlog work.

## 5.3 Wire Conformance Family

### TEST-WIRE-001: Raw HTTP Assertions

Wire suite MUST assert request/response behavior at HTTP boundary (status, headers, content type, payload schema).

### TEST-WIRE-002: Stale vs Current Build Paths

Wire suite MUST separately verify current-build and stale-build `vorma_json` flows.

### TEST-WIRE-003: Redirect Priority Coverage

Wire/client contract tests MUST verify redirect priority behavior (`X-Vorma-Reload` > native redirect > `X-Client-Redirect`).

### TEST-WIRE-004: Bootstrap Contract Coverage

Wire suite MUST validate SSR bootstrap payload keys and head marker comments expected by frontend runtime.

## 5.4 Build/Dev Conformance Family

### TEST-BUILD-001: Mode Matrix Coverage

Build suite MUST cover CLI matrix combinations of `--dev`, `--hook`, and `--no-binary`.

### TEST-BUILD-002: Artifact Contract Coverage

Build suite MUST validate stage-1/stage-2 paths files, generated TS outputs, route manifest shape, and output naming constraints.

### TEST-BUILD-003: Dev Callback Behavior Coverage

Dev suite MUST validate fast route rebuild and template reload callback paths, including fallback behavior when callback endpoints fail.

### TEST-BUILD-004: Vite Integration Coverage

Build suite MUST verify Vite integration behaviors required by spec (output prefixes, invalidation endpoint semantics, public URL rewrite behavior).

## 5.5 Frontend Runtime Conformance Family

### TEST-FE-001: Runtime API Contract Coverage

Frontend conformance suite MUST validate exported `vorma/client` APIs and observable behavior only.

### TEST-FE-002: Navigation/Loading Continuity Coverage

Frontend suite MUST include deterministic tests for:

- navigation state transitions,
- submit/revalidate continuity,
- redirect-chain continuity,
- asset/client-loader wait continuity.

### TEST-FE-003: History/Scroll Coverage

Frontend suite MUST cover:

- same-document hash behavior,
- cross-document POP behavior,
- page-refresh scroll restoration window.

### TEST-FE-004: Event Contract Coverage

Frontend suite MUST validate event names, payload shapes, dispatch sequencing expectations, and listener cleanup behavior.

### TEST-FE-005: Adapter Parity Coverage

Frontend suite MUST include parity assertions for React/Preact/Solid adapters where behavior is contractually shared.

## 5.6 Interop Regression Family

### TEST-INTEROP-001: Vorma-Visible Kit Inheritance Coverage

Interop suite MUST include checks for Vorma-visible inherited behavior from kit dependencies where those behaviors are normative to Vorma contracts.

### TEST-INTEROP-002: Upstream Drift Detection

Given kit behavior changes in a way that may affect Vorma contracts  
When evaluating update  
Then interop regression tests MUST detect and surface contract-impacting drift.

## 6. Test Case Design Rules

### TEST-CASE-001: Given/When/Then Structure

Conformance test cases SHOULD map clearly to requirement-style scenario framing.

### TEST-CASE-002: Positive + Negative Coverage

Given requirement has both expected and failure modes  
When coverage is authored  
Then suite SHOULD include both success and failure-path tests.

### TEST-CASE-003: Deterministic Time Control

Given async/debounce/timing-sensitive behaviors  
When tests are authored  
Then fake clocks or deterministic wait controls SHOULD be used where possible.

### TEST-CASE-004: Explicit Race Tests for Concurrency Clauses

Given requirement involves concurrency/race safety  
When tests are authored  
Then dedicated race-oriented tests SHOULD be separate from baseline functional tests.

### TEST-CASE-005: Canonical Fixtures

Reusable fixture builders SHOULD be used for:

- route trees,
- manifest payloads,
- redirect responses,
- bootstrap payloads,
- build artifact templates.

### TEST-CASE-006: Contract-Shape Assertions

Given JSON/HTML/artifact contracts  
When asserting payloads  
Then tests SHOULD validate required fields and invariants while tolerating explicitly optional/`omitempty` fields.

### TEST-CASE-007: No Global State Leakage Across Cases

Suites MUST isolate/cleanup shared state (timers, listeners, globals, temp files, env vars) between tests.

## 7. CI and Quality Gates

### TEST-CI-001: Required Commands

The project-level commands MUST remain valid as test execution entry points:

- `make gotest`
- `make tstest`.

### TEST-CI-002: Suite Reliability Expectations

Conformance suites SHOULD target high deterministic pass rates in CI and MUST avoid flaky-by-design timing dependencies.

### TEST-CI-003: Flake Policy

Given a test is flaky  
When detected  
Then it MUST be triaged with one of:

- fix determinism,
- quarantine with explicit tracking ticket,
- downgrade from release gate until stabilized.

### TEST-CI-004: Release Gate Inputs

Release readiness MUST include:

- passing conformance suites for changed requirement families,
- no unresolved high-priority missing traceability entries for changed behavior.

### TEST-CI-005: Fast vs Full Test Profiles

CI/Developer workflows SHOULD maintain:

- fast profile for local iteration,
- full profile for release confidence,

while keeping traceability semantics identical.

## 8. Coverage Reporting Model

### TEST-COV-001: Requirement Coverage Over Line Coverage

Line coverage MAY be tracked, but conformance confidence MUST be evaluated primarily by requirement-ID coverage.

### TEST-COV-002: Coverage Status Taxonomy

Coverage reporting SHOULD classify requirement coverage as:

- `pass`
- `partial`
- `missing`
- `blocked` (with reason).

### TEST-COV-003: Partial Coverage Disclosure

Given requirement is partially covered  
When reporting status  
Then missing scenario dimensions MUST be listed explicitly.

### TEST-COV-004: Change Impact Mapping

Given a PR touches behavior tied to requirement IDs  
When reporting test impact  
Then changed IDs and executed validating tests SHOULD be listed.

## 9. Test Data, Fixtures, and Security Considerations

### TEST-DATA-001: Safe Synthetic Data

Conformance fixtures MUST use non-production synthetic data by default.

### TEST-DATA-002: Sensitive Data Handling

If real-like sensitive values are unavoidable in fixtures, they MUST be anonymized and non-reversible.

### TEST-DATA-003: Filesystem Isolation

Build/dev conformance tests MUST run in isolated temp workspaces and MUST clean artifacts after execution.

### TEST-DATA-004: Network Isolation

Tests SHOULD avoid dependence on external network services unless explicitly designated as integration tests with controlled environment requirements.

## 10. Adoption Plan Requirements

### TEST-ADOPT-001: Establish Traceability Artifact

As part of adopting this spec, repository MUST create and maintain the
traceability artifact at:

`/Users/sjc/__code/river/specs/VORMA_TRACEABILITY_MATRIX.md`

for requirement-ID-to-test mapping.

### TEST-ADOPT-002: Backfill Existing Tests Into Matrix

Existing frontend Vitest suites SHOULD be mapped first to `FE-*` IDs, then expanded for missing cases.

### TEST-ADOPT-003: Create Dedicated Go Conformance Harnesses

Repository SHOULD add dedicated Go conformance suites for:

- backend runtime (`BR-*`),
- wire contract (`WIRE-*`),
- build/dev contract (`BUILD-*`) where feasible.

### TEST-ADOPT-004: Enforce Spec-First for New Behavior

Given new externally visible behavior is introduced  
When implementing  
Then required sequence SHOULD be:

1. add/update requirement IDs in spec,
2. add/update conformance tests,
3. implement/refactor code.

### TEST-ADOPT-005: Require Review-Time Traceability Check

PR review checklist SHOULD include confirmation that changed behavior has matching requirement-ID traceability updates.

## 11. Relation to Other Specs

- Backend runtime contracts: `/Users/sjc/__code/river/specs/VORMA_BACKEND_RUNTIME_SPEC.md`
- Wire contracts: `/Users/sjc/__code/river/specs/VORMA_WIRE_CONTRACT_SPEC.md`
- Build/dev contracts: `/Users/sjc/__code/river/specs/VORMA_BUILD_DEV_CONFORMANCE_SPEC.md`
- Frontend contracts: `/Users/sjc/__code/river/specs/VORMA_FRONTEND_RUNTIME_SPEC.md`
- Interop/supporting dependency map: `/Users/sjc/__code/river/specs/VORMA_KIT_INTEROP_SPEC.md`
- Traceability matrix: `/Users/sjc/__code/river/specs/VORMA_TRACEABILITY_MATRIX.md`
- Roadmap checklist: `/Users/sjc/__code/river/specs/SPECS_CHECKLIST.md`
