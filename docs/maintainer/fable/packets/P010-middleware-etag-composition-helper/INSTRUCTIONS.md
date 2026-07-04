# P010 — Middleware composition helper: make the etag/timeout footgun unrepresentable

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, and `docs/maintainer/REMINDERS.md`.
Inputs: ticket `docs/maintainer/tickets/etag-response-body-timeout-ordering-footgun/` (the
full root-cause trace, reproduction, and option analysis — its option 4 is what the
maintainer approved 2026-07-02), census F-23, and board's fixed call site in
`examples/board/src/bin/server.rs` (the teaching comment there explains the ordering).

## Context and authority

P007 proved that composing `vorma::middleware::etag()` inner to `response_body_timeout()`
— the order an app author naturally writes — silently produces zero ETags: the timeout
layer's body wrapper discards the exact `size_hint` the etag layer requires. The
maintainer approved the composed pre-ordered helper as the remediation (public middleware
surface — this approval is the packet's grant), ruled doc-only out, and deferred the
upstream tower-http fix to ticket `tower-http-timeoutbody-size-hint-upstream` (do NOT
touch anything upstream-facing).

## Scope

1. **Verify first.** Confirm the root cause against the vendored tower-http source in this
   tree (the trace is P007's; re-prove it here — read `TimeoutBody`'s Body impl and record
   the finding in your report AND as a verification note in the deferred upstream ticket).
2. **Design and land the composed helper** in `vorma::middleware`: a single public
   function returning the hint-sensitive layers pre-composed in the correct order, so an
   app that wants both cannot get the silent-degradation ordering. Design constraints, per
   AGENTS.md: intentional, non-leaky (no third-party types in the public signature — mind
   what the composed layer type exposes; if tower types would leak, wrap or rethink),
   footgun-free, and named so the behavior is impossible to misread (long and explicit
   beats short). Decide its exact granularity from the ticket's option-4 analysis and the
   real composition in board's server main — the helper must cover the proven-dangerous
   pair; whether it composes more of the standard stack is your design call to propose,
   with reasoning, sized against the no-speculative-surface rule. The individual layers
   remain public and usable (apps that compose by hand keep working; board's fixed
   hand-ordering stays valid).
3. **Pin it.** A test proving the helper's composition yields ETags on responses that flow
   through the timeout wrapper (the exact scenario that silently failed), plus whatever
   unit pins the helper's own contract needs. Do not pin third-party internals (the bad
   order's failure belongs to the ticket's repro, not the suite).
4. **Teach it.** Board's server main adopts the helper for the covered pair, with the
   teaching comment updated: why the helper exists, when to compose by hand instead.
   Rust-doc on the helper carries the same story for non-board readers.
5. Census F-23 updated to resolved-with-citation; the
   `etag-response-body-timeout-ordering-footgun` ticket deleted on completion (per
   tickets/README.md); the deferred upstream ticket gains your verification note but
   otherwise stays untouched and unfiled.

## Hard constraints

- Public surface changes limited to the granted helper (and its doc). No changes to the
  existing individual middleware functions' signatures or behavior.
- No upstream-facing action of any kind (no issues, no PRs, no patches to vendored
  sources).
- No new dependencies; tests never cheat; teaching bar for board changes; scoped fmt
  writes only (formatter hazards documented in
  `oxfmt-markdown-corruption-and-nonconvergence` — fenced blocks, no bold spanning code
  spans, diff after writes); no git actions; no network installs; unexpectedly dirty
  unowned files are escalations, never cleanup.

## Definition of done

- Root cause re-verified against vendored source and recorded.
- Helper landed with pins; board adopted; census/tickets updated as specified.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check, board tsgo,
  vitest; `make loom-tasks` untouched-green.
- REPORT.md per template (or full content returned in-message if the report-file guardrail
  fires), including the exact final public signature for the review record.
