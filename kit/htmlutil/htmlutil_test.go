package htmlutil

import (
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/htmltestutil"
)

func TestTemplates(t *testing.T) {
	tests := []struct {
		name          string
		data          Element
		expected      string
		expectedError string
	}{
		{
			name:     "Self-closing without attributes",
			data:     Element{Tag: "input"},
			expected: "<input />",
		},
		{
			name:     "Self-closing with attributes",
			data:     Element{Tag: "input", Attributes: map[string]string{"type": "text", "value": "example"}},
			expected: `<input type="text" value="example" />`,
		},
		{
			name:     "Self-closing with boolean attributes",
			data:     Element{Tag: "input", BooleanAttributes: []string{"checked"}},
			expected: `<input checked />`,
		},
		{
			name:     "Self-closing with both attributes",
			data:     Element{Tag: "input", Attributes: map[string]string{"type": "text"}, BooleanAttributes: []string{"checked"}},
			expected: `<input type="text" checked />`,
		},
		{
			name:     "Non-self-closing without attributes",
			data:     Element{Tag: "div", TextContent: "Hello"},
			expected: `<div>Hello</div>`,
		},
		{
			name:     "Non-self-closing with attributes",
			data:     Element{Tag: "div", Attributes: map[string]string{"id": "main", "class": "container"}, TextContent: "Hello"},
			expected: `<div id="main" class="container">Hello</div>`,
		},
		{
			name:     "Non-self-closing with boolean attributes",
			data:     Element{Tag: "div", BooleanAttributes: []string{"hidden"}, TextContent: "Hello"},
			expected: `<div hidden>Hello</div>`,
		},
		{
			name:     "Non-self-closing with both attributes",
			data:     Element{Tag: "div", Attributes: map[string]string{"id": "main"}, BooleanAttributes: []string{"hidden"}, TextContent: "Hello"},
			expected: `<div id="main" hidden>Hello</div>`,
		},
		{
			name:     "Custom element with hyphens",
			data:     Element{Tag: "my-custom-element", TextContent: "Content"},
			expected: `<my-custom-element>Content</my-custom-element>`,
		},
		{
			name:     "Data attribute with hyphens",
			data:     Element{Tag: "div", Attributes: map[string]string{"data-info": "value"}, TextContent: "Content"},
			expected: `<div data-info="value">Content</div>`,
		},
		{
			name:     "Attribute with colon",
			data:     Element{Tag: "div", Attributes: map[string]string{"xlink:href": "url"}, TextContent: "Content"},
			expected: `<div xlink:href="url">Content</div>`,
		},
		{
			name:     "Attribute with period",
			data:     Element{Tag: "div", Attributes: map[string]string{"data.version": "1.0"}, TextContent: "Content"},
			expected: `<div data.version="1.0">Content</div>`,
		},
		{
			name:     "Empty InnerHTML",
			data:     Element{Tag: "div", TextContent: ""},
			expected: `<div></div>`,
		},
		{
			name:     "InnerHTML with special characters",
			data:     Element{Tag: "div", DangerousInnerHTML: "Content with <b>bold</b>"},
			expected: `<div>Content with <b>bold</b></div>`,
		},
		{
			name:     "TextContent with special characters",
			data:     Element{Tag: "div", TextContent: "Content with <b>bold</b>"},
			expected: `<div>Content with &lt;b&gt;bold&lt;/b&gt;</div>`,
		},
		{
			name:     "Nil attributes and boolean attributes",
			data:     Element{Tag: "div", Attributes: nil, BooleanAttributes: nil, TextContent: "Content"},
			expected: `<div>Content</div>`,
		},
		{
			name:     "Non-standard self-closing tag",
			data:     Element{Tag: "custom", SelfClosing: true},
			expected: `<custom />`,
		},
		{
			name:     "TrustedAttributes override Attributes",
			data:     Element{Tag: "div", Attributes: map[string]string{"class": "unsafe"}, AttributesKnownSafe: map[string]string{"class": "safe"}, TextContent: "Content"},
			expected: `<div class="safe">Content</div>`,
		},
		{
			name:     "Attribute values with special characters",
			data:     Element{Tag: "div", Attributes: map[string]string{"data-info": `This is a "quote" and a <tag>`}, TextContent: "Content"},
			expected: `<div data-info="This is a &quot;quote&quot; and a &lt;tag&gt;">Content</div>`,
		},
		{
			name:     "Multiple boolean attributes",
			data:     Element{Tag: "input", BooleanAttributes: []string{"checked", "disabled"}},
			expected: `<input checked disabled />`,
		},
		{
			name:     "Boolean attributes with special characters in names",
			data:     Element{Tag: "input", BooleanAttributes: []string{"data-checked", "aria-hidden"}},
			expected: `<input data-checked aria-hidden />`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Render the template to get the actual result.
			result, err := RenderElement(&tt.data)
			if tt.expectedError != "" {
				if err == nil || !strings.Contains(err.Error(), tt.expectedError) {
					t.Errorf("expected error %q, got %v", tt.expectedError, err)
				}
				return
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// Check for double spaces in the output.
			if hasDoubleSpaces(string(result)) {
				t.Errorf("output contains double spaces: %s", result)
			}

			// Parse both the expected and actual HTML.
			expectedNode, err := htmltestutil.ParseHTML(tt.expected)
			if err != nil {
				t.Fatalf("error parsing expected HTML: %v", err)
			}
			resultNode, err := htmltestutil.ParseHTML(string(result))
			if err != nil {
				t.Fatalf("error parsing result HTML: %v", err)
			}

			// Compare the parsed nodes structurally (ignoring attribute order).
			if !htmltestutil.CompareNodes(expectedNode, resultNode) {
				t.Errorf("expected HTML structure does not match actual structure.\nExpected: %s\nGot: %s", tt.expected, result)
			}
		})
	}
}

