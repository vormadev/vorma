# `vorma/client`

Stable runtime client API for Vorma applications.

## Runtime Scope

`vorma/client` runtime state is process-wide (window-global) and
singleton-based.

- Supported model is one Vorma app/version per browser window.
- Multiple mounted Vorma runtimes in the same window are not supported.

## Navigation Contract

- Use `vormaNavigate(...)` for navigation to Vorma routes.
- Use `revalidate()` to refresh current route data without adding a history
  entry.

## `getHistoryInstance()` Escape Hatch

`getHistoryInstance()` returns the raw `history` instance (`npm:history`) for
advanced integrations that intentionally opt into low-level behavior.

Use it when you need:

- raw reads of `history.location.state`,
- direct analytics/telemetry listeners on history updates, or
- coordination with non-Vorma URL consumers that require raw history semantics.

Do not use it for Vorma route navigation:

- Do not call `history.push(...)` or `history.replace(...)` to navigate to Vorma
  routes.
- Those mutations bypass Vorma route-data fetching + render commit
  orchestration.
- Use `vormaNavigate(...)` for route navigation and `revalidate()` for data
  refreshes.

## Reserved Query Params

Vorma reserves internal query-param keys for route-data transport:

- `vorma_json`: build marker for route-data requests.
- `dpl`: deployment routing marker used for revalidation when Vercel skew
  protection is active.

Applications should treat these keys as reserved and avoid reusing them for
application-level query semantics.
