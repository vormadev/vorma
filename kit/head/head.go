package head

import (
	"fmt"
	"hash/fnv"
	"html/template"
	"maps"
	"slices"
	"sort"
	"strings"
	"sync"

	"github.com/vormadev/vorma/kit/htmlutil"
)

const AttrAnyValue = "\x00__vorma_headels_any__"

/////////////////////////////////////////////////////////////////////
/////// RENDERER
/////////////////////////////////////////////////////////////////////

type Renderer struct {
	meta_start          string
	meta_end            string
	rest_start          string
	rest_end            string
	once                sync.Once
	unique_rules_by_tag map[string][]*rule_attrs
}

func NewRenderer(namespace string) *Renderer {
	return &Renderer{
		meta_start: comment(namespace, "meta-start"),
		meta_end:   comment(namespace, "meta-end"),
		rest_start: comment(namespace, "rest-start"),
		rest_end:   comment(namespace, "rest-end"),
	}
}

func comment(namespace, val string) string {
	return fmt.Sprintf("<!-- %s-%s -->", namespace, val)
}

func (r *Renderer) InitDedupeRules(b *Builder) {
	r.once.Do(func() {
		r.unique_rules_by_tag = make(map[string][]*rule_attrs)

		// Build default rules without mutating the caller's Builder.
		defaults := NewBuilder()
		defaults.Add(Tag("title"))
		defaults.Meta(defaults.Name("description"))
		defaults.Meta(defaults.Name("viewport"))
		defaults.Meta(defaults.Name("robots"))
		defaults.Meta(defaults.AttrExists("charset"))
		defaults.Link(defaults.Rel("icon"))
		defaults.Link(defaults.Rel("canonical"))
		defaults.Meta(defaults.Property("og:title"))
		defaults.Meta(defaults.Property("og:description"))
		defaults.Meta(defaults.Property("og:url"))
		defaults.Meta(defaults.Property("og:type"))
		defaults.Meta(defaults.Property("og:locale"))
		defaults.Meta(defaults.Property("og:site_name"))
		defaults.Meta(defaults.Property("og:determiner"))

		var sources []*htmlutil.Element
		sources = append(sources, defaults.Elements()...)
		if b != nil {
			sources = append(sources, b.Elements()...)
		}

		seen_hashes := make(map[string]map[uint64]bool)
		for _, rule := range sources {
			h := hash_element(rule)
			if _, exists := seen_hashes[rule.Tag]; !exists {
				seen_hashes[rule.Tag] = make(map[uint64]bool)
			}
			if !seen_hashes[rule.Tag][h] {
				seen_hashes[rule.Tag][h] = true
				attrs := extract_rule_attrs(rule)
				r.unique_rules_by_tag[rule.Tag] = append(
					r.unique_rules_by_tag[rule.Tag],
					attrs,
				)
			}
		}
	})
}

// Prepared holds classified head elements ready for rendering.
type Prepared struct {
	Title *htmlutil.Element
	Meta  []*htmlutil.Element
	Rest  []*htmlutil.Element
}

const rough_avg_el_len = 80
const panic_render_nil_input = "head: Render input cannot be nil"

func (r *Renderer) Render(
	input *Prepared,
) (template.HTML, error) {
	r.InitDedupeRules(nil)
	if input == nil {
		panic(panic_render_nil_input)
	}

	meta_size := len(r.meta_start) + len(r.meta_end)
	rest_size := len(r.rest_start) + len(r.rest_end)
	estimated := meta_size + rest_size + 4
	if input.Title != nil {
		estimated += rough_avg_el_len
	}
	estimated += len(input.Meta) * rough_avg_el_len
	estimated += len(input.Rest) * rough_avg_el_len

	var b strings.Builder
	b.Grow(estimated)

	if input.Title != nil {
		if err := htmlutil.RenderElementToBuilder(input.Title, &b); err != nil {
			return "", fmt.Errorf("error rendering title: %w", err)
		}
		b.WriteString("\n")
	}

	b.WriteString(r.meta_start)
	b.WriteString("\n")
	for _, el := range input.Meta {
		if err := htmlutil.RenderElementToBuilder(el, &b); err != nil {
			return "", fmt.Errorf("error rendering meta head el: %w", err)
		}
		b.WriteString("\n")
	}
	b.WriteString(r.meta_end)
	b.WriteString("\n")

	b.WriteString(r.rest_start)
	b.WriteString("\n")
	for _, el := range input.Rest {
		if err := htmlutil.RenderElementToBuilder(el, &b); err != nil {
			return "", fmt.Errorf("error rendering rest head el: %w", err)
		}
		b.WriteString("\n")
	}
	b.WriteString(r.rest_end)

	return template.HTML(b.String()), nil
}

