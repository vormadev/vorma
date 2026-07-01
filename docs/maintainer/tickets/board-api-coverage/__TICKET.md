# Board API Coverage

Status: open

This is the ongoing Board/example/public-API coverage workstream. It is broader than
"regenerate API inventories": the inventories are one artifact inside the same stream.

Board exists to pressure-test Vorma and must cover 100% of public Vorma APIs. Period.
Board can grow whatever features are needed for coverage. If an API has no existing Board
feature, add or invent a Board feature that uses it in a realistic app flow. When Board
exposes framework friction, fix the framework and keep Board coverage complete. Do not
treat Board as merely an example app.

Supporting artifacts in this directory:

- `docs/maintainer/board-example/README.md`: durable Board coverage contract. Active
  API-specific gaps and work-in-progress coverage lists belong in tickets, not in that
  durable guide.
- `API_DESIGN.md`: public API campaign context, rulings, and old execution plan. Treat it
  as historical input; reconcile every claim against current code before relying on it.
- `RESPONSE_PROTOCOL_DESIGN.md`: response/error protocol design context.
- `PRESSURE_TEST_CENSUS.md`: Board API census and findings ledger. It contains resolved
  findings as well as useful coverage expectations; do not treat every row as open work.
- `api_inventory_rust.txt`: Rust API inventory regenerated from the current tree on
  2026-06-23. Treat it as a conservative mechanical input, not adjudication.
- `api_inventory_ts.txt`: TypeScript API inventory regenerated from the current package
  exports/source entrypoints on 2026-06-23. Treat it as a conservative mechanical input,
  not adjudication.
- `frontend-client-coverage.md`: current Board browser/client coverage context. It lives
  here because browser/client coverage is part of the Board API coverage initiative, not a
  separate peer workstream.

Important current facts to preserve:

- Resources declare full URL patterns in the same observable URL space as views. There is
  no framework API mount.
- `ResourceKind` is generated-client/revalidation classification, not dispatch policy.
- Vorma mutations auto-revalidate route data by default. Manual `revalidate()` after a
  normal mutation is usually wrong.
- Scoped middleware patterns are URL patterns with no hidden rewriting. Scope captures do
  not become route params or splats.
- GET/HEAD resources and views can overlap; only specificity ties are illegal.
- Splat patterns match one-or-more segments. `/docs` and `/docs/*` are distinct.
- The branded not-found idiom is a root catch-all view, but it is an app-level fallback,
  not an HTTP 404. Once the catch-all matches, the route is found and renders HTTP 200.
  The UI may say "not found" or "404"; the view must not try to set HTTP status.
- Splat fallback views must not become inferred layout parents for other views. The
  generated client contract should put the catch-all under `/`, not put `/*` in every
  other view's parent list.
- `vorma/kit/*` must remain generic utility surface. Do not add Board/Vorma-backend
  helpers to kit packages. Board uses the public kit entry points as normal app
  utilities, not as framework internals.
- Non-JSON resource outputs are part of the current API story. Current code has
  `ResourceBody` / `ResourceOutput`, generated `Blob` output typing, and Board attachment
  request tests that assert stored bytes, stored content type, content disposition, and
  HEAD body suppression. Do not reopen the old raw-body gap unless current code regresses.
- `FormData` request input and `ResourceBody`/`Blob` response output are the same design
  family: typed wire representations for non-JSON payloads, not reasons to bypass the
  generated typed client.
- Browser/client coverage belongs to this ticket. Do not split it into a peer top-level
  Board frontend ticket.
- Board's generated client seed currently proves `FormData` input for `/api/stories` and
  `Blob` output for `/api/stories/:story_id/attachment`.
- Current follow-up homes outside this ticket:
    - Task error to `ViewExit`/`HttpExit` conversion friction belongs to
      `../task-error-exit-conversions/`.
    - TestApp cookie continuation ergonomics belong to `../testapp-cookie-continuation/`.
