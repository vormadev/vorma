# P001b — Fix: Linux dev loop never rebuilds on source change

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, then `STATE.md`, then this file. Repo-root `AGENTS.md` and
`docs/maintainer/REMINDERS.md` bind you as well. The full diagnosis dossier for this bug
is `docs/maintainer/tickets/linux-dev-changes-rebuild-not-triggered/__TICKET.md` — read
it before touching code; do not re-derive what it already establishes.

## Context

P001 recorded the first full Linux gate baseline. Everything is green except
`make e2e-smoke`'s final scenario: `test-dev-changes -variant react` fails
deterministically because a source-file change never triggers a dev rebuild. The dev
session starts cleanly (Vite ready, app server ready) and then the watcher loop
(`crates/vorma-build/src/entrypoint.rs:172`, `watcher.recv_settled(...)`) never yields a
change — total silence in the dev log, no error, no rebuild spawn. This entire pipeline
first COMPILED on Linux the same day (it sat behind a `waitid` compile error), so the
Linux/inotify path has never executed before. macOS (FSEvents) is the
currently-working reference behavior.

Known facts (evidence in the ticket): prod and plain-dev e2e scenarios pass; the dev
watcher unit tests pass on Linux; the fixture `root_dir` is absolute
(`env!("CARGO_MANIFEST_DIR")`); the watcher START does not error (session comes up).

## Scope

1. **Root-cause first, with evidence.** Split the hypothesis space empirically before
   changing anything. The fastest split: determine whether raw notify events reach the
   watcher callback at all during the failing scenario.
   - Temporary debug instrumentation (e.g. eprintln of raw events in the
     `start_dev_file_watcher` callback) is allowed while diagnosing and MUST be removed
     before the packet completes.
   - Targeted repro (fails in ~seconds once built):
     `cd tests/framework && cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev-changes -variant react`
   - If raw events arrive → the bug is classification (path shapes, exclusions,
     glob/slash handling in `dev_watcher.rs`). If none arrive → the bug is watch
     registration (root selection, notify inotify recursion, inotify limits — check
     `/proc/sys/fs/inotify/max_user_watches` and `max_user_instances`; note Vite's own
     watcher consumes inotify resources in the same session).
   - Also check how the harness writes the marker file
     (`tests/framework/src/bin/bombadil.rs`, dev-changes flow): plain `fs::write` vs
     atomic temp-write+rename produce different inotify event kinds/paths.
2. **Pin the bug.** Once root-caused, write the cheapest test that fails on the broken
   behavior and passes on the fix. Prefer a `dev_watcher.rs` unit test that reproduces
   the exact real-session condition (e.g. a real notify watcher on a temp tree with the
   same root/entry shapes and the same write pattern). If the bug is only reproducible
   in a full dev session, pin what is pinnable at unit level and say so explicitly in
   the report.
3. **Fix it.** Minimal, root-cause-level fix in `vorma-build`. No workarounds that
   mask the mechanism (no retry loops, no polling, no debounce inflation).
4. **Verify.** Definition of done below.

## Hard constraints

- **No polling.** REMINDERS.md binds: dev events arrive through event channels only;
  the tiny post-event settle debounce is the only time-based element. A fix that
  introduces polling or periodic wakeups is an automatic rework.
- **macOS behavior stays intact.** FSEvents is the working arm. If the fix lands in
  shared logic, explain in the report why it cannot regress macOS (reasoning from the
  code is acceptable; a macOS run is not available on this machine).
- **Semantic-change protocol.** This packet grants you exactly one semantic change:
  making Linux dev rebuilds fire on source changes (matching the documented/macOS
  behavior). Anything else you believe needs changing: ticket + escalate in REPORT.md.
- Scope is `crates/vorma-build` (plus its tests) and, only if the root cause genuinely
  lives there, the fixture harness under `tests/framework`. Touch nothing else. Do not
  touch `bench.results.txt` files. No allow-attributes, no test weakening.
- Do NOT run `pnpm install` or any network installation (sandbox; everything needed is
  already installed: node_modules, Chrome via `~/.local/bin/chrome*` symlinks, all
  cargo tooling). `make e2e-smoke`'s ts-build step may run pnpm EXEC commands — those
  are local and fine.
- Remove every piece of temporary debug instrumentation before writing REPORT.md.
- If a needed command is blocked by a sandbox permission denial, do not fight it:
  record exactly what you ran and the denial in REPORT.md and continue with what runs.
  Fable re-runs full verification at review.

## Definition of done

- Root cause stated in REPORT.md with the evidence that proves it (not "works now").
- Pin test(s) added; report shows them red on the broken code (paste) and green after.
- `cargo test -p vorma-build --all-targets` green.
- `cargo clippy --workspace --all-targets -- -D warnings` and `cargo fmt --all --check`
  green.
- Targeted repro green:
  `framework-bombadil -- test-dev-changes -variant react` passes.
- `make e2e-smoke` green end to end if runnable under your sandbox; otherwise run the
  three scenario commands from the Makefile individually and report each.
- `REPORT.md` in this directory per the template in `docs/maintainer/fable/README.md`,
  including every judgment call and any escalations.
