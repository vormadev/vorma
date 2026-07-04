# P017 — ARCHITECTURE.md accuracy pass against shipped code

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and the
document under review: `docs/maintainer/ARCHITECTURE.md`.

## Context

AGENTS.md: architecture-level changes must update ARCHITECTURE.md in the same change so it
does not drift. A large volume of architecture-adjacent work has landed since it was last
audited (P002 fingerprinting/poll-once, P004 engine fragments + single-invocation fast
path, P008 exits/testing surface, P010 middleware composition, P019 shared-type dedup,
P020 app! expansion shape, the board maintenance-worker pattern). This packet audits every
claim in ARCHITECTURE.md against the shipped code and corrects drift.

## Scope

1. Claim-by-claim verification: for each assertion in ARCHITECTURE.md, verify against
   current source (cite file:line in your report per claim: accurate / drifted / missing).
   The Phase D packet REPORTs are your map of what changed.
2. Correct drifted claims in place; add missing architecture-level facts introduced by the
   accepted packets (concise — this is a map, not a manual; rust-doc is the
   documentation).
3. Structural findings about the DOCUMENT (organization, staleness patterns) may land
   directly; anything implying CODE changes is a finding for the batch triage.

## Hard constraints

- Docs-only packet: zero source-file changes. Tests never touched. Scoped fmt writes only
  (formatter hazards in ticket oxfmt-markdown-corruption-and-nonconvergence: fenced blocks
  only, no bold spanning code spans, diff after writes, verify convergence). No git
  actions; no network installs; unexpectedly dirty unowned files are escalations, never
  cleanup.

## Definition of done

- Every ARCHITECTURE.md claim verified with the per-claim table in the report; corrections
  landed; `make ts-fmt-check` green.
- REPORT.md content delivered IN the final message body.
