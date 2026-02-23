# kit/matcher

`github.com/vormadev/vorma/kit/matcher`

Path-pattern matcher for router-like workloads.

It supports:

- static segments (`/users`)
- dynamic params (`/users/:id`)
- splats (`/files/*`)
- single best-match lookup

## Import

```go
import "github.com/vormadev/vorma/kit/matcher"
```

## Core Types

- `Matcher`: registers patterns and resolves paths.
- `RegisteredPattern`: normalized form of a registered pattern.
- `BestMatch`: result of `FindBestMatch`.

For nested/layout match stacks, use
`github.com/vormadev/vorma/kit/nestedmatcher`.

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
- `Quiet`: `false` (duplicate-registration warnings enabled)

## Match APIs

### `FindBestMatch`

Returns exactly one route using internal precedence/score rules.

- exact static matches are preferred when available
- dynamic segments capture params by segment name
- splats capture the remaining segments
- returns `(*BestMatch, false)` when nothing matches

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
- Dynamic params match non-empty segments.
- `RegisteredPattern.NormalizedSegments()` returns a copy of normalized segment
  metadata, so mutating that returned slice does not mutate matcher internals.

## API Reference

### Types

- `type BestMatch`
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
- `func (m *Matcher) DynamicParamPrefix() rune`
- `func (m *Matcher) SplatSegmentIdentifier() rune`

### `RegisteredPattern` methods

- `func (rp *RegisteredPattern) OriginalPattern() string`
- `func (rp *RegisteredPattern) NormalizedPattern() string`
- `func (rp *RegisteredPattern) NormalizedSegments() []Segment`
