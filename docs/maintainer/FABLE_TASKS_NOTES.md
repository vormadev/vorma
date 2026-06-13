# Fable Tasks Notes

These notes preserve the `vorma-tasks` workstream from the Fable histories. This file is
especially important because the final transcript ended during an unresolved trust and
semantics issue: several task-crate changes landed or were proposed without the right
approval flow, and current code still reflects some of those unresolved states.

## Source Coverage

- Main transcript ranges: `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7.jsonl` lines 10,936-11,586
  cover the tasks first-principles review, benchmark harness correction, mimalloc/loom
  work, `Task::new` constructor discussion, `run_parallel` semantic change, singleflight,
  and fingerprint hash audit.
- Current reconciliation: `crates/vorma-tasks/src/task.rs`,
  `crates/vorma-tasks/src/key.rs`, `crates/vorma-tasks/src/loom_tests.rs`,
  `crates/vorma-tasks/benches/tasks.rs`, `examples/board/src/repo.rs`, and `Makefile`.

## Current Unresolved State

As of this notes pass, the current code still shows these transcript-critical unresolved
items:

- `Task::new` still takes `Duration` as its first argument, with `Duration::ZERO` used as
  the no-extended-cache sentinel.
- `ExecCtx::run_parallel` still uses `FuturesUnordered` on one poller and has an in-code
  comment saying siblings interleave at await points rather than spawning runtime tasks.
- `crates/vorma-tasks/src/key.rs` still fingerprints task inputs with `rustc_hash::FxHasher`.
- No `Task::singleflight` constructor exists.
- `make loom-tasks` exists and `rust-gate` includes `loom-tasks`.
- `mimalloc` is default-on in the `vorma` framework crate; standalone matcher/tasks bench
  binaries declare it directly because they do not link `vorma`.

These are not all equally bad. The loom and allocator decisions are ratified/current. The
constructor split, singleflight, spawned parallelism restore, and SipHash restore remain
the forward work package from the final transcript.

## Tasks Crate Is Sovereign

`vorma-tasks` is not a framework-private HTTP request helper. It is a general-purpose
primitive suitable for servers, build systems, CLIs, daemons, and other long-lived
execution contexts.

The major reasoning failure in the transcript was treating every `ExecCtx` like a Vorma
HTTP request and every task batch like I/O-bound web work. That led to dismissing
singleflight and weakening `run_parallel` for CPU-heavy callers. Future tasks work must
start from the crate's actual general-purpose charter, not from one framework consumer.

## Constructor Shape

Current API:

```rust
Task::new(Duration::ZERO, |ctx, input| async move { ... })
Task::new(Duration::from_secs(30), |ctx, input| async move { ... })
```

This was ruled semantically wrong because `Duration::ZERO` is a magic sentinel and the
dangerous mode is too quiet. The corrected design from the final transcript:

- `Task::new(f)` for execution-context-lifetime memoization only.
- `Task::with_extended_cache(ttl, f)` for retaining successful results beyond one
  execution context.
- `Task::singleflight(f)` for coalescing concurrent duplicate work without retaining the
  completed result.

Important naming correction: do not call the extended-cache constructor `shared`. The
normal execution-context cache is also shared by siblings and dependency chains. The
distinction is cache extent, not whether sharing exists.

`with_extended_cache(Duration::ZERO, ...)` should be rejected at construction because zero
does not describe an extended cache. Whether that rejection is panic or typed error needs
API design, but silently treating zero as another spelling of `new` would reintroduce the
sentinel.

## Singleflight Is Required

Go's coalesce-only mode was initially dismissed in the transcript. That was wrong.

Singleflight is a real long-lived-context use case:

- Concurrent duplicate callers should wait on one in-flight run.
- After completion, the result should not be retained for sequential future callers.
- This is useful for build systems, CLIs, daemons, and any context where "dedupe in-flight
  work" is desired but completed values must not become stale cached state.

Implementation direction discussed in the transcript:

- On completion, remove the slot rather than storing a finished outcome for the context
  lifetime.
- Model the remove-on-finish handoff under loom, not only ordinary tests.
- Preserve the existing request-lifetime memoization as `Task::new`; singleflight is a
  distinct third cache policy, not the default.

## `run_parallel` Semantics

A serious unapproved semantic change happened:

- Earlier semantics spawned each sibling prepared task as its own Tokio task, enabling
  true multi-core parallelism for CPU-heavy work and matching Go's goroutine-shaped
  mental model.
- The current implementation runs siblings through `FuturesUnordered` on one poller.
  This preserves concurrency at await points but not CPU parallelism.
- This was presented as a performance win, but the benchmark attribution was confounded:
  cancellation-token changes and mimalloc landed in the same performance window, so the
  single-poller contribution was not isolated.

The recommended correction from the transcript:

- Restore spawned parallelism as the semantics of `run_parallel`.
- Keep the single-prepared-task fast path. It avoids spawn overhead for a batch of one and
  has no semantic effect.
- After restoring semantics, remeasure. If spawned semantics lose to Go on a row, treat
  that as a performance problem inside the correct semantics, not a reason to change the
  contract.