- Board's real server binary demonstrates the public Tower middleware helpers in
  `vorma::middleware`, including configured ETags via `strong`, `max_body_size`, and a
  skip predicate that reads request method, URI, and headers.
- Board's server binary demonstrates `vorma::bind_addr`, `vorma::is_dev`, and
  `vorma::is_build` at the app-server boundary.

Current reconciliation snapshot, 2026-06-23:

- Inventories in this directory were regenerated from the current tree.
- The old raw-body/download API gap is closed in current code. Evidence:
  `crates/vorma/src/resource_body.rs`, Board's `STORY_ATTACHMENT` resource in
  `examples/board/src/resources.rs`, `examples/board/src/client/vorma.gen.ts` emitting
  `Blob` output for `/api/stories/:story_id/attachment`, and the Board attachment request
  test asserting bytes/content-type/content-disposition/HEAD behavior.
- The old "no Rust-side theme helper" note is stale. Current shape: `vorma/kit/theme`
  remains generic TS kit surface, and Vorma also exposes `vorma::kit::theme::script(None)`
  as the Rust document-builder convenience for inlining that exact pre-paint script. Board
  uses the Rust helper in `examples/board/src/document.rs`.
- The old "kit is out of scope for Board coverage" note is stale under the current
  100%-coverage rule. Current Board uses `vorma/kit/theme` for theme state, and the
  diagnostics view uses the public converters, cookies, csrf, debounce, fmt, json,
  listeners, and result kit entry points in normal browser utility code.
- Board demonstrates `Middleware::with_methods` through a POST/DELETE-scoped middleware
  that echoes an optional app-owned CSRF header. The generated client decorator imports
  the Rust-exported `csrf_header` constant instead of duplicating the header string.
- Board demonstrates redirects from all three handler families: resource redirect with an
  explicit `SEE_OTHER` after story submit, middleware redirect for anonymous moderation
  access, and view redirect for canonical lowercase user routes.
- The API mount question is resolved: resources declare full URL patterns. Historical
  census text about omitted API mounts describes the pre-removal state and is followed by
  the landed "API mount is dead" resolution.
- Do not infer browser behavior from request-level `TestApp` tests. Browser, Vite,
  generated-client, and dev-loop behavior need direct tests when semantic coverage is
  missing.
- Current Board exercises public assets through `ctx.public_url(...)`,
  `DocumentBuildCtx::{request, public_url}`, `TestAppBuilder::with_public_asset(...)`,
  `HtmlAttribute::{r#type, attr}`, `SafeHtml::{script_content, style_content}`, and the
  low-level `HeadBuilder::link(...)` attribute path. Public Vorma APIs still need Board
  coverage even when framework-owned tests also cover them.

What to do:

- Reconcile the old API/Board source artifacts in this directory against current code and
  current generated contracts. When an old finding is stale, say why and where the current
  truth lives.
- Confirm Board still covers the public API surface it is supposed to cover after the
  latest task, resource-body, middleware, matcher, and generated-client changes.
- Add or update focused tests when the coverage claim depends on behavior rather than mere
  declarations.
- Spawn narrower tickets for real follow-up work that is not part of the broad API
  coverage pass.

Verification expectations:

- Rust behavior changes need focused `cargo test` coverage in the affected crate and
  request-level `vorma::testing::TestApp` proof when request semantics are involved.
- Wire-contract changes need regenerated shared fixtures and TypeScript proof against
  those fixtures.
- Generated-client/API changes need the relevant TypeScript typecheck/tests.
- Runtime/client/dev-loop changes need e2e proof before being called done.

Done means:

- Inventories in this directory are fresh and reproducible.
- The Board/API coverage map reflects current code, not stale milestone docs.
- Historical artifacts in this directory have been reconciled into current task-local
  context, with stale claims corrected or explicitly marked stale.
- Any remaining Board/API gaps are either fixed or split into explicit ticket dirs with
  enough context for a cold agent.
