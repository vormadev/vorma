package htmlutil

import (
	"strings"
	"testing"

	"golang.org/x/net/html"
)

func parse_html(input string) (*html.Node, error) {
	return html.Parse(strings.NewReader(input))
}

func compare_nodes(n1, n2 *html.Node) bool {
	if n1.Type != n2.Type || n1.Data != n2.Data {
		return false
	}
	if len(n1.Attr) != len(n2.Attr) {
		return false
	}
	attr_map := make(map[string]string)
	for _, a := range n1.Attr {
		attr_map[a.Key] = a.Val
	}
	for _, a := range n2.Attr {
		if attr_map[a.Key] != a.Val {
			return false
		}
	}
	c1, c2 := n1.FirstChild, n2.FirstChild
	for c1 != nil && c2 != nil {
		if !compare_nodes(c1, c2) {
			return false
		}
		c1 = c1.NextSibling
		c2 = c2.NextSibling
	}
	return c1 == nil && c2 == nil
}

func has_double_spaces(s string) bool {
	return strings.Contains(s, "  ")
}

func TestTemplates(t *testing.T) {
	tests := []struct {
		name           string
		data           Element
		expected       string
		expected_error string
	}{
		{
			name:     "Self-closing without attributes",
			data:     Element{Tag: "input"},
			expected: "<input />",
		},
		{
			name: "Self-closing with attributes",
			data: Element{
				Tag: "input",
				Attributes: map[string]string{
					"type":  "text",
					"value": "example",
				},
			},
			expected: `<input type="text" value="example" />`,
		},
		{
			name: "Self-closing with boolean attributes",
			data: Element{
				Tag:               "input",
				BooleanAttributes: []string{"checked"},
			},
			expected: `<input checked />`,
		},
		{
			name: "Self-closing with both attributes",
			data: Element{
				Tag:               "input",
				Attributes:        map[string]string{"type": "text"},
				BooleanAttributes: []string{"checked"},
			},
			expected: `<input type="text" checked />`,
		},
		{
			name:     "Non-self-closing without attributes",
			data:     Element{Tag: "div", TextContent: "Hello"},
			expected: `<div>Hello</div>`,
		},
		{
			name: "Non-self-closing with attributes",
			data: Element{
				Tag: "div",
				Attributes: map[string]string{
					"id":    "main",
					"class": "container",
				},
				TextContent: "Hello",
			},
			expected: `<div id="main" class="container">Hello</div>`,
		},
		{
			name: "Non-self-closing with boolean attributes",
			data: Element{
				Tag:               "div",
				BooleanAttributes: []string{"hidden"},
				TextContent:       "Hello",
			},
			expected: `<div hidden>Hello</div>`,
		},
		{
			name: "Non-self-closing with both attributes",
			data: Element{
				Tag:               "div",
				Attributes:        map[string]string{"id": "main"},
				BooleanAttributes: []string{"hidden"},
				TextContent:       "Hello",
			},
			expected: `<div id="main" hidden>Hello</div>`,
		},
		{
			name:     "Custom element with hyphens",
			data:     Element{Tag: "my-custom-element", TextContent: "Content"},
			expected: `<my-custom-element>Content</my-custom-element>`,
		},
		{
			name: "Data attribute with hyphens",
			data: Element{
				Tag:         "div",
				Attributes:  map[string]string{"data-info": "value"},
				TextContent: "Content",
			},
			expected: `<div data-info="value">Content</div>`,
		},
		{
			name: "Attribute with colon",
			data: Element{
				Tag:         "div",
				Attributes:  map[string]string{"xlink:href": "url"},
				TextContent: "Content",
			},
			expected: `<div xlink:href="url">Content</div>`,
		},
		{
			name: "Attribute with period",
			data: Element{
				Tag:         "div",
				Attributes:  map[string]string{"data.version": "1.0"},
				TextContent: "Content",
			},
			expected: `<div data.version="1.0">Content</div>`,
		},
		{
			name:     "Empty InnerHTML",
			data:     Element{Tag: "div", TextContent: ""},
			expected: `<div></div>`,
		},
		{
			name: "InnerHTML with special characters",
			data: Element{
				Tag:                "div",
				DangerousInnerHTML: "Content with <b>bold</b>",
			},
			expected: `<div>Content with <b>bold</b></div>`,
		},
		{
			name: "TextContent with special characters",
			data: Element{
				Tag:         "div",
				TextContent: "Content with <b>bold</b>",
			},
			expected: `<div>Content with &lt;b&gt;bold&lt;/b&gt;</div>`,
		},
		{
			name: "Nil attributes and boolean attributes",
			data: Element{
				Tag:               "div",
				Attributes:        nil,
				BooleanAttributes: nil,
				TextContent:       "Content",
			},
			expected: `<div>Content</div>`,
		},
		{
			name:     "Non-standard self-closing tag",
			data:     Element{Tag: "custom", SelfClosing: true},
			expected: `<custom />`,
		},
		{
			name: "TrustedAttributes override Attributes",
			data: Element{
				Tag:                 "div",
				Attributes:          map[string]string{"class": "unsafe"},
				AttributesKnownSafe: map[string]string{"class": "safe"},
				TextContent:         "Content",
			},
			expected: `<div class="safe">Content</div>`,
		},
		{
			name: "Attribute values with special characters",
			data: Element{
				Tag: "div",
				Attributes: map[string]string{
					"data-info": `This is a "quote" and a <tag>`,
				},
				TextContent: "Content",
			},
			expected: `<div data-info="This is a &quot;quote&quot; and a &lt;tag&gt;">Content</div>`,
		},
		{
			name: "Multiple boolean attributes",
			data: Element{
				Tag:               "input",
				BooleanAttributes: []string{"checked", "disabled"},
			},
			expected: `<input checked disabled />`,
		},
		{
			name: "Boolean attributes with special characters in names",
			data: Element{
				Tag:               "input",
				BooleanAttributes: []string{"data-checked", "aria-hidden"},
			},
			expected: `<input data-checked aria-hidden />`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := RenderElement(&tt.data)
			if tt.expected_error != "" {
				if err == nil ||
					!strings.Contains(err.Error(), tt.expected_error) {
					t.Errorf(
						"expected error %q, got %v",
						tt.expected_error,
						err,
					)
				}
				return
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if has_double_spaces(string(result)) {
				t.Errorf("output contains double spaces: %s", result)
			}

			expected_node, err := parse_html(tt.expected)
			if err != nil {
				t.Fatalf("error parsing expected HTML: %v", err)
			}
			result_node, err := parse_html(string(result))
			if err != nil {
				t.Fatalf("error parsing result HTML: %v", err)
			}

			if !compare_nodes(expected_node, result_node) {
				t.Errorf(
					"HTML structure mismatch.\nExpected: %s\nGot: %s",
					tt.expected,
					result,
				)
			}
		})
	}
}

