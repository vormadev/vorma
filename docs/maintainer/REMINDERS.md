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
- During dev builds, the app-server binary is the only app-linked binary: it is compiled
  once per iteration and the same executable answers live-state and then serves. There is
  no separate build-entry compile to parallelize against.
- The server cargo target comes from app config (`ServerBuildTarget`) and is bound at
  session start (read in-process, guaranteed fresh). A mid-session target change must fail
  the rebuild with a restart instruction — silently compiling the old target or implying a
  live re-target is possible are both unacceptable. There is no separate build-entry
  target input anywhere: the entry shim is never rebuilt mid-session.
- `vorma-matcher` and `vorma-tasks` are sovereign crates, not subservient to the Vorma
  framework (which is composed of the `vorma`, `vorma-build`, `vorma-client-wasm`, and
  `vorma-macros` crates). As such, `vorma-matcher` and `vorma-tasks` should not implement
  "pet features" or opinionated semantics required or requested by Vorma framework. If
  Vorma needs or wants special or more opinionated semantics, it must layer them on top of
  `vorma-matcher` or `vorma-tasks`, as applicable.
- HMR code must be dev-gated via Vite so it isn't bundled into production binaries.
- Dev events must NOT be driven by polling or periodic wakeup loops. File changes and
  shutdown must arrive through event channels. Local dev changes must be handled
  immediately, subject only to a tiny post-event filesystem-settle debounce; 100ms-class
  debounce/poll intervals are unacceptable.
- Everything must do the LEAST amount of necessary work to ensure correctness. This is not
  an optional enhancement; it's a correctness invariant. This is true for the dev server,
  the runtime server, everything.
- Always use blake3 for hashing unless an actual non-owned protocol requires something
  else (e.g., CSP hashes requiring sha256).

## Vite dev-server lifecycle

- Public-asset saves must NEVER restart Vite. Freshness comes from targeted invalidation:
  the plugin records asset→module edges at transform time, and the build sends changed
  source keys to the `/assets-changed` control endpoint, which invalidates exactly the
  referencing modules. Restarting Vite (`/cfg-changed`) is reserved for changes to the
  `VitePluginConfig` payload itself (entry module, view-module list, ignored patterns,
  dedupe list) — never for filemap-only changes, and never unconditionally on app
  rebuilds.
