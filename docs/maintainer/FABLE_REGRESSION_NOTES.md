# Fable Regression Notes

These notes preserve the regression-review context from the Fable histories. They are not
a changelog. They record which subtle behaviors were lost, which fixes later landed, and
which facts future work must preserve or re-check.

## Reading Map

The `36126369-0da4-45a2-83f9-65bd07a35805` session contains the refactor regression
campaign. Its subagents produced finder reports, verifier reports, and later rechecks.
Some subagents were interrupted before a final report; those are explicitly marked below.

## High-Level Refactor Assessment

The architecture assessment judged the rewrite direction sound:

- One canonical framework graph should own normalized declarations.
- Immutable execution plans/snapshots should drive request handling.
- Dev and production generation should use explicit candidate/commit transitions.
- Browser/client internals and Rust server/build internals should meet through fixed
  wire/output contracts.
- Facts should have one owner and should be projected, not rediscovered.

The same assessment also warned that the rewrite made several modules very large:
`framework_graph`, `execution_engine`, `runtime_app`, `dev_build`, `generation_epoch`,
`projection_compiler`, and `static_outputs`. Large files were not automatically judged
wrong, but future changes in these files need especially careful boundary review.

## Build And Dev-Loop Regressions Found

The first regression pass found these concrete lost behaviors:

- Process termination originally killed only direct wrapper children. Because app server
  and Vite were started through wrappers, the real processes could survive rebuilds and
  shutdown. Old code had process-group/tree termination with graceful shutdown before
  hard kill.
- The dev mux originally buffered complete backend responses and stripped/failed upgrade
  semantics. That broke SSE, long-polling, chunked streaming, and WebSocket app routes in
  dev while production did not have the mux in the same path.
- The dev loop originally lost debounce/coalescing. Multiple filesystem events from one
  logical save could queue multiple full rebuilds.
- In-flight rebuilds originally could not be cancelled. New file changes and shutdown
  were observed only between long blocking subprocess calls.
- Failed activation originally rolled back in-memory generation state but not already
  published files. That could leave `src/vorma.gen.ts`, manifest files, and public output
  describing candidate generation N+1 while the running server still served generation N.
- Dev generation originally double-compiled projections and double-wrote generated
  TypeScript for one rebuild.
- Generated TypeScript writes originally temp-renamed even when bytes were unchanged,
  causing watched-file churn and avoidable Vite work.
- Public static assets and critical CSS originally got reprocessed on rebuild paths that
  did not logically touch them.
- A relative path from a package-manager directory to a source file outside that directory
  could become a dev module URL containing `../`. Browser URL normalization and Vite's
  out-of-root `/@fs/` behavior make this a dangerous dev-boot/HMR mismatch.
- Dev watcher path classification against a non-canonical root was judged plausible to
  fail on macOS symlinked roots where FSEvents reports canonical paths. The verifier noted
  this was likely not a new regression because the old watcher had a comparable lexical
  mismatch risk.

## Build And Dev-Loop Fixes Later Rechecked

A later recheck subagent reported the following as fixed in the then-current code:

- Process groups/tree termination were fixed with process-group creation for spawned
  processes, SIGTERM grace then SIGKILL on Unix, and `taskkill /T /F` on Windows.
- Async cancellation of collecting child processes used the same termination path.
- Dev mux proxying was fixed to stream HTTP/1.1 bodies and handle bidirectional upgrade
  connections.
- In-flight build cancellation was fixed with `BuildProcessCancel`, cancellation-aware
  process runners, and dev-loop event collection that cancels stale app-server
  generation work when newer app-generation changes arrive.
- Debounce/coalescing was fixed with a short watcher-settle interval and queue draining
  before rebuild.
- Disk rollback on failed activation was fixed by capturing generated TypeScript,
  manifest, and public-output files before publishing and restoring them on activation
  failure.
- Static/CSS fast paths were added so client revalidation, static generation, and full app
  server generation are separate dev-loop work kinds.
- Projection compilation and generated-TypeScript writing were consolidated so each
  generation uses precomputed projections and byte-compare short-circuiting.
- Pure Rust/app-server changes no longer require public asset rehashing or CSS rebundling
  unless the change intent requires it.

Future dev-loop work must preserve these properties. Reintroducing direct-child-only
termination, whole-response mux buffering, no cancellation, no rollback, or unconditional
watched-file rewrites would be a regression.

## Runtime Request-Path Regressions Found

The runtime request-path review found these issues:

- Middleware no longer gated route handlers in the original rewrite. Middleware and the
  handler started concurrently, so a terminal auth middleware could suppress output but
  could not prevent handler side effects. A verifier confirmed this was a regression
  against old behavior: old middleware ran before handlers and returned early on terminal
  responses.
- Typed handler response/head handles originally held `MutexGuard`s over the same
  response-effects mutex. Holding both handles could deadlock a request.
- Top-level nullable query inputs originally decoded to `null` because root nullable
  handling checked for an empty query key.
- Nested record query decoding originally dropped the parent key path, looking up `child`
  rather than `parent.child`.
- Map query inputs originally fell through to scalar decoding, while the TS parser and
  serializer treated maps as dotted-prefix entries.
- Empty-array encoding diverged: TS serialized an empty array as `key=` and parsed it back
  as `[]`; Rust decoded the present empty value as `[""]` for string arrays or an invalid
  number for numeric arrays.
