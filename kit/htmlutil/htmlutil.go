// Package htmlutil provides safe HTML element construction and rendering.
package htmlutil

import (
	"fmt"
	"html/template"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/vormadev/vorma/kit/bytesutil"
	"github.com/vormadev/vorma/kit/cryptoutil"
	"github.com/vormadev/vorma/kit/id"
)

// Element represents a single HTML element with attributes and content.
type Element struct {
	Tag                 string            `json:"tag,omitempty"`
	Attributes          map[string]string `json:"attributes,omitempty"`
	AttributesKnownSafe map[string]string `json:"attributesKnownSafe,omitempty"`
	BooleanAttributes   []string          `json:"booleanAttributes,omitempty"`
	TextContent         string            `json:"textContent,omitempty"`
	DangerousInnerHTML  string            `json:"dangerousInnerHTML,omitempty"`
	SelfClosing         bool              `json:"-"`
}

// https://html.spec.whatwg.org/multipage/syntax.html#void-elements
// If you need to self-close a tag not on this list, set SelfClosing to true.
var self_closing_tags = []string{
	"area", "base", "br", "col", "embed", "hr", "img",
	"input", "link", "meta", "source", "track", "wbr",
}

/////////////////////////////////////////////////////////////////////
/////// SHA256 / NONCE
/////////////////////////////////////////////////////////////////////

// ComputeContentSha256 returns the base64-encoded SHA-256 hash of the
// element's DangerousInnerHTML content.
func ComputeContentSha256(el *Element) (string, error) {
	if el == nil {
		return "", fmt.Errorf("element cannot be nil")
	}
	hash := cryptoutil.Sha256Hash([]byte(el.DangerousInnerHTML))
	return bytesutil.ToBase64(hash[:]), nil
}

// SetSha256Integrity sets the integrity attribute to the provided external hash.
func SetSha256Integrity(el *Element, external_hash string) (string, error) {
	if el == nil {
		return "", fmt.Errorf("element cannot be nil")
	}
	if external_hash == "" {
		return "", fmt.Errorf("no sha256 hash provided for external resource")
	}
	if el.AttributesKnownSafe == nil {
		el.AttributesKnownSafe = make(map[string]string)
	}
	el.AttributesKnownSafe["integrity"] = "sha256-" + external_hash
	return external_hash, nil
}

// AddNonce generates a random nonce and sets it on the element.
func AddNonce(el *Element, length uint8) (string, error) {
	if el == nil {
		return "", fmt.Errorf("element cannot be nil")
	}
	if el.AttributesKnownSafe == nil {
		el.AttributesKnownSafe = make(map[string]string)
	}
	if length == 0 {
		length = 16
	}
	nonce, err := id.New(length)
	if err != nil {
		return "", fmt.Errorf("could not generate nonce: %w", err)
	}
	el.AttributesKnownSafe["nonce"] = nonce
	return nonce, nil
}

/////////////////////////////////////////////////////////////////////
/////// RENDERING
/////////////////////////////////////////////////////////////////////

// RenderElement renders an element to template.HTML.
func RenderElement(el *Element) (template.HTML, error) {
	var b strings.Builder
	if err := RenderElementToBuilder(el, &b); err != nil {
		return "", fmt.Errorf("could not render element: %w", err)
	}
	return template.HTML(b.String()), nil
}

// RenderElementToBuilder renders an element into the provided builder.
func RenderElementToBuilder(el *Element, b *strings.Builder) error {
	if el == nil {
		return fmt.Errorf("element cannot be nil")
	}
	if b == nil {
		return fmt.Errorf("html builder cannot be nil")
	}

	escaped_tag := template.HTMLEscapeString(el.Tag)
	if escaped_tag == "" {
		return fmt.Errorf("element has no tag")
	}

	is_self_closing := slices.Contains(self_closing_tags, escaped_tag) ||
		el.SelfClosing
	escaped_attrs := combine_attributes(el)

	b.WriteString("<")
	b.WriteString(escaped_tag)

	if len(escaped_attrs) > 0 {
		keys := slices.Collect(maps.Keys(escaped_attrs))
		sort.Strings(keys)
		for _, key := range keys {
			write_attribute(b, key, escaped_attrs[key])
		}
	}

	for _, bool_attr := range el.BooleanAttributes {
		b.WriteString(" ")
		b.WriteString(template.HTMLEscapeString(bool_attr))
	}

	if is_self_closing {
		b.WriteString(" />")
	} else {
		b.WriteString(">")
		b.WriteString(combine_inner_html(el))
		b.WriteString("</")
		b.WriteString(escaped_tag)
		b.WriteString(">")
	}

	return nil
}

// RenderModuleScriptToBuilder renders a <script type="module"> tag.
func RenderModuleScriptToBuilder(src string, b *strings.Builder) error {
	return RenderElementToBuilder(&Element{
		Tag:                 "script",
		AttributesKnownSafe: map[string]string{"type": "module", "src": src},
	}, b)
}

/////////////////////////////////////////////////////////////////////
/////// ESCAPING
/////////////////////////////////////////////////////////////////////

// EscapeIntoTrusted returns a copy of el with all attributes escaped and
// merged into AttributesKnownSafe, and TextContent escaped into
// DangerousInnerHTML. The result is safe for rendering without further escaping.
func EscapeIntoTrusted(el *Element) Element {
	return Element{
		Tag:                 el.Tag,
		Attributes:          nil,
		AttributesKnownSafe: combine_attributes(el),
		BooleanAttributes:   slices.Clone(el.BooleanAttributes),
		TextContent:         "",
		DangerousInnerHTML:  combine_inner_html(el),
		SelfClosing:         el.SelfClosing,
	}
}

/////////////////////////////////////////////////////////////////////
/////// PRIVATE
/////////////////////////////////////////////////////////////////////

func write_attribute(b *strings.Builder, key, value string) {
	b.WriteString(" ")
	b.WriteString(key)
	b.WriteString(`="`)
	b.WriteString(value)
	b.WriteString(`"`)
}

// combine_attributes merges Attributes (escaped) and AttributesKnownSafe
// (unescaped values) into a single map with escaped keys.
func combine_attributes(el *Element) map[string]string {
	out := make(
		map[string]string,
		len(el.Attributes)+len(el.AttributesKnownSafe),
	)
	for k, v := range el.Attributes {
		out[template.HTMLEscapeString(k)] = template.HTMLEscapeString(v)
	}
	for k, v := range el.AttributesKnownSafe {
		out[template.HTMLEscapeString(k)] = v
	}
	return out
}

// combine_inner_html returns DangerousInnerHTML if set, otherwise
// HTML-escaped TextContent.
func combine_inner_html(el *Element) string {
	if el.DangerousInnerHTML != "" {
		return el.DangerousInnerHTML
	}
	if el.TextContent != "" {
		return template.HTMLEscapeString(el.TextContent)
	}
	return ""
}
