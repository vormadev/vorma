# Wave Configuration Plan (JSON-First)

Parent roadmap: `FRAMEWORK_IDEAL_END_STATE_PLAN.md`.

## Handoff Freshness Rules

- This file is a focused sub-plan for Wave configuration work.
- Canonical cross-framework execution order lives in
  `FRAMEWORK_IDEAL_END_STATE_PLAN.md`.
- Keep this file future-looking only.
- Do not add implemented/completed/changelog logs.

## Objective

Keep JSON as the default authored config UX while improving framework internals
and deployment flexibility.

## Hard Constraints

- JSON remains the default authoring surface.
- Config changes must not increase route-authoring ceremony.
- Runtime determinism and reload correctness are framework responsibilities.
- Optional advanced config paths are allowed only when they do not degrade the
  default path.

## Target Configuration Model

- Primary authored input: `wave.config.json`.
- Strict schema validation with clear source-oriented diagnostics.
- Deterministic normalization into a single internal config IR.
- Explicit config dependency tracking for reliable reload behavior.
- Optional provider/config adapters remain opt-in, not default.

## Open Work

### 1) JSON Path Clarity

1. Simplify JSON surface names and defaults where confusing.
2. Ensure every config field maps to one clear runtime behavior.
3. Remove any duplicated or overlapping config knobs.

### 2) Dependency and Reload Correctness

1. Make config dependency declarations explicit and easy to verify.
2. Ensure edits to declared dependencies always trigger rebuild/restart.
3. Add diagnostics that explain which dependency triggered each rebuild.

### 3) Packaging and Asset Strategy

1. Keep embedding optional for production builds.
2. Keep fast non-embed dev path by default.
3. Ensure deployment adapters (for example Vercel vs Docker) can choose embed or
   filesystem strategies without extra app boilerplate.

### 4) Developer Experience

1. Keep Wave app bootstrap path concise and obvious.
2. Remove config boilerplate that does not add real flexibility.
3. Keep one default path while preserving low-level escape hatches.

### 5) Test and Conformance Coverage

1. Add matrix tests across JSON edits, dependency-triggered reloads, and
   deployment packaging modes.
2. Add deterministic output checks for repeated config-load cycles.
3. Add failure-mode tests for malformed configs and dependency drift.

## Decisions To Lock

1. Final dependency declaration shape for reload triggers.
2. Final embed/filesystem selection mechanism exposed to applications.
3. Final diagnostic output format for config/reload reasoning.

## Immediate Config Next Steps

1. Lock one explicit dependency declaration shape for JSON-authored config.
2. Lock one explicit embed/filesystem packaging selection API with no duplicated
   default paths.
3. Add diagnostics output contracts for reload-trigger reasoning.
4. Add conformance tests for dependency-triggered reload behavior and packaging
   mode coverage.

## Exit Criteria

- JSON-first configuration is concise, validated, and deterministic.
- Reload behavior is predictable and explainable from diagnostics.
- Deployment packaging choices are flexible without default-path boilerplate.
- Config path aligns with the global UX contract in
  `FRAMEWORK_IDEAL_END_STATE_PLAN.md`.
