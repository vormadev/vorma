# P001 — Green All Gates And Record The Baseline

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md` and `STATE.md`. Repo-root `AGENTS.md` also binds you.

## Context

A large body of staged work landed without a final gate run. The workspace tests
(549), loom models (7), and fmt are green; clippy is red on exactly one lint; the TS
gate and e2e have not been run at all since the work landed. This packet makes every
gate green, runs the ones nobody ran, and records the true baseline.

## Scope

1. **Fix the clippy error** at `examples/board/src/lib.rs:113`:
   `needless_update` — a struct literal spells out every field and then also spreads
   `..vorma::TsGenConfig::default()`. Preferred fix: delete the spread line. If deleting
   it does not compile (fields exist that the literal does not name), keep the spread
   and delete whichever explicit fields merely restate defaults. Never silence with an
   allow attribute.
2. **Run every gate and capture output:**
   - `cargo test --workspace --all-targets` and `cargo test --workspace --doc`
   - `cargo clippy --workspace --all-targets -- -D warnings`
   - `cargo fmt --all --check`
   - `make loom-tasks`
   - `make rust-gate` (the full gate, including policy, doc, bench compile, wasm build,
     package, fuzz)
   - `make ts-gate`
   - `make e2e-smoke`
3. **Update `docs/maintainer/fable/STATE.md`** "Gates" section to reflect reality after
   your run (including TS/e2e now being verified, with date).

## Hard constraints

- This packet changes at most the one board file (plus STATE.md). Any other red gate
  step is a finding to escalate in `REPORT.md`, not something to fix here — file a
  ticket per `docs/maintainer/tickets/README.md` and reference it.
- Do not touch `bench.results.txt` files; benchmark recording belongs to P002.
- No allow attributes, no test or lint weakening, no skipped gate steps. If a step
  cannot run on this machine, say so explicitly in the report with the error.

## Definition of done

- `cargo clippy --workspace --all-targets -- -D warnings` exits clean.
- Every command in scope item 2 has its result pasted in `REPORT.md` (tail is fine for
  long output, but include the pass/fail summary lines verbatim).
- STATE.md gates section updated.
- `REPORT.md` follows the template in `docs/maintainer/fable/README.md`.
