# Router Request Path Review

Status: open

Audit the full Vorma request path after the matcher/API-mount/scoped-middleware work. This
is not just a matcher-crate review. The object is the whole path from declared app routes
through graph validation, execution-plan construction, runtime classification, middleware
selection, view/resource execution, response effects, and generated-client assumptions
where request semantics leak into TypeScript.

Why this exists:

- The API mount was removed. Resources and views now share one observable URL space.
- Resource/view conflicts are resolved by matcher specificity, not by URL partition.
- Scoped middleware was added as declarative `patterns`/`methods`, with no hidden URL
  rewriting and no scope-capture params.
- GET/HEAD fallback, 404/405 classification, public-static precedence, raw resource
  bodies, redirects, and terminal response effects all sit on the same request path.
- Several large request-path changes landed close together; the review needs to check
  their combined behavior, not only each isolated patch.

Use this ticket, current code, and the task-local Board API artifacts under
`../board-api-coverage/`.

Primary code paths to review:

- `crates/vorma-matcher/src/*`: `FlatMatcher`, `NestedMatcher`, `find_overlap`,
  `compare_specificity`, dirty-path behavior, catch-all behavior, splats, and params.
- `crates/vorma-contract/src/graph_validation.rs`: resource/view conflict validation,
  public-static conflict validation, and method-sensitive validation.
- `crates/vorma-contract/src/execution_plan.rs`: resource/view/middleware lowering into
  runtime matchers and method tables.
- `crates/vorma/src/execution_engine.rs`: request classification, resource/view choice,
  404/405 behavior, phase execution, and terminal suppression.
- `crates/vorma/src/public_app.rs`: public declarations and docs that users read as API
  truth.
- `crates/vorma/src/response_finalizer.rs`, `resource_body.rs`, and related response code:
  redirects, terminal errors, HEAD body suppression, and raw/non-JSON bodies.
- `examples/board/src/*`: request-path pressure-test coverage and stale comment cleanup.

Facts that must stay true:

- `vorma-matcher` is sovereign. Vorma route policy layers on top; do not add Vorma-only
  concepts to the matcher crate.
- GET/HEAD resources and views share one URL space. There is no framework API mount.
- `ResourceKind` is generated-client/revalidation classification, not dispatch policy.
- Specificity is the doctrine. Overlap is normal; only unresolved specificity ties are
  invalid.
- `compare_specificity` is the one public ordering. Runtime dispatch and graph validation
  must not grow separate scoring systems.
- `find_overlap` must stay tied to real matcher behavior and its property/oracle tests.
- Public static assets are not route patterns; public-static base conflicts are separate
  from route specificity.
- Splat patterns match one or more segments. `/docs` and `/docs/*` are distinct.
- Root catch-all views are the branded not-found idiom, but they are app-level fallback
  views. Once the catch-all matches, the route is found and renders HTTP 200; the UI may
  say "404", but views have no HTTP status surface.
- Scoped middleware patterns are literal URL patterns. A `/mod` scope must not match an
  `/api/mod/...` resource.
- Scope-pattern captures do not become handler params or splats; route params/splats come
  from the matched view/resource route.
- Middlewares execute phase-parallel but commit in declaration order. Terminal middleware
  suppresses later middleware and all handlers.
- HEAD suppresses the response body while preserving the would-have-been metadata.
- Raw/non-JSON resource success bodies are represented by `ResourceBody`, are typed as
  `Blob` for TypeScript, set the internal resource-body marker header for client decode,
  and are covered by Board attachment tests.

What to do:

- Reconcile current code against the code paths and invariants above.
- Read comments/docs in the primary code paths and remove stale API-mount-era language.
- Confirm tests pin the key semantics at the right layer: matcher crate for pattern
  behavior, contract crate for graph validation, framework request tests for
  classification/execution, and Board for end-to-end app coverage.
- Add tests where a claim is currently only implied by implementation.
- If the review discovers an unrelated but real API or frontend gap, create or update the
  relevant ticket instead of burying it in this one.

Done means:

- The current request path is coherently documented in this ticket and/or source docs.
- Stale API-mount comments are gone.
- Matcher, graph-validation, runtime classification, middleware scoping, HEAD/405/404,
  redirect/error, and raw-body semantics are either already pinned or newly pinned by
  focused tests.
- Board still exercises the public request-path story end-to-end. If coverage requires a
  new Board feature, add or invent one that uses the API in a realistic app flow.
- Any remaining gaps are split into specific ticket dirs with enough context for a cold
  agent.
