# Board Frontend And Client Coverage

This is task-local context for `board-api-coverage`, not a separate ticket.

Board must cover Vorma's browser/client APIs as part of the same 100% public API coverage
initiative. Do not split browser/client coverage into a peer workstream from server,
resource, request, or generated-contract coverage.

Board is also a user-facing teaching example. Code under `examples/board` must be readable
as an example application: comments should teach the Vorma APIs being used, explain when
those APIs are appropriate, and call out subtleties a real app author should understand.

Current Board client coverage snapshot from 2026-06-23:

- `examples/board/src/views.rs` declares twelve client view modules, and all twelve files
  exist under `examples/board/src/client/views/`.
- Submit view posts real `FormData` through the generated client.
- Story view uses `clientLoader`, `serverPromise`, `useClientLoaderData`, comment
  mutation, and typed `Blob` attachment retrieval through `apiClient.queryOrThrow`.
- Search view uses the app-owned `useApiQuery` wrapper and `useRouteSync`.
- User, docs, mod, and diagnostics views route through Vite. Mod uses typed mutation
  controls and consumes generated `ModAction`.
- Layout uses `usePatternViewData`, `usePatternClientLoaderData`, imperative
  `getRouteState`, `getWorkState`, and `revalidate`, and renders route/work/build-skew
  observer state from `onRouteUpdate`, `onWorkUpdate`, and `onBuildSkewDetected`.
- App setup uses `apiDecorator`, reusing a generated `csrf_header` constant when a
  readable CSRF cookie exists, plus `revalidateOnWindowFocus` object form,
  `useViewTransitions`, `workIndicator`, and `linkDefaultProps`.
- Story view uses `runClientLoaderOnHmr`, `beforeRouteYield`, `beforeRouteCommit`, typed
  `QueryError` display, and comment-draft route guarding.
- Search view uses explicit `prefetch`, `cancelPrefetch`, `toHref`, `navigate`, and
  non-throwing `apiClient.query`.
- Submit view uses `workIndicator.track` and `workIndicator.isActive` for client-side file
  parsing before upload.
- Mod diagnostics uses non-throwing `apiClient.mutate`, typed `MutationError` display,
  route-specific `errorBoundary`, and a parent default-boundary trigger.
- Mod diagnostics also demonstrates the public `vorma/kit/*` browser utilities:
  converters for encoding round trips, cookies/csrf for readable client cookie tokens,
  debounce for local diagnostics work, fmt/json/result for stable diagnostic payloads, and
  listeners for focus-aware app-owned browser behavior.
- Not-found uses the root catch-all view as a normal app-level fallback: the route is
  found and renders HTTP 200 while the UI explains that Board has no page for the URL.
  Its handler reads first-value and repeated-value query metadata, then uses
  `HttpSearchParams::iter` to count the app-owned `from` pairs while leaving the raw HTTP
  request semantics intact.
- Board consumes generated app extras and public-asset helpers: `front_page_size`,
  `keyboard_shortcuts`, `csrf_header`, and `vormaPublicUrl("mark.svg")`. It also
  exercises broader `Link` props: `prefetchDelayMs`, `attributeMatchRules`,
  `visitOnPointerDown`, `replace`, `scrollToTop`, `skipWorkIndicator`, and history
  `state`.
- `cargo run -p vorma-board-example` builds the Board browser bundle and emits chunks for
  every declared client view module.

Teaching-example rules to preserve:

- Board should demonstrate app-owned wrappers around `apiClient`, not expose users to
  framework-internal React Query machinery.
- `useApiMutation` takes endpoint identity at hook creation and variables at mutate time.
- `useApiQuery` belongs with search: queries take full args at hook creation because args
  are cache identity; the query key comes from `apiClient.toIdentityArray(args)`, and the
  function delegates to `queryOrThrow`.
- Vorma mutations auto-revalidate route data by default; do not add manual route
  revalidation unless a mutation deliberately opts out.
- `workIndicator.track` is for non-Vorma async work the app wants reflected globally.
  Vorma-owned API work is already tracked by the client core.
- Public kit helpers are generic app utilities. Board may use them where they make normal
  app code clearer, but kit must not grow Board-specific or Vorma-backend-specific
  helpers.
- If a Board feature exists mainly to demonstrate a public API, keep it user-facing:
  explain the Vorma use case plainly in the source or README. Do not mention maintainer
  ledgers, historical handoff context, tickets, or private coverage process in Board
  source files.

Historical cleanup already performed:

- Board replaced the remaining browser/client coverage value from the earlier legacy
  example. Keep new browser/client API coverage in Board rather than adding another
  example surface.
- `examples/board/tests/app.rs` previously contained
  `task_override_injects_a_story_load_failure`. That was framework-semantic coverage:
  task override injection plus generic server-error wire behavior. It was removed from
  Board and equivalent coverage was added to `crates/vorma/tests/in_memory_test_app.rs`.
- Board comments were rewritten away from maintainer/process language and toward teaching
  comments for application authors.
