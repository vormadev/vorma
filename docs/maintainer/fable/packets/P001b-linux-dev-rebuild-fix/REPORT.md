# P001b Report

## Root cause

On Linux the dev rebuild was spawned correctly on a source change, then **cancelled and
re-spawned in an infinite loop by the rebuild's own compiler reads**, so it never finished
and the marker never updated.

Mechanism, proven empirically (evidence below):

1. Editing `src/dev_marker.rs` produces an inotify `Modify(Data)` event that classifies as
   `ServerRecompile`; `recv_settled` returns it and the dev loop spawns a rebuild
   (`spawn_dev_rebuild` -> `cargo build ... framework-serve`).
2. That `cargo`/`rustc`, running from CWD `tests/framework`, **opens the
   `src/**/\*.rs`sources it is compiling for reading**. inotify emits`IN*OPEN`/`IN_CLOSE_NOWRITE`for each, which`notify`surfaces as`Access(Open(*))`/`Access(Close(Read))`.
3. `DevFileWatchPlan::classify_event` did not look at the event _kind_ -- it classified
   purely by path. `src/lib.rs` (and siblings) match `src/**/*.rs`, so each read event
   classified as `{GeneralWatch, ServerRecompile}` -- i.e. a change requiring an
   app-server rebuild.
4. While a rebuild is in flight, `collect_inflight_dev_loop_event` treats a
   `ServerRecompile` change as "source changed again": it `cancel.cancel()`s the running
   build and queues a fresh one. So the compiler's own file reads retire the compiler that
   produced them. The next rebuild starts, reads the same files, is cancelled again --
   forever. Total silence in the framework log (no rebuild ever _completes_ or broadcasts)
   exactly matches the ticket symptom.

macOS is immune because **FSEvents does not emit open/read events at all** -- it reports
content/rename/metadata _mutations_, not accesses. So on macOS the compiler merely reading
its sources produces no events and the rebuild completes. This Linux path first compiled
the same day (behind the fixed `waitid` error), which is why it had never run before.

The ticket's "prime suspect" (path-shape divergence in `relative_event_path` /
`watch_root_for_source`) was ruled out with direct evidence: the checkout is fully
canonical (no symlinks), watch roots register as absolute canonical paths, raw inotify
events arrive as absolute canonical paths, and classification of the real marker path
already yields `{GeneralWatch, ServerRecompile}` correctly. Classification of the _right_
path was never the problem; classification of the _wrong event kind_ (a read) was.

### Evidence collected (all on this machine, targeted repro)

- **Raw events arrive as absolute canonical paths, classification of the marker works.** A
  throwaway probe registering the real watch roots against the real `tests/framework` tree
  and doing a plain `fs::write` to `src/dev_marker.rs` saw `Modify(Data(Any))` /
  `Access(Close(Write))` on `/.../tests/framework/src/dev_marker.rs` (absolute), and the
  real react plan classified that path as `{GeneralWatch, ServerRecompile}`,
  `requires_app_server_generation=true`. This split the hypothesis space to
  "classification of event kind", not registration or path shape.
- **Instrumented real session (raw-event log written outside the watched tree to avoid
  feedback).** During the failing window: `src/dev_marker.rs` seen once as
  `ServerRecompile`, then a **sustained stream of `src/lib.rs` `Access(Open(Any))` events
  at a ~130 ms cadence**, all classified `{GeneralWatch, ServerRecompile}`. Distribution
  over ~10 k events: 368 `src/lib.rs` opens vs 7 `dev_marker.rs` events. The real dev
  server log showed only startup lines -- no "Rebuilding", no second compile, no error.
- **Cargo restart churn confirms the cancel-restart loop directly.** Sampling processes
  once per second showed a _brand-new_ `cargo build ... framework-serve` PID every sample
  (etimes=0s each), i.e. the rebuild being killed and re-spawned ~continuously. The old
  `framework-serve` kept serving marker-a the whole time.
