# P014 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-02) was blocked from writing
this file by the recurring harness report-file guardrail; content re-emitted on request
and placed by Fable.

## What changed

Thirty files, all in `crates/vorma-build/src/` (+1175/−191, doc-dominated, two small
disclosed mechanical fixes), plus one ticket update:

- `lib.rs` — crate-root doc rewritten around the crate's actual shape: a "one door in"
  framing (`run` is the sole reachable item; everything else — hundreds of pub items
  across 33 files — exists only for cross-module visibility inside private mods and is
  unreachable from outside the crate, confirmed against cargo doc's rendered output and a
  repo-wide grep). A `no_run` doctest shows the real one-liner app pattern.
  `#![deny(rustdoc::broken_intra_doc_links)]` added (missing_docs predated, green).
- `entrypoint.rs` — `run` documented in full (both CLI modes traced against
  `parse_build_command`); `build_production`/`start_dev_server`/`BuildEntrypointError`
  documented honestly as unreachable-from-outside internal facade functions.
- Twenty-six further modules — every public item raised from one-line stubs to the
  teaching bar, cross-referenced to callers/callees, honest about
  framework-integration-vs-production-vs-test-only audience. Every doctrine claim traced
  against the implementation before being written; several first drafts corrected
  mid-sweep (see Decisions).
- `dev_build.rs` (2553 lines) — the central dev-session state machine
  (`StartedDevGeneration`) and both production entry points fully documented; module doc
  explains the four-scenario generation-update shape.
- `dev_watcher.rs` — semantics-frozen: module doc states the read-vs-write event gating is
  load-bearing (the P001b fix) and points at the doctrine comment; references the standing
  watch-scope ticket at the exact registration site.
- `process_runner.rs` — disclosed doc-accuracy fix: a stale `GO_PARITY_AUDIT finding 6`
  reference (no such document exists anywhere in the repo) corrected to the real
  `windows-child-exit-watcher` ticket.
- `output_lock.rs` — disclosed zero-behavior test rename:
  `acquire_blocks_second_acquirer_until_release` →
  `acquire_fails_fast_for_second_acquirer_until_release` after reading the underlying
  `paranoid::local_lock::ProcessLock` source in full: it is a heartbeat-lease lock that
  fails fast on contention, never blocks — the old name was actively misleading.
  Assertions unchanged.
- Two doctest attempts required inventing public surface; caught, reverted, replaced with
  explicit "no doctest, here's why" notes.
- Flake ticket appended with the full mechanism diagnosis; NOT deleted (fix not landed).

No public API changes (cargo doc's rendered surface is still exactly `run`;
`tests/public_api.rs` unchanged, passing). Dev-loop pipeline semantics untouched.

## Decisions made

1. **Flake: diagnosed precisely, not fixed** (dedicated section below) — the
   highest-judgment call of the packet.
2. **Entrypoint pub items left un-narrowed:** zero reachable external callers, but
   visibility narrowing is adjacent to public-API shape and ungranted — documented
   honestly as Finding 1 instead of silently narrowed.
3. **The `process_ready.rs` 25ms readiness poll is NOT a REMINDERS violation:** the
   no-polling invariant scopes to dev EVENTS (file changes, shutdown, via channels); this
   polls the unavoidable "has the just-spawned child started accepting HTTP yet" question
   (no portable OS signal exists), bounded and terminating on
   response/exit/cancel/deadline. The distinction is now documented in the module doc.
4. **Doctest scope pulled in hard:** exactly one doctest (crate-root `run`, `no_run` — it
   genuinely spawns processes); everything else prose, honestly noting non-reachability,
   rather than doctests against fabricated surface.
5. **Doc claims verified against source, not names:** the ProcessLock semantics (read from
   crates.io source); write_file_atomically vs publish_public_static_outputs idempotency
   mechanisms are NOT the same (byte-comparison vs content-addressed-existence — caught
   and corrected); the kind_override REMINDERS cross-reference verified against the actual
   emission logic; event/deps behavior re-read from implementing code.

## Doc-sweep stats

433 public items (108 types/traits, 305 fns/methods, 21 consts/statics) — all raised to
the teaching bar. This crate's "framework-integration" framing means LITERALLY unreachable
from outside (unlike the other crates) — stated plainly at the root and reinforced
per-item. Doctests: 0 → 1 (deliberate; see Decisions 4). Three test-only/zero-pub files
confirmed, not skipped.

## Flake disposition: diagnosed with precision, left ticketed

