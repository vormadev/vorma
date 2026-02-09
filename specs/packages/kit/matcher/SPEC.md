# kit/matcher Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/matcher`

## Scope

Package-owned matching behavior for route/path parsing, pattern normalization, registration, and runtime match selection.

## Requirements

- `KIT-MATCHER-001` ParseSegments contract.
`ParseSegments` MUST:
return `[]` for empty path, return `[""]` for `/`, ignore a leading slash, collapse repeated interior slashes, and preserve trailing slash as a terminal empty segment.
- `KIT-MATCHER-002` Options defaults and validation.
`New`/`mungeOptsToDefaults` MUST default dynamic prefix to `:`, splat rune to `*`, and explicit index segment to empty string; explicit index segment containing `/` MUST panic.
- `KIT-MATCHER-003` Pattern normalization contract.
`NormalizePattern` MUST canonicalize dynamic segments to `:<name>`, splat segments to `*`, and index segments to empty segment form. When explicit index segment is configured, non-root trailing slash in input patterns MUST panic, and `/<explicitIndexSegment>` suffix MUST normalize to trailing slash index form.
- `KIT-MATCHER-004` Registration ownership and duplicate handling.
`RegisterPattern` MUST store static-only patterns in static registry and patterns containing dynamic/splat segments in dynamic registry/trie. Duplicate normalized patterns MAY be registered; latest registration wins for lookup, and warning logging MUST occur unless `Options.Quiet` is true.
- `KIT-MATCHER-005` Best-match precedence and scoring.
`FindBestMatch` MUST prefer exact static match first, then static match after trimming request trailing slash, then dynamic/splat trie search. Trie scoring MUST prefer static edge (`+2`) over dynamic edge (`+1`). Dynamic parameters MUST NOT match empty segment.
- `KIT-MATCHER-006` Best-match output extraction.
For best-match results, `Params` MUST contain captured dynamic segment values keyed by normalized param name, and `SplatValues` MUST contain remaining path segments for root splat (`/*`) and terminal non-root splat patterns.
- `KIT-MATCHER-007` Nested-match baseline behavior.
`FindNestedMatches` MUST strip request trailing slash before matching, include empty-root static match when registered, include static prefix matches, and include root catch-all fallback when no full static path match exists.
- `KIT-MATCHER-008` Nested-match pruning and precedence.
`FindNestedMatches` MUST prune less-specific catch-all/index/non-root-splat matches when more specific matches are present, and MUST resolve equal-depth dynamic-vs-splat conflicts based on whether request depth equals or exceeds longest matched pattern depth.
- `KIT-MATCHER-009` Nested-match validity and ordering.
`FindNestedMatches` MUST return `ok=false` when final match set cannot consume the full request path (unless splat-qualified), and MUST order results deterministically: non-index before index, then by normalized segment length, then lexicographic normalized pattern tie-break.
- `KIT-MATCHER-010` Option token equivalence.
Custom option runes (`DynamicParamPrefixRune`, `SplatSegmentRune`, `ExplicitIndexSegment`) MUST preserve matching semantics equivalent to default token behavior after normalization.
