# P004 Report — Part 1 (measurement baseline)

Provenance note: the executor (Sonnet 5 subagent, 2026-07-01) was blocked from writing
this file by a harness guardrail ("Subagents should return findings as text, not write
report files") and returned the full content in its final message; Fable placed it here
verbatim-in-substance, restructured only to match the protocol template headings. Part 2
was not started, as instructed.

## What changed

- `crates/vorma/benches/engine.rs` — new engine bench fixture (design below).
- `crates/vorma/Cargo.toml` — bench target registration.
- `Makefile` — `bench-engine` target following the `bench-matcher`/`bench-tasks`
  per-machine recording pattern; added to phony list.
- `Cargo.lock` — updated by the manifest change.
- `docs/maintainer/bench-results/vorma/sjc-z390-aorus-pro-wifi.bench.results.txt` — new,
  the recorded baseline (pure `make bench-engine` redirect).
- `docs/maintainer/fable/STATE.md` — engine baseline table added (packet DoD).

## Fixture design

A "request" in this bench is an already-collected `http::Request<Bytes>` (no socket, no
adapter) driven through `TestApp::handle_request`, which calls exactly the same code
`CommittedRuntimeService::handle_request` does in production
(`CommittedRuntimeApp::handle_http_request_with_document_provider` — classify, execute,
finalize), returning `http::Response<Bytes>`. The only exclusions are body
collection/limits and HEAD suppression, which live in the transport-facing `Service::call`
wrapper one layer up and are adapter concerns by the packet's own framing. One `TestApp`
registers: a static view; a 4-level nested dynamic view chain (`/`, `/shelves/:shelf_id`,
`.../boards/:board_id`, `.../cards/:card_id`, each carrying its own param); a JSON
mutation resource; a `ResourceBody` binary resource (4KiB fixed payload, no task/DB
dependency); a root catch-all view (`/*`); and two unscoped middlewares that run on every
request except in the dedicated no-middleware control app. A second app variant omits the
catch-all to isolate a genuine classifier-miss 404. All 9 packet-specified rows are
present (the depth-4 chain gives "3+ deep" margin); every row rotates two realistic paths
per the matcher-bench convention.

## Decisions made

1. Added a 10th row beyond the packet's 9 — `request_through_middleware_chain_0_control` —
   a same-route zero-middleware control app, so the middleware row measures the middleware
   tax rather than route cost.
2. Two fixture-design issues found and fixed within Part 1's own scope (not deferred):
   `method_not_allowed` needed PUT rather than GET to be a genuine mismatch, and the
   middleware row needed a true control.
3. No optimization work of any kind; Part 2 untouched pending Fable's ANALYSIS.md.

## Gate results

All green: `make rust-bench` (includes `benches/engine.rs`);
`cargo clippy --workspace --all-targets -- -D warnings` (0 warnings);
`cargo fmt --all --check` + fuzz manifest (clean);
`cargo test --workspace --all-targets` + `--doc` — 551 tests + 1 doctest = 552 passed, 0
failed, matching the STATE.md baseline, no regressions.

## Benchmarks

Recorded via `make bench-engine` on an idle machine (verified via top/uptime/ps after
waiting out an unrelated 100%-CPU process — see escalation 2), written to
`docs/maintainer/bench-results/vorma/sjc-z390-aorus-pro-wifi.bench.results.txt`:

```
os: linux
arch: x86_64
crate: vorma
cpu: Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz
static_view_render                                      31970      48798 ns/op
nested_dynamic_view_chain_4_deep                        20355      70348 ns/op
json_resource_small_input_output                        59825      25051 ns/op
resource_body_binary_resource                           61535      24100 ns/op
request_through_middleware_chain_2                      32740      44903 ns/op
request_through_middleware_chain_0_control              42950      39500 ns/op
not_found_catch_all_view                                32610      45061 ns/op
not_found_bare_no_catch_all                            685335       2163 ns/op
head_request_to_resource                                64810      24518 ns/op
method_not_allowed                                     375995       4048 ns/op
```

No Go-era baseline exists for this surface; this recording IS the baseline.

### First-read of where time goes (executor)

The two rows that skip handler/view execution entirely — `not_found_bare_no_catch_all`
(2.2µs) and `method_not_allowed` (4.0µs) — are nearly two orders of magnitude cheaper than
every row that reaches a handler (24–71µs band), which is the clearest signal that
classification/matching is not where the cost lives; the weight sits in
decode/execute/finalize. Within handler-reaching rows, the two resource rows are cheapest
(~24–25µs) despite one carrying a 4KiB payload, while the view/HTML-rendering rows cost
noticeably more (~40–49µs) for comparably small handler bodies, pointing at document/HTML
finalization as a real, separate contributor. The isolated middleware pair costs ~5.4µs
(~14%) over its same-route zero-middleware control — a plausible tax for two
contract-required parallel spawns plus effects-merging, not obviously alarming standalone.
The 4-deep nested chain is the single most expensive row (70.3µs) and scales worse than 4x
a single view, consistent with per-node decode/execute/effects-merge cost stacking across
depth rather than one fixed cost.

## Escalations / open questions

1. **REPORT.md could not be written by the executor** (harness guardrail); placed by Fable
   from the executor's final message.
2. **Machine was not idle at task start** — an unrelated 100%-CPU TVM/ROCm build plus two
   concurrent sibling agent sessions were active. The executor did not interfere (outside
   packet authority); it waited (~8 min bounded poll) for the unrelated process to exit
   and load to drop to idle-desktop baseline (97%+ idle), confirmed via top/uptime/ps,
   then recorded. Three same-conditions runs showed normal run-to-run variance only.
   Flagged because this is a shared desktop, not a dedicated bench box.
3. **The gitStatus snapshot handed to the executor said "(clean)" while the tree carried
   the session's uncommitted accepted work** — a harness snapshot-staleness discrepancy,
   not a repo problem. The executor touched none of the pre-existing dirty files; its
   STATE.md edit is a pure insertion.

No blocking finding on the owned-request entry point (it exists cleanly:
`TestApp::handle_request` / `CommittedRuntimeService::handle_request`).

## Discovered out-of-scope work

None; no tickets filed. The two fixture-design issues found were fixed within Part 1's own
scope.
