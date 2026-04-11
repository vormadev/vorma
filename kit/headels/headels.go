package headels

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
/////// INSTANCE (RENDERING AND DEDUP)
/////////////////////////////////////////////////////////////////////

type Instance struct {
	meta_start          string
	meta_end            string
	rest_start          string
	rest_end            string
	once                sync.Once
	unique_rules_by_tag map[string][]*rule_attrs
}

func NewInstance(namespace string) *Instance {
	return &Instance{
		meta_start: comment(namespace, "meta-start"),
		meta_end:   comment(namespace, "meta-end"),
		rest_start: comment(namespace, "rest-start"),
		rest_end:   comment(namespace, "rest-end"),
	}
}

func comment(namespace, val string) string {
	return fmt.Sprintf("<!-- %s-%s -->", namespace, val)
}

func (inst *Instance) InitUniqueRules(e *HeadEls) {
	inst.once.Do(func() {
		inst.unique_rules_by_tag = make(map[string][]*rule_attrs)

		// Build default rules without mutating the caller's HeadEls.
		defaults := New()
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
		sources = append(sources, defaults.Collect()...)
		if e != nil {
			sources = append(sources, e.Collect()...)
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
				inst.unique_rules_by_tag[rule.Tag] = append(
					inst.unique_rules_by_tag[rule.Tag],
					attrs,
				)
			}
		}
	})
}

// SortedAndPreEscapedHeadEls holds classified head elements ready for rendering.
type SortedAndPreEscapedHeadEls struct {
	Title *htmlutil.Element
	Meta  []*htmlutil.Element
	Rest  []*htmlutil.Element
}

const rough_avg_el_len = 80
const panic_render_nil_input = "headels: Render input cannot be nil"
const panic_render_empty_input = "headels: Render input cannot be empty"

func (inst *Instance) Render(input *SortedAndPreEscapedHeadEls) (template.HTML, error) {
	inst.InitUniqueRules(nil)
	if input == nil {
		panic(panic_render_nil_input)
	}
	if input.Title == nil && len(input.Meta) == 0 && len(input.Rest) == 0 {
		panic(panic_render_empty_input)
	}

	meta_size := len(inst.meta_start) + len(inst.meta_end)
	rest_size := len(inst.rest_start) + len(inst.rest_end)
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

	b.WriteString(inst.meta_start)
	b.WriteString("\n")
	for _, el := range input.Meta {
		if err := htmlutil.RenderElementToBuilder(el, &b); err != nil {
			return "", fmt.Errorf("error rendering meta head el: %w", err)
		}
		b.WriteString("\n")
	}
	b.WriteString(inst.meta_end)
	b.WriteString("\n")

	b.WriteString(inst.rest_start)
	b.WriteString("\n")
	for _, el := range input.Rest {
		if err := htmlutil.RenderElementToBuilder(el, &b); err != nil {
			return "", fmt.Errorf("error rendering rest head el: %w", err)
		}
		b.WriteString("\n")
	}
	b.WriteString(inst.rest_end)

	return template.HTML(b.String()), nil
}

