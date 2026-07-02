# P005 Review

Reviewer: Fable (orchestrator). Date: 2026-07-01. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** The audit is the deliverable and it is excellent: every F-row
adjudicated with citations, census drift corrected in place per the packet's instruction,
the two stragglers landed at the teaching bar, and the two narrow real gaps escalated with
honest reasoning instead of contrived coverage. Board's census status after P005: **12/17
implemented as described, 3 census-text drift corrections, 0 missing.**

## Independent verification performed (Fable)

- Change surface: exactly two board client files (+52/−9) plus the audit artifact and
  census updates; `git diff --stat -- crates/` is empty, proving the Rust-untouched claim
  (Rust gates therefore stand from the prior recorded runs).
- Both diffs code-reviewed: the two-caches teaching (Vorma `prefetch()` vs react-query
  `prefetchQuery`/`ensureQueryData` on the same intent signals) is precise and
  user-facing; naming follows repo casing rules; the intent-handler extraction is properly
  DRY; the honest "react-query has no prefetch-cancellation to mirror" note is exactly the
  kind of comment the board contract wants.
- Scoped gates re-run by Fable: board tsgo exit 0, full-repo oxlint zero diagnostics,
  vitest 851/851.
- Audit artifact (`CENSUS_COMPLETION_P005.md`) spot-checked; census corrections read as
  corrections with inline notes, not silent rewrites.
- REPORT.md placed by Fable (the report-file guardrail fired again; content returned
  in-message per the packet's contingency instruction).

## Rulings issued at this review (Fable, per the working-with-the-maintainer rule)

- **F-21 accepted as recommended:** `DocumentAttributes::known_safe_attribute`/
  `boolean_attribute` are discharged by `crates/vorma/tests/public_api.rs`; the coverage
  lands as a rider added to P008 (which already works in that test suite).
- **F-22 accepted as recorded:** the multi-value `FormData` accessors get a board home
  only if/when multi-attachment submit becomes a real feature; census row suffices, no
  ticket directory.
- **Stale ticket deleted:** `ts-lint-react-redundant-type-warnings` — the executor
  observed, and Fable re-verified, zero oxlint diagnostics at HEAD (the ticket's
  verification condition is met); if type-aware warnings ever return, a fresh ticket
  captures fresh facts.

## Findings

No issues found. (The executor's escalation of Fable's own concurrent orchestration edits
as unexpectedly-dirty-unowned-state was the protocol working exactly as directed —
observed, preserved, reported, untouched.)

## Consequence

P005 closes. Next per the Phase C dispatch order: P008 (ruled API additions + the F-21
rider), then P006 (task-runtime teaching, written with the new `?` idiom), then P007 (prod
build verification).
