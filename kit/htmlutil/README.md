# kit/htmlutil

`github.com/vormadev/vorma/kit/htmlutil`

`htmlutil` is a small HTML element renderer for server-side output, with
explicit support for:

- escaped vs trusted attributes/content
- CSP nonce/hash helpers
- SRI integrity helper
- deterministic rendering into a `strings.Builder`

## Import

```go
import "github.com/vormadev/vorma/kit/htmlutil"
```

## Core Type

```go
el := &htmlutil.Element{
	Tag:        "script",
	TextContent: "console.log('safe escaped text')",
}
```

Field behavior:

- `Attributes` values are HTML-escaped on render
- `AttributesKnownSafe` values are written as trusted raw values
- if the same attribute key exists in both maps, `AttributesKnownSafe` wins
- `BooleanAttributes` render as standalone keys (`defer`, `disabled`, etc.)
- `TextContent` is escaped
- `DangerousInnerHTML` is not escaped (trusted only)
- `SelfClosing` forces `<tag />` even for non-void tags

## Rendering

```go
html, err := htmlutil.RenderElement(el)
```

For high-throughput paths:

```go
var b strings.Builder
if err := htmlutil.RenderElementToBuilder(el, &b); err != nil {
	return err
}
```

`RenderElementToBuilder` returns errors for nil element, nil builder, or missing
tag.

`RenderModuleScriptToBuilder` is a convenience helper for:

```html
<script type="module" src="..."></script>
```

## CSP and SRI Helpers

### Add a nonce

```go
nonce, err := htmlutil.AddNonce(el, 0) // 0 -> default length 16
```

### Compute SHA-256 hash for inline trusted content

```go
hash, err := htmlutil.ComputeContentSha256(el)
```

This hashes `DangerousInnerHTML` content.

### Set SRI integrity

```go
_, err := htmlutil.SetSha256Integrity(el, externalHashBase64)
```

This sets `integrity="sha256-<hash>"` in trusted attributes.

## Escaping Into Trusted Form

`EscapeIntoTrusted` returns a new `Element` where escaped-safe values are
already consolidated into trusted fields.  
Useful when you need to escape once, then render many times.

## API Coverage

### Types

- `type Element`

### Exported Struct Fields

- `Element.Attributes map[string]string`
- `Element.AttributesKnownSafe map[string]string`
- `Element.BooleanAttributes []string`
- `Element.DangerousInnerHTML string`
- `Element.SelfClosing bool`
- `Element.Tag string`
- `Element.TextContent string`

### Functions

- `func AddNonce(el *Element, len uint8) (string, error)`
- `func ComputeContentSha256(el *Element) (string, error)`
- `func EscapeIntoTrusted(el *Element) Element`
- `func RenderElement(el *Element) (template.HTML, error)`
- `func RenderElementToBuilder(el *Element, htmlBuilder *strings.Builder) error`
- `func RenderModuleScriptToBuilder(src string, htmlBuilder *strings.Builder) error`
- `func SetSha256Integrity(el *Element, externalSha256Hash string) (string, error)`
