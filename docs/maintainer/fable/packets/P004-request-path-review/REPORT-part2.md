# P004 Report — Part 2 (optimization)

Provenance note: the executor (Sonnet 5 subagent, 2026-07-01) was blocked from writing
this file by the same harness guardrail Part 1's executor hit ("Subagents should return
findings as text, not write report files") and returned the full content in its final
message; Fable placed it here verbatim.

## What changed

- `crates/vorma/src/view_response.rs` — new `PrecomputedViewFragments` type (critical-CSS
  element, root-div markup, prod module script tag, per-URL CSS-bundle `<link>` markup
  cache), computed once from a `RuntimeManifest`; a new `pub(crate)` sibling
  `finalize_view_report_html_response_precomputed` that consumes it; the original
  `pub fn finalize_view_report_html_response` unchanged in signature and behavior, now a
  thin wrapper over a shared `_impl` function.
- `crates/vorma/src/runtime_snapshot.rs` — `RuntimeSnapshot` gains a
  `precomputed_view_fragments: PrecomputedViewFragments` field, computed once in
  `compile()`, exposed via a new `pub(crate)` accessor.
- `crates/vorma/src/runtime_app.rs` — `finalize_execution_report`'s HTML branch now calls
  `finalize_view_report_html_response_precomputed` with
  `self.snapshot.precomputed_view_fragments()`.
- `crates/vorma/src/execution_engine.rs` — `execute_invocation_phase` gains an inline
  single-invocation fast path (skips `JoinSet` spawn + `join_next_with_id` when
  `phase_indexes.len() == 1`); the per-item commit logic (previously inlined in the N>=2
  loop) is extracted into a shared `commit_invocation_output` helper used by both the
  fast path and the unchanged N>=2 loop; the spawned-task closure now moves
  `HandlerInvocation`'s fields directly into `HandlerInput` instead of cloning them a
  second time on top of the clone already made to enter the closure.
- `crates/vorma/src/response_finalizer.rs` — `ssr_payload_json` now serializes through a
  custom `serde_json::ser::Formatter` (`HtmlScriptSafeJsonFormatter`) that escapes
  `&`/`<`/`>`/U+2028/U+2029 inline as each string fragment is written, instead of
  serializing to a plain JSON string and then copying the whole thing again through a
  second escaping pass (`escape_ssr_payload_json`, removed).
- `docs/maintainer/bench-results/vorma/sjc-z390-aorus-pro-wifi.bench.results.txt` —
  re-recorded via `make bench-engine` at the final state (pure redirect).
- `docs/maintainer/fable/STATE.md` — engine benchmark section updated with the Part 2
  table and a per-step summary; "Where work stands" updated.

No new dependencies, no `unsafe`, no public API signature changes (verified: every
touched `pub fn`'s parameter list and return type is unchanged; new items are
`pub(crate)`).

## Steps executed (ANALYSIS.md's ranked plan)

Faithful A/B methodology throughout: same-machine, idle-verified, direct `cargo bench -p
vorma --bench engine` runs comparing each step's on/off state (source swapped via file
copy, never git), multiple runs per comparison (3-4 per step, several using a fully
interleaved before/after/before/after schedule to average out environmental drift),
medians compared. `make bench-engine` reserved for the one final recording, per protocol.