- **Gap analysis refuted the alternative "settle-loop starvation" theory.** Longest
  gap-free (<10 ms) run in the event stream was 0.02 s; there were thousands of >=10 ms
  gaps (max 2.8 s). So `recv_settled(10 ms)` returns promptly -- the settle loop is fine;
  the failure is the in-flight cancellation, not settle starvation.
- **End-to-end pin over real notify reproduces it deterministically** (see below): opening
  a watched `src/main.rs` for reading, with no write, delivered a real `{ServerRecompile}`
  change on the pre-fix code.

## What changed

- `crates/vorma-build/src/dev_watcher.rs`
    - `DevFileWatchPlan::classify_event` now gates on event kind first: non-mutating
      access events are dropped before path classification. One-line guard plus a new
      `event_kind_mutates_source(&notify::EventKind) -> bool` free function (with a
      doctrine comment explaining the Linux feedback loop and why macOS is unaffected).
    - Filter policy: drop `Access(Open/Read/Close(Read|Any|Execute)/Any)` (pure open/read
      noise); keep every mutation -- `Create`, `Modify`, `Remove`, the `IN_CLOSE_WRITE` ->
      `Access(Close(Write))` write-completion signal, and the imprecise `Any`/`Other`
      catch-alls (treated as possible mutations so no real change is dropped).
    - Two pin tests: `read_only_access_events_never_classify_as_source_change`
      (deterministic, synthetic `notify::Event`s through `classify_event`) and
      `real_watcher_ignores_reads_but_sees_writes` (real notify watcher on a temp tree:
      reads -> no change, write -> `ServerRecompile`).
    - `StartedDevFileWatcher::recv_relevant_within` (test-only): a deadline-bounded
      receive of the next framework-relevant change, so the "no change" outcome is
      assertable without the blocking `recv_settled` hanging.

Scope stayed inside `crates/vorma-build`; no harness (`tests/framework`) or other crate
changes were needed. No `bench.results.txt` touched. No allow-attributes; no test
weakening.

## Decisions made

- **Fix at the event-kind boundary, not by pruning `.bombadil` from the OS watch.** While
  diagnosing I found a second real problem: the whole-root `.` watch recursively covers
  `.bombadil` (cargo target + Vite cache) and `.dist.*` even though `!.bombadil/**`
  excludes them from _classification_. That is wasteful (violates the least-work
  invariant) and worth fixing, but it is **not** the cause of this bug: the fatal events
  were reads of `src/**/*.rs` -- genuinely watched, genuinely matching source files -- not
  `.bombadil` noise. Pruning excluded dirs from the OS watch would not have stopped the
  compiler's reads of its own sources from cancelling the rebuild. The event-kind gate is
  the true root-cause fix; the watch-scope waste is filed as a separate ticket (below)
  rather than folded in, per the "file a ticket, don't expand the packet" rule.
- **Keep `Access(Close(Write))` as a mutation signal** rather than dropping all `Access`.
  A plain `fs::write` already emits `Modify(Data)`, so dropping all access events would
  still catch it; but `IN_CLOSE_WRITE` is the canonical "a write just completed" signal
  and keeping it is strictly safer against write patterns that a backend might report only
  as a close. It cannot reintroduce the loop: compiler reads are `Close(NOWRITE)` ->
  `Close(Read)`, which is dropped.
- **Treat `EventKind::Any` / `Other` as mutations.** These are the imprecise catch-alls a
  backend emits when it cannot classify a native event (and `Any` is `notify`'s default in
  imprecise mode). Dropping them could silently swallow a real change on some backend, so
  they are retained. On the inotify/FSEvents backends in use they are not the read events
  that caused this bug.
- **Pin at unit + real-notify level, not full dev session.** The bug is fully reproducible
  at unit level (synthetic event kinds) and at real-notify level (temp tree, open vs
  write), both of which fail red on the pre-fix code. A full dev-session test is not
  needed to pin the mechanism and would be far more expensive and flaky. The full targeted
  repro is used for end-to-end verification instead.

