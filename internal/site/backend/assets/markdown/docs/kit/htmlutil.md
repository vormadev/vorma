---
title: htmlutil
description:
    HTML element construction and rendering with escaping and CSP/SRI support.
---

```go
import "github.com/vormadev/vorma/kit/htmlutil"
```

## Element

```go
type Element struct {
    Tag                 string
    Attributes          map[string]string // escaped on render
    AttributesKnownSafe map[string]string // not escaped on render
    BooleanAttributes   []string
    TextContent         string            // escaped on render
    DangerousInnerHTML  string            // not escaped on render
    SelfClosing         bool
}
```

## Rendering

```go
func RenderElement(el *Element) (template.HTML, error)
func RenderElementToBuilder(el *Element, htmlBuilder *strings.Builder) error
func RenderModuleScriptToBuilder(src string, htmlBuilder *strings.Builder) error
```

## Escaping

Escape all fields into trusted equivalents:

```go
func EscapeIntoTrusted(el *Element) Element
```

## CSP / SRI Helpers

Compute SHA-256 hash of inline content (for CSP header):

```go
func ComputeContentSha256(el *Element) (string, error)
```

Set integrity attribute for external resource (SRI):

```go
func SetSha256Integrity(el *Element, sha256Hash string) (string, error)
```

Generate and add nonce attribute:

```go
func AddNonce(el *Element, len uint8) (string, error)
```
