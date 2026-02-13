# Maybe Proposals Ledger

Last updated: 2026-02-13 Owner: active agent

Purpose: keep a single durable record of major proposals discussed during the
aggressive Wave/Vorma refactor, including what was accepted, rejected, deferred,
or partially implemented, so context is not lost across handoffs.

Hard UX/DX guardrail (non-negotiable):

- Baseline UX floor is commit `4dc9191a`.
- One backend route must not require editing a second backend file.
- Full-stack route addition should be backend route file + frontend route file,
  not extra registration boilerplate files.

## Proposal Matrix

| ID  | Proposal                                                               | Decision                                                          | Implementation Status        | DX Guardrail                                                                         |
| --- | ---------------------------------------------------------------------- | ----------------------------------------------------------------- | ---------------------------- | ------------------------------------------------------------------------------------ |
| P01 | One composition manifest as single source of truth for runtime + build | Keep, but only at feature/module inclusion level                  | Partial                      | Must not add per-route wrapper ceremony                                              |
| P02 | First-class module contract (`Module`, `DependsOn`, hooks, patterns)   | Not default path; keep only if it stays optional and low-ceremony | Partial internals exist      | Must not force multi-file route registration                                         |
| P03 | Strict route DSL + strict manifest generation (no silent skips)        | Keep                                                              | Partial                      | Dynamic/unresolved route defs should fail build, not warn-and-skip                   |
| P04 | Route IDs as primary key; pattern as metadata                          | Reject for now                                                    | Not implemented              | Pattern-first remains easier to grep and reason about                                |
| P05 | Explicit lifecycle phases with deterministic order                     | Keep internally                                                   | Partial                      | Internal determinism only; no extra app-builder burden                               |
| P06 | Typed immutable context extension pipeline                             | Defer                                                             | Not implemented              | Keep current simple typed ctx decorators unless new model is strictly simpler        |
| P07 | Structural large-app package boundaries                                | Keep                                                              | Not implemented              | Enforce via checks/tooling, not extra route ceremony                                 |
| P08 | Explicit app client instance (`createAppClient`)                       | Defer                                                             | Not implemented              | Do not regress current simple global helper path until replacement is equally simple |
| P09 | Unify loader/query/mutation into one operation primitive               | Reject for now                                                    | Not implemented              | Current explicit surface is clearer for app authors                                  |
| P10 | Remove all implicit singleton behavior                                 | Defer                                                             | Partial                      | Only do if zero UX regression                                                        |
| P11 | Config-as-code default                                                 | Rejected                                                          | Not target                   | JSON-first config is the chosen default DX                                           |
| P12 | Explicit transport/serialization codec contracts                       | Defer                                                             | Not implemented              | Avoid introducing complexity before clear user need                                  |
| P13 | Stable plugin API as kernel boundary                                   | Defer                                                             | Not implemented              | Internal cleanup first, public plugin contract later                                 |
| P14 | Wave devserver pure planning pipeline (`classify -> plan -> execute`)  | Keep                                                              | Partial                      | Internal robustness refactor; no DX regression                                       |
| P15 | Artifact DAG with fingerprinted incremental rebuild nodes              | Keep                                                              | Not implemented              | Internal performance/scaling gain only                                               |
| P16 | `wave explain` / `wave doctor` diagnostics                             | Keep                                                              | Not implemented              | Must improve clarity for large-app debugging                                         |
| P17 | Remove magic `"DevBuildHook"` command sentinel in hooks                | Keep                                                              | Not implemented              | Replace hidden string behavior with explicit typed field(s)                          |
| P18 | Hide dev/prod split behind single entrypoint                           | Keep                                                              | Implemented in scaffold path | Users should not maintain dev/prod Wave plumbing files                               |

## Locked Rejections

These are intentionally rejected unless explicitly revisited:

- Route IDs as primary identity for everything.
- Config-as-code as the default authoring path.
- Any framework-default route registration model that requires touching
  additional backend registration files per route.

## Active Priorities

Priority order for remaining high-value work:

1. Finish strict route parsing behavior for frontend route definitions.
2. Continue Wave internal planner/executor hardening.
3. Remove `DevBuildHook` magic string semantics.
4. Add diagnostics (`wave explain`, `wave doctor`).
5. Design artifact DAG refactor.

## Update Rules

When a proposal changes state:

1. Update `Decision`.
2. Update `Implementation Status`.
3. Add one short note in commit/PR description referencing the proposal ID.
