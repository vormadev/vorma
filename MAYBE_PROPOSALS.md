# Maybe Proposals Backlog

Purpose: keep a forward-looking list of candidate framework changes so we do not
lose ideas while refactoring.

## Handoff Freshness Rules

- This file tracks candidate ideas only.
- It is not the canonical execution order.
- Canonical execution order lives in `FRAMEWORK_IDEAL_END_STATE_PLAN.md`.
- Keep this file future-looking only.
- Do not add implemented/completed/changelog logs.

## Hard UX/DX Guardrails

- Baseline UX floor is commit `4dc9191a`.
- Adding one backend route must not require editing a second backend file.
- Adding one full-stack route should be backend route file + frontend route
  file, without extra backend registration ceremony.
- Determinism must be enforced internally by tooling, not pushed onto app
  authors via extra boilerplate.
- Framework should not enforce how applications organize their internal package
  structure.

## Active Candidates

| ID  | Proposal                                                      | Current Direction | Acceptance Gate                                                                 |
| --- | ------------------------------------------------------------- | ----------------- | ------------------------------------------------------------------------------- |
| P03 | Strict frontend route DSL + strict manifest generation        | Keep              | Unresolved/dynamic frontend route definitions fail fast with clear diagnostics  |
| P05 | Explicit internal lifecycle phases                            | Keep internally   | No user-facing ceremony increase                                                |
| P14 | Wave devserver pure pipeline (`classify -> plan -> execute`)  | Keep              | Clear deterministic planner output and simpler reasoning in core tooling        |
| P15 | Artifact DAG with fingerprinted incremental rebuild nodes     | Keep              | Large route-count rebuild cost drops without changing app authoring shape       |
| P19 | Central diagnostics (`wave explain`, `wave doctor`) expansion | Keep              | Diagnostics must explain triggers, route conflicts, and rebuild reasons clearly |

## Parked For Revisit

| ID  | Proposal                                         | Revisit Condition                                                                       |
| --- | ------------------------------------------------ | --------------------------------------------------------------------------------------- |
| P06 | Typed immutable context extension pipeline       | Revisit only if it is simpler than current typed context decorators for common app code |
| P08 | Explicit app client instance (`createAppClient`) | Revisit only if default usage is at least as simple as current global helper path       |
| P10 | Remove all implicit singleton behavior           | Revisit only with zero regression to default-path ergonomics                            |
| P12 | Explicit transport/serialization codec contracts | Revisit when concrete user demand exists and shape remains low-ceremony                 |
| P13 | Stable plugin API as kernel boundary             | Revisit after core internal boundaries settle                                           |

## Rejected Unless Reopened

- Route IDs as framework-primary identity for default authoring.
- Unifying loader/query/mutation into one default operation primitive.
- Switching the default authored configuration path away from JSON.
- Feature/module composition manifest as a default or required framework model.
- First-class module-contract API as a default or required framework model.
- Helper-name-based backend route discovery (for example scanning only
  `NewLoader`/`NewAction` call names).
- Any default route model that requires editing extra backend registration files
  for each route.
- Any framework-enforced application package-organization model.

## Next Decisions To Make

1. Finalize strict AST route-definition constraints and explicit failure modes.
2. Define the first artifact-DAG cut that delivers measurable rebuild wins.
3. Define the next diagnostics expansion cut for route/codegen/rebuild
   reasoning.
4. Keep this proposal backlog synchronized with locked decisions in
   `FRAMEWORK_IDEAL_END_STATE_PLAN.md`.
