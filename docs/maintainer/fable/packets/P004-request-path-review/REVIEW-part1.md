# P004 Part 1 Review

Reviewer: Fable (orchestrator). Date: 2026-07-01. Executor: Sonnet 5 subagent (first
Sonnet-5 dispatch, per maintainer directive).

## Verdict

**Part 1 accepted.** The fixture drives the genuine production path, the recording is
reproducible, the added control row was the right measurement instinct, the machine-idle
discipline under adverse conditions was handled exactly per doctrine, and every gate is
green. Part 2 remains blocked on Fable's ANALYSIS.md, next.

## Independent verification performed (Fable)

- **Real-path claim verified in source:** `TestApp::handle_request`
  (`crates/vorma/src/testing.rs:100`) forwards to
  `self.host.service().handle_request(request)` — the `CommittedRuntimeService` entry real
  adapters call. The fixture (`crates/vorma/benches/engine.rs`, 497 lines) registers apps
  through the real `app!`/`view!`/`resource!` macros with realistic handler bodies; no
  stub shortcuts. The allocator rule is handled correctly (vorma's default mimalloc
  feature declares the global allocator; the bench declares no second one, with the
  constraint documented in-source).
- **Dependency discipline:** `crates/vorma/Cargo.toml` gained only the repo's own
  `vorma-bench` harness (dev-dep, required by the recording convention) plus the
  `[[bench]]` registration; `Cargo.lock` +1 line accordingly. No third-party additions.
- **Makefile:** `bench-engine` follows the per-machine pattern exactly.
- **Reproducibility:** an independent direct bench run (not touching the recording)
  reproduced all 10 rows within ~1–9% (e.g. `static_view_render` 46.1 vs 48.8µs,
  `not_found_bare` 2.15 vs 2.16µs).
- **Gates:** executor ran the full set green (552 tests); Fable re-verified bench
  compile + the recording run.
- **STATE.md engine table:** present and matching the recording.

## Findings

Exhaustive plain list, per AGENTS.md review rules:

1. The executor could not write REPORT.md (harness guardrail specific to its session:
   "Subagents should return findings as text, not write report files" — a behavior not
   seen in the three prior executor sessions). Content was returned in-message and placed
   by Fable with a provenance note. Process friction only; no content loss.

No other issues found.

## Baseline shape (for ANALYSIS.md, next)

The striking structure: non-handler rows cost 2.2–4.0µs while every handler-reaching row
costs 24–71µs — the pipeline weight is decode/execute/finalize, not matching (the
matcher's whole contribution is ~0.1–0.4µs per the matcher tables). View/HTML rows run
~40–49µs vs ~24–25µs for resource rows, implicating document/HTML finalization; the 4-deep
nested chain (70.3µs) scales worse than 4× a single view; the two-middleware tax is ~5.4µs
over its control. Fable writes ANALYSIS.md decomposing these against the engine internals
(execution engine, payload projection, response finalizer, head/document pipeline) before
any Part 2 dispatch.
