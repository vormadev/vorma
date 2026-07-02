# P009 — Board multi-attachment submit (the FormData multi-value home)

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and
`docs/maintainer/board-example/README.md` (the board contract — the "add or invent a Board
feature ... realistic app flow ... say that plainly" clause is this packet's charter).
Inputs: census rows F-22 and its corrected ruling (2026-07-02) in
`docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md`, and the F3 submit
feature as it exists in board today.

## Context

Board's submit flow accepts one optional attachment, so `FormData`'s multi-value accessor
family — `fields()`, `files()`, `fields_named(name)`, `files_named(name)`, `texts(name)`,
`FormFile::into_body` — has no call site anywhere in the repo's app code. The corrected
ruling: these are app-useful primitives and board owes them a home via the
census-sanctioned mechanism — a realistic invented feature. The feature may exist
primarily to demonstrate the API; the code must say so plainly in user-facing terms and
teach when an application reaches for each accessor.

## Scope

1. Extend story submit to accept MULTIPLE attachments under one repeated field name
   (standard `<input type="file" multiple>` semantics on the wire), read server-side via
   `files_named` — teaching the repeated-name model multipart is built on.
2. Add a repeated-value non-file group to the same form (e.g. a tag/topic checkbox set)
   read via `texts(name)` — the repeated text counterpart.
3. Use `FormFile::into_body` where ownership of the file body is wanted (storing
   attachment bytes), and the grouped/aggregate accessors (`fields`/`files`) where they
   read naturally (e.g. validation that iterates everything submitted). EVERY accessor in
   the multi-value family gets a teaching call site — design the feature so each lands as
   the natural way to write that code (that is the design work of this packet). "No honest
   home" is not an available outcome: board is a teaching tool, contrived by design (see
   the board README's 2026-07-02 ruling). If you conclude an accessor cannot be used
   sensibly AT ALL, that is evidence of an API-design defect — stop and escalate with the
   analysis; never record a waiver.
4. F-21 reversal (ruled 2026-07-02): `DocumentAttributes::boolean_attribute` and
   `known_safe_attribute` are app-facing document APIs and get board call sites in the
   document shell — e.g. a boolean attribute rendered name-only (teach what "boolean
   attribute" means in HTML) and a known-safe attribute with a teaching comment on the
   trust boundary (when bypassing escaping is safe and when it never is). The
   `public_api.rs` usability coverage from the P008 rider stays as well; it is additional,
   not a substitute.
5. Client side: the submit view gains the multiple-file input and the checkbox group;
   attachment display wherever stories already show their attachment. Keep the UI minimal
   — this is a form-handling teaching feature, not a design project.
6. Board tests: extend the multipart submit tests to repeated fields (multiple files under
   one name, the checkbox group, the validation rejection path) — as always, written to
   teach app testing.
7. Census: update F-21/F-22 and the completion table (`CENSUS_COMPLETION_P005.md`) to
   landed-with-citations.

## Hard constraints

- No framework/public-API changes (friction = census F-row + ticket, never a fix here).
  Storage/schema changes inside board are fine (it owns its SQLite schema).
- Teaching bar throughout; comments explain when an app reaches for each accessor, in
  user-facing terms, no audit language.
- Regenerate `vorma.gen.ts` via the real board production build if shared types change;
  never hand-edit it.
- Scoped fmt writes only; no git actions; no network installs; unexpectedly dirty unowned
  files are escalations, never cleanup.

## Definition of done

- The feature works end to end (submit with several attachments + tags; stored; rendered),
  every multi-value accessor has an honest cited call site or a recorded census note for
  why not.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check, board tsgo,
  ts-lint, vitest.
- REPORT.md per template (or full content returned in-message if the report-file guardrail
  fires, for the orchestrator to place).