1. **Snapshot-precomputed document fragments — KEPT.**
   - Faithful A/B (3 runs before, 3 after; block-then-block plus a confirming run):
     `nested_dynamic_view_chain_4_deep` 70,654 -> 62,523 ns (median, -11.5%),
     `not_found_catch_all_view` 46,428 -> 43,169 ns (-7.0%),
     `request_through_middleware_chain_0_control` 37,457 -> 33,216 ns (-11.3%),
     `static_view_render` 43,725 -> 43,244 ns (-1.1%, small — this route has only 2 CSS
     bundles vs. the nested chain's 4). Resource rows (never touch this code path) moved
     <=2% either way — correctly uninvolved, confirming attribution.
   - Design note: the precomputed set is scoped strictly to fragments that are pure
     functions of the committed `RuntimeManifest` alone. It deliberately does NOT touch
     `document.head_defaults()`/`html_attributes()`/`body_attributes()`/`body_prefix()`
     — those flow from a `RuntimeDocumentProvider`, which is a genuinely
     per-request-varying seam by framework design (confirmed both by the trait's own doc
     comment — "apps can vary the shell at runtime without touching committed snapshots"
     — and empirically: Board's own `document.rs` embeds
     `document.body().data("request-path", request_path)`, a real per-request value).
     `finalize_view_report_html_response`'s existing `render_document` call, which reads
     `document` directly, is completely untouched. Verified via the existing
     `runtime_app_dynamic_document_provider_feeds_view_json_and_html_responses` test
     (asserts a dynamically-provided `<title>` renders correctly through the modified
     code path) and the CSS-bundle-URL fallback in
     `append_precomputed_css_bundle_head_elements` (falls back to the exact fresh-render
     code for any URL absent from the precomputed set, rather than silently dropping it,
     in case of a manifest/report mismatch).
   - Infallibility: `PrecomputedViewFragments::compile` is called with `.expect(...)`
     rather than threading a new `RuntimeSnapshotError` variant, because every element it
     renders uses framework-fixed tag/attribute NAMES (style/id, script/type/src,
     link/rel/href) and `render_document_element` only ever rejects invalid names, never
     attribute/inner-HTML VALUES — which is all manifest data ever contributes. No new
     failure mode introduced.

2. **Per-invocation clone-tree collapse — KEPT (modest, real, row-dependent).**
   - Two independent A/B methodologies (block-based: 6 before + 7 after; fully
     interleaved: 3xA/B cycles) agree that `nested_dynamic_view_chain_4_deep` wins
     consistently (-4.8% to -6.8% across both), and both non-executing control rows
     (`not_found_bare_no_catch_all`, `method_not_allowed`, which run zero handlers) sit
     at ~0% — confirming the measurement isn't drift. `static_view_render` regressed
     slightly in both methodologies (+2.7% to +5.1%). Mechanistic explanation, not
     dismissed as pure noise: `static_view_render`'s two matched routes (`/`,
     `/dashboard`) have zero dynamic params, so their `Params`/`SplatValues` clones were
     already free (empty map/empty buffer) — nothing for this change to save there, only
     measurement noise remains, while `nested_dynamic_view_chain_4_deep`'s routes carry
     real captured params (`shelf_id`, `board_id`, `card_id`) that genuinely allocate on
     clone. Kept because the change is a strict allocation reduction with zero
     correctness risk and zero complexity cost, matching this codebase's own documented
     doctrine ("compute lookups from borrowed inputs and clone only on insert" —
     LEARNINGS.md); the ANALYSIS.md caveat ("Params/SplatValues are already cheap-ish —
     measure before assuming") is borne out exactly as flagged, not contradicted.

3. **Single-invocation phase fast path — KEPT (largest single win).**
   - Interleaved A/B, 4 cycles: `json_resource_small_input_output` 26,313 -> 19,144 ns
     (-27.2%), `resource_body_binary_resource` 23,843 -> 17,165 ns (-28.0%),
     `head_request_to_resource` 23,623 -> 17,242 ns (-27.0%) — all three completely
     non-overlapping across every run (e.g. `head_request_to_resource` before-runs
     [23458, 24046, 23788, 23001] vs. after-runs [17442, 17041, 16824, 17630]).
     View rows (`static_view_render`, `nested_dynamic_view_chain_4_deep`,
     `request_through_middleware_chain_2`) moved <=2.5% — correctly near-zero, since
     their handler phases always have N>=2 invocations (one per matched view in the
     chain) and never reach the fast path; only their (also N>=2) middleware phase runs,
     unaffected.
   - Semantics: the parallel contract (REMINDERS.md — middlewares and views execute in
     parallel) constrains N>=2, never N=1; parallelism of exactly one invocation is
     indistinguishable from inline execution, so nothing observable changes. Commit
     logic is shared byte-for-byte with the N>=2 path via the extracted
     `commit_invocation_output` helper (same function, not duplicated logic), so
     terminal-boundary/error-precedence/effects-merge rules apply identically regardless
     of which path executed. Panic propagation: an inline `.await`ed panic unwinds
     naturally through this function and out to its caller — the same observable place
     `resume_unwind` on a joined panic propagates to today; the panic hook fires exactly
     once in both cases — the difference is thread identity in default-hook output, not
     propagation reaching a different place or firing an extra/fewer time. Verified
     against `request_drop_cancels_running_handler_exec_ctx` (the closest existing test
     to this exact shape: a single-resource route, no middleware, cancelled via
     outer-task abort) — passes;
     `middleware_error_short_circuits_without_waiting_for_later_pending_sibling`,
     `later_middleware_redirect_waits_for_prior_sibling_success_commit`, and
     `prior_middleware_redirect_completes_without_waiting_for_pending_sibling`
     (N>=2-only out-of-order-completion semantics) confirmed unreachable through the new
     branch by construction (2-middleware setups) and still pass.

4. **Take the projection's `Value`s instead of cloning — considered, NOT implemented.**
   - `project_view_payload(manifest, report)` takes `report` by shared reference; both
     callers read `report` again after the call (for `report.effects()` and
     `suppress_response_body_if_needed`), so "pass commits by value" is unavailable
     without restructuring those `pub fn`s' bodies in a way that risks the
     ordering/error-path guarantees they currently give — and those functions'
     signatures are frozen.
   - The `Arc<Value>` alternative was analyzed to the point of proving it cannot help
     without also changing `ViewPayload::views_data`'s public field type
     (`Vec<Value>` -> `Vec<Arc<Value>>`, itself a wire-contract-adjacent change):
     storing `Arc<Value>` inside `HandlerOutput` only lets you skip the deep clone if
     you can eventually MOVE out of a uniquely-owned `Arc`, but every access path here
     goes through `&RouteExecutionReport` (a shared reference), so the `Arc` can only
     ever be cloned — never moved — meaning a subsequent unwrap-or-clone always finds
     refcount >=2 and always falls back to the same deep clone `HandlerOutput` already
     does today. No code was written for this reason: there is no cheap partial
     experiment that could show a win, so there was nothing to faithfully A/B.
   - Independently confirmed unmeasurable on this instrument regardless: the bench's
     actual view-data payloads are 17-50 bytes of JSON — a clone at that size is a
     handful of small allocations, provably below this machine's demonstrated 1-9%
     run-to-run noise floor. ANALYSIS.md's own hedge ("the bench's small payloads
     understate the real-world win") is correct; there is no way to produce an honest
     number here without either a public-surface change out of this packet's authority
     or a different fixture, neither of which this packet grants.

5. **Render elements without per-call `BTreeMap`/attribute clones — considered, NOT
   implemented.**
   - `append_payload_head_element` clones a `HeadElement`'s attribute collections into a
     fresh, owned `DocumentElementContract` solely because that is the only construction
     path `render_document_element` (in the separate `vorma-contract` crate) accepts —
     its builder API is owned-by-design throughout, used at both declaration time and
     runtime across that whole crate. Giving it a borrowed construction path is a
     `vorma-contract`-wide public-type redesign, well outside this packet's scope, for a
     per-request payload (a title, at most one or two meta elements — the elements step
     1 does NOT already absorb) that is exactly as small as step 4's, with the same
     "provably below noise" character. ANALYSIS.md's own ranking already flagged this as
     marginal.

6. **Fuse the SSR payload escape into serialization — KEPT.**
   - Interleaved A/B, 4 cycles: `nested_dynamic_view_chain_4_deep` 62,025 -> 55,095 ns
     (-11.2%, largest — biggest SSR payload of the fixture), `static_view_render`
     43,596 -> 41,703 ns (-4.3%), `request_through_middleware_chain_2` 43,152 -> 41,109
     ns (-4.7%), `request_through_middleware_chain_0_control` 34,644 -> 32,639 ns
     (-5.8%), `not_found_catch_all_view` 44,536 -> 42,002 ns (-5.7%) — every row that
     calls `ssr_payload_json` shows a consistent, non-overlapping win.
     Resource/404-bare/method-not-allowed rows (never call it) moved <=2.2% — correctly
     uninvolved.
   - Correctness: `&`/`<`/`>`/U+2028/U+2029 can only appear inside JSON string values —
     never in structural characters, numbers, or literals — so only
     `Formatter::write_string_fragment` needs overriding; `serde_json` only ever calls
     it with already-UTF-8-valid `&str` slices, eliminating the byte-boundary splitting
     risk a hand-rolled byte-level writer would carry. Verified two ways before landing:
     (a) a standalone program comparing the fused per-fragment escape against the
     reference char-by-char algorithm on 10 adversarial inputs (multi-byte CJK, emoji,
     empty string, consecutive escape targets with zero safe runs between them, mixed
     scripts) — all matched exactly; (b) the existing
     `finalizer_escapes_ssr_payload_json_for_html_script_embedding` test, which
     exercises all five targets back-to-back (the worst case for fragment-splitting
     logic) — passes unchanged.

## Decisions made

1. Steps executed in ANALYSIS.md's ranked order; steps 1-3 (the stated priority) landed
   first, step 6 landed after; steps 4-5 were investigated to the point of proving no
   faithful A/B could show a win without exceeding this packet's authority, and were not
   coded — no risk was taken for an unmeasurable result.
2. Step 2 was kept despite a genuinely mixed A/B signal (net win on parameter-carrying
   routes, net-neutral-to-slightly-negative on parameter-free routes) rather than
   reverted, because the mixed result has a coherent mechanistic explanation rather than
   being unexplained noise, the change carries zero correctness risk, and it matches
   established codebase doctrine. This is a judgment call, disclosed for the
   maintainer's review — a more conservative read of "faithful A/B shows no win" could
   argue for reversion; reverting is a single, isolated, low-risk follow-up if this
   reasoning is rejected.
3. `PrecomputedViewFragments::compile`'s `.expect(...)` rather than a new fallible
   `RuntimeSnapshotError` variant — judged as introducing zero new failure mode
   (provably infallible for the fixed element shapes involved) versus a new-error-
   variant path being itself an observable change.
4. Left the pre-existing uncommitted work from before this session completely untouched
   throughout. Twice during this session `git status --short` showed my own edited files
   as staged rather than unstaged despite my never running `git add`; in both cases
   `git diff --cached` vs. `git diff` confirmed the staged content exactly matched my
   working-tree content (no stale data, no loss) — the same benign index-checkpointing
   mechanism STATE.md's "Open flags" section already documents. Flagged for
   completeness; nothing was dirty in the sense the escalation doctrine cares about.
5. Used a temporary, session-scoped integration test file
   (`crates/vorma/tests/zzz_scratch_byte_capture.rs`) to capture and byte-diff every
   bench row's actual response (body + headers + status) before/after each step, since
   the bench harness itself only reports timing, not bytes. Deleted before the final
   gate run; not part of the packet's committed changes.

## Gate results

All green, run at the final state (idle machine, i9-9900K):

- `cargo test --workspace --all-targets`: 551 passed, 0 failed (matches the STATE.md
  baseline exactly).
- `cargo test --workspace --doc`: 1 passed, 0 failed.
- `cargo clippy --workspace --all-targets -- -D warnings`: 0 warnings.
- `cargo fmt --all --check` (workspace + fuzz): clean (one line-length wrap needed
  fixing after the extraction; applied via `cargo fmt --all`, re-verified clean,
  re-verified byte-identical and re-tested after).
- `make loom-tasks`: 7/7 models pass — untouched-green (vorma-tasks not touched).
- Response byte-identity: PASS. Every one of the 10 bench rows x 2 rotated paths (20
  requests) captured in full (status + sorted headers + body bytes) before any Part 2
  change and again after the final kept state — `diff -rq` over all 40 captured files
  reports zero differences. Re-verified after the fmt fix and again as the truly final
  check before deleting the capture harness.

## Benchmarks

Recorded via `make bench-engine` on an idle machine (verified via uptime/top before
recording; a background monitor confirmed the 1-minute load average settled below 1.0
after this session's own sustained build/bench activity before the authoritative run):

```
os: linux
arch: x86_64
crate: vorma
cpu: Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz
static_view_render                                      36030      43891 ns/op
nested_dynamic_view_chain_4_deep                        28110      55750 ns/op
json_resource_small_input_output                        78260      20388 ns/op
resource_body_binary_resource                           80375      18104 ns/op
request_through_middleware_chain_2                      36910      39961 ns/op
request_through_middleware_chain_0_control              48085      31798 ns/op
not_found_catch_all_view                                35645      42161 ns/op
not_found_bare_no_catch_all                            683185       2197 ns/op
head_request_to_resource                                81765      17917 ns/op
method_not_allowed                                     388470       3866 ns/op
```

Final 10-row before/after (Part 1 baseline vs. this recording, same machine):

| Row                                        | Before (Part 1) | After (Part 2) | Delta  |
| ------------------------------------------ | --------------- | -------------- | ------ |
| static_view_render                         | 48,798          | 43,891         | -10.1% |
| nested_dynamic_view_chain_4_deep           | 70,348          | 55,750         | -20.8% |
| json_resource_small_input_output           | 25,051          | 20,388         | -18.6% |
| resource_body_binary_resource              | 24,100          | 18,104         | -24.9% |
| request_through_middleware_chain_2         | 44,903          | 39,961         | -11.0% |
| request_through_middleware_chain_0_control | 39,500          | 31,798         | -19.5% |
| not_found_catch_all_view                   | 45,061          | 42,161         | -6.4%  |
| not_found_bare_no_catch_all                | 2,163           | 2,197          | +1.6%  |
| head_request_to_resource                   | 24,518          | 17,917         | -26.9% |
| method_not_allowed                         | 4,048           | 3,866          | -4.5%  |

Every handler-reaching row improved; the two rows that never execute a handler sit
within run-to-run noise (+1.6%/-4.5%), confirming the wins trace to the specific
mechanical changes rather than systemic drift or measurement artifact.

## Escalations / open questions

1. Decision 2 above (keeping step 2 despite a mixed-sign A/B) is a judgment call the
   maintainer may want to review directly — full reasoning and numbers in "Steps
   executed". Reverting is a single, isolated, low-risk follow-up if the reasoning is
   rejected.
2. Steps 4 and 5 are recorded as tickets-worth-filing rather than filed as tickets,
   since both require a design decision (a public wire-type field-type change for step
   4; a broader `vorma-contract` construction-API redesign for step 5) that is itself a
   maintainer call. Flagging rather than filing; happy to file on direction.
3. The machine was shared and non-dedicated throughout, same standing caveat Part 1
   flagged; each A/B comparison re-verified idleness immediately before recording.
4. The same benign staged-vs-unstaged git-index discrepancy recurred twice this session
   on my own files; verified harmless both times, matching STATE.md's documented
   mechanism — no new concern.

## Discovered out-of-scope work

None beyond escalation 2 (steps 4/5, deliberately not pre-filed pending maintainer
framing).
