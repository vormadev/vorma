# P013 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed** — with its headline being exactly what a thermo-nuclear pass
exists to produce: a confirmed, empirically reproduced framework bug
(`tsgen-shared-type-phase-name-collision`) that every prior suite missed because no test
ever used one named type as both a route input and a route output. The checklist verdict
honestly FAILs bug-free on it rather than glossing; the doc sweep documents the limitation
where users would meet it; the escalation is design-decision-shaped and correctly not
"fixed" unilaterally.

## Independent verification performed (Fable)

- Surface: 11 files, both crates only (+1139/−26); the bug ticket exists.
- Test counts re-verified by Fable: 581 all-targets + 41 doctests = 622, zero failures;
  trybuild pins untouched.
- The direct landing (matcher-builder consolidation) is the shared-util checklist item
  executed, private-only.
- Doc-accuracy corrections (input_schema claim; clone-cost attribution) are the
  pre-existing-comments-are-not-infallible rule applied with traces.

## Findings triage (Fable)

- Finding 1 (the bug) goes to the maintainer NOW, decision-ready — it is a real design
  bug, not surface polish, and does not join the batched triage.
- Findings 2 (coverage gaps) — mechanical test additions; folded as a rider candidate for
  P015 (the vorma-crate pass, which owns the facade tests) rather than a separate packet.
- Findings 3-6 recorded; 3 joins the batched Phase D-end triage via the report (no
  separate ticket needed — cleared items 4/6/7 are recorded against re-litigation).

## Findings

No executor issues found.

## Consequence

P013 closes. Three of six crates documentation-complete. Next: P014 (vorma-build), with
the maintainer's name-collision ruling running in parallel whenever it arrives.
