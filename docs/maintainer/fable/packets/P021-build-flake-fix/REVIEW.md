# P021 Review

Reviewer: Fable (orchestrator). Date: 2026-07-02. Executor: Sonnet 5 subagent.

## Verdict

**Accepted. Packet closed; the flake ticket closes with it.** The ruled fix landed exactly
as sanctioned — test-scoped, bounded, fail-loud — and the execution exceeded the
diagnosis: the executor discovered the ticket had conflated two distinct collision windows
(`dev_mux_port`'s immediate rebind vs `app_server_port`'s much later one through a
previously PANICKING test path a `Result` retry could never have caught) and closed both,
then swept all 12 bind sites in the crate to prove scope completeness. The judgment calls
are all correct on review: variant-matched detection (message strings are locale-dependent
and foreign to the crate's idiom), fresh ports per attempt (same-port retry
deterministically loses against the real contender class: sibling tests holding ports for
whole test lifetimes), fresh fakes per attempt (side-effect double-counting), two
module-local helpers over a forced shared one. Verification met the ticket's bar with
margin: 12 clean loops under genuine load with raw logs manually verified — the "never
summary-only greps" lesson from this ticket's own P003-era origin, honored.

## Independent verification performed (Fable)

- `cargo test -p vorma-build --lib`: 136/136 under Fable's own hand.
- `tests/public_api.rs`: zero diff vs HEAD (the Rider B no-op proof holds).
- The ticket's FIXED section present; append-only history intact.

## Corrections to the record (Fable)

- The disclosed "unexpectedly dirty" `vite_plugin_contract.rs` was NOT the harness
  index-staging mechanism the executor attributed it to: it was Fable's own queued P017
  rustdoc fix, landed at 17:00 (pre-dispatch) exactly as recorded in P017's REVIEW. The
  executor's ACTION was exactly right — investigate mtimes, leave alone, disclose — which
  is the escalation discipline working; only the attribution is corrected here.
- Rider B also retires the P014 finding 1 row from the batch triage (it was re-classified
  from "breaking surface change" to proven no-op before dispatch; now landed and
  golden-verified).

## Consequence

The workspace's only known flake is fixed by construction. Test count: 593 (+2 Rider A).
The flake ticket is deleted per the completed-work convention (its diagnostic history
lives on in P014/P015/P021 REPORTs; the doctrine lesson goes to LEARNINGS). Phase D is now
FULLY closed pending only the maintainer's triage rulings.
