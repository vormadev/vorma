# P014 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** 433 public items to the teaching bar with the honest audience
framing (this crate's surface is provably unreachable externally except `vorma_build::run`
— documented as such, stronger than "framework-facing"); the flake root-caused to
mechanism with a statistical reproduction rather than fixed outside the grant — exactly
the right restraint, and the grant question came back to Fable where it belonged. Two
zero-behavior direct landings (a stale comment pointing at a nonexistent document,
corrected to the real ticket; a test renamed to match its verified fail-fast semantics).
One self-caught-and-reverted mistake disclosed (inventing public surface to make a doctest
possible — caught, reverted, documented).

## Independent verification performed (Fable)

- Executor's gates: 588/0 workspace (twice), 43 doctests, clippy/fmt/doc-build clean, loom
  7/7 untouched-green, release build clean, audit clean modulo the allowed advisory —
  corroborated by the concurrent P019 session's independent 588/0 run on the same tree
  state.
- The flake diagnosis reviewed: the TOCTOU mechanism is airtight and the
  64-thread/128k-attempt reproduction is the standard of proof this repo wants. **Fable
  ruling recorded on the ticket:** the test-scoped retry-on-collision is the sanctioned
  fix shape (production invariant untouched); standing grant for the next
  vorma-build-touching executor.
- Findings triage: surface/structure items held in `build-release-quality-findings` for
  the batched Phase D-end ruling; cleared items recorded against re-litigation.

## Findings

No executor issues found.

## Consequence

P014 closes. Four of six crates documentation-complete; the flake has a ruled,
fully-specified fix awaiting a carrier. Next: P015 (vorma + vorma-client-wasm — the
largest sweep; its P013 coverage rider was already landed by P019, so it carries none).
