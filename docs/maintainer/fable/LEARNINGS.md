# Durable Learnings

Technical doctrine and hard-won facts that remain true going forward. Not history: if a
fact stops mattering, delete it. Working-protocol rules live in the repo-root
`AGENTS.md`; this file holds framework knowledge.

## Matcher doctrine (crates/vorma-matcher)

- **Specificity is the single organizing principle.** One total order decides every
  competition: segment score (static/index = 2, dynamic = 1, splat = 0), then leftmost
  differing position rank, then trailing-splat loses, then longer wins.
  `compare_specificity` in `pattern.rs` is the one home; matching, graph validation, and
  dispatch all delegate to it. Nothing may reimplement the ordering.
- **Overlap between routes is the normal condition.** The only illegal state is a tie:
  two patterns of identical shape competing for the same path. Graph compile rejects
  GET/HEAD resource-vs-view ties only; everything else coexists and specificity
  adjudicates at dispatch.
- **The dirty-path rule:** at most one trailing slash is tolerated as noise; an empty
  segment never matches anything; a doubled slash anywhere means no match; params and
  splat values never contain empty strings; the root catch-all matches the root path
  with empty splat values.
- **The catch-all cover rule:** a prefix hit is not a match. `/*` yields only to an
  entry whose chain actually completes through the path; dead prefixes drop out and the
  catch-all claims the path. This is what makes `/*` the complete custom-404 answer.
- **An index pattern claims its parent path on its own shape** — no registered
  non-index sibling required, static and dynamic spellings identical.
- These four behaviors were corrections of bugs inherited from the Go original,
  maintainer-ratified as corrections (not divergences). The property-model suite in
  `tests/matching_semantics.rs` re-derives all semantics independently; the overlap
  oracle suite brute-forces `find_overlap` against exhaustive path enumeration. Any
  semantic change must first go red in those suites.
- Public capture types (`Params`, `SplatValues`) are opaque vorma-owned structs exposing
  only `&str`/`String` views. Storage (hash maps, small vectors, shared buffers) is an
  implementation detail and must stay out of public signatures.

## Tasks doctrine (crates/vorma-tasks)

- **General-purpose primitive, not a request helper.** Execution contexts model builds,
  CLIs, and daemons as much as HTTP requests. Judge every design choice by the crate's
  own contract. Two consequences already ratified: parallel batches use true spawned
  parallelism (CPU work must scale across cores, like Go's goroutines), and the
  single-flight policy exists (coalesce concurrent duplicates, retain nothing).
- **Three cache policies, declared per task:** `memoized` (retained for the execution
  context's lifetime), `extended_cache(ttl)` (additionally retained across contexts;
  zero TTL rejected at compile time), `single_flight` (coalesce in-flight only). Both
  memoization layers are "shared" — the distinction is cache *extent*, which is why the
  extended policy is not called "shared".
- Deliberate semantics, not accidents: local memoization retains non-cancellation
  errors; the extended cache does not retain errors; a success produced by a run that
  was cancelled mid-flight is discarded (inherited from Go on purpose); cycle detection
  returns an error instead of deadlocking (improvement over Go, which relied on
  compile-time init cycles).
- **The slot wait/notify protocol is loom-verified** (`make loom-tasks`, models in
  `src/loom_tests.rs`). The founding bug: waiters that check slot state before
  registering with the notifier lose wakeups forever, because tokio's `notify_waiters`
  stores no permit. The register-then-recheck-then-await ordering is load-bearing; a
  permanent should-panic loom model pins the broken ordering. The loom signal is a
  semantic stand-in for tokio `Notify` (wakes only pre-registered waiters, no permit) —
  that fidelity caveat is accepted and documented; everything else under loom is the
  production code itself.
- Task inputs are attacker-influenceable in server contexts (for example a task keyed by
  a path parameter with an extended cache), so fingerprint hashing is a denial-of-service
  surface: it must not be a trivially collidable hash. It is also the per-resolve hot
  path, so it must be cheap. Keyed SipHash via a process-random key satisfies both;
  unkeyed truncated cryptographic hashes are slower and weaker against offline
  precomputation; raw speed hashes are unacceptable here. (The matcher's maps are not
  such a surface: their keys are registered patterns, which attackers cannot insert.)
- Tasks are const-constructed static nodes declared with the `task!` macro; bodies are
  capture-free by construction; `Task` is a Copy handle with a lazily assigned process
  ID. Runtime-constructed tasks are not part of the model.

## Performance conventions

- **The bar, maintainer-set: beat the Go implementation's numbers, not match them.**
  Go-era baselines live in the old Go repo (`kit/matcher/results.bench.txt`,
  `kit/tasks/bench.txt`) and are mirrored in `STATE.md` tables.
- Benchmarks use the internal `vorma-bench` harness (Go-style output: detected
  os/arch/crate/cpu header, one line per benchmark, median of timed batches).
  `bench.results.txt` files are pure redirects of `make bench-*` targets — the terminal
  output, the recorded file, and what a human scans are the same text. Machine-specific
  numbers in the repo are fine; the header labels them.
- Vorma apps run mimalloc by default via a default-on `mimalloc` cargo feature on the
  `vorma` crate (any crate in a binary's graph may declare the global allocator; opt out
  with `default-features = false`). Bench binaries of crates that do not link `vorma`
  declare the allocator themselves so recorded numbers reflect the app-default
  environment. Never declare the allocator in a library crate that vorma-linking
  binaries also link — one binary allows only one declaration.
- Proven hot-path techniques in this codebase: compute lookups from borrowed inputs and
  clone only on insert; rebuild captures positionally once at emit instead of threading
  them through walks; carry candidates as references; inline small collections
  (smallvec) for stacks and buckets; store registered values on tree nodes instead of
  re-looking-up by string; gate observer/clock work on observer presence; share one
  buffer for multi-segment captures. Third-party types powering these stay behind
  vorma-owned opaque surfaces.
- Benchmark attribution discipline: never bundle a semantic change with mechanical
  optimizations in one measured step; the founding counterexample was a concurrency-model
  change credited with wins that mostly came from a cheaper cancellation token measured
  in the same batch.

## Testing conventions

- Semantic changes require a red pin first: write the test asserting the corrected
  behavior, watch it fail, get maintainer agreement, then change the code.
- The matcher's independent property model and brute-force overlap oracles are the
  template for "coverage at least as good as Go": port Go suites case-for-case, then add
  the adversarial layers Go never had (property models, oracles, loom, trybuild UI tests
  for compile-time contracts, deterministic clocks instead of sleeps).
- Concurrency liveness cannot be pinned by ordinary tests (a suite can sit green on top
  of a real deadlock); loom models are the tool, kept tiny (two or three threads, few
  sync operations) and coupled to named protocol steps on the production types.

## Gotchas worth their weight

- tokio `Notify::notify_waiters` wakes only already-registered waiters and stores no
  permit. Any wait loop must register (`Notified::enable`) before re-checking the
  condition it waits on.
- `tokio::select!` subscription setup has real per-call cost on hot paths; a single
  noop-waker poll of the primary future before wiring cancellation branches skips that
  cost for bodies that complete immediately, without changing observable semantics
  (post-completion cancellation checks still apply).
- Custom `Hasher` passthroughs that rely on derived-`Hash` field order are brittle;
  implement `Hash` manually on the key type so the routed value is structural, not
  positional.
- criterion's multi-line statistical output is why the bench harness exists; do not
  reintroduce it.
- `cargo bench -p <crate> --bench <name>` avoids the lib target's empty "running 0
  tests" noise in recorded files.
