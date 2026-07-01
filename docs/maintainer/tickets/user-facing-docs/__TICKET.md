# Docs Quality And Comprehensiveness

Status: open

Improve Vorma's user-facing documentation without inventing a separate narrative manual.
The official strategy is source-adjacent documentation:

- Rustdoc and JSDoc at the API boundary.
- Board as the canonical tutorial.
- Generated/reference docs from source comments, exported types, and generated contracts.
- Maintainer docs only for maintainer-only concerns.

Do not write docs to explain around bad names, footguns, or unsettled APIs. Fix those
first. Do not create standalone prose that duplicates Board or source-level API docs
unless a concrete need appears.

Source context:

- `AGENTS.md`, especially "Documentation Strategy" and "Examples And API Coverage".
- `docs/maintainer/board-example/README.md`
- `examples/board/README.md`
- `examples/board/src/**`
- `crates/*/src/**`
- `packages/vorma/**`
- `../board-api-coverage/API_DESIGN.md`
- `../board-api-coverage/frontend-client-coverage.md`
- `../board-api-coverage/PRESSURE_TEST_CENSUS.md`
- `../create-vorma-rewrite/__TICKET.md`

Quality standard:

- Public Rust APIs should have Rustdoc where users need contract, lifecycle, error,
  panic, type, or usage guidance.
- Public TypeScript APIs should have JSDoc where users need hover-time contract,
  lifecycle, error, type, or usage guidance.
- Board should teach Vorma composition through real code and comments. The comments should
  explain when to use APIs, why Board uses them, and what subtleties matter.
- Generated/reference docs should derive from source wherever possible.
- Maintainer docs should not become user tutorials. If a maintainer doc contains durable
  user-facing guidance, move that guidance to Rustdoc/JSDoc or Board and keep only the
  maintainer coordination facts.

Current truth that source-adjacent docs and Board comments should teach:

- Resources declare full URL patterns; there is no framework API mount.
- Views are framework-owned rendering segments, not HTTP documents.
- `ViewExit` has no status; `HttpExit` does.
- Resource errors use JSON envelopes; success stays bare JSON unless using the
  non-JSON/raw body API.
- Vorma mutations auto-revalidate route data by default.
- App-owned wrappers are the React Query integration story.
- `workIndicator.track` is for non-Vorma async work.
- Splat patterns match one-or-more segments.
- A root catch-all view is the branded not-found idiom, but it renders as a normal
  app-level fallback with HTTP 200. Views have no HTTP status surface.
- Scoped middleware patterns are URL patterns with no hidden rewrites.
- Generic kit packages must not grow Vorma-specific backend helpers.
- The app-server binary is the live-state emitter and runtime server; there is no separate
  hot-loop build-entry binary.

What to do:

- Audit public Rust APIs for missing or weak Rustdoc.
- Audit public TypeScript APIs for missing or weak JSDoc.
- Audit Board comments against the teaching-example standard.
- Remove or rewrite any maintainer/user-doc boundary violations.
- Prefer improving names and API shapes over adding explanatory docs for confusing APIs.

Done means:

- A user can learn API contracts from Rustdoc/JSDoc.
- A user can learn application composition from Board.
- Maintainer docs remain maintainer-facing.
- No stale standalone user-doc initiative exists unless a concrete need has been found and
  ticketed separately.