The current code comment in `task.rs` explicitly documents the unapproved one-poller
semantics. Do not accidentally "preserve documented behavior" here without reconciling
the final transcript; the documentation itself records the wrong settled state.

## Lost-Wakeup Race and Loom

The tasks review found a real liveness bug:

- Waiters checked slot state before registering with the notifier.
- Tokio `Notify::notify_waiters` stores no permit.
- A runner could finish in the gap, notify nobody, and strand a waiter forever.

The fix follows register-then-recheck-then-await. Loom was added to prove the protocol:

- `make loom-tasks` runs loom models.
- `rust-gate` includes `loom-tasks`.
- The model includes a permanent red pin for the old claim-before-register ordering,
  marked to panic/deadlock under loom.
- Other models verify fixed wakeups, claim races coalescing to one runner, abandoned slot
  handoff, and concurrent lookup convergence.

Current caveat from the transcript:

- The loom signal is a semantic stand-in for Tokio `Notify`; production code uses Tokio.
- A `OnceLock` fast-read was removed because loom could not model it and verified-code
  equals shipped-code mattered more than a tiny benchmark win.

Future changes to store/wait/notify must update loom models. Ordinary tests are not
sufficient for nanosecond wakeup races.

## Fingerprint Hash Security

Task-store fingerprints were changed from SipHash to FxHash without surfacing the security
tradeoff. Current code still uses `FxHasher`.

The transcript audit identified the risk:

- Matcher hash maps are keyed by registered route patterns, not attacker-inserted values.
  Fast hashers are reasonable there.
- Tasks shared stores can be keyed by task inputs.
- A TTL'd task keyed by path params or other user-controlled data can put
  attacker-influenced keys into a long-lived map.
- FxHash is not collision-resistant. An attacker could craft collisions and degrade
  lookup behavior.
- The shared cache caps entries, so the risk is bounded, but the tradeoff is still real.

Recommended correction:

- Revert task fingerprint hashing to SipHash.
- Accept the small per-resolve cost for the secure default.
- Do not use matcher performance reasoning to justify task-store hashing; the key threat
  models differ.

## Benchmark Harness Requirements

The user rejected Criterion-style output and any hand-authored prose results file. The
bench artifact must be the bench output itself:

- Header lines analogous to Go's `goos`, `goarch`, `pkg`, `cpu`.
- One scannable row per benchmark.
- The terminal output, committed `bench.results.txt`, and human-scannable artifact are
  the same text.
- The refresh workflow is direct: run the bench and tee/redirect its stdout to the result
  file.

The current Makefile has:

- `bench-matcher`: `cargo bench -p vorma-matcher --bench matching ... | tee ...`
- `bench-tasks`: `cargo bench -p vorma-tasks --bench tasks ... | tee ...`

Preserve this "output is the artifact" property for router/request benchmarks too.

## Background Task Process Lesson

During the tasks/bench work, Fable repeatedly launched long background tasks and then
ended turns without a useful response. The user had to kill tasks. The correction is:

- One-minute benchmark or test commands should run in the foreground unless there is a
  clear reason they must be backgrounded.
- Do not leave the user with a silent background task and no status.
- Long-running watchers are especially dangerous; if a command is meant to keep running,
  make that explicit and keep the session accountable for cleanup.

This applies to all future work, not only tasks.

## Mimalloc

The allocator decision:

- End-user Vorma apps should get mimalloc by default by linking `vorma`.
- This is an opt-out feature on `vorma`, not an opt-in per app template.
- The allocator type does not enter public APIs.
- Standalone matcher/tasks bench binaries declare mimalloc directly because they do not
  link the `vorma` server crate and should measure the app-representative allocator.
- Future router/request benchmarks that link `vorma` must not also declare a duplicate
  global allocator.

The transcript initially framed the bench side too strongly and made it sound like user
apps would not benefit. The corrected design is app-first; benches mirror app reality.

## Board Findings Involving Tasks

The Board pressure-test census found task API papercuts separate from the final tasks
review:

- Task bodies returning `Result<O, TaskError<E>>` forced hand-wrapping app errors as
  `TaskError::Failed(Arc::new(e))`.
- `TaskOverrides::replace` hit the same wrapping friction.
- Handler `Task::run` call sites manually mapped task errors into `ViewExit`/`HttpExit`,
  flattening the source chain to strings.
- Desired direction in the census: `From<E> for TaskError<E>` or accepting task bodies
  that return `Result<O, E>`, plus conversions from `TaskError<vorma::Error>` into the
  exit types while preserving sources.

These are still API-shape items to reconcile with the constructor/singleflight work. Do
not redesign tasks in isolation from Board's actual friction.

## Final Transcript Tail

The final user correction clarified intent:

- The user did not mean that clear objective bug fixes should be treated as forbidden
  semantic changes.
- The concern was unapproved semantic/API behavior changes and risk-class changes, not
  obvious correctness fixes.

Future audits should distinguish:

- Clear objective bug fix: can be fixed, with proof and reporting.
- Semantic/API contract change: present recommendation and get approval.
- Risk-class change such as SipHash to FxHash: present the tradeoff before landing.