// Helper function to check for double spaces.
func hasDoubleSpaces(s string) bool {
	return strings.Contains(s, "  ")
}

func TestAddSha256HashInline(t *testing.T) {
	tests := []struct {
		name        string
		element     Element
		expectError bool
	}{
		{
			name:        "Valid InnerHTML",
			element:     Element{TextContent: "Some content"},
			expectError: false,
		},
		{
			name:        "Empty InnerHTML",
			element:     Element{TextContent: ""},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AddSha256HashInline(&tt.element)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestAddSha256HashExternal(t *testing.T) {
	tests := []struct {
		name         string
		element      Element
		externalHash string
		expectError  bool
	}{
		{
			name:         "Valid external hash",
			element:      Element{},
			externalHash: "validhash",
			expectError:  false,
		},
		{
			name:         "Empty external hash",
			element:      Element{},
			externalHash: "",
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := AddSha256HashExternal(&tt.element, tt.externalHash)
			if tt.expectError {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				// Check that the integrity attribute was added
				expectedIntegrity := "sha256-" + tt.externalHash
				if tt.element.AttributesKnownSafe["integrity"] != expectedIntegrity {
					t.Errorf("integrity attribute not set correctly, expected %q, got %q", expectedIntegrity, tt.element.AttributesKnownSafe["integrity"])
				}
			}
		})
	}
}

func TestAddNonce(t *testing.T) {
	tests := []struct {
		name        string
		element     Element
		len         uint8
		expectError bool
	}{
		{
			name:        "Default nonce length",
			element:     Element{},
			len:         0,
			expectError: false,
		},
		{
			name:        "Custom nonce length",
			element:     Element{},
			len:         32,
			expectError: false,
		},
		{
			name:        "Zero nonce length",
			element:     Element{},
			len:         0,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nonce, err := AddNonce(&tt.element, tt.len)
			if tt.expectError {
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
				expectedLen := int(tt.len)
				if tt.len == 0 {
					expectedLen = 16
				}
				if len(nonce) != expectedLen {
					t.Errorf("nonce length mismatch, expected %d, got %d", expectedLen, len(nonce))
				}
			}
		})
	}
}

func TestEscapeAllIntoNewMap(t *testing.T) {
	el := Element{
		Attributes:          map[string]string{"class": "my & class", "onclick": "alert('XSS')"},
		AttributesKnownSafe: map[string]string{"data-safe": "<safe>"},
	}
	attributes := combineIntoDangerousAttributes(&el)
	expected := map[string]string{
		"class":     "my &amp; class",
		"onclick":   "alert(&#39;XSS&#39;)",
		"data-safe": "<safe>",
	}
	if len(attributes) != len(expected) {
		t.Errorf("expected %d attributes, got %d", len(expected), len(attributes))
	}
	for k, v := range expected {
		if attributes[k] != v {
			t.Errorf("attribute %q mismatch, expected %q, got %q", k, v, attributes[k])
		}
	}
}

func TestEscapeIntoTrusted(t *testing.T) {
	el := Element{
		Tag:        "div",
		Attributes: map[string]string{"class": "my & class"},
	}
	newEl := EscapeIntoTrusted(&el)
	if newEl.Attributes != nil {
		t.Errorf("expected Attributes to be nil")
	}
	if newEl.AttributesKnownSafe["class"] != "my &amp; class" {
		t.Errorf("TrustedAttributes not set correctly")
	}
	// Ensure other fields are copied correctly
	if newEl.Tag != el.Tag {
		t.Errorf("Tag not copied correctly")
	}
	if newEl.BooleanAttributes != nil {
		t.Errorf("BooleanAttributes not copied correctly")
	}
	if newEl.TextContent != el.TextContent {
		t.Errorf("InnerHTML not copied correctly")
	}
	if newEl.SelfClosing != el.SelfClosing {
		t.Errorf("SelfClosing not copied correctly")
	}
}
