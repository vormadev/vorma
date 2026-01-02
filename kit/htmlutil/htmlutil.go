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

type Element struct {
	Tag                 string            `json:"tag,omitempty"`
	Attributes          map[string]string `json:"attributes,omitempty"`
	AttributesKnownSafe map[string]string `json:"attributesKnownSafe,omitempty"`
	BooleanAttributes   []string          `json:"booleanAttributes,omitempty"`
	TextContent         string            `json:"textContent,omitempty"`
	DangerousInnerHTML  string            `json:"dangerousInnerHTML,omitempty"`
	SelfClosing         bool              `json:"-"`
}

var (
	// see https://html.spec.whatwg.org/multipage/syntax.html#void-elements
	// If you need something to self-close something that isn't on this list, set the SelfClosing field to true
	selfClosingTags = []string{
		"area", "base", "br", "col", "embed", "hr", "img",
		"input", "link", "meta", "source", "track", "wbr",
	}
)

func AddSha256HashInline(el *Element) (string, error) {
	if el.AttributesKnownSafe == nil {
		el.AttributesKnownSafe = make(map[string]string)
	}
	sha256Hash := cryptoutil.Sha256Hash([]byte(el.DangerousInnerHTML))
	sha256HashBase64 := bytesutil.ToBase64(sha256Hash[:])
	return sha256HashBase64, nil
}

func AddSha256HashExternal(el *Element, externalSha256Hash string) (string, error) {
	if el.AttributesKnownSafe == nil {
		el.AttributesKnownSafe = make(map[string]string)
	}
	if externalSha256Hash == "" {
		return "", fmt.Errorf("no sha256 hash provided for external resource")
	}
	el.AttributesKnownSafe["integrity"] = "sha256-" + externalSha256Hash
	return externalSha256Hash, nil
}

func AddNonce(el *Element, len uint8) (string, error) {
	if el.AttributesKnownSafe == nil {
		el.AttributesKnownSafe = make(map[string]string)
	}
	if len == 0 {
		len = 16
	}
	nonce, err := id.New(len)
	if err != nil {
		return "", fmt.Errorf("could not generate nonce: %w", err)
	}
	el.AttributesKnownSafe["nonce"] = nonce
	return nonce, nil
}

func RenderElement(el *Element) (template.HTML, error) {
	var htmlBuilder strings.Builder

	err := RenderElementToBuilder(el, &htmlBuilder)
	if err != nil {
		return "", fmt.Errorf("could not render element: %w", err)
	}

	return template.HTML(htmlBuilder.String()), nil
}

func RenderElementToBuilder(el *Element, htmlBuilder *strings.Builder) error {
	escapedTag := template.HTMLEscapeString(el.Tag)
	if escapedTag == "" {
		return fmt.Errorf("element has no tag")
	}

	isSelfClosing := slices.Contains(selfClosingTags, escapedTag) || el.SelfClosing

	escapedAttributes := combineIntoDangerousAttributes(el)
	hasAttributes := len(escapedAttributes) > 0

	htmlBuilder.WriteString("<")
	htmlBuilder.WriteString(escapedTag)

	if hasAttributes {
		escapedKeys := slices.Collect(maps.Keys(escapedAttributes))
		sort.Strings(escapedKeys)
		for _, escapedKey := range escapedKeys {
			writeAttribute(htmlBuilder, escapedKey, escapedAttributes[escapedKey])
		}
	}

	for _, booleanAttribute := range el.BooleanAttributes {
		htmlBuilder.WriteString(" ")
		htmlBuilder.WriteString(template.HTMLEscapeString(booleanAttribute))
	}

	if isSelfClosing {
		htmlBuilder.WriteString(" />")
	} else {
		htmlBuilder.WriteString(">")

		htmlBuilder.WriteString(string(combineIntoDangerousInnerHTML(el)))

		htmlBuilder.WriteString("</")
		htmlBuilder.WriteString(escapedTag)
		htmlBuilder.WriteString(">")
	}

	return nil
}

func writeAttribute(htmlBuilder *strings.Builder, key, value string) {
	htmlBuilder.WriteString(" ")
	htmlBuilder.WriteString(key)
	htmlBuilder.WriteString(`="`)
	htmlBuilder.WriteString(value)
	htmlBuilder.WriteString(`"`)
}

func combineIntoDangerousAttributes(el *Element) map[string]string {
	attributes := make(map[string]string, len(el.Attributes)+len(el.AttributesKnownSafe))
	for k, v := range el.Attributes {
		escapedKey := template.HTMLEscapeString(k)
		attributes[escapedKey] = template.HTMLEscapeString(v)
	}
	for k, v := range el.AttributesKnownSafe {
		escapedKey := template.HTMLEscapeString(k)
		attributes[escapedKey] = v
	}
	return attributes
}

func combineIntoDangerousInnerHTML(el *Element) string {
	if el.DangerousInnerHTML != "" {
		return el.DangerousInnerHTML
	}
	if el.TextContent != "" {
		return template.HTMLEscapeString(el.TextContent)
	}
	return ""
}

func EscapeIntoTrusted(el *Element) Element {
	return Element{
		Tag:                 el.Tag,
		Attributes:          nil,
		AttributesKnownSafe: combineIntoDangerousAttributes(el),
		BooleanAttributes:   el.BooleanAttributes,
		TextContent:         "",
		DangerousInnerHTML:  combineIntoDangerousInnerHTML(el),
		SelfClosing:         el.SelfClosing,
	}
}

func RenderModuleScriptToBuilder(src string, htmlBuilder *strings.Builder) error {
	return RenderElementToBuilder(&Element{
		Tag:                 "script",
		AttributesKnownSafe: map[string]string{"type": "module", "src": src},
	}, htmlBuilder)
}
