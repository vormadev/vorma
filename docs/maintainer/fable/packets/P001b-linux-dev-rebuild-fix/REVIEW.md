# P001b Review

Reviewer: Fable (orchestrator). Date: 2026-07-01. Executor: Opus subagent dispatched by
Fable under maintainer authorization.

## Verdict

**Accepted. Packet closed.** The root cause is proven with converging empirical evidence,
the fix is minimal and lands exactly at the mechanism, the pins are real (demonstrated red
on pre-fix behavior), and every definition-of-done item passes — re-verified independently
by Fable, including the one item the executor's sandbox could not run.

## Root cause (accepted)

inotify emits events for bare reads (`IN_OPEN`/`IN_CLOSE_NOWRITE` → notify
`Access(Open/Close(Read))`); the dev watcher classified events by path only, so the
rebuild's own cargo/rustc reading `src/**/*.rs` classified as `ServerRecompile` and the
in-flight-rebuild handler cancelled and re-spawned the build forever. FSEvents emits no
access events, which is why macOS never saw it. The executor's evidence chain — raw-event
instrumentation of the real session (marker once, then a ~130ms stream of `src/lib.rs`
opens), per-second process sampling showing a fresh `cargo build` PID every sample, a gap
analysis refuting the settle-starvation alternative, and a real-notify reproduction —
establishes this beyond doubt. Notably, the ticket's prime suspect (path-shape divergence)
was tested and ruled out with direct evidence rather than assumed away.

## Independent verification performed (Fable)

- **Diff audit:** the entire change is `crates/vorma-build/src/dev_watcher.rs` (+180/−34):
  one kind-gate at the top of `classify_event`, the `event_kind_mutates_source` free
  function (Access counts only as `Close(Write)`; `Create`/`Modify`/`Remove` mutate;
  `Any`/`Other` conservatively retained), a doctrine comment explaining the feedback loop
  and the macOS no-op, two pin tests, and `#[cfg(test)]`-only helpers (no public API
  surface added). No leftover instrumentation (grep for probes/eprintln/dbg: clean).
- `cargo test -p vorma-build --all-targets`: **134 + 1 passed, 0 failed**, both pins green
  (`read_only_access_events_never_classify_as_source_change`,
  `real_watcher_ignores_reads_but_sees_writes`).
- `cargo fmt --all --check`: clean.
  `cargo clippy --workspace --all-targets -- -D warnings`: clean.
- **`make e2e-smoke` (full aggregate, exit 0)** — run by Fable with pnpm escalation since
  the executor's sandbox forbade `ts-install`: test-prod (react/preact/solid, full browser
  suites), test-dev (react), and test-dev-changes (react) all green;
  `[react] server Rust rebuild observed` — the previously-failing step — passes.
- Discovered-work ticket
  (`docs/maintainer/tickets/dev-watch-excludes-not-pruned-from-os-watch/`) reviewed:
  correctly scoped as a separate least-work/robustness issue (not this bug's cause),
  cold-agent-ready, includes the FSEvents platform-split consideration and proper
  verification expectations.

## Findings

No issues found.

## Note for the maintainer's commit (transient)

A harness checkpoint `git add`-ed the tree mid-diagnosis, so the git INDEX holds a stale
`dev_watcher.rs` containing a temporary probe test that is not in the working tree. The
working tree is correct and instrumentation-free (verified). Neither agent performed git
actions. Stage from the working tree (e.g. `git add -A` before committing); do not commit
the current index as-is.

## Consequence

P001b was the sole remaining item of P001. With this acceptance, **P001's definition of
done is fully met** — see P001's REVIEW.md final disposition and STATE.md for the recorded
baseline.
