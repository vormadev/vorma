# Fable Matcher Notes

These notes preserve the forward-going matcher doctrine from the Fable histories. They
cover `vorma-matcher` itself and the framework decisions that now depend on it.

## Source Coverage

- Main transcript ranges: `9e3bfc5a-bb96-4b96-ad2c-24995f8ad7d7.jsonl` lines 8,462-10,935
  cover the API-mount debate, exact overlap requirement, typed matcher split, full
  matcher review, Go comparison, correctness fixes, and performance work.
- Current reconciliation: `crates/vorma-matcher/src/lib.rs`,
  `crates/vorma-matcher/src/overlap.rs`, `crates/vorma-matcher/src/pattern.rs`,
  `crates/vorma-contract/src/graph_validation.rs`,
  `docs/maintainer/PRESSURE_TEST_CENSUS.md`, and benchmark result files.

## Sovereignty of `vorma-matcher`

`vorma-matcher` is a sovereign generic crate, not a private helper for Vorma framework.
Vorma-specific policy belongs in `vorma-contract` or `vorma`, layered on top.

This distinction mattered during overlap work. The final state is acceptable because the
matcher crate exposes generic typed matcher concepts and generic overlap/specificity
operations. Vorma chooses how to use them for view/resource conflict validation.

Avoid docs in `vorma-matcher` that justify behavior with Vorma-only language such as
"view", "resource", "shadowing validation", or "api mount". Use matcher vocabulary:
flat matcher, nested matcher, pattern claim, overlap, witness, specificity.

## Typed Matcher Split

The old public dual-mode `Matcher` concept was retired:

- `MatcherBuilder` remains the grammar and registration authority.
- `MatcherBuilder::finish_flat()` returns `FlatMatcher`.
- `MatcherBuilder::finish_nested()` returns `NestedMatcher`.
- `FlatMatcher` exposes flat matching only.
- `NestedMatcher` exposes nested matching only.
- The shared engine is private implementation detail.

The reason was not cosmetic. The flat and nested matchers intentionally have different
literal semantics in edge cases, so a public value with both methods made it too easy to
ask the wrong question. This mirrors the useful Go shape at the layer that mattered:
Go's lower matcher had both methods, but Go's router layer exposed separate normal and
nested routers. Rust now makes that distinction directly in the public matcher API.

Do not reintroduce a public dual-mode matcher to "simplify" imports. The type split is a
semantic guardrail.

## Overlap Detection

The overlap requirement came from killing the API mount. The maintainer's bar was exact:
if overlap detection cannot be deterministically correct by construction in all cases,
the API mount must stay. It could not be a "usually works" heuristic.

Current design:

- Public API: `vorma_matcher::find_overlap(left, right) -> Option<Overlap>`.
- `Overlap` includes a concrete witness path and the claiming patterns from both sides.
- The function accepts typed matcher sides through `OverlapSide`; it is not a parser
  reimplementation.
- It generates a finite candidate family and evaluates candidates using the real
  matchers. Candidate completeness is pinned by a brute-force enumeration oracle across
  matcher pairings.
- This is "correct by construction" because the overlap implementation delegates
  acceptance and winner selection to the same matcher implementations production uses.

Important discoveries from the read-through:

- Trailing slashes are semantic. The early clean segment-vector analysis missed paths
  such as `/users/`.
- Flat and nested matchers define different claim relations; exact overlap must respect
  the matcher type on each side.
- Dynamic index patterns have context-dependent behavior. The final relation and pins
  capture the corrected intended claim behavior rather than guessing from one registration
  shape.

Do not replace `find_overlap` with a hand-written pattern-vs-pattern shortcut unless it is
proven equivalent to the real matcher behavior and retains the witness/oracle coverage.

## Specificity Doctrine

The final route-conflict doctrine is specificity, not "one owner per path":

- Overlap is normal.
- A conflict exists only when two overlapping patterns have equal specificity in a
  context where the framework cannot choose an owner.
- `vorma_matcher::compare_specificity(a, b) -> Ordering` is the one public ordering.
- Runtime matching order and graph validation must agree with `compare_specificity`.
- `better_than` and any walk-time scoring should delegate to this ordering or be tested
  against it. There must not be multiple independent specificity implementations.

The maintainer corrected two false starts:

- Treating every view/resource overlap as illegal banned legitimate layouts.
- Special-casing only the root catch-all was still wrong. The root catch-all works because
  it is least specific, not because it has bespoke framework treatment.

Examples that must remain legal:

- A view `/s/:story_id` and GET resource `/s/export` coexist; the static pattern owns its
  path.
- A root catch-all view coexists with GET/HEAD resources.
- GET resources under a splat view are decided by specificity, not by mount partition.

Examples that should error:

- A GET/HEAD resource and a view with the same shape and equal specificity over a shared
  witness path.
- The validation axis is HTTP method, not `ResourceKind`. A GET-method resource with
  kind `Mutation` still participates in view/resource tie validation.

## API Mount Removal Dependency

