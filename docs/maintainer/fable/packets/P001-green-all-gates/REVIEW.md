# P001 Review

Reviewer: Fable (orchestrator). Date: 2026-07-01. Executor: GLM 5.2 (OpenCode via Venice
AI), dispatched by the maintainer.

## Verdict

The report is **accepted as an honest stop-and-escalate**: the in-scope work is correct,
the stop honored the packet's hard constraints, and every escalation is real. The packet
itself **remains open** — see the Closeout section below: the maintainer ruled on the
three blockers, Fable executed the rulings, and every gate is now recorded green except
one e2e scenario that found a real Linux bug (the packet's one remaining item).

## Independent verification performed

Fable re-verified every load-bearing claim on Linux, rustc 1.96.0 (the same environment
class as the executor's run):

- **Change surface:** exactly the files the report claims — `examples/board/src/lib.rs`
  (one line deleted), `STATE.md`, `REPORT.md`, two ticket directories. No
  `bench.results.txt` touched. No commits made.
- **Board fix:** diff inspected. `TsGenConfig` (`crates/vorma/src/config.rs:85`) has
  exactly three fields (`out_file`, `extra_types`, `extra_ts`), all named in the board
  literal; the deleted spread is precisely what `needless_update` flags, and deletion is
  the packet-preferred fix. Compile confirmation on Linux is impossible until blocker 1
  lands (board depends on `vorma-build` via `vorma`); textual verification only, disclosed
  by the executor and re-done independently here.
- **Blocker 1 reproduced verbatim:** `cargo check -p vorma-build` fails with E0382 at
  `process_runner.rs:458` — `nix::sys::wait::Id` is non-`Copy`, moved into `waitid`, and
  the `Err(EINTR) => continue` arm makes a second iteration reachable. Source read; the
  ticket's analysis and recommended fix (loop-local rebind) are accurate and
  behavior-preserving.
- **Blocker 2 reproduced verbatim:** `cargo test -p vorma-tasks --tests` — 48/48
  integration tests pass; trybuild `task_new_removed` fails only on E0599 wording drift
  ("no function or associated item" → "no associated function or constant"); the other two
  UI cases (`extended_cache_zero`, `task_macro_rejects_unresolved_task_type_path`) pass.
- **Matcher gate:** `cargo test -p vorma-matcher --all-targets` green — 57 tests across 7
  test binaries (3+7+24+8+10+3+2), exit 0.
- **fmt:** `cargo fmt --all --check` clean.
- **Loom:** `make loom-tasks` equivalent run — 7/7 models pass, test names matching the
  protocol models described in `LEARNINGS.md`.
- **Tickets:** both are format-compliant (`__TICKET.md` in slug directories) and written
  for a cold agent: reproduction, impact chain, recommended fix, verification steps.
- **STATE.md rewrite:** honest. Records the Linux reality, explains why the prior
  macOS-recorded snapshot claimed only one clippy lint, adds the blockers as open flags,
  leaves the benchmark tables untouched.

## Findings

Exhaustive plain list, per AGENTS.md review rules:

1. The report's `vorma-matcher --all-targets` gate evidence pastes only the final test
   binary's summary ("2 passed") out of the seven binaries the target runs (57 tests
   total). The claim itself is true — re-verified green — but the pasted evidence
   under-represents the run. Full counts are now recorded in this review.
2. STATE.md's line "on macOS the board fix is sufficient to clear that specific lint" is
   an inference, not a machine-verified result (no post-fix macOS run exists). The
   inference is sound — the deleted spread is exactly what the lint flags — and it becomes
   machine-verified the moment workspace clippy runs anywhere post-fix.

No other issues found.

## What remains to close P001

1. Maintainer ruling on blocker 1 → apply the loop-local `Id` rebind in
   `crates/vorma-build/src/process_runner.rs`; the workspace build unblocks.
2. Maintainer ruling on blocker 2 → rebless
   `crates/vorma-tasks/tests/ui/task_new_removed.stderr`; verify the diff is wording-only.
3. Full `make rust-gate` on the unblocked workspace; record results.
4. `make ts-gate` and `make e2e-smoke` under explicit pnpm escalation (or a maintainer-run
   on an unrestricted machine); record results.
5. Final STATE.md gates update; delete the two blocker tickets per `tickets/README.md`;
   update this review to "accepted, packet closed".

## Fable positions on the requested rulings

Stated here so the record is durable; the maintainer decides.

1. **Fix blocker 1 now via the ticket's loop-local rebind; no broader Linux exit-watcher
   design review first.** The design comment in the source (observe exit without reaping
   via `WNOWAIT`; `terminate`/Drop keep sole reaping responsibility) is coherent, and the
   bug is mechanical — the code was authored on macOS and never compiled on Linux. Holding
   P002–P004 behind a design review buys nothing. The existing
   `windows-child-exit-watcher` ticket covers the unrelated Windows stub.
2. **Rebless the one `.stderr` and stop there.** trybuild pins exact compiler output by
   design; wording-drift reblesses are the accepted maintenance cost of that design, and
   any "version-resilience" normalization would weaken the pin. Decline the enhancement.
3. **Run the TS and e2e gates from the orchestrator session under explicit pnpm
   escalation**, so the whole baseline is recorded from one machine in one pass. A
   maintainer-run elsewhere is the fallback.
4. **No fresh macOS parity run is needed to close P001**; this Linux machine is now the
   gate-recording machine. The real cross-machine question is P002's benchmark baselines
   (recorded on an Apple M3 Max); that ruling belongs to P002 dispatch, not P001.

**Recommendation:** authorize Fable to execute remaining items 1–5 directly. Both fixes
are one-line, mechanical, and independently verified; an executor round-trip adds latency,
not safety, and the gate itself is the check that matters. Alternative: author a P001a
packet and dispatch it. Maintainer decides.

## Closeout (2026-07-01, executed by Fable under maintainer authorization)

The maintainer authorized the recommendation above ("Please proceed"). Executed and
verified:

1. **Blocker 1 fixed.** Loop-local `Id` rebind in
   `crates/vorma-build/src/process_runner.rs` (construction moved inside the retry loop;
   observable behavior unchanged: observe-without-reaping via `WNOWAIT`, retry on `EINTR`,
   return otherwise). The workspace compiles on Linux for the first time.
2. **Blocker 2 fixed.** `crates/vorma-tasks/tests/ui/task_new_removed.stderr` reblessed
   via `TRYBUILD=overwrite`; diff verified to be exactly the two E0599 wording lines (same
   error code, same span); `api_diagnostics` green.
3. **`make rust-gate`: green, all steps** — fmt, policy (audit: one allowed unsound
   warning, ticketed; deny: all four checks ok), clippy `-D warnings`, tests (548 + 1
   doctest = 549 passed — matches the macOS-era recording exactly), loom 7/7, build, doc
   `-D warnings`, bench compile, client-wasm (regenerated the tracked artifact —
   reproducibility ticketed), package (six crates), fuzz (both targets, 4096 runs, no
   findings; machine caveat recorded in STATE.md and ticketed: miniconda `cc` shadowing
   requires a linker override for the fuzz step on this box).
4. **`make ts-gate`: green** — install, fmt, lint (10 pre-existing warnings ticketed),
   tsgo 8/8 projects, vitest 851/851.
5. **`make e2e-smoke`: red on exactly one scenario, and it is a real bug.** test-prod
   green (react/preact/solid, full browser property suites, Chrome 148); test-dev (react)
   green; test-dev-changes (react) deterministically red — a source change never triggers
   a dev rebuild on Linux (the watcher surfaces no event). Full diagnosis, suspects, and
   fix constraints in ticket `linux-dev-changes-rebuild-not-triggered`. Fixing it is
   semantic dev-pipeline work outside this closeout's authorization — escalated to the
   maintainer.

Bookkeeping: resolved tickets `linux-waitid-id-moved` and
`vorma-tasks-trybuild-compiler-drift` deleted per `tickets/README.md`. New tickets filed:
`anyhow-rustsec-2026-0190-upgrade`, `ts-lint-react-redundant-type-warnings`,
`linux-conda-cc-shadowing-fuzz-link`, `wasm-artifact-binaryen-version-pinning`,
`linux-dev-changes-rebuild-not-triggered`. STATE.md carries the full gate record and the
machine-provisioning notes. No commits made (git policy); the working tree holds the two
fixes, the regenerated wasm artifact, formatter normalization from the gates' write-mode
fmt steps, and all documentation updates.

## Final disposition

**P001 is CLOSED, accepted (2026-07-01).** The one remaining item — the Linux dev-changes
rebuild bug — was fixed by packet P001b (root cause: inotify access-event feedback; see
`packets/P001b-linux-dev-rebuild-fix/REVIEW.md`), after which the full `make e2e-smoke`
aggregate passed with exit 0. Every definition-of-done item is met: clippy clean, every
gate run and green, STATE.md records the true baseline. The discovered bug was the packet
working as intended — the first honest Linux gate run surfaced a real Linux-only defect
that macOS development never could have caught.
