# Miscellaneous Vorma Framework Reminders

- We must not ship a view/resource or client loader manifest to the client so as to not
  inflate the bundle.
- We must keep the generated TS view/resource arrays type-level-only (and non-exported) so
  as to not inflate the bundle. This is why the user has to manually apply the "kind"
  override for API calls that need non-default revalidation behavior. This is an
  acceptable tradeoff to not import the actual view/resource arrays at runtime. Inflated
  bundle would be a greater evil than having to manually pass the resource kind to API
  calls once in a while.
- Vorma middlewares must always execute in parallel. This is a contract, not an
  optimization.
- View handlers must always execute in parallel. This is a contract, not an optimization.
- Client loaders must always execute in parallel to the maximum extent possible based on
  what we "know" at the time. If we know two client loaders exist before the server
  response starts, we should start those two right away. Others that get discovered upon
  receiving the server response should then be flighted in parallel and awaited before
  committing the route. This is a contract, not an optimization.
- During dev builds, live state binary build/call should be as parallel as possible with
  the actual app server build. This is a contract, not an optimization.
- Build binary location must not come from the app config. It must come from the build
  binary itself. Otherwise we imply that changing the path string pointing to the binary
  in fact can make the dev server stop and restart itself, which is of course impossible.
  Making that value be passed from the build call itself avoids that false implication.
- `vorma-matcher` and `vorma-tasks` are sovereign crates, not subservient to the Vorma
  framework (which is composed of the `vorma`, `vorma-build`, `vorma-client-wasm`, and
  `vorma-macros` crates). As such, `vorma-matcher` and `vorma-tasks` should not implement
  "pet features" or opinionated semantics required or requested by Vorma framework. If
  Vorma needs or wants special or more opinionated semantics, it must layer them on top of
  `vorma-matcher` or `vorma-tasks`, as applicable.
