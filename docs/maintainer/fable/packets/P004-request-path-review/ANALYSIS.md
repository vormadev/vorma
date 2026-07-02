# P004 ANALYSIS — request-path cost decomposition and the Part 2 plan

Written by Fable from Part 1's recording
(`docs/maintainer/bench-results/vorma/sjc-z390-aorus-pro-wifi.bench.results.txt`,
reviewed in `REVIEW-part1.md`) and a source read of the execution path. This document is
Part 2's work order. The semantics fence from INSTRUCTIONS.md applies to every row of
the plan: matching results, execution model (the parallel contracts), error selection,
commit ordering, effects merging, response bytes — all frozen. Every change below is
mechanical, and each must be measured as its own step via `make bench-engine`
before/after per the attribution doctrine.

## The pipeline, as read

`TestApp::handle_request` → `CommittedRuntimeService::handle_request` →
`runtime_app.rs:254 handle_http_request_with_document_provider` → classify
(`execution_engine.rs:472`, matcher-backed, ~0.1–0.4µs per the matcher tables) →
`execute` (`execution_engine.rs:526`) → finalize per report kind
(`runtime_app.rs:322`).

`execute` for a handler-reaching request:

1. `handlers.request_exec_ctx()` — one vorma-tasks `ExecCtx` per request (Drop cancels).
2. Middleware id match + per-route decode: query params into a fresh
   `BTreeMap<String, Vec<String>>` (`decode_query_params`), typed input decode per view
   / resource (`input_decoder`).
3. `execute_invocations` (`execution_engine.rs:884`): per-invocation handler Arc lookup,
   `ordered_execution_contexts` (one child ctx per invocation), then TWO
   `JoinSet`-barriered phases (middlewares, then handlers — the parallel contracts).
   Per invocation, per phase (`execute_invocation_phase`, :973): `invocation.clone()`
   (pattern String + params + splat + `decoded_input` serde `Value`), Arc clones, an
   `AssetCapabilities` clone, a raw `pending.spawn(...)`, and INSIDE the spawned task a
   second clone set into `HandlerInput` (pattern/params/splat/decoded_input again,
   :997–1001). Outputs are committed in order; typed handlers serialize their output to
   `serde_json::Value` at the boundary (`typed_handler.rs:591`).
4. View HTML finalization (`view_response.rs:129`): `project_view_payload` —
   **deep-clones every committed view's output `Value`**
   (`payload_projection.rs:61`) — then `prepare_payload_head` rebuilds head elements,
   then per-element `render_document_element` calls, each allocating fresh `BTreeMap`s
   and cloning attribute sets (`view_response.rs:append_*`), building `head_markup` and
   `body_markup` strings, `ssr_payload_json` (serialize) plus a second full-copy escape
   pass (`response_finalizer.rs:500–514`), a `format!` for the CONSTANT root div, and
   finally full-document assembly wrapping the two markup slots.

The dev-only sha256 (CSP hash for the dev refresh script, `view_response.rs:330`)
returns empty in prod — not a bench cost.

## The arithmetic (Linux i9-9900K baseline)

Known unit costs: tokio spawn+join floor ~2.4µs/task (P002 attribution); matcher
~0.1–0.4µs; child ctx ~0.1µs; the measured two-middleware tax is 5.4µs
(`request_through_middleware_chain_2` 44.9µs vs its control 39.5µs → ~2.7µs per
middleware ≈ spawn floor + phase bookkeeping, consistent).

- Non-handler rows: `not_found_bare` 2.2µs, `method_not_allowed` 4.0µs — classify +
  minimal finalize. Confirms matching is noise.
- Handler-reaching shared floor: `json_resource` 25.1µs with a trivial finalize
  (`serde_json::to_vec` + headers). Net of ~5.4µs middleware phase, ~2.5µs handler
  spawn, ~1µs ctx/classify: **≈16µs of engine machinery per request** — the clone
  trees, decode, Value boundary, two-phase JoinSet bookkeeping, report/commit plumbing,
  response base.
- View HTML premium: `static_view_render` 48.8µs − resource-row 25.1µs ≈ **24µs of
  projection + head/element rendering + payload JSON + escape + document assembly** for
  a near-empty page. This is the largest single block in the whole path.