## Gate results

`cargo fmt --all --check` -- clean (no diff).

`cargo test -p vorma-build --all-targets`:

    test result: ok. 134 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out
    test result: ok. 1 passed; 0 failed; 0 ignored; 0 measured; 0 filtered out   (public_api.rs)

Both new pins included:

    test dev_watcher::tests::read_only_access_events_never_classify_as_source_change ... ok
    test dev_watcher::tests::real_watcher_ignores_reads_but_sees_writes ... ok

Pins are real (red before the fix). With the kind gate disabled (`&& false`) the pre-fix
behavior is restored and both fail:

    read/open access event Access(Open(Any)) must not classify as a source change
    reads produced a framework-relevant change: Some(DevFileChange { ..., intents: {ServerRecompile} })

`cargo clippy --workspace --all-targets -- -D warnings` -- clean (exit 0, no warnings).

### e2e scenarios

`make e2e-smoke` as a whole was **not** run because its `ts-build` prerequisite runs
`ts-install` (`pnpm install`), which is forbidden in this sandbox. Per the packet, the
three scenario commands were run individually from the framework fixture, reusing the
already-built `packages/vorma/.dist` and `node_modules`:

- `framework-bombadil -- test-dev-changes -variant react` -- **GREEN** (exit 0). The exact
  ticket failure. Harness log:

          [react] initial server marker observed
          [react] server Rust rebuild observed
          [react] browser critical CSS and HMR checks observed
          [react] client view module refresh observed

    The marker rebuild (marker-a -> marker-b) now settles and completes; no cargo restart
    churn.

- `framework-bombadil -- test-dev -variant react` -- **GREEN** (exit 0). All four browser
  test phases passed (30s root, 15s nested, 10s counter, 10s latency).
- `framework-bombadil -- test-prod` -- **GREEN** (exit 0). All three adapters (react,
  preact, solid) built deployments A/B + server binaries and passed their full Bombadil
  browser property suites plus the build-skew suite.

## Benchmarks

None. This packet makes no performance-sensitive change and records no benchmarks.

## Escalations / open questions

None blocking. One design observation, filed as a ticket rather than actioned here:

- The OS-level dev watch registers a recursive watch on the whole app root (`.`), which
  descends into `.bombadil` (cargo target + Vite cache) and `.dist.*` even though
  `!.bombadil/**` excludes them from classification. Exclusions are classification-only;
  they do not prune the OS watch. This wastes inotify watches and processing on large
  generated trees (violating the least-work invariant) and is a latent exhaustion/feedback
  risk on big projects. Not the cause of this bug and out of packet scope; ticketed for a
  proper exclusion-aware watch-registration design.

## Discovered out-of-scope work

- `docs/maintainer/tickets/dev-watch-excludes-not-pruned-from-os-watch/__TICKET.md` --
  make dev-watch exclusions prune the OS watch descent (not just classification), so the
  watcher does the least necessary work and cannot be flooded/exhausted by generated trees
  like `.bombadil`.

## Note on repository/index state (informational, no action taken)

At session start the working tree already had a set of **pre-existing staged changes**
from prior packet work (`process_runner.rs`, the `fable/` docs, `examples/board`, the
tracked wasm artifact, several tickets). I did not create or stage those and did not touch
them. During diagnosis, an environment snapshot appears to have `git add`-ed the working
tree at a moment when a temporary probe test was present, so the **git index** for
`crates/vorma-build/src/dev_watcher.rs` contains a stale copy that includes a
`zzz_probe_real_session_classification` test which is **not** in the working tree (the
working tree is clean of all temporary instrumentation -- verified). I performed no git
actions (no add/reset/checkout), per protocol, so I left the index as-is; the working tree
is the source of truth and is correct. Flagging so the maintainer re-stages from the
working tree rather than the index when committing.
