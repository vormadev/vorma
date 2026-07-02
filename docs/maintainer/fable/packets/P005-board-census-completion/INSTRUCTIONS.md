# P005 — Board census completion: audit the feature ledger, land the stragglers

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and
`docs/maintainer/board-example/README.md` (the board contract, including the 2026-07-01
coverage ruling). Your primary input is the census:
`docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md` (features F1–F17 and
the findings ledger). P003's artifact
(`docs/maintainer/tickets/board-api-coverage/INVENTORY_VS_BOARD_P003.md`) covered the API
axis; this packet covers the FEATURE axis — whether each census feature exists in
`examples/board` as described.

## Context

The census is board's feature contract, and it warns about itself: "do not treat every row
as open work without checking current code." Most of F1–F16 is believed landed; the known
stragglers are the two census-named no-dead-surface pairings: the **F9 search view**
(which carries `useApiQuery` + `apiQueryOptions` prefetch composition — the `/api/search`
resource already exists and is tested) and the **F3 attachment import-parse** (which
carries `workIndicator.track` for non-vorma async). The F-17-ruled task-teaching rows are
NOT this packet (they are P006); the prod build is NOT this packet (P007).

## Scope

1. **Row-by-row completion audit.** For each of F1–F17 (and the member-level second
   sweep), verify against current board code: implemented-as-described (cite file:line),
   implemented-differently (describe), or missing. Write the table to
   `docs/maintainer/tickets/board-api-coverage/CENSUS_COMPLETION_P005.md`.
2. **Implement the two known stragglers** per the census's own design language:
    - F9 search view: a `/search` view using the app-owned `useApiQuery(args)` wrapper
      (queryKey via `apiClient.toIdentityArray`, queryFn via `queryOrThrow`) over the
      existing search resource, with `apiQueryOptions(args)` exported and composed for
      prefetch (`prefetchQuery`/`ensureQueryData`), and `useRouteSync({debounceMs})`
      syncing the search box to the URL (census F15 line).
    - F3 import parse: the submit flow's client-side attachment parse (FileReader-class
      non-vorma async) tracked through `workIndicator.track(promise)` so the work bar
      reflects it — the census-designated home for `track`.
3. **Close small gaps the audit finds** (mechanical rows that follow existing board
   patterns and the teaching bar). Anything needing design or framework opinion: escalate
   in the report as a proposed follow-up packet row, and add census F-rows for genuine
   friction per the ledger's conventions.
4. Update the census: mark the F17 exceptions closed; record audit corrections directly in
   the census where a row's description has drifted from reality (the census is a living
   document).

## Hard constraints

- Board is user-facing teaching material: comments teach APIs (why/when), never
  audit/process language. Framework semantic tests stay out of board.
- No framework/public-API changes; friction is recorded (census F-row + ticket), not fixed
  here.
- F-18 discipline: never run `oxfmt --write .` globally; scoped writes or `--check` only.
- No git actions; no network installs (node_modules is current). An unexpectedly dirty
  file you did not author is an escalation, never cleanup.
- An honest incomplete report beats silent judgment calls.

## Definition of done

- `CENSUS_COMPLETION_P005.md` exists with a verdict + citation for every F-row.
- F9 search view and F3 import-parse landed at the teaching bar; `useApiQuery`,
  `apiQueryOptions`, and `workIndicator.track` have their census-designated homes (no dead
  surface).
- Gates green: `cargo test --workspace --all-targets` + `--doc`, clippy `-D warnings`, fmt
  check, board tsgo project, ts-lint, vitest; regenerate `vorma.gen.ts` via the real build
  if shared types changed.
- REPORT.md per the protocol template (if the report-file guardrail blocks the write,
  return the full content in your final message for the orchestrator to place).
