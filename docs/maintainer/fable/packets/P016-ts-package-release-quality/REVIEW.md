# P016 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** The TS surface now matches the Rust side: 176/176 public
exports documented at the teaching bar across 13 entry points, with census F-7's
integration-pattern mandate honored (workIndicator, the api-client family, route/work
state) and F-8's auto-revalidation default stated loudly. The enforcement mechanism is
exactly what the minimal-tooling rule wants — a single checked-in compiler-API script,
zero new dependencies, resolving re-exports to real declarations so docs live once, wired
into ts-gate, and proven to catch regressions rather than assumed. The adapter-divergence
discipline was real: a shallow diff suggested near-identity and a full-file diff then
caught Preact's signal-return semantics — the kind of miss that would have shipped wrong
docs.

## Independent verification performed (Fable)

- The enforcement gate re-run green under Fable's own hand (176/176); both new tickets
  exist; Makefile wiring reviewed (ts-jsdoc-coverage in ts-gate; the new tsconfig in
  ts-typecheck).
- Notable quality moments endorsed: the checker's own first result distrusted and a
  stale-.dist resolution bug fixed before trusting it (149 false positives eliminated by a
  source-anchored paths map); the converters.ts judgment call REVERSED when the checker
  flagged it (the gate's strictness kept over a special-case exemption); the
  bare-re-export documentation limitation solved by documented local re-declarations;
  create-vorma given an honest STALE marker per its standing rewrite ticket instead of
  polished docs on doomed code.
- Generated-types verdict (negative) traced through three layers and confirmed empirically
  — the ticket is decision-ready for the generator work.

## Findings triage (Fable)

Findings 1-3 held in `ts-package-release-quality-findings` for the batch triage (finding 1
— `_index.ts` dropping eight explicitly-public-labeled items — is the standout: dead
public-looking surface one hop from being published, two items proven load-bearing).
Finding 4 (TsGen doc-comment carriage) stands as its own ticket; Fable endorses it as a
strong candidate for the docs-era work since the generated file is most users' first
contact with their own types.

## Findings

No executor issues found.

## Consequence

P016 closes. ALL documentation sweeps are done — Rust and TS surfaces are
documentation-complete with durable enforcement on both sides. Remaining in Phase D: P017
(ARCHITECTURE.md accuracy), P018 (packaging dry-runs), then the batched surface-polish
triage (now six findings tickets + two standalone) to the maintainer.
