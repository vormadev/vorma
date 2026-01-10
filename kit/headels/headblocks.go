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

type Instance struct {
	metaStart        string
	metaEnd          string
	restStart        string
	restEnd          string
	once             sync.Once
	uniqueRulesByTag map[string][]*ruleAttrs
}

const prefix = `<!-- data-`

func NewInstance(dataAttribute string) *Instance {
	return &Instance{
		metaStart: prefix + dataAttribute + suffix("meta-start"),
		metaEnd:   prefix + dataAttribute + suffix("meta-end"),
		restStart: prefix + dataAttribute + suffix("rest-start"),
		restEnd:   prefix + dataAttribute + suffix("rest-end"),
	}
}

func suffix(val string) string {
	return fmt.Sprintf(`="%s" -->`, val)
}

func (inst *Instance) InitUniqueRules(e *HeadEls) {
	inst.once.Do(func() {
		inst.uniqueRulesByTag = make(map[string][]*ruleAttrs)

		// Build default rules internally without mutating input
		defaults := New()
		defaults.Add(Tag("title"))
		defaults.Meta(defaults.Name("description"))

		var sources []*htmlutil.Element
		sources = append(sources, defaults.Collect()...)
		if e != nil {
			sources = append(sources, e.Collect()...)
		}

		seenHashes := make(map[string]map[uint64]bool)
		for _, rule := range sources {
			hash := hashElement(rule)
			if _, exists := seenHashes[rule.Tag]; !exists {
				seenHashes[rule.Tag] = make(map[uint64]bool)
			}
			if !seenHashes[rule.Tag][hash] {
				seenHashes[rule.Tag][hash] = true
				attrs := extractRuleAttrs(rule)
				inst.uniqueRulesByTag[rule.Tag] = append(inst.uniqueRulesByTag[rule.Tag], attrs)
			}
		}
	})
}

type SortedAndPreEscapedHeadEls struct {
	Title *htmlutil.Element
	Meta  []*htmlutil.Element
	Rest  []*htmlutil.Element
}

const roughSafeAvgElLen = 80

func (inst *Instance) Render(input *SortedAndPreEscapedHeadEls) (template.HTML, error) {
	inst.InitUniqueRules(nil)

	metaSize := len(inst.metaStart) + len(inst.metaEnd)
	restSize := len(inst.restStart) + len(inst.restEnd)
	estimatedSize := metaSize + restSize + 4 // Newlines
	if input.Title != nil {
		estimatedSize += roughSafeAvgElLen
	}
	estimatedSize += len(input.Meta) * roughSafeAvgElLen
	estimatedSize += len(input.Rest) * roughSafeAvgElLen

	var b strings.Builder
	b.Grow(estimatedSize)

	if input.Title != nil {
		if err := htmlutil.RenderElementToBuilder(input.Title, &b); err != nil {
			return "", fmt.Errorf("error rendering title: %w", err)
		}
		b.WriteString("\n")
	}

	b.WriteString(inst.metaStart)
	b.WriteString("\n")
	for _, el := range input.Meta {
		if err := htmlutil.RenderElementToBuilder(el, &b); err != nil {
			return "", fmt.Errorf("error rendering meta head el: %w", err)
		}
		b.WriteString("\n")
	}
	b.WriteString(inst.metaEnd)
	b.WriteString("\n")

	b.WriteString(inst.restStart)
	b.WriteString("\n")
	for _, el := range input.Rest {
		if err := htmlutil.RenderElementToBuilder(el, &b); err != nil {
			return "", fmt.Errorf("error rendering rest head el: %w", err)
		}
		b.WriteString("\n")
	}
	b.WriteString(inst.restEnd)

	return template.HTML(b.String()), nil
}

func (inst *Instance) ToSortedAndPreEscapedHeadEls(els []*htmlutil.Element) *SortedAndPreEscapedHeadEls {
	inst.InitUniqueRules(nil)

	deduped := inst.dedupeHeadEls(els)

	headEls := &SortedAndPreEscapedHeadEls{
		Meta: make([]*htmlutil.Element, 0, len(deduped)),
		Rest: make([]*htmlutil.Element, 0, len(deduped)),
	}

	for _, el := range deduped {
		safeEl := htmlutil.EscapeIntoTrusted(el)
		switch {
		case isTitle(&safeEl):
			headEls.Title = &safeEl
		case isMeta(&safeEl):
			headEls.Meta = append(headEls.Meta, &safeEl)
		default:
			headEls.Rest = append(headEls.Rest, &safeEl)
		}
	}

	return headEls
}

var hashSeparator = []byte{0}