func (r *Renderer) Prepare(
	els []*htmlutil.Element,
) *Prepared {
	r.InitDedupeRules(nil)

	deduped := r.dedup_head_els(els)

	out := &Prepared{
		Meta: make([]*htmlutil.Element, 0, len(deduped)),
		Rest: make([]*htmlutil.Element, 0, len(deduped)),
	}

	for _, el := range deduped {
		safe := htmlutil.EscapeIntoTrusted(el)
		switch {
		case safe.Tag == "title":
			out.Title = &safe
		case safe.Tag == "meta":
			out.Meta = append(out.Meta, &safe)
		default:
			out.Rest = append(out.Rest, &safe)
		}
	}

	return out
}

/////////////////////////////////////////////////////////////////////
/////// DEDUP
/////////////////////////////////////////////////////////////////////

type dedupe_key struct {
	tag      string
	rule_idx int
}

func (r *Renderer) dedup_head_els(els []*htmlutil.Element) []*htmlutil.Element {
	result := make([]*htmlutil.Element, 0, len(els))
	seen_rule := make(map[dedupe_key]int)
	seen_hash := make(map[uint64]int)

	for _, el := range els {
		if el == nil {
			continue
		}

		if rules, ok := r.unique_rules_by_tag[el.Tag]; ok {
			matched := false
			for ri, rule := range rules {
				if matches_rule(el, rule) {
					key := dedupe_key{tag: el.Tag, rule_idx: ri}
					if pos, exists := seen_rule[key]; exists {
						result[pos] = el
					} else {
						seen_rule[key] = len(result)
						result = append(result, el)
					}
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}

		h := hash_element(el)
		if pos, exists := seen_hash[h]; exists {
			result[pos] = el
		} else {
			seen_hash[h] = len(result)
			result = append(result, el)
		}
	}

	return result
}

/////////////////////////////////////////////////////////////////////
/////// HASHING
/////////////////////////////////////////////////////////////////////

var hash_sep = []byte{0}

func hash_element(el *htmlutil.Element) uint64 {
	h := fnv.New64a()

	h.Write([]byte(el.Tag))
	h.Write(hash_sep)

	// Collect and sort attribute keys for deterministic hashing.
	attr_keys := make([]string,
		0,
		len(el.Attributes)+len(el.AttributesKnownSafe),
	)
	for k := range el.Attributes {
		attr_keys = append(attr_keys, k)
	}
	for k := range el.AttributesKnownSafe {
		if _, exists := el.Attributes[k]; !exists {
			attr_keys = append(attr_keys, k)
		}
	}
	sort.Strings(attr_keys)

	for _, k := range attr_keys {
		if v, ok := el.Attributes[k]; ok {
			h.Write([]byte("a:"))
			h.Write([]byte(k))
			h.Write([]byte("="))
			h.Write([]byte(v))
			h.Write(hash_sep)
		}
		if v, ok := el.AttributesKnownSafe[k]; ok {
			h.Write([]byte("t:"))
			h.Write([]byte(k))
			h.Write([]byte("="))
			h.Write([]byte(v))
			h.Write(hash_sep)
		}
	}

	bool_attrs := slices.Clone(el.BooleanAttributes)
	sort.Strings(bool_attrs)
	for _, attr := range bool_attrs {
		h.Write([]byte("b:"))
		h.Write([]byte(attr))
		h.Write(hash_sep)
	}

	if len(el.DangerousInnerHTML) > 0 {
		h.Write([]byte("i:"))
		h.Write([]byte(el.DangerousInnerHTML))
		h.Write(hash_sep)
	}
	if len(el.TextContent) > 0 {
		h.Write([]byte("c:"))
		h.Write([]byte(el.TextContent))
		h.Write(hash_sep)
	}

	h.Write([]byte("s:"))
	if el.SelfClosing {
		h.Write([]byte("1"))
	} else {
		h.Write([]byte("0"))
	}

	return h.Sum64()
}

/////////////////////////////////////////////////////////////////////
/////// RULE MATCHING
/////////////////////////////////////////////////////////////////////

type rule_attrs struct {
	attrs   map[string]string
	trusted map[string]string
	boolean []string
}

func extract_rule_attrs(rule *htmlutil.Element) *rule_attrs {
	return &rule_attrs{
		attrs:   maps.Clone(rule.Attributes),
		trusted: maps.Clone(rule.AttributesKnownSafe),
		boolean: slices.Clone(rule.BooleanAttributes),
	}
}

func matches_rule(el *htmlutil.Element, rule *rule_attrs) bool {
	check := func(key, expected string) bool {
		if expected == AttrAnyValue {
			_, inAttrs := el.Attributes[key]
			_, inTrusted := el.AttributesKnownSafe[key]
			return inAttrs || inTrusted
		}
		if v, ok := el.Attributes[key]; ok && v == expected {
			return true
		}
		if v, ok := el.AttributesKnownSafe[key]; ok && v == expected {
			return true
		}
		return false
	}

	for k, v := range rule.attrs {
		if !check(k, v) {
			return false
		}
	}
	for k, v := range rule.trusted {
		if !check(k, v) {
			return false
		}
	}
	for _, b := range rule.boolean {
		if !slices.Contains(el.BooleanAttributes, b) {
			return false
		}
	}
	return true
}

/////////////////////////////////////////////////////////////////////
/////// HIGH-LEVEL DSL
/////////////////////////////////////////////////////////////////////

type element_def interface{ Type() element_def_kind }

type element_def_kind string

const (
	type_tag        element_def_kind = "tag"
	type_attr       element_def_kind = "attribute"
	type_bool_attr  element_def_kind = "boolean-attribute"
	type_inner      element_def_kind = "inner-html"
	type_text       element_def_kind = "text-content"
	type_self_close element_def_kind = "self-closing"
)

type Tag string
type Attr struct {
	attr       [2]string
	known_safe bool
}
type BooleanAttribute string
type InnerHTML string
type TextContent string
type SelfClosing bool

func (a *Attr) KnownSafe() *Attr { a.known_safe = true; return a }

func (Tag) Type() element_def_kind              { return type_tag }
func (Attr) Type() element_def_kind             { return type_attr }
func (BooleanAttribute) Type() element_def_kind { return type_bool_attr }
func (InnerHTML) Type() element_def_kind        { return type_inner }
func (TextContent) Type() element_def_kind      { return type_text }
func (SelfClosing) Type() element_def_kind      { return type_self_close }

/////////////////////////////////////////////////////////////////////
/////// BUILDER
/////////////////////////////////////////////////////////////////////

// Builder is an ordered collection of HTML head elements.
// It is NOT safe for concurrent use; callers must synchronize externally.
type Builder struct {
	els []*htmlutil.Element
}

// FromElements wraps existing elements into a Builder.
func FromElements(els []*htmlutil.Element) *Builder {
	return &Builder{els: els}
}

// NewBuilder creates an empty Builder.
func NewBuilder() *Builder {
	return &Builder{els: make([]*htmlutil.Element, 0)}
}

// Add appends a new element built from the provided definitions.
// Panics if no Tag is provided.
func (b *Builder) Add(defs ...element_def) {
	el := new(htmlutil.Element)
	el.Attributes = make(map[string]string)
	el.AttributesKnownSafe = make(map[string]string)
	el.BooleanAttributes = make([]string, 0)

	for _, def := range defs {
		switch def.Type() {
		case type_tag:
			el.Tag = string(def.(Tag))
		case type_attr:
			a := def.(*Attr)
			if a.known_safe {
				el.AttributesKnownSafe[a.attr[0]] = a.attr[1]
			} else {
				el.Attributes[a.attr[0]] = a.attr[1]
			}
		case type_bool_attr:
			el.BooleanAttributes = append(el.BooleanAttributes,
				string(def.(BooleanAttribute)),
			)
		case type_inner:
			el.DangerousInnerHTML = string(def.(InnerHTML))
		case type_text:
			el.TextContent = string(def.(TextContent))
		case type_self_close:
			el.SelfClosing = bool(def.(SelfClosing))
		default:
			panic(fmt.Sprintf("unknown type %T", def))
		}
	}

	if el.Tag == "" {
		panic("head element added without a Tag")
	}

	b.els = append(b.els, el)
}

// Append appends all elements from other into b.
func (b *Builder) Append(other *Builder) {
	if other == nil {
		return
	}
	b.els = append(b.els, other.els...)
}

// Elements returns the underlying element slice.
func (b *Builder) Elements() []*htmlutil.Element {
	return b.els
}

// SelfClosing returns a SelfClosing(true) definition.
func (b *Builder) SelfClosing() SelfClosing { return SelfClosing(true) }

// DangerousInnerHTML returns an InnerHTML definition.
func (b *Builder) DangerousInnerHTML(content string) InnerHTML {
	return InnerHTML(content)
}

// TextContent returns a TextContent definition.
func (b *Builder) TextContent(content string) TextContent {
	return TextContent(content)
}

/////// Tag helpers

func (b *Builder) AttrExists(name string) *Attr {
	return b.Attr(name, AttrAnyValue)
}

func (b *Builder) Title(title string) {
	b.Add(Tag("title"), TextContent(title))
}

func (b *Builder) Description(desc string) {
	b.Meta(b.Name("description"), b.Content(desc))
}

func (b *Builder) Meta(defs ...element_def) {
	b.Add(append(defs, Tag("meta"))...)
}

func (b *Builder) Link(defs ...element_def) {
	b.Add(append(defs, Tag("link"))...)
}

func (b *Builder) Script(defs ...element_def) {
	b.Add(append(defs, Tag("script"))...)
}

func (b *Builder) Style(defs ...element_def) {
	b.Add(append(defs, Tag("style"))...)
}

/////// Attribute helpers

func (b *Builder) Attr(name, value string) *Attr {
	return &Attr{attr: [2]string{name, value}}
}

func (b *Builder) BoolAttr(name string) BooleanAttribute {
	return BooleanAttribute(name)
}

func (b *Builder) Property(prop string) *Attr {
	return b.Attr("property", prop)
}

func (b *Builder) Name(name string) *Attr {
	return b.Attr("name", name)
}

func (b *Builder) Content(content string) *Attr {
	return b.Attr("content", content)
}

func (b *Builder) Rel(rel string) *Attr {
	return b.Attr("rel", rel)
}

func (b *Builder) Href(href string) *Attr {
	return b.Attr("href", href)
}

func (b *Builder) Src(src string) *Attr {
	return b.Attr("src", src)
}

func (b *Builder) Type(t string) *Attr {
	return b.Attr("type", t)
}

func (b *Builder) Charset(charset string) *Attr {
	return b.Attr("charset", charset)
}

func (b *Builder) As(as string) *Attr {
	return b.Attr("as", as)
}

func (b *Builder) CrossOrigin(co string) *Attr {
	return b.Attr("crossorigin", co)
}

/////// Common combinations

func (b *Builder) MetaPropertyContent(prop, content string) {
	b.Meta(b.Property(prop), b.Content(content))
}
func (b *Builder) MetaNameContent(name, content string) {
	b.Meta(b.Name(name), b.Content(content))
}
func (b *Builder) MetaCharset(charset string) {
	b.Meta(b.Charset(charset))
}
