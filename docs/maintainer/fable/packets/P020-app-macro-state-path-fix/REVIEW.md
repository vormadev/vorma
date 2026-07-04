# P020 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** The Fable-authorized footgun fix landed exactly as the grant
specified — accepted-input widening only, existing callers proven byte-unchanged (the
strongest form of that pin: old code untouched and green, plus explicit checks of the two
real-app callers) — and the packet's standout is Decision 4: the doctest gate caught a
genuine, subtle interaction (rustdoc's synthetic-main wrapping vs Rust's function-body
module-tree rule), which was root-caused with an isolated rustc repro, proven to affect no
real caller, and fixed at the doctest with the repo's own convention rather than
contorting the macro.

## Independent verification performed (Fable)

- The alias present in the macro (both emission and `super::` reference); the pin file
  runs 3/3 in isolation under Fable's own re-run; findings ticket shows Finding 1 RESOLVED
  with 2/3 held.
- Gate numbers consistent: 591/0 (+3 over P015's 588), doctests 52/0, loom 7/7.
- Judgment calls endorsed: the location correction (packet phrasing was Fable's own loose
  shorthand — the finding's citation was authoritative), private-no-pub alias,
  all-three-forms pinning, existing-callers-untouched reasoning.

## Findings

No executor issues found.

## Consequence

P020 closes; the `app!` state-path footgun is dead by design, not by documentation. In
flight: P016 (TS/jsdoc). Remaining: P017, P018, the batched surface triage.