The API mount removal depends on matcher correctness:

- `find_overlap` finds shared paths.
- `compare_specificity` decides whether the shared path is ambiguous.
- `vorma-contract` emits `ResourceViewSpecificityTie` when a view and GET/HEAD resource
  overlap with equal specificity.
- Runtime classification compares best view and resource claims with the same ordering.

If future matcher changes weaken overlap/specificity guarantees, the API-mount-free URL
model becomes unsound. Treat overlap tests and specificity tests as framework-critical,
not merely crate-local niceties.

## Corrected Semantics From Go

The matcher review established a principle: Go compatibility is not bug compatibility.
If the Go matcher clearly missed an invariant it would have wanted, Rust should record the
change as a correction, not preserve the bug.

Corrections landed under that principle:

- **Catch-all cover rule**: a prefix hit is not a complete match. The root catch-all yields
  only to an entry whose chain completes through the path. Dead prefixes drop out and the
  fallback can claim the path.
- **Dirty-path rule**: at most one trailing slash is tolerated as noise. Empty segments do
  not match anything. Params and splat values cannot contain empty strings.
- **Nested flatten dead gate**: an unreachable `params.is_empty()` gate was deleted.
- **Index claim correction**: an index pattern under a dynamic directory can claim its
  parent path without requiring a non-index sibling registration.

These were red-pinned before the fixes and the property model was updated. Future
regression reviews should treat them as intentional Rust semantics, not accidental
departures.

## Param Grammar Tightening

Go accepted empty and duplicate dynamic param names and its fixtures pinned last-wins data
loss. Rust rejects those at registration:

- Param names must be non-empty.
- Param names must be valid identifiers for the generated TypeScript surface.
- Duplicate param names are illegal.

This tightening is objectively correct because the typed TS client cannot safely model
empty or duplicate keys, and last-wins capture collapse is silent data loss. Do not reopen
it as a compatibility question.

## Splat Semantics

Splat patterns match one or more segments, not their bare root. A `/docs/*` route does not
cover `/docs`. The honest structure is a plain `/docs` route plus a splat child. Board is
intended to demonstrate that docs-era pattern.

Do not "fix" this by making splat zero-or-more without a fresh design pass. It would alter
specificity, overlap, and catch-all behavior.

## Deep Trees and Non-Recursive Clone/Drop

`SegmentNode` had an iterative `Drop` because deeply nested route trees can overflow the
stack if dropped recursively. The derived `Clone` had the same stack-overflow hazard. The
fix was a hand-written iterative `Clone` and a deep-pattern test that clones a builder at
33,000 segments.

Preserve both properties:

- Dropping a very deep matcher tree must not recurse.
- Cloning a very deep builder or matcher engine must not recurse.

This matters because builder and engine cloning is reachable through actual crate APIs.

## Performance Doctrine

The maintainer set the bar explicitly: Rust matcher performance should beat Go, not merely
match it. The final reported state did beat every Go benchmark row.

Important performance changes:

- Registered patterns are stored directly on tree nodes; matching does not hop from a
  string key back into a map for every candidate.
- Static patterns ride the nested tree instead of being rediscovered through growing
  prefix hashes.
- Walks track pattern references and rebuild captures positionally at emit.
- Candidate stacks use inline capacity.
- Param keys are shared from the registered pattern where possible.
- Splat captures use one backing buffer rather than one allocation per segment.
- Public APIs expose std/vorma-owned types only. Internal storage choices stay private.

The CompactString incident is a major pitfall:

- A performance pass temporarily exposed third-party string/storage types through public
  matcher and framework APIs.
- That was rejected. Third-party storage types must not leak into public APIs for
  performance-only reasons.
- The final opaque `Params` and `SplatValues` types kept plain `&str`/`String` public
  methods and allowed faster internals.

If a future performance pass needs a non-std storage type, hide it behind a Vorma-owned
type before it touches public signatures.

## Bench Output Requirements

The benchmark result file must be the direct bench output:

- One machine/header block similar to Go's `goos`, `goarch`, `pkg`, `cpu` fields.
- One scannable row per benchmark.
- No prose preamble.
- No post-processing-only format that differs from terminal output.
- Refresh workflow should be direct command output piped or teed to `bench.results.txt`.

The user explicitly rejected both a lecture-like results file and unscannable Criterion
noise as durable artifacts. The current Makefile records matcher benches through
`cargo bench -p vorma-matcher --bench matching ... | tee crates/vorma-matcher/bench.results.txt`.
Keep the bench harness output itself human-scannable.

## Process Rules From This Thread

- Do not assert what the maintainer's memory was "really pointing at."
- When a semantics change is not an obvious bug fix, show the failing pin and get
  agreement before changing core matcher semantics.
- When a finding needs a ruling, give the recommendation first and explain from first
  principles. Do not hand the maintainer an under-explained corner case as homework.
- Do not run matcher tests against the wasm binding as if wasm were an independent
  implementation. It is the same matcher compiled to wasm, not Go's old separate TS
  matcher backend.
