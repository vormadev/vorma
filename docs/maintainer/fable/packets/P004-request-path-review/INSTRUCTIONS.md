# P004 — Request-Path Performance Review

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md` and `STATE.md`. Repo-root `AGENTS.md` binds you. Prerequisites: P001 and
P002 accepted (the engine sits on vorma-tasks; measuring on top of the regressed crate
would mis-attribute costs).

**Status: partially blocked on Fable.** Part 1 (benchmark construction) is executable
now. Part 2 (optimization) requires Fable's analysis addendum (`ANALYSIS.md` in this
directory), which will be written from Part 1's recorded numbers. Do not start Part 2
without it.

## Context

The matcher and tasks crates now dramatically beat their Go baselines, but a fast
matcher is pointless if the request machinery around it is slow (maintainer framing).
The goal: measure and optimize from the moment a request becomes owned by vorma to the
moment vorma returns its result — the adapter (hyper/axum/etc.) is out of scope on both
ends. Standing ticket: `docs/maintainer/tickets/router-request-path-review/`.
The matcher review's proven technique catalog is in `LEARNINGS.md` under "Performance
conventions".

## Part 1 — Owned-request-to-response benchmarks (executable now)

1. Add a `bench-engine` benchmark target to `crates/vorma` using the `vorma-bench`
   harness (see existing `crates/vorma-matcher/benches/matching.rs` for the shape; the
   crate links `vorma`, so do NOT declare a global allocator in the bench — the vorma
   mimalloc default feature provides it; see LEARNINGS on the one-declaration rule).
2. Fixture: a representative in-process app built the way the engine tests build one
   (see `crates/vorma/tests/in_memory_test_app.rs` and the engine's own test fixtures) —
   no sockets, no adapter. The measured call is the engine's owned-request entry point
   returning its finished response.
3. Rows (one line each, Go-style names): static view render; nested dynamic view chain
   (3+ deep with params); JSON resource (small input/output); `ResourceBody` binary
   resource; request through a middleware chain (2 middlewares); 404 (catch-all view);
   404 (bare, no catch-all registered); HEAD request to a resource; method-not-allowed.
   Rotate realistic paths per row as the matcher benches do.
4. `Makefile`: add `bench-engine` following the existing `bench-matcher`/`bench-tasks`
   pattern (build check first, then run teeing into `crates/vorma/bench.results.txt`);
   add it to the phony list. Record once and include the table in the report.
5. There are no Go-era baselines for this surface. The recorded table IS the baseline;
   STATE.md gains an engine table section (add it).

## Part 2 — Optimization (blocked on Fable's ANALYSIS.md)

Expected shape, for planning only: Fable reads Part 1's recording, decomposes rows
against known costs (matcher, tasks, projection, header work, allocation counts),
writes `ANALYSIS.md` with targeted changes packet-style. Semantics are frozen exactly
as in P002; anything observable escalates.

## Hard constraints

- No public API changes; no adapter-layer changes; no new dependencies without
  escalation.
- Fixture must exercise the real engine path (no stub handlers that skip projection or
  effects merging); if the engine lacks a clean owned-request entry point for benching,
  that is a finding to escalate, not a reason to bench a private shortcut.
- Benchmarks recorded only via the new `make bench-engine`. Full gate green including
  the new target compiling under `rust-bench`.

## Definition of done (Part 1)

- `make bench-engine` exists, runs, and `crates/vorma/bench.results.txt` is recorded.
- Report contains the table plus a one-paragraph first-read of where time appears to
  go (no changes yet).
- STATE.md gains the engine baseline table.
- Gate green; report per template.
