# P003 — Board 100% API Coverage Audit

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md` and `STATE.md`. Repo-root `AGENTS.md` binds you — its "Examples And API
Coverage" section is the policy this packet executes. Prerequisites: P001 accepted; P002
not required.

## Context

`examples/board` is the canonical teaching example and living API pressure-test app,
required to cover 100% of public Vorma APIs (policy in `AGENTS.md` and
`docs/maintainer/board-example/README.md`). The notes example was deleted under that
policy, orphaning coverage that only it exercised — known orphans: an `extended_cache`
task used through a real app, and a request-level test suite with patterns board's tests
do not all replicate. The standing inputs live in
`docs/maintainer/tickets/board-api-coverage/` (including `PRESSURE_TEST_CENSUS.md`) and
`docs/maintainer/tickets/api-inventory-tooling/`.

## Scope

1. Produce the public API inventory: every public item of the `vorma` crate (including the
   `vorma::tasks` module surface: the `task!` macro policies, `ParallelBatch`,
   observers/overrides/clock, `CancelToken`), the app-declaration macros, `ResourceBody`,
   and the generated-TS-facing surface. Use or extend the approach in the
   `api-inventory-tooling` ticket; record the method in the report.
2. Cross the inventory against board: for each item, cite where board exercises it (file
   and line), or mark it uncovered.
3. For uncovered items, produce a coverage plan table: item, proposed board home, teaching
   angle (board comments teach — see the policy), size estimate. Include the notes orphans
   explicitly (an `extended_cache` task fits board naturally — for example a cached
   dashboard/stats read; the request-level test patterns fold into board's existing test
   suite style).
4. Implement the uncontroversial rows (mechanical additions that follow existing board
   patterns). Escalate any row that requires new framework opinion, awkward fits, or API
   friction discoveries — friction findings are census material; add them to the census
   file with the ticket's conventions.

## Hard constraints

- Board is user-facing teaching material: no maintainer/process notes in board source or
  board README; comments teach the API, not the audit.
- Framework semantic tests belong in framework-owned suites, not board tests (policy). If
  you find a semantic gap while auditing, ticket it — do not write a framework test inside
  board.
- No public API changes to any crate. API friction found here is recorded, not fixed.
- Full gate green when done (including `make ts-gate` — board has generated TS).

## Definition of done

- The inventory-vs-board table exists in the report (or as a committed artifact under the
  `board-api-coverage` ticket directory, referenced from the report).
- Every inventory item is either covered (with citation), newly covered by this packet, or
  escalated with a reason.
- Gate green; report per template; census updated for any friction findings.
