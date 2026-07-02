# P007 — Board production build and serve, verified end to end

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and
`docs/maintainer/board-example/README.md`. Inputs: census F11 (dev-loop realism, server
main, `vorma_build::run`) in
`docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md`, and the roadmap's
Phase C prod-build item.

## Context

Board's production BUILD is known to run (P003 regenerated `vorma.gen.ts` through it), but
"the build command exits 0" is not the feature. The census promises a real app: that means
the produced production artifact set boots, serves real pages from the committed manifest
(HTML with hydration payloads, generated client assets, public assets), and behaves as a
deployed board would. Nobody has verified that end to end on this machine and recorded it.
This packet is verification-first: prove the whole prod path, fix what is genuinely broken
within board's own code, and escalate anything that implicates the framework.

## Scope

1. **Run the full production pipeline for board:** the build binary (`vorma_build::run`)
   producing the production artifact set, then boot the production server binary against
   those artifacts (`is_build`/`is_dev` honest, real `bind_addr`).
2. **Verify served behavior against the running prod server** — request and check, at
   minimum: the front page (HTML + hydration payload present, client build id header), a
   story page (per-story head/meta), a mutation round-trip (vote or login —
   auto-revalidation semantics intact), the docs splat route, a public asset, and the
   branded catch-all fallback. Record the exact commands and outputs in the report.
   Scripted checks belong in the report or a board test if (and only if) they read as a
   good example of testing a Vorma app; do not build speculative harness tooling.
3. **Fix board-owned prod gaps** found by verification (missing assets wiring, config
   mistakes, seed/bootstrap assumptions that only held in dev). Framework-implicated
   failures are escalations with evidence, never worked around.
4. **Teach it:** whatever the prod story requires of an app (build binary, server main
   branches, env expectations) must be readable in board source with teaching comments,
   per the board contract.
5. If a maintainer-facing convenience is genuinely warranted for repeatable verification
   (e.g. a `make` target that builds and smoke-checks board prod), propose it in the
   report with the exact minimal shape — do not add speculative tooling surface
   unilaterally (AGENTS.md minimal-tooling rule).

## Hard constraints

- No framework/public-API changes; escalate with evidence instead.
- Teaching bar for all board-source changes; no process language.
- Ports/processes: clean up every server you start; leave no orphans.
- No git actions; no network installs (everything needed is installed — if the prod
  pipeline itself demands a network fetch, that is a finding to report, not to work
  around); scoped fmt writes only; unexpectedly dirty unowned files are escalations.
- An honest "prod path broken at step X with this evidence" report is success.

## Definition of done

- The full pipeline (build → boot → serve) demonstrated with recorded commands and outputs
  for every check in scope item 2.
- Any board-owned fixes landed at the teaching bar; any framework findings escalated with
  evidence and census F-rows/tickets filed.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check, board tsgo,
  vitest.
- REPORT.md per template (or full content returned in-message if the file-write guardrail
  fires, for the orchestrator to place).
