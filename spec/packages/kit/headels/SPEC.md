# kit/headels Specification

Status: Active  
Last Updated: 2026-02-09  
Owner: `kit/headels`

## Scope

This package owns HTML head-element construction, deduplication, classification,
and rendering behavior exposed by `kit/headels`.

## Requirement Catalog

### Instance and Rule Initialization

#### KIT-HEADELS-001: Instance Marker Contract

Given `NewInstance(dataAttribute)`  
When called  
Then it MUST build marker comments in this exact shape:

- `<!-- data-<dataAttribute>="meta-start" -->`
- `<!-- data-<dataAttribute>="meta-end" -->`
- `<!-- data-<dataAttribute>="rest-start" -->`
- `<!-- data-<dataAttribute>="rest-end" -->`

#### KIT-HEADELS-002: Unique Rule Bootstrap Sources

Given `InitUniqueRules(e)`  
When first initialization runs  
Then rule sources MUST include:

- default internal rules for one `title` and one `meta[name="description"]`, and
- any rules collected from provided `e` (when non-nil).

Rule uniqueness MUST be deduplicated per tag using element hashing.

#### KIT-HEADELS-003: One-Time Initialization and Input Preservation

Given `InitUniqueRules`  
When called multiple times  
Then initialization MUST run once (`sync.Once`) and subsequent calls MUST NOT
rebuild rule state.

Given an input `HeadEls`  
When used for initialization  
Then the input collection MUST NOT be mutated.

### Rule and Hash Semantics

#### KIT-HEADELS-004: Element Hash Canonicalization

Given `hashElement(el)`  
When computing hash identity  
Then hash input MUST include:

- tag,
- regular attributes,
- trusted attributes,
- boolean attributes,
- dangerous inner HTML,
- text content,
- self-closing state.

Attribute keys and boolean attributes MUST be sorted before hashing so map/slice
order does not change identity.

#### KIT-HEADELS-005: Rule Matching Across Attribute Stores

Given `matchesRule(el, rule)`  
When evaluating key/value requirements  
Then matching MUST accept values found in either `Attributes` or
`AttributesKnownSafe`.

When rule boolean attributes are present  
Then all required booleans MUST be present on the element.

### Dedupe and Classification

#### KIT-HEADELS-006: Rule-Based Deduplication

Given `dedupeHeadEls(els)` and a tag with unique rules  
When an element matches a rule  
Then dedupe MUST key by `(tag, rule-index)` and use last-write-wins replacement
while preserving first insertion position.

Rule checks MUST run in rule order, and the first matching rule MUST be used.

#### KIT-HEADELS-007: Hash-Based Deduplication and Nil Handling

Given elements that do not match a unique rule  
When deduping  
Then dedupe MUST key by `hashElement` and use last-write-wins replacement while
preserving first insertion position.

Given nil elements in input  
When deduping  
Then nil entries MUST be ignored.

#### KIT-HEADELS-008: Sorted-and-PreEscaped Conversion

Given `ToSortedAndPreEscapedHeadEls(els)`  
When called  
Then it MUST:

- initialize rules if needed,
- dedupe first,
- run each element through `htmlutil.EscapeIntoTrusted`, and
- classify into exactly one bucket: `Title`, `Meta`, or `Rest`.

Title classification is `tag == "title"`; meta classification is
`tag == "meta"`; all others go to `Rest`.

### Rendering

#### KIT-HEADELS-009: Render Section Layout

Given `Render(input)`  
When rendering succeeds  
Then output MUST be ordered as:

1. optional title line,
2. `meta-start` marker,
3. meta lines,
4. `meta-end` marker,
5. `rest-start` marker,
6. rest lines,
7. `rest-end` marker.

Each rendered element and each marker boundary transition MUST be newline
separated as implemented.

#### KIT-HEADELS-010: Render Error Wrapping

Given `Render(input)`  
When element rendering fails  
Then returned errors MUST wrap section-specific messages:

- `error rendering title: ...`
- `error rendering meta head el: ...`
- `error rendering rest head el: ...`

### HeadEls Builder API

#### KIT-HEADELS-011: Collection Construction and Storage Model

Given `New()`  
When called  
Then it MUST return an empty collection.

Given `FromRaw(els)`  
When called  
Then it MUST wrap the provided slice as-is (no deep copy).

#### KIT-HEADELS-012: Add Type Mapping Semantics

Given `Add(defs...)`  
When definitions are applied  
Then mapping MUST be:

- `Tag` -> `Element.Tag`,
- `Attr` -> regular or trusted attributes based on `KnownSafe`,
- `BooleanAttribute` -> `BooleanAttributes`,
- `InnerHTML` -> `DangerousInnerHTML`,
- `TextContent` -> `TextContent`,
- `SelfClosing` -> `SelfClosing`.

#### KIT-HEADELS-013: Add Panic Preconditions

Given `Add(defs...)`  
When no `Tag` is supplied  
Then it MUST panic with a missing-tag failure.

#### KIT-HEADELS-014: AddElements and Collect Contracts

Given `AddElements(other)`  
When called  
Then it MUST append a clone of `other.Collect()` into receiver storage.

Given `Collect()`  
When called  
Then it MUST return a cloned slice so caller slice mutation does not mutate
internal storage.

#### KIT-HEADELS-015: Concurrency Contract

`HeadEls` mutation/collection operations (`Add`, `AddElements`, `Collect`) MUST
remain safe for concurrent use through internal locking.

#### KIT-HEADELS-016: Helper Method Expansion Contracts

Given high-level helpers  
When called  
Then they MUST expand to canonical `Add(...)` compositions:

- tag helpers: `Title`, `Description`, `Meta`, `Link`, `Script`, `Style`,
- attribute helpers: `Attr`, `Name`, `Content`, `Property`, `Rel`, `Href`,
  `Src`, `Type`, `Charset`, `As`, `CrossOrigin`,
- other helpers: `BoolAttr`, `SelfClosing`, `TextContent`, `DangerousInnerHTML`,
  `MetaPropertyContent`, `MetaNameContent`.

## Scenario Catalog

- `KHS-001`: instance marker construction.
- `KHS-002`: unique-rule bootstrap and one-time initialization.
- `KHS-003`: hash identity and rule matching semantics.
- `KHS-004`: rule-based dedupe and hash-fallback dedupe behavior.
- `KHS-005`: dedupe handling for nil elements and mixed element content.
- `KHS-006`: sorted/preescaped conversion and title/meta/rest classification.
- `KHS-007`: render layout and section marker output.
- `KHS-008`: render error wrapping behavior.
- `KHS-009`: `Add` mapping semantics and panic preconditions.
- `KHS-010`: high-level helper expansion behavior.
- `KHS-011`: `AddElements`/`Collect` copy semantics.
- `KHS-012`: concurrent mutation/collection safety.
