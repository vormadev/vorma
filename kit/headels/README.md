# kit/headels

`github.com/vormadev/vorma/kit/headels`

`headels` helps you build HTML `<head>` elements in code, then:

- dedupe them with stable rules
- split into title/meta/rest buckets
- escape them for safe rendering
- render a final head HTML block with section markers

## Import

```go
import "github.com/vormadev/vorma/kit/headels"
```

## Typical Workflow

### 1) Build head elements

```go
h := headels.New()
h.Title("Dashboard")
h.Description("Admin dashboard")
h.MetaPropertyContent("og:title", "Dashboard")
h.Link(
	h.Rel("stylesheet"),
	h.Href("/assets/app.css"),
	h.SelfClosing(),
)
```

### 2) Prepare for rendering

```go
inst := headels.NewInstance("app")
sorted := inst.ToSortedAndPreEscapedHeadEls(h.Collect())
```

### 3) Render final HTML

```go
headHTML, err := inst.Render(sorted)
if err != nil {
	return err
}
_ = headHTML
```

`Render` wraps output with comment markers based on your instance attribute:

- `<!-- data-<attr>="meta-start" --> ... <!-- data-<attr>="meta-end" -->`
- `<!-- data-<attr>="rest-start" --> ... <!-- data-<attr>="rest-end" -->`

## Deduping Rules

- by default, only one `<title>` and one `<meta name="description">` survive
- for matching unique rules, later elements replace earlier ones
- other elements dedupe by content hash (later duplicate wins)
- you can extend unique rules once via `InitUniqueRules`

## Safety Notes

- normal attributes are escaped
- `Attr.KnownSafe()` marks an attribute value as trusted/pre-escaped
- `DangerousInnerHTML` injects raw HTML and should only be used for trusted
  content

## Builder Helpers

- high-level tags: `Title`, `Description`, `Meta`, `Link`, `Script`, `Style`
- attribute helpers: `Name`, `Content`, `Property`, `Rel`, `Href`, `Src`,
  `Type`, `Charset`, `As`, `CrossOrigin`
- content helpers: `TextContent`, `DangerousInnerHTML`, `SelfClosing`
- generic form: `Add(...)` if you need full control

## API Coverage

### Types

- `type Attr`
- `type BooleanAttribute`
- `type HeadEls`
- `type InnerHTML`
- `type Instance`
- `type SelfClosing`
- `type SortedAndPreEscapedHeadEls`
- `type Tag`
- `type TextContent`

### Exported Struct Fields

- `SortedAndPreEscapedHeadEls.Meta []*htmlutil.Element`
- `SortedAndPreEscapedHeadEls.Rest []*htmlutil.Element`
- `SortedAndPreEscapedHeadEls.Title *htmlutil.Element`

### Functions

- `func FromRaw(els []*htmlutil.Element) *HeadEls`
- `func New() *HeadEls`
- `func NewInstance(dataAttribute string) *Instance`

### Methods

- `func (h *HeadEls) Add(defs ...typeInterface)`
- `func (h *HeadEls) AddElements(other *HeadEls)`
- `func (h *HeadEls) As(as string) *Attr`
- `func (h *HeadEls) Attr(name, value string) *Attr`
- `func (h *HeadEls) BoolAttr(name string) BooleanAttribute`
- `func (h *HeadEls) Charset(charset string) *Attr`
- `func (h *HeadEls) Collect() []*htmlutil.Element`
- `func (h *HeadEls) Content(content string) *Attr`
- `func (h *HeadEls) CrossOrigin(crossOrigin string) *Attr`
- `func (h *HeadEls) DangerousInnerHTML(content string) InnerHTML`
- `func (h *HeadEls) Description(description string)`
- `func (Attr) GetType() htmlutilType`
- `func (BooleanAttribute) GetType() htmlutilType`
- `func (InnerHTML) GetType() htmlutilType`
- `func (SelfClosing) GetType() htmlutilType`
- `func (Tag) GetType() htmlutilType`
- `func (TextContent) GetType() htmlutilType`
- `func (h *HeadEls) Href(href string) *Attr`
- `func (inst *Instance) InitUniqueRules(e *HeadEls)`
- `func (a *Attr) KnownSafe() *Attr`
- `func (h *HeadEls) Link(defs ...typeInterface)`
- `func (h *HeadEls) Meta(defs ...typeInterface)`
- `func (h *HeadEls) MetaNameContent(name, content string)`
- `func (h *HeadEls) MetaPropertyContent(property, content string)`
- `func (h *HeadEls) Name(name string) *Attr`
- `func (h *HeadEls) Property(property string) *Attr`
- `func (h *HeadEls) Rel(rel string) *Attr`
- `func (inst *Instance) Render(input *SortedAndPreEscapedHeadEls) (template.HTML, error)`
- `func (h *HeadEls) Script(defs ...typeInterface)`
- `func (h *HeadEls) SelfClosing() SelfClosing`
- `func (h *HeadEls) Src(src string) *Attr`
- `func (h *HeadEls) Style(defs ...typeInterface)`
- `func (h *HeadEls) TextContent(content string) TextContent`
- `func (h *HeadEls) Title(title string)`
- `func (inst *Instance) ToSortedAndPreEscapedHeadEls(els []*htmlutil.Element) *SortedAndPreEscapedHeadEls`
- `func (h *HeadEls) Type(type_ string) *Attr`
