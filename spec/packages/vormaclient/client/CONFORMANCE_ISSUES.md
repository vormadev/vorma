# vormaclient/client Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Type | Affected Requirements | Summary | Status |
|---|---|---|---|---|
| VCI-016 | impl-bug-candidate | FE-SKIP-005 | Skip param/splat guard appears to inspect innermost loader-bearing match rather than required outermost match. | open |
| VCI-017 | impl-bug-candidate | FE-FETCH-005, FE-FETCH-006 | Invalid highest-priority reload signal currently suppresses fallback evaluation of lower-priority redirect signals. | open |
| VCI-032 | impl-bug-candidate | FE-NAV-009 | Stale-origin revalidation short-circuit currently occurs after stale payload side effects can already run. | open |
| VCI-033 | impl-bug-candidate | FE-LINK-013 | Prefetch stop path appears to compute lookup key without applying search/hash overrides used by prefetch start path. | open |
| VCI-036 | impl-bug-candidate | API-UI-004, FE-UI-010, FE-UI-011 | Preact typed pattern-based helper memoization appears stale across route-change matched-pattern updates. | open |
| VCI-037 | impl-bug-candidate | FE-UI-012 | UI adapters can get stuck when route-level component identity transitions from undefined to defined after initial render. | open |
| VCI-038 | impl-bug-candidate | FE-UI-013 | Preact terminal component-absent outlet branch inserts a synthetic wrapper node while React/Solid remain node-empty. | open |
| VCI-039 | harness-constraint | FE-NAV-008, FE-NAV-011 | Legacy frontend tests encode stale debounce/coalescing timing commentary (5ms) while current runtime and strict spec contract use 8ms. | open |
| VCI-054 | impl-bug-candidate | FE-FETCH-012 | Route-data handling currently treats HTTP 304 as allowed but then fails due to unconditional JSON-required path. | open |
| VCI-055 | impl-bug-candidate | FE-LINK-019 | Repeated prefetch start() calls can stack pending timers while stop() clears only the latest handle. | open |