func TestComputeContentSha256(t *testing.T) {
	tests := []struct {
		name         string
		element      Element
		expect_error bool
	}{
		{
			name:         "Valid InnerHTML",
			element:      Element{TextContent: "Some content"},
			expect_error: false,
		},
		{
			name:         "Empty InnerHTML",
			element:      Element{TextContent: ""},
			expect_error: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ComputeContentSha256(&tt.element)
			if tt.expect_error && err == nil {
				t.Errorf("expected error, got nil")
			}
			if !tt.expect_error && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestSetSha256Integrity(t *testing.T) {
	tests := []struct {
		name          string
		element       Element
		external_hash string
		expect_error  bool
	}{
		{
			name:          "Valid external hash",
			element:       Element{},
			external_hash: "validhash",
			expect_error:  false,
		},
		{
			name:          "Empty external hash",
			element:       Element{},
			external_hash: "",
			expect_error:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := SetSha256Integrity(&tt.element, tt.external_hash)
			if tt.expect_error {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				expected_integrity := "sha256-" + tt.external_hash
				if tt.element.AttributesKnownSafe["integrity"] != expected_integrity {
					t.Errorf("integrity attribute: expected %q, got %q", expected_integrity, tt.element.AttributesKnownSafe["integrity"])
				}
			}
		})
	}
}

func TestAddNonce(t *testing.T) {
	tests := []struct {
		name         string
		element      Element
		length       uint8
		expect_error bool
	}{
		{
			name:         "Default nonce length",
			element:      Element{},
			length:       0,
			expect_error: false,
		},
		{
			name:         "Custom nonce length",
			element:      Element{},
			length:       32,
			expect_error: false,
		},
		{
			name:         "Zero nonce length",
			element:      Element{},
			length:       0,
			expect_error: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nonce, err := AddNonce(&tt.element, tt.length)
			if tt.expect_error {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.element.AttributesKnownSafe["nonce"] != nonce {
					t.Errorf("nonce attribute not set correctly")
				}
				expected_len := int(tt.length)
				if tt.length == 0 {
					expected_len = 16
				}
				if len(nonce) != expected_len {
					t.Errorf("nonce length: expected %d, got %d", expected_len, len(nonce))
				}
			}
		})
	}
}

func TestCombineAttributes(t *testing.T) {
	el := Element{
		Attributes: map[string]string{
			"class":   "my & class",
			"onclick": "alert('XSS')",
		},
		AttributesKnownSafe: map[string]string{"data-safe": "<safe>"},
	}
	attrs := combine_attributes(&el)
	expected := map[string]string{
		"class":     "my &amp; class",
		"onclick":   "alert(&#39;XSS&#39;)",
		"data-safe": "<safe>",
	}
	if len(attrs) != len(expected) {
		t.Errorf("expected %d attributes, got %d", len(expected), len(attrs))
	}
	for k, v := range expected {
		if attrs[k] != v {
			t.Errorf("attribute %q: expected %q, got %q", k, v, attrs[k])
		}
	}
}

func TestEscapeIntoTrusted(t *testing.T) {
	el := Element{
		Tag:        "div",
		Attributes: map[string]string{"class": "my & class"},
	}
	trusted := EscapeIntoTrusted(&el)
	if trusted.Attributes != nil {
		t.Errorf("expected Attributes to be nil")
	}
	if trusted.AttributesKnownSafe["class"] != "my &amp; class" {
		t.Errorf("AttributesKnownSafe not set correctly")
	}
	if trusted.Tag != el.Tag {
		t.Errorf("Tag not copied correctly")
	}
	if trusted.BooleanAttributes != nil {
		t.Errorf("BooleanAttributes not copied correctly")
	}
	if trusted.TextContent != el.TextContent {
		t.Errorf("TextContent not copied correctly")
	}
	if trusted.SelfClosing != el.SelfClosing {
		t.Errorf("SelfClosing not copied correctly")
	}
}

func TestEscapeIntoTrusted_DoesNotAliasBooleanAttributes(t *testing.T) {
	el := Element{
		Tag:               "script",
		BooleanAttributes: []string{"async"},
	}

	trusted := EscapeIntoTrusted(&el)
	el.BooleanAttributes[0] = "defer"

	if trusted.BooleanAttributes[0] != "async" {
		t.Fatalf(
			"trusted element boolean attributes mutated via source aliasing: got %q",
			trusted.BooleanAttributes[0],
		)
	}
}

func TestComputeContentSha256NoMutation(t *testing.T) {
	el := &Element{DangerousInnerHTML: "<b>x</b>"}
	if _, err := ComputeContentSha256(el); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if el.AttributesKnownSafe != nil {
		t.Fatal("ComputeContentSha256 should not mutate AttributesKnownSafe")
	}
}

func TestRenderElementToBuilderNilGuards(t *testing.T) {
	var b strings.Builder
	if err := RenderElementToBuilder(nil, &b); err == nil {
		t.Fatal("expected error for nil element")
	}
	if err := RenderElementToBuilder(&Element{Tag: "div"}, nil); err == nil {
		t.Fatal("expected error for nil builder")
	}
}
