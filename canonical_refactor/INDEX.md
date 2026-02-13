# Canonical Refactor Hub

This directory is the single source of truth for the refactor program.
Superseded planning files were removed and folded into this hub.

## Branch And Commit Checkpoints

- `main`: `0a94922d` (`2026-01-21`)
- UX/DX floor commit: `4dc9191a` (`2026-01-06`, `v0.83.0`)
- stable refactor branch checkpoint: `refactor-2026-1` at `446e64aa`
  (`2026-02-07`)
- active refactor branch checkpoint: `refactor-2026-7` at `06e98cfe`
  (`2026-02-12`)

## Mandatory Update Protocol

- At the end of every refactor work leg, update this hub.
- Update `BREAKING_CHANGES_LEDGER.md` for any new or changed public behavior.
  Future plans must remain under `not-yet-accepted` until explicitly accepted.
- Update `ACTIVE_EXECUTION_PLAN.md` when focus, tracks, or audit findings
  change.
- Update `API_SIMPLIFICATION_AUDIT.md` when new API findings appear.
- Keep all docs forward-looking and decision-oriented; avoid changelog-style
  narrative.
- Keep Wave configuration direction consistent: JSON-only authored config.

## File Map

- `RULES_AND_GUARDRAILS.md`: non-negotiable constraints.
- `LOCKED_DECISIONS_AND_ARCHITECTURE.md`: locked decisions and architecture
  rationale.
- `ACTIVE_EXECUTION_PLAN.md`: active refactor tracks and Wave audit findings.
- `IDEA_BACKLOG.md`: future candidates and zany ideas backlog.
- `CLIENT_RUNTIME_TRACK.md`: client runtime-specific plan and validation gates.
- `API_SIMPLIFICATION_AUDIT.md`: public API simplification analysis.
- `BREAKING_CHANGES_LEDGER.md`: next-release breaking changes against `main`.
