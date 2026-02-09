# kit/matcher Conformance Issues

Status: Active  
Last Updated: 2026-02-09

| Issue ID | Requirement(s) | Summary | Status |
|---|---|---|---|
| `KIT-MATCHER-ISSUE-001` | `KIT-MATCHER-005`, `KIT-MATCHER-009`, `KIT-MATCHER-013`, `KIT-MATCHER-014`, `KIT-MATCHER-038`, `KIT-MATCHER-039`, `KIT-MATCHER-040` | Intent-validation: panic/duplicate/helper API behaviors are currently source-only contracts (explicit-index panic, duplicate warning path, `NormalizedSegments()` copy, slash helpers, `JoinPatterns`). Add explicit conformance tests or confirm source-only acceptance. | open |
| `KIT-MATCHER-ISSUE-002` | `KIT-MATCHER-016` | Intent-validation: trie child deduping keys dynamic/splat children by `paramName`; an empty-name dynamic segment (`:`) and splat (`*`) share key `\"\"`, making sibling behavior potentially registration-order dependent and weakly specified by tests. Confirm intended contract. | open |
| `KIT-MATCHER-ISSUE-003` | `KIT-MATCHER-006`, `KIT-MATCHER-007`, `KIT-MATCHER-008`, `KIT-MATCHER-011`, `KIT-MATCHER-012`, `KIT-MATCHER-020`, `KIT-MATCHER-021`, `KIT-MATCHER-029`, `KIT-MATCHER-031`, `KIT-MATCHER-032` | Intent-validation: these requirements are partially test-backed but still depend on source-only algorithm details (classification/canonicalization and nested/best-match tie branches). Confirm expected branch semantics and close partial-to-covered gaps. | open |