func (inst *Instance) ToSortedAndPreEscapedHeadEls(
	els []*htmlutil.Element,
) *SortedAndPreEscapedHeadEls {
	inst.InitUniqueRules(nil)

	deduped := inst.dedup_head_els(els)

	out := &SortedAndPreEscapedHeadEls{
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

func (inst *Instance) dedup_head_els(els []*htmlutil.Element) []*htmlutil.Element {
	result := make([]*htmlutil.Element, 0, len(els))
	seen_rule := make(map[dedupe_key]int)
	seen_hash := make(map[uint64]int)

	for _, el := range els {
		if el == nil {
			continue
		}

		if rules, ok := inst.unique_rules_by_tag[el.Tag]; ok {
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

type type_interface interface{ Type() htmlutil_type }

type htmlutil_type string

const (
	type_tag        htmlutil_type = "tag"
	type_attr       htmlutil_type = "attribute"
	type_bool_attr  htmlutil_type = "boolean-attribute"
	type_inner      htmlutil_type = "inner-html"
	type_text       htmlutil_type = "text-content"
	type_self_close htmlutil_type = "self-closing"
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

func (Tag) Type() htmlutil_type              { return type_tag }
func (Attr) Type() htmlutil_type             { return type_attr }
func (BooleanAttribute) Type() htmlutil_type { return type_bool_attr }
func (InnerHTML) Type() htmlutil_type        { return type_inner }
func (TextContent) Type() htmlutil_type      { return type_text }
func (SelfClosing) Type() htmlutil_type      { return type_self_close }

/////////////////////////////////////////////////////////////////////
/////// HEAD ELS COLLECTION
/////////////////////////////////////////////////////////////////////

// HeadEls is an ordered collection of HTML head elements.
// It is NOT safe for concurrent use; callers must synchronize externally.
type HeadEls struct {
	els []*htmlutil.Element
}

// FromRaw wraps existing elements into a HeadEls.
func FromRaw(els []*htmlutil.Element) *HeadEls {
	return &HeadEls{els: els}
}

// New creates an empty HeadEls.
func New() *HeadEls {
	return &HeadEls{els: make([]*htmlutil.Element, 0)}
}

// Add appends a new element built from the provided definitions.
// Panics if no Tag is provided.
func (h *HeadEls) Add(defs ...type_interface) {
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

	h.els = append(h.els, el)
}

// AddElements appends all elements from other into h.
func (h *HeadEls) AddElements(other *HeadEls) {
	if other == nil {
		return
	}
	h.els = append(h.els, other.els...)
}

// Collect returns the underlying element slice.
func (h *HeadEls) Collect() []*htmlutil.Element {
	return h.els
}

// SelfClosing returns a SelfClosing(true) definition.
func (h *HeadEls) SelfClosing() SelfClosing { return SelfClosing(true) }

// DangerousInnerHTML returns an InnerHTML definition.
func (h *HeadEls) DangerousInnerHTML(content string) InnerHTML {
	return InnerHTML(content)
}

// TextContent returns a TextContent definition.
func (h *HeadEls) TextContent(content string) TextContent {
	return TextContent(content)
}

/////// Tag helpers

func (h *HeadEls) AttrExists(name string) *Attr {
	return h.Attr(name, AttrAnyValue)
}

func (h *HeadEls) Title(title string) {
	h.Add(Tag("title"), TextContent(title))
}

func (h *HeadEls) Description(desc string) {
	h.Meta(h.Name("description"), h.Content(desc))
}

func (h *HeadEls) Meta(defs ...type_interface) {
	h.Add(append(defs, Tag("meta"))...)
}

func (h *HeadEls) Link(defs ...type_interface) {
	h.Add(append(defs, Tag("link"))...)
}

func (h *HeadEls) Script(defs ...type_interface) {
	h.Add(append(defs, Tag("script"))...)
}

func (h *HeadEls) Style(defs ...type_interface) {
	h.Add(append(defs, Tag("style"))...)
}

/////// Attribute helpers

func (h *HeadEls) Attr(name, value string) *Attr {
	return &Attr{attr: [2]string{name, value}}
}

func (h *HeadEls) BoolAttr(name string) BooleanAttribute {
	return BooleanAttribute(name)
}

func (h *HeadEls) Property(prop string) *Attr {
	return h.Attr("property", prop)
}

func (h *HeadEls) Name(name string) *Attr {
	return h.Attr("name", name)
}

func (h *HeadEls) Content(content string) *Attr {
	return h.Attr("content", content)
}

func (h *HeadEls) Rel(rel string) *Attr {
	return h.Attr("rel", rel)
}

func (h *HeadEls) Href(href string) *Attr {
	return h.Attr("href", href)
}

func (h *HeadEls) Src(src string) *Attr {
	return h.Attr("src", src)
}

func (h *HeadEls) Type(t string) *Attr {
	return h.Attr("type", t)
}

func (h *HeadEls) Charset(charset string) *Attr {
	return h.Attr("charset", charset)
}

func (h *HeadEls) As(as string) *Attr {
	return h.Attr("as", as)
}

func (h *HeadEls) CrossOrigin(co string) *Attr {
	return h.Attr("crossorigin", co)
}

/////// Common combinations

func (h *HeadEls) MetaPropertyContent(prop, content string) {
	h.Meta(h.Property(prop), h.Content(content))
}
func (h *HeadEls) MetaNameContent(name, content string) {
	h.Meta(h.Name(name), h.Content(content))
}
func (h *HeadEls) MetaCharset(charset string) {
	h.Meta(h.Charset(charset))
}