- POST or other non-GET/HEAD requests to view-only paths returned 404, not 405. The
  verifier confirmed this was a pre-existing HTTP semantics wart rather than a refactor
  regression.
- `views_data` was built with `filter_map`, so a view output with no data would shift
  later view data positionally. The verifier judged this plausible but normally
  unreachable through supported public registration because even unit outputs serialize
  as JSON `null`.
- Self-closing non-void head/document tags could render invalid HTML such as
  `<script ... />`. The verifier judged this a pre-existing design hazard requiring
  explicit opt-in, not a rewrite regression.

## Runtime Fixes Later Partially Rechecked

An interrupted fresh-eyes/recheck subagent still gathered useful current-code evidence:

- There was a test named `resource_execution_gates_handler_until_middleware_finishes`,
  and its body asserted the handler had not run while middleware was blocked. This
  indicates the middleware-gating regression had been addressed in the checked state.
- `TypedHandlerContext::response()` and `head()` returned handles that owned
  `Arc<Mutex<ResponseEffects>>` rather than holding the lock guard immediately, and a test
  named `typed_runtime_handler_allows_overlapping_response_and_head_handles` exercised
  overlapping handles. This indicates the self-deadlock bug had been addressed.
- `input_decoder.rs` contained tests for present top-level nullable records, missing
  nullable records, maps from dotted entries, empty map markers, and empty array markers.
  The recheck was interrupted before a final verdict, so future query-decoder work should
  still verify the implementation, but the transcript shows explicit regression tests were
  added.

## TypeScript And Vite Findings

The TypeScript/Vite review produced these durable points:

- The original claim that `handleHotUpdate` suppressed all importer chains whenever any
  chain reached a view was refuted. The verifier found suppression gated to changed view
  modules themselves.
- The original claim that CSS changes imported by views had native CSS HMR suppressed was
  refuted. CSS modules were not themselves view module ids, so the hook fell through.
- `publicUrl()` handling regressed for static template literals. Old regex handling
  accepted backtick-delimited static strings; the AST rewrite only accepted ESTree
  `Literal` string args and rejected `TemplateLiteral`.
- The view dependency HMR event handler lacked try/catch around dynamic imports and used
  `return` rather than `continue` on a non-string URL. This could abort remaining updates
  and produce an unhandled rejection, especially for a directly edited view whose default
  HMR had been suppressed.
- `debounce.cancel()` intentionally changed semantics from forever-pending promises to
  rejecting pending promises. This fixed a hang/leak shape but creates an external
  unhandled-rejection hazard for consumers that call `.then(...)` without a rejection
  handler. In-repo consumers were judged safe at the time.
- Removal of `resolve_outlet_slot` from the internal export surface was refuted as a
  public breakage because it was under an `./__internal` export and had been replaced by
  adapter-base state delivered to adapters.

## Cleanup And Performance Findings

The cleanup/performance subagents found:

- Slash normalization had been duplicated, but later evidence showed the local wrappers
  called `vorma_matcher::ensure_leading_and_trailing_slash` while preserving per-field
  empty-value errors. That is the right shape: centralize normalization, keep local error
  context.
- Dev and production Vite command construction duplicated command scaffolding and error
  variants in the early rewrite. Reconcile current code before changing; a later recheck
  of this item was interrupted before a final report.
- Some dev-build candidate publishing functions were near-verbatim duplicates in the
  early rewrite. Later build-loop fixes may have changed this; reconcile current code
  before acting.
- Dead public orchestration functions were confirmed in the early rewrite. Verify current
  call sites before deleting anything.
- Test fixture duplication increased when `test_support.rs` was deleted in the early
  rewrite. Verify the current test-support shape before adding or removing helpers.
- Hot-path request cloning and execution-plan deep clones were flagged. A later partial
  recheck saw `ExecutionPlan` storing `Arc<ResourcePlanNode>`/`Arc<ViewPlanNode>` and
  resource routes nested by method, suggesting some plan-clone issues were fixed. The
  recheck was interrupted before a final performance verdict.
- SSR payload escaping via chained `String::replace` calls was flagged as a per-request
  cost. The later recheck was interrupted before a final verdict.

## Test Coverage Warning

The removed-behavior audit reported a large deletion of black-box Rust runtime tests.
Inline unit tests and smaller public API tests replaced some coverage, and later work
introduced an in-memory `vorma::testing::TestApp` harness. The durable warning is that
runtime behavior claims need observable HTTP/request tests, not only narrow unit tests.
Middleware ordering, terminal effects, nested view behavior, public assets, resources,
body/context behavior, redirects, and error propagation all need black-box coverage.

## Interrupted Subagents

- `agent-a287fa6b9c7e98fac` started rechecking performance and cleanup fixes but was
  interrupted before any verdicts.
- `agent-a4bb7e54fcfa92eea` started rechecking runtime fixes and gathered evidence for
  middleware gating, overlapping response/head handles, and input-decoder tests, but was
  interrupted before final verdicts.
- `agent-a2f26614b39113d61` started a fresh-eyes Rust review after fixes and gathered
  source evidence across dev loop, process runner, mux, input decoder, and runtime
  manifests, but was interrupted before final findings.
