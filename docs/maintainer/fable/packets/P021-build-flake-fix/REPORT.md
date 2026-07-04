# P021 Report

Provenance note: delivered by the executor (Sonnet 5 subagent, 2026-07-02) in its final
message per the anti-truncation convention; Fable placed it verbatim (transport escaping
undone).

## What changed

- `crates/vorma-build/src/dev_mux.rs` — added `start_dev_mux_retrying_port_collisions`
  (`#[cfg(test)]`), a bounded-retry (8 retries, 9 attempts) wrapper around the whole
  `allocate_test_port()`-then-`start_loopback_dev_mux_server()` sequence, re-deriving a
  fresh candidate port each attempt; migrated all three of the ticket's named test callers
  onto it. Added `start_loopback_dev_mux_server_rejects_zero_port` (Rider A): direct unit
  test asserting `start_loopback_dev_mux_server(0, ..)` returns
  `DevMuxError::InvalidPort`.
- `crates/vorma-build/src/dev_build.rs` — converted `spawn_ready_server_on_port`
  (test-only) from a panicking `.unwrap()` to `std::io::Result<JoinHandle<()>>`, and
  `FakeProcessRunner::start_inheriting_stdio` (test-only) now propagates that as
  `BuildProcessError::Start` through the existing production error chain instead of
  discarding it. Added `start_dev_generation_retrying_port_collisions` + its return bundle
  `StartedDevGenerationForTest` (`#[cfg(test)]`), a shared bounded-retry helper covering
  BOTH of `allocate_loopback_port()`'s real downstream rebind consumers (`dev_mux_port`
  via `start_loopback_dev_mux_server`, `app_server_port` via
  `spawn_ready_server_on_port`); migrated all five of the module's dev-generation-starting
  tests onto it, including the ticket-named
  `dev_server_recompile_update_reuses_committed_static_outputs`. This also collapsed five
  copies of an identical ~24-line setup block into five short destructuring calls.
- `crates/vorma-build/src/dev_refresh.rs` — added
  `refresh_origin_policy_rejects_zero_port` (Rider A): direct unit test asserting
  `DevRefreshOriginPolicy::new(0)` returns `DevRefreshError::InvalidBrowserOriginPort`,
  independent of the dev-mux-level test.
- `crates/vorma-build/src/entrypoint.rs` (Rider B) — narrowed `build_production`,
  `start_dev_server`, `BuildEntrypointError` from `pub` to `pub(crate)`; updated the
  module doc's visibility description.
- `docs/maintainer/tickets/vorma-build-lib-test-flake-under-load/__TICKET.md` — appended a
  dated "FIXED (P021, 2026-07-02)" section (append-only, prior history untouched),
  formatted with scoped `oxfmt` run three times to confirm true convergence (identical
  checksum on the last two runs).

Production code is untouched everywhere except Rider B's visibility narrowing (a proven
no-op — see below). Every port-collision-fix change lives inside `#[cfg(test)]` scope,
including the panic-to-`Result` conversion in
`spawn_ready_server_on_port`/`FakeProcessRunner` (both test-only types).

## Decisions made

- **Retry detection is variant-matched, never string-matched.** Both retry helpers check
  the specific error variant shape (`DevMuxError::Bind`, or the
  `DevBuildError::DevMux`/`DevBuildError::AppServer` pair), never a stringified message.
  Verified early that `io::Error::to_string()` for a real `AddrInUse` renders
  platform/locale-dependent text that does not match a synthetic `ErrorKind`-derived
  string, and confirmed via grep that every existing precedent in this codebase uses typed
  `.kind() == ErrorKind::X` matching, never message parsing.
