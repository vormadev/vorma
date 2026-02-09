# kit/matcher Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/matcher`

## Scope

Package-owned matching behavior for route/path parsing, pattern normalization, registration, and runtime match selection.

## Terminology

- "Pattern" means an original registered route pattern.
- "Normalized pattern" means matcher canonical representation (`RegisteredPattern.normalizedPattern`).
- "Index segment" means empty terminal segment (`""`) in normalized segment list.
- "Root catch-all" means normalized pattern `/*`.
- "Non-root splat" means a splat terminal segment where pattern depth is greater than one segment.

## Requirements

- `KIT-MATCHER-001` ParseSegments empty/root handling.
`ParseSegments("")` MUST return empty slice; `ParseSegments("/")` MUST return one empty segment (`[""]`).
- `KIT-MATCHER-002` ParseSegments slash handling.
`ParseSegments` MUST ignore exactly one leading slash, collapse repeated interior slashes by skipping empty interior segments, and preserve trailing slash as terminal empty segment.
- `KIT-MATCHER-003` ParseSegments segment preservation.
`ParseSegments` MUST preserve literal segment bytes (including unicode) and MUST NOT apply URL-decoding or rune rewriting.
- `KIT-MATCHER-004` Option defaults.
Matcher construction MUST default dynamic prefix rune to `:`, splat segment rune to `*`, explicit index segment to empty string, and quiet mode to `false`.
- `KIT-MATCHER-005` Option validation.
Explicit index segment containing `/` MUST panic during option munging.
- `KIT-MATCHER-006` Segment-type classification.
Segment typing MUST classify empty segment as index, configured splat rune as splat, configured dynamic prefix rune as dynamic, else static.
- `KIT-MATCHER-007` Dynamic canonicalization.
During normalization, dynamic segments MUST be canonicalized to `:<name>` independent of configured dynamic rune.
- `KIT-MATCHER-008` Splat canonicalization.
During normalization, splat segments MUST be canonicalized to `*` independent of configured splat rune.
- `KIT-MATCHER-009` Explicit-index trailing-slash rule.
When explicit index segment mode is enabled, non-root patterns ending with trailing slash MUST panic.
- `KIT-MATCHER-010` Explicit-index suffix conversion.
When explicit index segment mode is enabled, pattern suffix `/<ExplicitIndexSegment>` MUST normalize to trailing index representation.
- `KIT-MATCHER-011` Normalized pattern string construction.
Normalized pattern string MUST be built from normalized segments with leading slash and interior slash separators, and non-index trailing slash MUST be trimmed.
- `KIT-MATCHER-012` RegisteredPattern metadata derivation.
Normalization MUST populate last segment type, last-segment-is-index flag, last-segment-is-non-root-splat flag, and dynamic param segment count.
- `KIT-MATCHER-013` Static-vs-dynamic registry split.
RegisterPattern MUST store all-static normalized patterns in static map and all other patterns in dynamic map + trie.
- `KIT-MATCHER-014` Duplicate registration behavior.
Registering an already-normalized pattern MUST replace existing map entry. Warning MUST be logged unless matcher quiet mode is enabled.
- `KIT-MATCHER-015` Dynamic trie node scoring.
Registration MUST assign static-edge score increment `+2`, dynamic-edge increment `+1`, splat increment `+0` for final best-match comparison.
- `KIT-MATCHER-016` Dynamic trie child identity.
Dynamic/splat child dedupe identity MUST be based on `paramName` key as implemented (including empty-name behavior).
- `KIT-MATCHER-017` Best-match static first.
FindBestMatch MUST check exact static map match before dynamic search.
- `KIT-MATCHER-018` Best-match trailing-slash static fallback.
If request path ends with slash, FindBestMatch MUST also check static map using path with trailing slash removed.
- `KIT-MATCHER-019` Best-match dynamic empty-segment exclusion.
Dynamic parameter nodes MUST NOT match empty request segments.
- `KIT-MATCHER-020` Best-match DFS eligibility.
Dynamic candidate is eligible when depth consumed all segments, or current node is splat, or trailing-slash end condition is satisfied.
- `KIT-MATCHER-021` Best-match score precedence.
Higher score candidate MUST replace lower score candidate. Equal-score replacement behavior is source-order dependent (first-kept).
- `KIT-MATCHER-022` Best-match parameter extraction.
When selected pattern has dynamic segments, params MUST map normalized dynamic names to aligned request segments by segment index.
- `KIT-MATCHER-023` Best-match splat extraction.
Selected pattern `/*` or terminal non-root splat MUST expose SplatValues containing remaining request segments from splat segment onward.
- `KIT-MATCHER-024` Best-match miss behavior.
If no candidate is found, FindBestMatch MUST return `(nil, false)`.
- `KIT-MATCHER-025` Nested matching path pre-processing.
FindNestedMatches MUST strip trailing slash from incoming real path before matching.
- `KIT-MATCHER-026` Nested matching root bootstrap.
If empty pattern route exists, it MUST be added as initial match candidate.
- `KIT-MATCHER-027` Nested matching empty-path special case.
For normalized empty real path: include `/` static index if present; else include root catch-all (`/*`) if present with empty splat; then return.
- `KIT-MATCHER-028` Nested static prefix scanning.
Nested matching MUST add static prefix matches for each consumed segment and include final trailing-slash static variant check at full depth.
- `KIT-MATCHER-029` Nested dynamic search gate.
Dynamic DFS search and root catch-all pre-add MUST run only when full static exact path was not found.
- `KIT-MATCHER-030` Nested catch-all suppression.
Root catch-all match MUST be removed when more-specific matches exist, except when empty-root pattern coexistence rule preserves two-match baseline.
- `KIT-MATCHER-031` Nested shorter splat/index pruning.
If matches exist at greater pattern depth, shorter-depth index and non-root splat matches MUST be removed.
- `KIT-MATCHER-032` Nested same-depth dynamic/splat conflict rule.
At longest matched depth, if both dynamic and splat exist: dynamic wins when request depth equals longest depth; splat wins when request depth exceeds longest depth.
- `KIT-MATCHER-033` Nested deterministic ordering.
Result ordering MUST sort index routes last; otherwise by ascending normalized segment length, then lexicographic normalized pattern tie-break.
- `KIT-MATCHER-034` Nested validity gate.
If final candidate cannot consume request depth (and is not splat-qualified), FindNestedMatches MUST return `(nil, false)`.
- `KIT-MATCHER-035` Nested sole-empty invalid case.
For non-root/non-empty request path, sole match set containing only empty pattern MUST be treated as no match.
- `KIT-MATCHER-036` Nested output source.
Nested output Params and SplatValues MUST come from final ordered (deepest/effective) match element.
- `KIT-MATCHER-037` Option token equivalence.
Custom option runes for dynamic/splat/index representations MUST preserve matching semantics after canonical normalization.
- `KIT-MATCHER-038` RegisteredPattern copy semantics.
`RegisteredPattern.NormalizedSegments()` MUST return a copy so caller mutation cannot mutate matcher internals.
- `KIT-MATCHER-039` Slash helper behavior.
`Has/Ensure/Strip` slash helper exported functions MUST behave exactly per source implementation and remain contract-bearing APIs.
- `KIT-MATCHER-040` JoinPatterns behavior.
`JoinPatterns` MUST join a normalized base and incoming pattern using source-defined slash-merge rules (no double slash when base ends and incoming starts with slash).
- `KIT-MATCHER-041` RegisteredPattern accessor behavior.
`RegisteredPattern.NormalizedPattern()` and `RegisteredPattern.OriginalPattern()` MUST return the stored normalized/original strings without additional transformation.
- `KIT-MATCHER-042` Matcher option getter behavior.
`GetExplicitIndexSegment()`, `GetDynamicParamPrefixRune()`, and `GetSplatSegmentRune()` MUST return the matcher's effective configured option values.
