# P009 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** Every multi-value `FormData` accessor has a teaching call
site with a genuinely distinct role — the standout being `fields_named` as a group-shape
check versus `texts` as value extraction, real design rather than name-checking — and the
F-21 document-shell call sites landed with a provably-escaping demo value (the `&` in
`known_safe_attribute`'s input makes the trust boundary observable, and a byte-for-byte
body test pins it). The `fields()` sweep closed a real pre-existing validation gap (story
`body` had no length cap). Census F-21/F-22 are LANDED with citations. The executor also
did live-browser verification of the whole flow.

## Independent verification performed (Fable)

- Accessor call sites confirmed present with teaching comments (`resources.rs:151-279`);
  document-shell sites confirmed (`document.rs:31,43`).
- Gates re-run by Fable: 575 workspace tests, zero failures (matches the executor exactly;
  board 28/28); clippy clean; rust fmt clean; board tsgo clean; vitest 851/851; repo-wide
  `make ts-fmt-check` green after Fable normalized its own freshly-placed review-artifact
  files; corruption-signature scan clean.
- Report placed by Fable (guardrail); `tsgen-drafter-oxfmt-idempotency` ticket filed from
  the executor's flagged finding.

## Rulings and endorsements (Fable)

- Endorsed: the schema evolution (board owns its SQLite schema; the stale gitignored dev
  DB deletion is regeneration workflow, not a hand-edit); the nested attachment route with
  the cross-story 404 guard; front-page display correctly out of scope; the honest
  `into_body`-via-clone comment.
- The executor's "Defect 3" addition to the oxfmt ticket (space injection inside code
  spans at wrap boundaries, with pre-existing instances located at
  `PRESSURE_TEST_CENSUS.md:171,173,247,629` and `CENSUS_COMPLETION_P005.md:58-59`) is
  accepted as recorded; the pre-existing instances remain queued as repo-wide cleanup
  alongside the upstream report.

## Findings

No executor issues found.

## Consequence

P009 closes: board's census coverage is now fully landed across F-21/F-22, and the
multi-value form-handling surface teaches end to end (server accessors, client UI,
tamper-rejection tests). Board tests 20 → 28; workspace 575. Next: P007 (prod build
verification) closes Phase C.
