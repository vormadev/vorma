# kit/matcher

`github.com/vormadev/vorma/kit/matcher`

Path-pattern matcher for router-like workloads.

It supports:

- static segments (`/users`)
- dynamic params (`/users/:id`)
- splats (`/files/*`)
- index routes (trailing slash style, or explicit index segment style)
- single best-match lookup and nested-layout match stacks

## Import

```go
import "github.com/vormadev/vorma/kit/matcher"
```

## Core Types

- `Matcher`: registers patterns and resolves paths.
- `RegisteredPattern`: normalized form of a registered pattern.
- `BestMatch`: result of `FindBestMatch`.
- `FindNestedMatchesResults`: result of `FindNestedMatches`.

## Quick Start

```go
m := matcher.New(nil)
m.RegisterPattern("/")
m.RegisterPattern("/users")
m.RegisterPattern("/users/:id")
m.RegisterPattern("/files/*")

best, ok := m.FindBestMatch("/users/42")
if ok {
	_ = best.NormalizedPattern() // "/users/:id"
	_ = best.Params["id"]        // "42"
}
```

## Options

Defaults:

- `DynamicParamPrefix`: `':'`
- `SplatSegmentIdentifier`: `'*'`
- `ExplicitIndexSegmentIdentifier`: `""` (trailing-slash style indexes)
- `Quiet`: `false` (duplicate-registration warnings enabled)

### Explicit index segment mode

If you set `ExplicitIndexSegmentIdentifier` (for example `"_index"`):

- trailing slashes are not allowed for non-root patterns
- `/about/_index` behaves like the index route for `/about/`
- root index remains representable

## Match APIs

### `FindBestMatch`

Returns exactly one route using internal precedence/score rules.

- exact static matches are preferred when available
- dynamic segments capture params by segment name
- splats capture the remaining segments
- returns `(*BestMatch, false)` when nothing matches

### `FindNestedMatches`

Returns an ordered stack of matches for layout-style routing.

Use this when parent routes and a leaf route should all participate in
rendering.

Result fields:

- `Matches`: ordered list of matched registered patterns
- `Params`: params from the selected deepest/terminal match
- `SplatValues`: splat values from the selected deepest/terminal match

## Pattern Normalization Helpers

- `HasLeadingSlash`, `HasTrailingSlash`
- `EnsureLeadingSlash`, `EnsureTrailingSlash`, `EnsureLeadingAndTrailingSlash`
- `StripLeadingSlash`, `StripTrailingSlash`
- `ParseSegments`
- `JoinPatterns`

## Important Behavior Notes

- Register all patterns before serving traffic. `Matcher` is not designed as a
  concurrent registration/mutation structure.
- Duplicate registrations overwrite map entries and may log warnings when
  `Quiet` is false.
- `NormalizePattern` panics if `ExplicitIndexSegmentIdentifier` contains `/` or
  if invalid trailing-slash usage is provided in explicit-index mode.
- Dynamic params match non-empty segments.
- `RegisteredPattern.NormalizedSegments()` returns a copy of normalized segment
  metadata, so mutating that returned slice does not mutate matcher internals.
- Root catch-all `/*` is treated specially in nested matching to avoid
  overwhelming more specific matches.

## API Reference

### Types

- `type BestMatch`
- `type FindNestedMatchesResults`
- `type Match`
- `type Matcher`
- `type Options`
- `type Params`
- `type RegisteredPattern`
- `type Segment`
- `type SegmentType`

### Constructors and functions

- `func New(opts *Options) *Matcher`
- `func ParseSegments(path string) []string`
- `func JoinPatterns(rp *RegisteredPattern, pattern string) string`
- `func HasLeadingSlash(pattern string) bool`
- `func HasTrailingSlash(pattern string) bool`
- `func EnsureLeadingSlash(pattern string) string`
- `func EnsureTrailingSlash(pattern string) string`
- `func EnsureLeadingAndTrailingSlash(pattern string) string`
- `func StripLeadingSlash(pattern string) string`
- `func StripTrailingSlash(pattern string) string`

### `Matcher` methods

- `func (m *Matcher) RegisterPattern(originalPattern string) *RegisteredPattern`
- `func (m *Matcher) NormalizePattern(originalPattern string) *RegisteredPattern`
- `func (m *Matcher) FindBestMatch(realPath string) (*BestMatch, bool)`
- `func (m *Matcher) FindNestedMatches(realPath string) (*FindNestedMatchesResults, bool)`
- `func (m *Matcher) ExplicitIndexSegmentIdentifier() string`
- `func (m *Matcher) DynamicParamPrefix() rune`
- `func (m *Matcher) SplatSegmentIdentifier() rune`

### `RegisteredPattern` methods

- `func (rp *RegisteredPattern) OriginalPattern() string`
- `func (rp *RegisteredPattern) NormalizedPattern() string`
- `func (rp *RegisteredPattern) NormalizedSegments() []Segment`
