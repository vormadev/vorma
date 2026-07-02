# P008 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed.** Both granted surfaces landed exactly within their grants, the
Cancelled conversion is safe by traced argument (recorded as an in-code doctrine comment
where the next reader needs it) and pinned positionally in real-engine tests, and the
executor caught and fixed a genuine correctness bug in its own first draft before
reporting (the duplicate `Cookie:` header trap — `HeaderMap::get` returns only the first
repeated header, so jar + explicit cookies now merge into one header at `send()`, pinned
by two tests).

## Independent verification performed (Fable)

- Change surface matches the report exactly; `vorma-tasks` zero-diff vs HEAD
  (untouched-green claim proven); board `views.rs` retains exactly 3 `map_err` sites — the
  kept teaching contrast plus the two out-of-grant `vorma::Error` sites.
- `exit.rs` impls read: concrete only; `Failed(source)` attaches the application error
  whose own chain stays intact (pinned by source-traversal tests); payload-free variants
  use `Display` — no special case for `Cancelled`, with the reachability doctrine above
  the impls.
- Gates re-run by Fable: workspace tests zero failures (31 ok suites; 567 tests per the
  executor's runs, +16 over baseline), clippy clean, fmt clean, loom 7/7 (file-captured,
  exit 0).
- Grant discipline verified: no public surface beyond the grants (the F-21 rider
  deliberately asserted through existing crate-internal routes rather than exposing a new
  renderer path).
- Judgment calls reviewed and endorsed: the stale-token logout test correctly kept the
  stateless cookie form (a session jar would have honestly forgotten the replayed token
  and silently weakened the test); the shared `view_payload_uri` helper is proper DRY.

## Findings

No issues found.

## Consequence

P008 closes: `?` now works on task errors in handlers with chains preserved, and the
testing surface has its cookie-continuation story (`TestSession`, `TestResponseCookies`).
Census F-2/F-3/F-21 updated; both consumed tickets deleted. Next per the Phase C order:
P006 (task-runtime teaching, now written with the new `?` idiom), then P009, then P007.
