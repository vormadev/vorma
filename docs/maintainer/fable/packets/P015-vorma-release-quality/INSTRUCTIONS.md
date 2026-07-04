# P015 — vorma + vorma-client-wasm release-quality pass

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and the
review bar: `docs/maintainer/skills/thermo-nuclear-system-review/SKILL.md`. Model packets:
P011-P014 (same shape). This is the LARGEST sweep: the `vorma` crate is the framework's
entire app-facing surface (the P003/P005 census inventory —
`docs/maintainer/tickets/board-api-coverage/INVENTORY_VS_BOARD_P003.md` — enumerates it);
`vorma-client-wasm` is small (the matcher compiled to WASM).

## Context

Same Phase D shape. History that binds: P004's engine optimizations (precomputed
fragments, single-invocation fast path — doctrine comments load-bearing), P008's exit
conversions + TestSession, P010's middleware composition helper (its doc carries the
footgun story), P019's shared-type dedup (its docs are fresh — do not rewrite, only
cross-link). Standing tickets to reference not duplicate: engine-view-output-ownership,
contract-borrowed-element-construction, wasm-artifact-binaryen-version-pinning.

## Scope

1. **Doc sweep, both crates.** Every public item to the teaching bar; app-facing surface
   gets the full treatment (ctx types, exits, request/response types, head/ document
   builders, testing surface incl. TestSession, middleware incl. the composed helper, the
   app!/view!/resource! macros' docs, the tasks re-export module framing); deny attributes
   where absent; doctests where cheap and deterministic (TestApp-based doctests are fine
   if fast; nothing that binds ports or spawns processes).
2. **Thermo-nuclear review, FINDINGS MODE.** Direct-landable class as before
   (docs/tests/zero-behavior private cleanups). The engine request path is
   semantics-frozen (response bytes, parallel contracts, commit ordering); anything
   structural is an analyzed finding for the batch triage.
3. **Checklist verdict** per crate with evidence.

## Hard constraints

- Engine semantics FROZEN (P004's byte-identity bar is the standard if any request-path
  code is touched mechanically — capture/diff representative responses).
- No public API changes; no bench recordings (direct unrecorded engine-bench runs
  before/after if request-path code is touched; revert on regression).
- Tests never cheat; scoped fmt writes only; no git actions; no network installs;
  unexpectedly dirty unowned files are escalations, never cleanup.

## Definition of done

- Deny attributes green in both crates; doc build `-D warnings` clean; doctests pass.
- Findings report + checklist verdicts.
- Gates green: workspace tests + doctests, clippy `-D warnings`, fmt check,
  `make loom-tasks` untouched-green, board tsgo + vitest untouched-green, direct
  engine-bench comparison if request-path code touched.
- REPORT.md per template (or in-message on guardrail).
