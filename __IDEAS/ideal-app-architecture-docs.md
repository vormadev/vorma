# Ideal App Architecture Docs

## Idea

Document the ideal Vorma app architecture clearly.

Canonical guidance:

- Server loaders for stable route data
- Client loaders for browser-only route prework
- React Query or similar for live client data
- Task DAG for server composition/cache
- Actions for typed API calls
- Route revalidation only for actual route data invalidation

## Why

Vorma has a strong model, but users should feel guided toward it by default.
Good docs can prevent apps from building avoidable local routing/data machinery.

## Notes

- This should be practical and example-driven.
- It should include known-good interop patterns for common browser state/data
  libraries.
