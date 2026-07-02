# P005 Report

Provenance note: the executor (Sonnet 5 subagent, 2026-07-01) was blocked from writing
this file by the recurring harness report-file guardrail and returned the full content in
its final message; Fable placed it verbatim (HTML transport escaping undone).

## What changed

- `examples/board/src/client/api.ts` — renamed the internal, unexported
  `api_query_options` to exported `apiQueryOptions` (camelCase, matching every other
  public-facing symbol in this file: `useApiQuery`, `useApiMutation`), and added a
  teaching comment explaining why it is exported separately from `useApiQuery` and how it
  composes with `query_client.prefetchQuery`/`ensureQueryData` — the react-query
  counterpart to Vorma's own `prefetch()`. This closes the F9 straggler's export half.
- `examples/board/src/client/views/search.view.tsx` — composed `apiQueryOptions` with both
  `prefetchQuery` (fire-and-forget cache warming on the "Go" button's hover/focus,
  alongside the existing Vorma route `prefetch()` on the same intent signal) and
  `ensureQueryData` (awaited before the imperative `navigate` call, so the destination
  view's first render already has warm react-query data). Extracted the duplicated
  hover/focus body into one `prefetch_search_intent` closure (was already duplicated
  pre-packet for the single `prefetch()` call; now duplicating two calls each, so
  extraction is warranted, not gratuitous). Comments teach the division-of-responsibility
  point directly (two independent caches, warmed by two independent APIs, on the same user
  action) — no audit/process language. This closes the F9 straggler's composition half.
- `docs/maintainer/tickets/board-api-coverage/CENSUS_COMPLETION_P005.md` — new. The
  row-by-row completion audit table (verdict + citation per F1–F17 row and the
  member-level second sweep), plus four new findings (F-19 through F-22).
- `docs/maintainer/tickets/board-api-coverage/PRESSURE_TEST_CENSUS.md` — F17's exempt
  parenthetical flipped from "LANDED except..." to "FULLY LANDED as of P005" and its
  `apiQueryOptions` line updated from aspirational to landed-and-cited; F1's "cursor/page
  pagination" corrected to "page pagination" with an inline note (no cursor concept ever
  existed in board); the `Link` member-sweep's `replace` -> "post-login" attribution
  corrected to the two flows that actually demonstrate it (profile tabs, search box
  sync/navigate); the tasks-member-sweep line and "Prior Adjudication List" item 2
  corrected — both previously claimed a `TaskOverrides::replace` failure-injection test
  lives in board's F16 suite; it does not and never did (it lives in
  `crates/vorma/tests/in_memory_test_app.rs`, per F17's own later ruling that
  `TaskOverrides` was moved out of Board). Appended F-19 (census-accuracy correction,
  F1+replace), F-20 (census-accuracy correction, TaskOverrides/F16), F-21 (PAPERCUT,
  `DocumentAttributes::known_safe_attribute`/ `boolean_attribute` have no honest Board
  call site, escalated rather than forced), F-22 (PAPERCUT, `FormData`'s
  multi-value/grouped accessors have no honest single-attachment-shaped call site,
  escalated rather than forced).

F3 (the attachment import-parse via `workIndicator.track`) was already correctly landed on
`file.text()` in `examples/board/src/client/views/submit.view.tsx` before this packet
started — the audit confirmed it as the census's designated home and made no changes
there.

## Decisions made

- **F9 prefetch composition placement:** chose to compose `apiQueryOptions` on the search
  view's own "Go" button (both `prefetchQuery` on hover/focus and `ensureQueryData` on
  click-before-navigate) rather than inventing a separate call site elsewhere, because the
  search view already demonstrates Vorma's own `prefetch()`/`cancelPrefetch()` pair on the
  exact same intent signals — putting the react-query counterpart right next to it is the
  most honest, least-contrived way to teach the division of responsibility the census's
  own F17 prose is built around (Vorma owns route/view data, react-query owns ad-hoc API
  data), and it required no new UI surface.
- **F-21 (DocumentAttributes low-level attribute methods): escalated, not forced.** Every
  candidate real-app use for a boolean or trusted-unescaped attribute on the root
  `<html>`/`<body>` element that I could construct was contrived (a functionless flag with
  no product purpose). The board contract explicitly requires that added coverage "say
  plainly in user-facing terms...when an application would use that API" — a fabricated
  flag cannot honestly clear that bar. Recommended folding this into
  `crates/vorma/tests/public_api.rs`'s existing
  `root_document_helpers_are_usable_externally` test instead (matching how `HeadBuilder`'s
  equally low-level defs are already handled one test above it in the same file), but did
  not make that change myself: it touches a framework-owned test file outside
  `examples/board`, and choosing between "invent a board feature," "add to public_api.rs,"
  or "leave as a recorded gap" is exactly the kind of design judgment call the packet says
  to escalate rather than resolve silently.
- **F-22 (FormData grouped accessors): escalated, not forced,** for the same reason —
  board's actual submit form is genuinely single-field/single-attachment shaped, and
  reaching the multi-value accessors honestly needs a real multi-attachment product
  feature, not a mechanical addition to existing code.