func hashElement(el *htmlutil.Element) uint64 {
	h := fnv.New64a()

	h.Write([]byte(el.Tag))
	h.Write(hashSeparator)

	attrKeys := make([]string, 0, len(el.Attributes)+len(el.AttributesKnownSafe))
	for k := range el.Attributes {
		attrKeys = append(attrKeys, k)
	}
	for k := range el.AttributesKnownSafe {
		if _, exists := el.Attributes[k]; !exists {
			attrKeys = append(attrKeys, k)
		}
	}
	sort.Strings(attrKeys)

	for _, k := range attrKeys {
		if v, ok := el.Attributes[k]; ok {
			h.Write([]byte("a:"))
			h.Write([]byte(k))
			h.Write([]byte("="))
			h.Write([]byte(v))
			h.Write(hashSeparator)
		}
		if v, ok := el.AttributesKnownSafe[k]; ok {
			h.Write([]byte("t:"))
			h.Write([]byte(k))
			h.Write([]byte("="))
			h.Write([]byte(v))
			h.Write(hashSeparator)
		}
	}

	boolAttrs := slices.Clone(el.BooleanAttributes)
	sort.Strings(boolAttrs)
	for _, attr := range boolAttrs {
		h.Write([]byte("b:"))
		h.Write([]byte(attr))
		h.Write(hashSeparator)
	}

	if len(el.DangerousInnerHTML) > 0 {
		h.Write([]byte("i:"))
		h.Write([]byte(el.DangerousInnerHTML))
		h.Write(hashSeparator)
	}
	if len(el.TextContent) > 0 {
		h.Write([]byte("c:"))
		h.Write([]byte(el.TextContent))
		h.Write(hashSeparator)
	}

	h.Write([]byte("s:"))
	if el.SelfClosing {
		h.Write([]byte("1"))
	} else {
		h.Write([]byte("0"))
	}

	return h.Sum64()
}

func (inst *Instance) dedupeHeadEls(els []*htmlutil.Element) []*htmlutil.Element {
	result := make([]*htmlutil.Element, 0, len(els))

	seenRule := make(map[string]int)
	seenHash := make(map[uint64]int)

	for _, el := range els {
		if el == nil {
			continue
		}

		if rules, hasRules := inst.uniqueRulesByTag[el.Tag]; hasRules {
			matchedRule := false
			for ruleIdx, rule := range rules {
				if matchesRule(el, rule) {
					ruleKey := fmt.Sprintf("rule:%s:%d", el.Tag, ruleIdx)

					if pos, exists := seenRule[ruleKey]; exists {
						result[pos] = el
					} else {
						seenRule[ruleKey] = len(result)
						result = append(result, el)
					}
					matchedRule = true
					break
				}
			}
			if matchedRule {
				continue
			}
		}

		contentHash := hashElement(el)

		if pos, exists := seenHash[contentHash]; exists {
			result[pos] = el
		} else {
			seenHash[contentHash] = len(result)
			result = append(result, el)
		}
	}

	return result
}

type ruleAttrs struct {
	attrs   map[string]string
	trusted map[string]string
	boolean []string
}

func extractRuleAttrs(rule *htmlutil.Element) *ruleAttrs {
	return &ruleAttrs{
		attrs:   maps.Clone(rule.Attributes),
		trusted: maps.Clone(rule.AttributesKnownSafe),
		boolean: slices.Clone(rule.BooleanAttributes),
	}
}

func matchesRule(el *htmlutil.Element, rule *ruleAttrs) bool {
	checkKeyValue := func(key, expectedValue string) bool {
		if actualValue, ok := el.Attributes[key]; ok && actualValue == expectedValue {
			return true
		}
		if actualValue, ok := el.AttributesKnownSafe[key]; ok && actualValue == expectedValue {
			return true
		}
		return false
	}

	for k, v := range rule.attrs {
		if !checkKeyValue(k, v) {
			return false
		}
	}

	for k, v := range rule.trusted {
		if !checkKeyValue(k, v) {
			return false
		}
	}

	for _, ruleAttr := range rule.boolean {
		if !slices.Contains(el.BooleanAttributes, ruleAttr) {
			return false
		}
	}

	return true
}

func isTitle(el *htmlutil.Element) bool {
	return el.Tag == "title"
}

func isMeta(el *htmlutil.Element) bool {
	return el.Tag == "meta"
}

/////////////////////////////////////////////////////////////////////
/////// HIGH LEVEL
/////////////////////////////////////////////////////////////////////

type typeInterface interface{ GetType() htmlutilType }

type htmlutilType string

const (
	typeTag              htmlutilType = "tag"
	typeAttribute        htmlutilType = "attribute"
	typeBooleanAttribute htmlutilType = "boolean-attribute"
	typeInnerHTML        htmlutilType = "inner-html"
	typeTextContent      htmlutilType = "text-content"
	typeSelfClosing      htmlutilType = "self-closing"
)

type Tag string
type Attr struct {
	attr      [2]string
	knownSafe bool
}
type BooleanAttribute string
type InnerHTML string
type TextContent string
type SelfClosing bool