- **Root cause:** `allocate_test_port()` (private, `#[cfg(test)]`, `dev_mux.rs`) binds
  `:0`, drops the listener, and hands the bare u16 to `start_loopback_dev_mux_server`'s
  own independent bind — a textbook discover-drop-rebind TOCTOU. Three tests use it.
- **Proven:** a minimal 64-thread reproduction of exactly this shape measured a 0.03%
  collision rate (128,000 attempts, 38 collisions) — sufficient to explain a rare flake
  under full-workspace parallel load.
- **Every other bind site (8 test call sites + the production function) already uses `:0`
  directly.** The helper exists only because the production `port == 0` rejection is a
  genuine, doubly-enforced invariant (also in `DevRefreshOriginPolicy::new`): a dev mux
  needs one stable known port that doubles as the allowed WebSocket Origin port.
- **Why not fixed:** the packet's grant was "exactly the switch-to-:0 shape." The
  test-only retry-on-AddrInUse wrapper is not literally that shape; the two fixes that are
  require touching the frozen production function's validation ordering/structure. Forcing
  either would improvise past a constraint. Ticket updated with mechanism, proof,
  per-candidate qualification analysis, and a concrete recommendation. [Fable ruling since
  recorded on the ticket: the test-scoped retry-on-collision is sanctioned.]
- No recurrence in ~15 targeted + 2 full-workspace runs this session — consistent with a
  sub-1% contention race; the mechanism proof stands independent of reproduction luck.

## Findings (analyzed, not implemented, unless marked landed)

1. Entrypoint pub items with zero reachable callers anywhere outside their defining file —
   held (visibility/surface class).
2. `dev_build.rs` size outlier (2553 lines, >4x next) traced in full: proportional to real
   domain complexity (four genuinely distinct rebuild scenarios). One narrow
   near-duplication exists (`build_and_publish_live_*` vs `..._public_static_*`) but
   differs in a real behavioral step (output-layout-lock reacquisition the static path
   correctly omits); unifying needs a control-flow-complicating parameter — the review
   bar's own anti-pattern. Held, not recommended mechanically.
3. LANDED: the stale `GO_PARITY_AUDIT` reference corrected (doc accuracy).
4. LANDED: the misleading blocking-semantics test name corrected (zero behavior).
5. Production `allocate_loopback_port()` shares the theoretical TOCTOU shape but is a
   materially lower-risk, industry-standard reserve-for-external-process pattern (three
   ports for three external processes; no way to pass an open fd to a Node child here).
   Documented precisely at the call site; not actioned.
6. Coverage gap: the two `port == 0` rejections have no direct unit test — entangled with
   the flake fix's design; lands with the ruled fix.
7. The 25ms readiness poll examined against REMINDERS and cleared with recorded reasoning
   (reported per report-all-issues even though the verdict is not-an-issue).
8. Cleared: the Vite RPC token's plain != compare — loopback-only, per-session
   32-byte-random token; a loopback-timing attacker already has environment access.
   Flagging it would be threat-model-blind dogma.
9. Cleared: the dev-refresh WebSocket origin policy is a strict exact allowlist
   (scheme/host/port) plus unguessable path token — sound, verified by reading.
10. The `null as unknown as typeof X` bundle-size-enforcement pattern named precisely in
    docs where the enforcing code lives.

No structural code-judo opportunity found: the public surface is minimal by construction,
module boundaries track real domain seams, and the one size outlier is accounted for.

## Checklist verdict

All 13 items PASS (full evidence table in the packet record); three carry attached
findings (Performant → 7; Comprehensive Tests → 6; Nothing weird → 3, fixed). No FAILs.

## Gate results

- vorma-build lib 134/134 (~15 runs, zero failures; flake did not recur — expected);
  workspace 588/0 twice; doctests 43/0 (vorma_build 1/1); clippy clean; fmt clean (zero
  diff, no write needed); doc build `-D warnings` clean; loom 7/7 untouched-green;
  public_api pin 1/1; release build clean; audit clean modulo the allowed
  RUSTSEC-2026-0190; forbid(unsafe_code) zero exceptions.

## Benchmarks

Not applicable — no bench suite; nothing performance-scale touched.

## Escalations / open questions

1. The flake fix disposition (ruled by Fable at review: test-scoped retry sanctioned).
2. Findings 1/2/5/6 held for the batched Phase D-end triage (Fable filed
   `build-release-quality-findings` at review).

## Discovered out-of-scope work

None beyond the above; the two standing tickets referenced at their exact code sites, not
duplicated.