- **Census correction method:** corrected drifted census prose in place (with an inline
  note explaining what changed and why) rather than only recording the correction in the
  new findings ledger entries, because the packet's own instruction says "record audit
  corrections directly in the census where a row's description has drifted from reality" —
  leaving the original wrong text standing and only noting the correction in a findings
  bullet would still mislead a reader who only skims the feature/member-sweep sections.
- **Did not touch F13's `HeadHandle`, `Middleware::with_patterns`/`with_methods`, or any
  other row I initially suspected might be a gap** after closer inspection showed they
  were correctly covered (`HeadHandle` is the public re-export name for the return type of
  every `ctx.head()` call board already makes; false alarm from a literal-string grep that
  couldn't see through the re-export). Recorded as "implemented as described" in the audit
  rather than silently dropped.
- **Left the two independently-appearing oxfmt reflow artifacts I found and fixed**
  (`DocumentAttributes:: known_safe_attribute` with a stray space, and
  `{cancel_token,is_cancelled, child}` with an inconsistent comma) as corrections rather
  than accepting oxfmt's scoped-write output verbatim — both were oxfmt breaking inside a
  backtick code span while prose-reflowing my new paragraphs, which produces
  syntactically-valid-but-wrong inline code. Did not chase every remaining comma-spacing
  micro-inconsistency between my new prose and the census's longstanding space-after-comma
  convention in older brace-group examples — that is a pre-existing stylistic choice in
  the census, not a defect, and re-litigating it repo-wide is out of this packet's scope.

## Gate results

All commands run from the repo root on 2026-07-01, after all code and doc changes landed.

Rust:

- `cargo test --workspace --all-targets` — 0 failures across every crate;
  `vorma-board-example` `tests/app.rs` 20/20, additionally confirmed by isolated re-run.
- `cargo test --workspace --doc` — 1 passed, 0 failed.
- `cargo clippy --workspace --all-targets -- -D warnings` — clean, exit 0.
- `cargo fmt --all -- --check` + fuzz manifest — both clean.

TypeScript:

- `pnpm exec tsgo -p examples/board --pretty false` — clean, exit 0 (confirmed twice).
- `make ts-typecheck` — exit 0, all 8 projects plus board. (The `tsdown` UNRESOLVED_IMPORT
  notices during the `.dist` build step are pre-existing bundler notices about workspace
  self-imports, present before this packet.)
- `pnpm exec oxlint --config=oxlint.config.ts .` — exit 0, zero diagnostics (confirmed via
  `--format=json`: `"diagnostics": []`). Note: STATE.md's P001 recording mentioned 10
  pre-existing type-aware warnings (ticketed); current HEAD shows zero, so that ticket
  appears already resolved independent of this packet.
- `pnpm exec vitest run --reporter=dot` — 851/851 across 42 files, confirmed twice.
  (Stderr lines during the run are deliberate error-path test fixtures, not failures.)
- Scoped `oxfmt --check` over `examples/board` and the census directory — clean. (Scoped
  checks/writes only, per F-18 discipline.)

`vorma.gen.ts` regeneration: not needed — zero Rust files changed (empty diff), so the
shared wire contract is provably unaffected; `vorma.gen.ts` itself shows no diff.

## Benchmarks

Not applicable — no performance-relevant code paths touched; no recordings changed.

## Audit summary (full detail in CENSUS_COMPLETION_P005.md)

- Implemented as described: F3*, F4, F5, F6, F7, F8, F9*, F10, F11, F12, F14, F17* (12
  of 17) — * marks the two stragglers this packet completed (F3 was already correctly
  landed pre-packet; F9 needed the export + prefetch composition) and F17, whose "no dead
  surface" clause depended on both.
- Implemented differently (citation/description drift only, corrected in the census in
  this pass): F1 (page-only, not cursor/page), F15 (`replace` demonstrated on tabs/search,
  not post-login), F16 (`TaskOverrides` test lives in a framework test file, not board's
  suite).
- Missing: none.
- Two small, real, narrow gaps within otherwise-covered rows, escalated rather than
  forced: F-21, F-22.

## Escalations / open questions

1. F-21: recommend folding into `public_api.rs`'s existing
   `root_document_helpers_are_usable_externally` test. (Ruled at review: accepted; lands
   as a P008 rider.)
2. F-22: recommend revisiting only if/when multi-attachment submit becomes a real feature.
   (Ruled at review: accepted as recorded; census row suffices.)
3. Pre-existing/concurrent dirty working-tree state not authored by this executor (fable
   README/ROADMAP/NEXT edits, ticket rulings, new P006/P007/P008 packet dirs) — observed,
   preserved, reported, untouched per protocol. All of it reads as coherent, legitimate
   concurrent Fable orchestration; nothing resembling damage.

## Discovered out-of-scope work

No new tickets filed. F-21/F-22 recorded as census findings per the packet's instruction
for gaps needing a design call.
