# Framework Ideal End-State Plan (Sub-1.0 Hard-Break Track)

Scope: Wave + Vorma + bootstrap + reference apps.

Wave config sub-plan: `WAVE_JSON_CONFIGURATION_PLAN.md`.

## Handoff Freshness Rules

- This is the canonical execution plan for cross-framework work.
- At the end of every agent response, verify and update this file first if any
  scope, decisions, sequence, or priorities changed.
- Keep this file future-looking only.
- Do not add implemented/completed/changelog logs.
- Keep exactly one canonical immediate-next-steps list in this file.

## Hard Constraints

- APIs may break; app-builder UX/DX may not regress.
- Baseline UX floor is commit `4dc9191a`.
- Adding one backend route must not require touching a second backend file.
- Adding one full-stack route must not require a third file purely for backend
  wiring.
- Deterministic outputs are internal framework/tooling responsibilities.
- No side-effect registration as a correctness requirement.
- Keep one obvious default path for common tasks.
- Applications may organize code however they want; framework defaults must not
  enforce a package-organization model.

## Locked Current Decisions

- JSON is the authored Wave config surface.
- Route authoring is location-flexible; frontend route-definition files use a
  strict static AST contract, and backend route discovery comes from
  symbol-resolved static analysis.
- One backend route addition must remain a one-backend-file edit.
- One full-stack route addition must remain backend route file plus frontend
  route file.
- Build determinism is enforced internally and must not add per-route user
  boilerplate.
- No default-path builder-pattern APIs.
- No framework-enforced large-app package boundary model.
- Backend route discovery must not depend on scaffold helper names.

## Target UX Contract

- Backend route authoring remains one-file for backend-only additions.
- Full-stack route authoring remains two-file for backend+frontend additions.
- Typed context extension remains straightforward and centralized.
- Low-level escape hatches remain available for headers, request transport,
  runtime hooks, and loader/action context shaping, and anything else that was
  possible for the user to control as of `4dc9191a`.

## Target Architecture

### 1) Route Authoring

- Location-flexible route definitions with no framework-enforced package shape.
- No per-route manual registration ceremony.
- No side-effect import dependency for correctness.

### 2) Determinism and Safety

- Stable sorted generated artifacts and manifests.
- Strict duplicate/conflict route detection with source-oriented errors.
- Strict frontend route-definition parsing rules with fail-fast behavior.
- Deterministic AST-to-manifest generation contract.
- Backend route discovery is derived from symbol-resolved static analysis, not
  helper-function naming conventions in source files.
- Backend AST discovery resolves canonical registration targets and wrapper
  forwarding paths while allowing arbitrary local helper names.
- Backend route method/pattern arguments must resolve to compile-time strings;
  unresolved or ambiguous shapes fail fast with source-oriented diagnostics.

Backend symbol-resolved discovery contract:

1. Load only Go files matched by `ServerRouteDefinitionPatterns` and analyze by
   package.
2. Resolve route registration by canonical registration targets, not by
   identifier text.
3. Support wrapper/helper functions by inferring parameter forwarding to
   canonical registration symbols.
4. Extract method/pattern only from compile-time string constants (including
   const concatenation); dynamic values are hard errors.
5. Emit deterministic sorted outputs and deterministic diagnostics including
   file/line/column.

False-positive safety contract:

1. Never infer routes from type usage (`vorma.Loader`, `vorma.Action`) alone.
2. Never infer routes from function-name text alone.
3. Count only call sites that resolve to canonical registration symbols (or
   wrappers proven to forward directly to those symbols).
4. If a call shape cannot be proven statically, fail as unsupported (false
   negative) instead of guessing (false positive).
5. Canonical selector/dot-import matches must reject shadowed identifiers using
   static identifier binding checks.

Chosen implementation approach:

1. Use symbol-resolved static callgraph analysis rooted at package
   initialization (`var` initializers and `init()`), then follow statically
   resolvable calls.
2. Route discovery anchors on canonical registration symbols and proven wrapper
   forwarding paths.
3. Do not introduce a new marker-return public API unless symbol-resolved
   discovery proves insufficient.

Will match:

1. Direct canonical registration calls.
2. Wrapper/helper calls with arbitrary names when forwarding to canonical
   registration is statically provable.
3. Compile-time string literal/const (including const concatenation)
   method/pattern arguments.

Will not match:

1. Dynamic dispatch (`interface{}` calls, reflection, function values assigned
   at runtime).
2. Runtime-computed method/pattern values.
3. Code paths that are not init-reachable.

### 3) Wave Core Robustness

- Devserver change handling follows explicit `classify -> plan -> execute` flow.
- Rebuild orchestration moves to a fingerprinted artifact DAG.
- Watcher/debouncer model stays bounded under heavy churn.
- CLI/library boundaries avoid hidden global side effects.

### 4) Diagnostics

- `wave explain` and `wave doctor` expose config dependency trigger reasoning.
- `wave explain` and `wave doctor` expose route conflict origin.
- `wave explain` and `wave doctor` expose rebuild plan decisions.

## Current Focus

- Finish Wave-core robustness track
  (`planner -> typed action plan -> artifact DAG`) while preserving locked UX
  decisions.

## Roadmap

### Phase A: Vorma Route and Determinism Model

1. Harden backend symbol-resolved AST discovery coverage and diagnostics
   (wrappers, dot-imports, direct mux registrations, ambiguity failures).
2. Lock strict AST contract for frontend route-definition files and stable
   AST-to-manifest generation ordering.
3. Add strict route conflict diagnostics with source-oriented failures.

### Phase B: Wave Core Planner and DAG

1. Complete pure planner extraction for devserver events.
2. Introduce artifact DAG and node fingerprinting.
3. Replace ad hoc work flags with typed rebuild action plans.

### Phase C: Core API Hardening

1. Tighten public API boundaries to avoid leaking internals.
2. Remove remaining hidden magic/sentinel behavior in core APIs.
3. Keep one obvious default path and remove duplicate config/runtime surfaces.

### Phase D: Conformance and Stress

1. Add UX conformance tests tied directly to the UX contract.
2. Add stress suites for high route count and watcher churn.
3. Add deterministic artifact output checks across repeated runs.

## Immediate Next 6 Steps

1. Expand backend symbol-resolved AST conformance coverage and unsupported-shape
   diagnostics.
2. Lock strict AST rules and fail-fast diagnostics for frontend route-definition
   files.
3. Finish Wave devserver pure planner extraction (`classify -> plan -> execute`)
   and remove remaining ad hoc event branching.
4. Introduce the first artifact-DAG cut with fingerprinted nodes and typed
   rebuild action plans.
5. Expand strict route conflict diagnostics and manifest validation failures.
6. Expand conformance/stress coverage for UX contract, deterministic artifacts,
   and watcher-churn behavior.

## Exit Criteria

- UX contract is enforced by automated conformance tests.
- Route/build determinism is stable under repeated generation runs.
- Wave planner and artifact DAG are the sole rebuild decision path.
- Diagnostics explain why rebuilds/routes decisions happen.
- No side-effect registration dependency exists in route discovery or build
  output generation.