- **The `app_server_port` collision (via `spawn_ready_server_on_port`) needed fixing too,
  not just the ticket's originally-suspected `dev_mux_port` path.** The ticket's P015 note
  says the named flaky test "calls `allocate_loopback_port()` twice" without saying which
  call collided. Both traced: `dev_mux_port` reaches a real bind almost immediately;
  `app_server_port` reaches its real bind much later (after dev-mux startup, Vite startup,
  TypeScript writes, RPC server startup), via a DIFFERENT, previously panicking code path
  (`spawn_ready_server_on_port`'s bare `.unwrap()`) that a `Result`-based retry could not
  have seen or recovered from at all. Since that helper is itself `#[cfg(test)]`-only,
  converting its panic into a propagated `Result` is fully test-scoped and closes a real,
  larger-window race the ticket's own diagnosis had not yet distinguished from the
  `dev_mux_port` one.
- **Retry re-derives fresh ports every attempt; it does not retry the same port number.**
  The most probable real contender under `--all-targets` parallelism is another
  concurrently-running dev-mux/dev-build test in the same crate holding its own allocated
  port for its entire multi-second test lifetime — a same-port retry would
  deterministically keep colliding against that class of contender; only a fresh-port
  retry is a real fix by construction.
- **`dev_build.rs`'s retry helper reconstructs every fake object from scratch each
  attempt** (`FakeVitePluginServer`, `FakeProcessRunner`, `FakeControlClient`), never
  reusing a losing attempt's fakes: a losing attempt can accumulate real side effects
  (recorded commands, control-client calls) before the collision surfaces; reuse would
  double-count that partial work in a later attempt's assertions.
- **Two separate retry helpers, not one shared across files.** The two helpers close over
  entirely different fake/config types with no common ground; a single shared body is not
  reachable from both modules without exposing test-only types across module boundaries.
  Per the packet's "shape the retry so it earns its existence" — each helper is used by 3
  (dev_mux) and 5 (dev_build) call sites respectively, well past the single-use-helper
  threshold.
- **Retry bound: 8 retries / 9 total attempts, identical in both files.** The diagnosing
  machine's 64-thread reproduction measured a 0.03% collision rate (38/128,000); this
  bound is deliberately generous headroom, and a genuinely broken (non-collision) caller
  fails on its very first attempt via a different error variant — the bound only governs
  how many fresh ports a real transient collision gets to try.
- **Rider B verification used `sha256sum` plus a direct rustdoc-artifact check, not just
  "the test still passes."** `crates/vorma-build/tests/public_api.rs` needed zero edits
  and its content hash was byte-identical before/after; additionally confirmed via
  `grep -rn "vorma_build::"` across the whole repo that `run` is the only external
  reference anywhere, and via `cargo doc --document-private-items` (with `-D warnings`)
  that the generated docs tree contains exactly one public item (`fn.run.html`).
- **The `vite_plugin_contract.rs` diff noticed mid-session is pre-existing, not mine.**
  Verified its filesystem mtime (17:00:28) predates the session's scratchpad-creation
  timestamp (17:08:09); swept every other modified file in the repo for any mtime after
  session start — none besides the four Rust files and the ticket deliberately edited.
  Disclosed per the packet's instruction, left exactly as found, no cleanup performed.
- **Verification loop used real added load.** 6 CPU-saturating processes (6 of 16 cores)
  ran concurrently with the loop (verified applied, stopped cleanly afterward); the
  captured logs confirm the board SQLite suite and the vorma-tasks bench binary genuinely
  ran concurrently with vorma-build's lib suite in every run — the exact conditions the
  ticket's diagnosis describes.
- **Restarted the loop once mid-run.** The first loop launch coincided with a separate
  foreground `cargo build` (a cosmetic panic-message wording fix), sharing the same
  `target/` lock. To avoid ambiguity about which source snapshot each run tested, the
  first loop was killed, its one partial log discarded, all remaining source edits made, a
  full clean gate pass run, and only then the final uninterrupted 12-run loop launched
  against a stable, final source snapshot — the loop reported here.

## Gate results

Workspace tests (single fresh pass after the loop): **593 passed, 0 failed** (vorma-build
lib: 136 = 134 baseline + 2 Rider A additions). Doctests: **52 passed**. Clippy
`-D warnings`: clean, re-confirmed after the loop and after the ticket edit.
`cargo fmt --all --check`: clean, same re-confirmations. `make loom-tasks`: **7/7**,
untouched-green.

**The loop (packet's bar of 10+):** `cargo test --workspace --all-targets` **12
consecutive times** under genuine parallel test-binary load plus 6 CPU-saturating
background processes. Every one of the 12 raw log files manually verified (never
summary-only greps) for `FAILED`, `panicked at`, `error:`/`error[`, and
`AddrInUse`/"Address already in use": **zero across all 12 runs.** vorma-build's lib suite
reported `test result: ok. 136 passed; 0 failed` byte-identically in all 12.

## Benchmarks

Not applicable — test-only helpers and one visibility keyword.

## Escalations / open questions

None that block. One disclosure: `vite_plugin_contract.rs` unexpectedly dirty mid-session;
investigated (mtime predates session), confirmed pre-existing, left as found. Small
unreconciled note: the ticket's P015 entry cites 588 workspace tests; the fresh
pre-existing count is 591 (+2 Rider A = 593) — fully explained by P016–P020 test additions
elsewhere in the interim.

## Discovered out-of-scope work

None filed. Swept every `TcpListener::bind` call site in the crate (12 total) to confirm
scope completeness: the only two genuinely TOCTOU-shaped patterns are the two the ticket
named, both now fully covered including both of `allocate_loopback_port()`'s real
downstream rebind consumers; every other bind is `:0`-bind-then-immediate-move with no
intervening drop, safe by construction.
