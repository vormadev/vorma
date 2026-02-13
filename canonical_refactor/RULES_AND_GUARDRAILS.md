# Refactor Rules And Guardrails

## Non-Negotiable Product Rules

- We are sub-`1.0`; breaking API changes are allowed and expected.
- We do not accept UX/DX regression for app authors versus `4dc9191a`.
- Adding one backend route must stay a one-backend-file edit.
- Adding one full-stack route must stay backend route file plus frontend route
  file only.
- Determinism is framework/tooling responsibility, not app-author boilerplate.
- No side-effect import dependency as a correctness requirement.
- No framework-enforced application package-organization model.
- Keep one obvious default path for common tasks.
- Escape hatches stay available for advanced use.

## Design And Code Quality Rules

- No builder-pattern APIs in Go.
- No duplicate default-path APIs for the same task.
- No back-compat adapters while sub-`1.0`.
- No “document the footgun” strategy; fix naming and design directly.
- Prefer explicit decision/execution phases over hidden branching.
- Stay DRY for complex logic.
- Dangerous operations must have explicit, impossible-to-misread names.

## Configuration And Naming Rules

- JSON is the sole authored Wave configuration path.
- Do not add or preserve app-facing Go config-as-code alternatives.
- Any potentially conflicting symbols/namespaces must be overrideable by app
  config.
- Defaults must avoid presumptuous generic names.

## Refactor Documentation Rules

- This canonical folder is the only refactor planning authority.
- Keep docs future-looking and execution-oriented.
- Record rejected ideas explicitly so they do not re-enter by accident.
