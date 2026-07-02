# P007 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed — and with it, Phase C is complete.** The verification-first
design did exactly what it exists for: the full production pipeline is proven end to end
(three identical replays; build → honest prod boot → seven served-behavior checks →
graceful shutdown of the P006 worker, zero orphans), and the one defect it found was real,
root-caused to an isolated minimal reproduction, fixed at the board layer with a teaching
comment, and escalated at the framework layer with evidence and ranked options.

## Independent verification performed (Fable)

- The board fix and its teaching comment confirmed in `server.rs` (the ordering rationale
  reads as user-facing middleware guidance, not process language); the framework ticket
  exists with the full trace.
- Gate results match Fable's own P009-review runs exactly (575/0, clippy, fmt, tsgo,
  vitest 851) — the executor's numbers are corroborated by independent runs on the same
  tree state.
- The two "pre-existing dirty" files its fmt check flagged were Fable's own
  freshly-written review artifacts (correctly left alone); normalized by Fable at this
  review, repo-wide fmt check green.
- Report placed by Fable (guardrail).

## Findings

No executor issues found.

## Rulings and dispositions (Fable)

- The `board-prod-smoke` make-target proposal is deferred: it duplicates part of
  `e2e-smoke`'s `test-prod` coverage, and the minimal-tooling rule wants demonstrated
  recurring need first. Recorded here; revisit if prod verification becomes a repeated
  manual chore.
- The etag/timeout remediation decision is the maintainer's (public middleware behavior).
  Fable's recommendation, from the repo's own composition doctrine ("if a lower-level
  primitive is easy to misuse, keep it private and expose a higher-level composition"):
  the composed pre-ordered helper, with an upstream tower-http `size_hint` fix pursued in
  parallel; doc-only is ruled out by the no-footgun-smoothed-by-docs rule.

## Consequence

Phase C is complete: P005, P008, P006, P009, P007 all accepted. Board is census-complete
(F1–F17 landed, F-21/F-22/F-23 resolved or ticketed), teaches the full app-useful surface
including the task lifecycle and multi-value forms, and its production pipeline is proven.
Workspace: 575 tests. Next: Phase D packet authoring (doc-comment sweeps per crate,
ARCHITECTURE.md accuracy pass, thermo-nuclear compliance pass, packaging dry-runs) — plus
the etag remediation packet once the maintainer rules.