func (a *Attr) KnownSafe() *Attr {
	a.knownSafe = true
	return a
}

func (Tag) GetType() htmlutilType              { return typeTag }
func (Attr) GetType() htmlutilType             { return typeAttribute }
func (BooleanAttribute) GetType() htmlutilType { return typeBooleanAttribute }
func (InnerHTML) GetType() htmlutilType        { return typeInnerHTML }
func (TextContent) GetType() htmlutilType      { return typeTextContent }
func (SelfClosing) GetType() htmlutilType      { return typeSelfClosing }

// HeadEls is a collection of HTML head elements.
// It is safe for concurrent use.
type HeadEls struct {
	mu  sync.Mutex
	els []*htmlutil.Element
}

func FromRaw(els []*htmlutil.Element) *HeadEls {
	return &HeadEls{els: els}
}

func New() *HeadEls {
	return &HeadEls{els: make([]*htmlutil.Element, 0)}
}

// Add appends a new element to the collection.
// Panics if no Tag is provided among the definitions.
func (h *HeadEls) Add(defs ...typeInterface) {
	el := new(htmlutil.Element)

	el.Attributes = make(map[string]string)
	el.AttributesKnownSafe = make(map[string]string)
	el.BooleanAttributes = make([]string, 0)

	for _, def := range defs {
		switch def.GetType() {
		case typeTag:
			el.Tag = string(def.(Tag))
		case typeAttribute:
			attr := def.(*Attr)
			if attr.knownSafe {
				el.AttributesKnownSafe[attr.attr[0]] = attr.attr[1]
			} else {
				el.Attributes[attr.attr[0]] = attr.attr[1]
			}
		case typeBooleanAttribute:
			attr := def.(BooleanAttribute)
			el.BooleanAttributes = append(el.BooleanAttributes, string(attr))
		case typeInnerHTML:
			el.DangerousInnerHTML = string(def.(InnerHTML))
		case typeTextContent:
			el.TextContent = string(def.(TextContent))
		case typeSelfClosing:
			el.SelfClosing = bool(def.(SelfClosing))
		default:
			panic(fmt.Sprintf("unknown type %T", def))
		}
	}

	if el.Tag == "" {
		panic("head element added without a Tag")
	}

	h.mu.Lock()
	h.els = append(h.els, el)
	h.mu.Unlock()
}

func (h *HeadEls) AddElements(other *HeadEls) {
	otherEls := other.Collect()
	h.mu.Lock()
	h.els = append(h.els, otherEls...)
	h.mu.Unlock()
}

func (h *HeadEls) Collect() []*htmlutil.Element {
	h.mu.Lock()
	defer h.mu.Unlock()
	return slices.Clone(h.els)
}

func (h *HeadEls) SelfClosing() SelfClosing {
	return SelfClosing(true)
}
func (h *HeadEls) DangerousInnerHTML(content string) InnerHTML {
	return InnerHTML(content)
}
func (h *HeadEls) TextContent(content string) TextContent {
	return TextContent(content)
}

/////// Tag helpers

func (h *HeadEls) Title(title string) {
	h.Add(Tag("title"), TextContent(title))
}
func (h *HeadEls) Description(description string) {
	h.Meta(h.Name("description"), h.Content(description))
}
func (h *HeadEls) Meta(defs ...typeInterface) {
	h.Add(append(defs, Tag("meta"))...)
}
func (h *HeadEls) Link(defs ...typeInterface) {
	h.Add(append(defs, Tag("link"))...)
}
func (h *HeadEls) Script(defs ...typeInterface) {
	h.Add(append(defs, Tag("script"))...)
}
func (h *HeadEls) Style(defs ...typeInterface) {
	h.Add(append(defs, Tag("style"))...)
}

/////// Attribute helpers

func (h *HeadEls) Attr(name, value string) *Attr {
	return &Attr{attr: [2]string{name, value}}
}
func (h *HeadEls) BoolAttr(name string) BooleanAttribute {
	return BooleanAttribute(name)
}
func (h *HeadEls) Property(property string) *Attr {
	return h.Attr("property", property)
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
func (h *HeadEls) Type(type_ string) *Attr {
	return h.Attr("type", type_)
}
func (h *HeadEls) Charset(charset string) *Attr {
	return h.Attr("charset", charset)
}
func (h *HeadEls) As(as string) *Attr {
	return h.Attr("as", as)
}
func (h *HeadEls) CrossOrigin(crossOrigin string) *Attr {
	return h.Attr("crossorigin", crossOrigin)
}

/////// Common combinations

func (h *HeadEls) MetaPropertyContent(property, content string) {
	h.Meta(h.Property(property), h.Content(content))
}
func (h *HeadEls) MetaNameContent(name, content string) {
	h.Meta(h.Name(name), h.Content(content))
}
