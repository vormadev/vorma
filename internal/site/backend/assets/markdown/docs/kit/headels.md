---
title: headels
description:
    HTML head element builder with deduplication, escaping, and rendering to
    template.HTML.
---

```go
import "github.com/vormadev/vorma/kit/headels"
```

## Instance

Manages rendering and deduplication rules:

```go
func NewInstance(dataAttribute string) *Instance
func (inst *Instance) InitUniqueRules(e *HeadEls)
func (inst *Instance) ToSortedAndPreEscapedHeadEls(els []*htmlutil.Element) *SortedAndPreEscapedHeadEls
func (inst *Instance) Render(input *SortedAndPreEscapedHeadEls) (template.HTML, error)
```

## HeadEls

Thread-safe collection of head elements:

```go
func New() *HeadEls
func FromRaw(els []*htmlutil.Element) *HeadEls
func (h *HeadEls) Add(defs ...typeInterface)
func (h *HeadEls) AddElements(other *HeadEls)
func (h *HeadEls) Collect() []*htmlutil.Element
```

## Element Definition Types

```go
type Tag string
type Attr struct{}  // use helper methods below
type BooleanAttribute string
type InnerHTML string
type TextContent string
type SelfClosing bool
```

Mark attribute as pre-escaped:

```go
func (a *Attr) KnownSafe() *Attr
```

## Tag Helpers

```go
func (h *HeadEls) Title(title string)
func (h *HeadEls) Description(description string)
func (h *HeadEls) Meta(defs ...typeInterface)
func (h *HeadEls) Link(defs ...typeInterface)
func (h *HeadEls) Script(defs ...typeInterface)
func (h *HeadEls) Style(defs ...typeInterface)
```

## Attribute Helpers

```go
func (h *HeadEls) Attr(name, value string) *Attr
func (h *HeadEls) BoolAttr(name string) BooleanAttribute
func (h *HeadEls) Name(name string) *Attr
func (h *HeadEls) Content(content string) *Attr
func (h *HeadEls) Property(property string) *Attr
func (h *HeadEls) Rel(rel string) *Attr
func (h *HeadEls) Href(href string) *Attr
func (h *HeadEls) Src(src string) *Attr
func (h *HeadEls) Type(type_ string) *Attr
func (h *HeadEls) Charset(charset string) *Attr
func (h *HeadEls) As(as string) *Attr
func (h *HeadEls) CrossOrigin(crossOrigin string) *Attr
```

## Content Helpers

```go
func (h *HeadEls) TextContent(content string) TextContent
func (h *HeadEls) DangerousInnerHTML(content string) InnerHTML
func (h *HeadEls) SelfClosing() SelfClosing
```

## Shorthand Helpers

```go
func (h *HeadEls) MetaPropertyContent(property, content string)
func (h *HeadEls) MetaNameContent(name, content string)
```

## Example

```go
h := headels.New()
h.Title("My Page")
h.Description("Page description")
h.Link(h.Rel("stylesheet"), h.Href("/style.css"))

inst := headels.NewInstance("app")
sorted := inst.ToSortedAndPreEscapedHeadEls(h.Collect())
html, err := inst.Render(sorted)
```
