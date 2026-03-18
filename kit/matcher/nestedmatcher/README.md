# kit/nestedmatcher

`github.com/vormadev/vorma/kit/nestedmatcher`

Nested route matcher for parent-to-leaf match stacks.

Use this package when one real path should resolve to multiple route patterns in
order, such as layout/loader style routing.

## Import

```go
import "github.com/vormadev/vorma/kit/nestedmatcher"
```

## Quick Start

```go
m := nestedmatcher.New(nil)

m.RegisterPattern("")
m.RegisterPattern("/users")
m.RegisterPattern("/users/:id")

results, ok := m.FindMatches("/users/42")
if ok {
	_ = results.Matches      // ordered root -> leaf matches
	_ = results.Params["id"] // "42"
}
```

## Options

Defaults:

- `DynamicParamPrefix`: `':'`
- `SplatSegmentIdentifier`: `'*'`
- `ExplicitIndexSegmentIdentifier`: `""` (disabled)
- `Quiet`: `false` (duplicate-registration warnings enabled)

## Match Behavior

- `FindMatches` returns every matched pattern in nested order.
- Dynamic params and splats are accumulated once for the resolved path.
- Returns `(*Results, false)` when no nested matches exist.

## Lifecycle And Concurrency Notes

- Register patterns before serving traffic.
- Treat matcher registration as setup-time mutation, not request-time mutation.
- `NormalizePattern` validates and returns normalized metadata without
  registering.

## Public API

### Types

- `type Matcher`
- `type Options`
- `type Params`
- `type RegisteredPattern`
- `type Segment`
- `type SegmentType`
- `type Match`
- `type Results`

### Constructor

- `func New(opts *Options) *Matcher`

### `Matcher` methods

- `func (m *Matcher) ExplicitIndexSegmentIdentifier() string`
- `func (m *Matcher) DynamicParamPrefix() rune`
- `func (m *Matcher) SplatSegmentIdentifier() rune`
- `func (m *Matcher) NormalizePattern(originalPattern string) *RegisteredPattern`
- `func (m *Matcher) RegisterPattern(originalPattern string) *RegisteredPattern`
- `func (m *Matcher) FindMatches(realPath string) (*Results, bool)`
