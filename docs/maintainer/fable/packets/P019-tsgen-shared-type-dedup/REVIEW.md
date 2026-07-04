# P019 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** The maintainer-ruled semantic fix landed exactly as granted,
with red-before/green-after proven by revert-in-place for both new pin classes and the
never-goes-red boundary class verified as such. The design choices are right: the
structural equivalence lives at the authoritative layer (graph validation — reachable
without the facade), with the facade as a fail-fast pre-filter that defers
collapse/divergence decisions to the single source of truth; nested named refs compare by
name (never phase-suffixed key), which the executor correctly identified as REQUIRED for
genuinely-shared nested types; and the divergence error text teaches the resolution in one
sentence, captured from real Display output rather than reconstructed.

## Independent verification performed (Fable)

- Fix present at both layers (`classify_shared_name_with` + `SharedTypeNameRelation` in
  contracts.rs; wired in graph_validation.rs and facade.rs); consumed ticket confirmed
  deleted; 13 files (+2020/−44) across exactly the granted crates; lib suites re-run
  green; executor's full workspace run: 588 tests, zero failures; loom untouched-green.
- The board rider's investigate-and-decline is well-evidenced (all 30 derive sites
  enumerated; the 5 Deserialize types all have distinct names; spot-checked pairs are
  intentional domain splits) — no contrived adoption, correctly reported.
- Coverage rider landed (serde-transparent, default-path), with one honest en-route
  finding recorded (serde itself forbids transparent+default-path on one field — not a
  Vorma bug).

## Findings

No executor issues found. (REPORT.md placement pending the executor's re-emit — the full
document was lost to the blocked write; the transmitted summary is authoritative in the
interim.)

## Consequence

P019 closes: shared round-tripping types now boot and export one TS type; genuine phase
divergence errors with a teaching message; the ordinary shared-`User` pattern works. The
P013 checklist's one FAIL (bug-free) is retroactively resolved. Phase D continues: P014 in
flight, then P015/P016/P017/P018 and the batched surface triage.
