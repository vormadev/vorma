# P021 — vorma-build port-allocation flake fix (sanctioned) + two riders

Read `docs/maintainer/fable/README.md` (the packet protocol binds you), then
`LEARNINGS.md`, `STATE.md`, repo-root `AGENTS.md`, `docs/maintainer/REMINDERS.md`, and the
governing ticket IN FULL:
`docs/maintainer/tickets/vorma-build-lib-test-flake-under-load/__TICKET.md` — the root
cause is fully diagnosed there and the fix shape is already ruled; your job is to land it
exactly as sanctioned and prove it.

## Scope

1. **The ruled flake fix (test-scoped only).** Per the ticket's Fable ruling + scope
   amendment: close the bind-`:0`/drop/rebind TOCTOU with retry-on-`AddrInUse` (bounded
   retries, fail loudly after N) covering BOTH allocation shapes as used from tests —
   `allocate_test_port()`'s three dev-mux test callers (`dev_mux.rs`) and
   `allocate_loopback_port()`'s test callers (`dev_build.rs`, helper ~line 1544).
   Production functions stay UNTOUCHED (the `port == 0` rejection is a doubly-enforced
   frozen invariant). No single-use-helper violations: shape the retry so it earns its
   existence (one shared test helper is fine if both modules can genuinely reach it;
   otherwise justify the shape you pick in the report).
2. **Rider A — the P014 coverage gap.** Add the missing direct unit tests: a real
   `port == 0` input is rejected by `DevMuxError::InvalidPort` and (independently) by
   `DevRefreshOriginPolicy::new`. Strengthen-only.
3. **Rider B — entrypoint pub-in-private narrowing (proven no-op).** `mod entrypoint` is
   private (`lib.rs:50`); only `run` is re-exported (`lib.rs:77`). Narrow
   `build_production`/`start_dev_server`/`BuildEntrypointError` (and any other
   pub-in-private items in that file) to `pub(crate)`. PROOF OBLIGATION: the crate's
   `tests/public_api.rs` golden must be byte-unchanged — if narrowing changes the public
   API surface in any way, STOP and escalate instead of landing. Update `entrypoint.rs`'s
   own module doc if it references the old visibility story.

## Verification (the ticket's own bar)

- Loop the FULL workspace suite under parallel load 10+ consecutive times with zero
  intermittent failures (capture full output to files under your scratchpad, never
  summary-only greps; report the loop count and any non-flake noise seen).
- The retried tests still fail loudly when their condition is genuinely broken (no
  weakening — demonstrate by reasoning about the retry bound, not by breaking the code).
- Gates: `cargo clippy --workspace --all-targets -- -D warnings`,
  `cargo fmt --all --check`, workspace tests + doctests green, `make loom-tasks`
  untouched-green.

## Hard constraints

- Read-only git ONLY (never stage/commit/push/restore); the large dirty tree is EXPECTED
  prior-packet work — an unexpectedly dirty file you did not author is an escalation,
  never cleanup. No network installs. Scoped fmt writes only. Tests never cheat: no
  timeout inflation, no tolerance hacks, no skips.
- Semantics frozen everywhere else; nothing in this packet grants production changes.

## Definition of done

Fix + riders landed, the 10+ loop clean, gates green, ticket updated with a dated "FIXED"
section describing what landed (append-only; never rewrite prior history). REPORT.md
content delivered IN the final message body.
