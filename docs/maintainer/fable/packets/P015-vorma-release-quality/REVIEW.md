# P015 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent (session
survived an accidental machine restart; resumed with state verified intact).

## Verdict

**Accepted. Packet closed.** The framework's entire app-facing surface is now at the
teaching bar (doctests 2→11), the client-wasm ABI has a real protocol contract, and the
packet's discipline was exemplary under adverse conditions: the request-path consolidation
carried P004's full byte-identity bar (7-response capture/diff, zero delta, three
no-regression bench runs), two of its own doc drafts were caught wrong against source
before shipping, and the restart interruption was absorbed without loss.

## Independent verification performed (Fable)

- Surface consistent (17 files incl. the two consolidation targets; +1276/−139 tree total
  in the touched areas); `lock_effects` helper present; vorma doctests 11/11 re-run green;
  findings ticket exists.
- Gate numbers corroborated across the session's independent runs (588/0 recurring).
- The "injection" disclosure re-diagnosed: the artifact matches the harness's genuine
  midnight date-rollover reminder (the engagement crossed into 2026-07-02); the disclosure
  instinct is exactly right, the diagnosis benign. No action.

## Rulings and dispositions (Fable)

- **Finding 1 (`app!` footgun): fix AUTHORIZED as packet P020** — doctrine-compelled
  (no-doc-smoothed-footguns), fix verified in isolation, purely widens accepted input with
  zero impact on existing callers. The interim doc warning converts to a plain statement
  of supported forms once the fix lands.
- **Finding 2 (dead identity surface): held for the batched Phase D-end triage** —
  public-surface deletion is the maintainer's; the recommendation (delete + re-point the
  two tests) goes in the batch with my endorsement.
- **Finding 4: the flake ruling's scope is AMENDED on the ticket** — the sanctioned
  test-scoped fix covers both port-allocation shapes' test callers.
- Finding 3's rejection recorded against re-litigation, endorsed.

## Findings

No executor issues found.

## Consequence

P015 closes. Five of six release-quality passes done; the Rust surface is
documentation-complete end to end. Next: P016 (TS/jsdoc) and P020 (the app! fix) dispatch
in parallel (disjoint surfaces); then P017/P018 and the batched triage.