- Depth scaling: `nested_4_deep` 70.3µs ≈ static + 3 × ~7.2µs per extra view (spawn
  ~2.4 + input decode + two clone sets + Value clone at projection + commit + payload
  chunk).

## Part 2 plan — ranked, one measured step each

1. **Precompute snapshot-constant document fragments at commit time.** The critical-CSS
   element, CSS-bundle links, module script tag, root-div markup (today a `format!` of
   constants per request), and every head element derived only from the manifest are
   identical for the life of a committed snapshot but are re-built, re-rendered, and
   re-allocated per request. Render them ONCE at snapshot commit (same code path, stored
   as strings on the snapshot/manifest) and `push_str` the cached fragments at
   finalize. Byte-identical output by construction. Expected: the biggest single win on
   all view rows (attacks the ~24µs block). Rows: `static_view_render`,
   `nested_4_deep`, `not_found_catch_all_view`.
2. **Collapse the per-invocation clone tree.** `HandlerInvocation`'s shared immutables
   (pattern, params, splat_values, decoded_input) are cloned once into the spawn closure
   and AGAIN into `HandlerInput`. Restructure so the invocation carries `Arc`s (or the
   spawn moves the single clone straight into `HandlerInput` with no second copy).
   `HandlerInput` field types may change internally but the public accessors on the ctx
   (`pattern()`, `params()`, etc.) keep their signatures. Expected: shaves all
   handler-reaching rows; scales with depth. Watch: `Params`/`SplatValues` are already
   cheap-ish — measure before assuming.
3. **Single-invocation phase fast path.** When a phase has exactly one invocation,
   `pending.spawn` + `join_next_with_id` is pure overhead over polling the future
   inline — parallelism of one is indistinguishable from inline execution, so the
   parallel contract (which constrains N≥2) is untouched; panic propagation must stay
   identical (resume_unwind on panic exactly as the JoinSet arm does). Skip the JoinSet
   entirely for the empty-middleware phase if it does not already short-circuit.
   Expected: ~2–2.5µs on every single-view row and the resource rows;
   `parallel-of-one` rows in real apps are the most common shape there is.
4. **Take the projection's Values instead of cloning.** `project_view_payload` clones
   every committed view's data tree (`payload_projection.rs:61`). If the report is not
   read after projection in the HTML/JSON paths, pass the commits by value (or store
   `Arc<Value>` in `HandlerOutput` so projection clones an Arc). Expected: scales with
   view count and payload size; the bench's small payloads understate the real-world
   win.
5. **Render elements without per-call BTreeMap/attribute clones.**
   `render_document_element` call sites allocate fresh `BTreeMap`s of owned Strings and
   clone attribute vectors per element per request (`view_response.rs` `append_*`
   helpers). For the fragments not covered by step 1 (dynamic title/meta), render from
   borrowed data. Expected: small; measure after 1 absorbs most element work.
6. **Fuse the SSR payload escape into serialization.** `ssr_payload_json` serializes to
   a String, then the escape pass copies the whole thing again char-by-char
   (`response_finalizer.rs:507`). Serialize through a writer that escapes inline (same
   escaping table, one pass, identical bytes). Expected: proportional to payload size;
   small on the bench, real on large pages.

Steps 1–3 are the priority; expect them to cut the static-view row by roughly a third
and the shared floor by ~15–25%. Steps 4–6 are follow-through, each cheap to attempt
and individually measured. Stop and record honestly wherever a step's A/B shows no
win — P002's reverted-refactor discipline applies.

## Explicitly OUT of scope for Part 2 (escalation-class, do not touch)

- The typed→`Value`→JSON double serialization (`typed_handler.rs:591` + finalizers).
  Restructuring the output boundary touches projection semantics and the wire contract;
  if Part 2's measurements suggest it is the next frontier, escalate with numbers.
- The two-phase barrier and spawn-per-invocation for N≥2 — contract, frozen
  (REMINDERS.md: middlewares and views always execute in parallel).
- Anything observable: response bytes, header order, error selection, commit order,
  cancellation behavior on terminal boundaries.

## Measurement protocol

Idle machine, one mechanical change per measured step, `make bench-engine` before/after
per step (direct runs for intermediate A/Bs; the recorded file is re-recorded only via
the make target at the final state), 10-row table with deltas in the report, plus the
full standard gate (workspace tests, clippy, fmt, loom untouched-green) at the end. The
final recording becomes the standing engine baseline in STATE.md.
