# Dev Route Debug Event Stream

## Idea

Add a dev-only event stream or hook for route lifecycle debugging.

Useful events:

- Route fetch start/finish
- Reason: initial, navigation, revalidation, prefetch
- Matched patterns
- Import URLs
- Hard reload/build mismatch events
- Client loader timings
- Server loader timings if available

## Why

These events make framework behavior much easier to inspect when debugging
navigation, revalidation, prefetch, and stale client builds.

## Notes

- This should be dev-focused.
- Keep production bundle cost in mind.
