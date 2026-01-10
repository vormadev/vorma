---
title: matcher
description:
    URL pattern matcher with dynamic params, splats, index routes, and nested
    matching.
---

```go
import "github.com/vormadev/vorma/kit/matcher"
```

## Matcher

```go
type Options struct {
    DynamicParamPrefixRune rune   // default: ':'
    SplatSegmentRune       rune   // default: '*'
    ExplicitIndexSegment   string // default: "" (trailing slash = index)
    Quiet                  bool   // suppress duplicate pattern warnings
}

func New(opts *Options) *Matcher
```

## Pattern Registration

```go
func (m *Matcher) RegisterPattern(originalPattern string) *RegisteredPattern
func (m *Matcher) NormalizePattern(originalPattern string) *RegisteredPattern
```

## Matching

Single best match:

```go
func (m *Matcher) FindBestMatch(realPath string) (*BestMatch, bool)

type BestMatch struct {
    *RegisteredPattern
    Params      Params      // e.g. {"id": "123"}
    SplatValues []string    // e.g. ["a", "b", "c"] for /*
}
```

Nested matches (for layout hierarchies like Remix/Next.js):

```go
func (m *Matcher) FindNestedMatches(realPath string) (*FindNestedMatchesResults, bool)

type FindNestedMatchesResults struct {
    Params      Params
    SplatValues []string
    Matches     []*Match    // ordered by depth, index routes last
}
```

## RegisteredPattern

```go
func (rp *RegisteredPattern) OriginalPattern() string
func (rp *RegisteredPattern) NormalizedPattern() string
func (rp *RegisteredPattern) NormalizedSegments() []*segment
```

## Path Utilities

```go
func ParseSegments(path string) []string  // "" → [], "/" → [""], "/a/b/" → ["a","b",""]
func JoinPatterns(rp *RegisteredPattern, pattern string) string  // joins normalized base with new pattern
func HasLeadingSlash(pattern string) bool
func HasTrailingSlash(pattern string) bool
func EnsureLeadingSlash(pattern string) string
func EnsureTrailingSlash(pattern string) string
func EnsureLeadingAndTrailingSlash(pattern string) string
func StripLeadingSlash(pattern string) string
func StripTrailingSlash(pattern string) string
```

## Pattern Syntax

- `/users` — static segment
- `/users/:id` — dynamic param (captured in `Params`)
- `/files/*` — splat/catch-all (captured in `SplatValues`)
- `/users/` or `/users/_index` — index route (trailing slash or explicit
  segment)
- `""` — empty string pattern (layout root, included in nested results for ALL
  paths)

## Priority Rules (FindBestMatch)

1. Static beats dynamic beats splat
2. Longer segment matches beat shorter ones
3. Index (`/`) beats empty string (`""`) beats root splat (`/*`)
4. When path is deeper than longest pattern: splat wins over dynamic

## Trailing Slash Behavior

- `/users/` does NOT match `/users/:id` (trailing slash requires index or splat)
- `/users/` with no index pattern will match `/users/*` if registered
- Exact static match wins even with trailing slash

## Example

```go
m := matcher.New(nil)
m.RegisterPattern("")           // layout root (appears in ALL nested results)
m.RegisterPattern("/")          // index route (only for exact "/" path)
m.RegisterPattern("/users")
m.RegisterPattern("/users/:id")
m.RegisterPattern("/files/*")

// Best match
match, ok := m.FindBestMatch("/users/123")
// match.Params["id"] == "123"

// Nested matches (returns all matching layouts in order)
results, ok := m.FindNestedMatches("/users/123")
// results.Matches: ["", "/users", "/users/:id"]
// results.Params: {"id": "123"}

// Root path nested matches
results, ok = m.FindNestedMatches("/")
// results.Matches: ["", "/"]
```
