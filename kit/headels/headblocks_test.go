package headels

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/htmlutil"
)

var test_instance = NewInstance("bob")

func TestGetHeadElements(t *testing.T) {
	route_data := &SortedAndPreEscapedHeadEls{
		Title: &htmlutil.Element{Tag: "title", TextContent: "Test Title"},
		Meta: []*htmlutil.Element{
			{
				Tag: "meta",
				Attributes: map[string]string{
					"name":    "description",
					"content": "Test Description",
				},
			},
		},
		Rest: []*htmlutil.Element{
			{
				Tag: "link",
				Attributes: map[string]string{
					"rel":  "stylesheet",
					"href": "/style.css",
				},
			},
		},
	}

	html, err := test_instance.Render(route_data)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if !strings.Contains(string(html), "<title>Test Title</title>") {
		t.Errorf("Expected title tag, but it's missing")
	}
	if !strings.Contains(string(html), `name="description"`) ||
		!strings.Contains(string(html), `content="Test Description"`) {
		t.Errorf("Expected meta description tag, but it's missing")
	}
	if !strings.Contains(string(html), `rel="stylesheet"`) ||
		!strings.Contains(string(html), `href="/style.css"`) {
		t.Errorf("Expected link tag, but it's missing")
	}
}

func TestRender_NilInputDoesNotPanicAndRendersMarkers(t *testing.T) {
	inst := NewInstance("nil-input")

	rendered, err := inst.Render(nil)
	if err != nil {
		t.Fatalf("Render(nil) returned error: %v", err)
	}

	text := string(rendered)
	if !strings.Contains(text, `data-nil-input="meta-start"`) {
		t.Fatalf("missing meta-start marker: %s", text)
	}
	if !strings.Contains(text, `data-nil-input="meta-end"`) {
		t.Fatalf("missing meta-end marker: %s", text)
	}
	if !strings.Contains(text, `data-nil-input="rest-start"`) {
		t.Fatalf("missing rest-start marker: %s", text)
	}
	if !strings.Contains(text, `data-nil-input="rest-end"`) {
		t.Fatalf("missing rest-end marker: %s", text)
	}
}

const (
	test_title         = "Test Title"
	test_title_2       = "Different Test Title"
	test_description   = "This is a test description."
	test_description_2 = "This is a different test description."
)

func TestDedupeHeadEls(t *testing.T) {
	tests := []struct {
		name     string
		input    []*htmlutil.Element
		expected []*htmlutil.Element
	}{
		{
			name: "No duplicates, with title and description",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": test_description,
					},
				},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "og:image",
						"content": "image.webp",
					},
				},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": test_description,
					},
				},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "og:image",
						"content": "image.webp",
					},
				},
			},
		},
		{
			name: "With duplicates",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": test_description,
					},
				},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": test_description_2,
					},
				},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": test_description_2,
					},
				},
			},
		},
		{
			name: "With duplicates TrustedAttributes",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{
					Tag: "meta",
					AttributesKnownSafe: map[string]string{
						"name":    "description",
						"content": test_description,
					},
				},
				{
					Tag: "meta",
					AttributesKnownSafe: map[string]string{
						"name":    "description",
						"content": test_description_2,
					},
				},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{
					Tag: "meta",
					AttributesKnownSafe: map[string]string{
						"name":    "description",
						"content": test_description_2,
					},
				},
			},
		},
		{
			name: "With duplicates mixed",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": test_description,
					},
				},
				{
					Tag: "meta",
					AttributesKnownSafe: map[string]string{
						"name":    "description",
						"content": test_description_2,
					},
				},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{
					Tag: "meta",
					AttributesKnownSafe: map[string]string{
						"name":    "description",
						"content": test_description_2,
					},
				},
			},
		},
		{
			name: "No title or description",
			input: []*htmlutil.Element{
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "keywords",
						"content": "go, test",
					},
				},
				{
					Tag: "link",
					Attributes: map[string]string{
						"rel":  "stylesheet",
						"href": "/style.css",
					},
				},
			},
			expected: []*htmlutil.Element{
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "keywords",
						"content": "go, test",
					},
				},
				{
					Tag: "link",
					Attributes: map[string]string{
						"rel":  "stylesheet",
						"href": "/style.css",
					},
				},
			},
		},
		{
			name: "Multiple titles and descriptions",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title, Attributes: nil},
				{Tag: "title", TextContent: test_title_2, Attributes: nil},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": "Description 1",
					},
				},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": "Description 2",
					},
				},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: test_title_2, Attributes: nil},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "description",
						"content": "Description 2",
					},
				},
			},
		},
		{
			name: "Different tags with same attributes",
			input: []*htmlutil.Element{
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "viewport",
						"content": "width=device-width, initial-scale=1",
					},
				},
				{
					Tag: "link",
					Attributes: map[string]string{
						"rel":  "stylesheet",
						"href": "/style.css",
					},
				},
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "viewport",
						"content": "width=device-width, initial-scale=1",
					},
				},
			},
			expected: []*htmlutil.Element{
				{
					Tag: "meta",
					Attributes: map[string]string{
						"name":    "viewport",
						"content": "width=device-width, initial-scale=1",
					},
				},
				{
					Tag: "link",
					Attributes: map[string]string{
						"rel":  "stylesheet",
						"href": "/style.css",
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := test_instance.dedup_head_els(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Log("Result:")
				for _, el := range result {
					t.Logf("%+v", el)
				}
				t.Log("Expected:")
				for _, el := range tt.expected {
					t.Logf("%+v", el)
				}
				t.Errorf("dedup_head_els() mismatch")
			}
		})
	}
}

func TestHashElement(t *testing.T) {
	elements := []*htmlutil.Element{
		{
			Tag: "link",
			Attributes: map[string]string{
				"rel":  "stylesheet",
				"href": "/style.css",
			},
		},
		{
			Tag: "link",
			Attributes: map[string]string{
				"rel":  "stylesheet",
				"href": "/other.css",
			},
		},
		{
			Tag: "meta",
			Attributes: map[string]string{
				"name":    "viewport",
				"content": "width=device-width",
			},
		},
		{Tag: "title", TextContent: "Page Title"},
		{
			Tag: "link",
			AttributesKnownSafe: map[string]string{
				"rel":  "stylesheet",
				"href": "/style.css",
			},
		},
		{Tag: "meta", BooleanAttributes: []string{"async"}},
		{Tag: "script", DangerousInnerHTML: "console.log('test');"},
	}

	hashes := make(map[uint64]int)
	for i, el := range elements {
		h := hash_element(el)
		if existing, exists := hashes[h]; exists {
			t.Errorf("Hash collision between elements %d and %d", existing, i)
		}
		hashes[h] = i
	}
}

func TestMatchesRule(t *testing.T) {
	tests := []struct {
		name     string
		element  *htmlutil.Element
		rule     *rule_attrs
		expected bool
	}{
		{
			name: "Exact match with regular attributes",
			element: &htmlutil.Element{
				Tag: "meta",
				Attributes: map[string]string{
					"name":    "description",
					"content": "Test",
				},
			},
			rule: &rule_attrs{
				attrs: map[string]string{"name": "description"},
			},
			expected: true,
		},
		{
			name: "Non-match with regular attributes",
			element: &htmlutil.Element{
				Tag: "meta",
				Attributes: map[string]string{
					"name":    "keywords",
					"content": "Test",
				},
			},
			rule: &rule_attrs{
				attrs: map[string]string{"name": "description"},
			},
			expected: false,
		},
		{
			name: "Match with trusted attributes",
			element: &htmlutil.Element{
				Tag: "meta",
				AttributesKnownSafe: map[string]string{
					"name":    "description",
					"content": "Test",
				},
			},
			rule: &rule_attrs{
				trusted: map[string]string{"name": "description"},
			},
			expected: true,
		},
		{
			name: "Match with boolean attributes",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"async", "defer"},
			},
			rule:     &rule_attrs{boolean: []string{"async"}},
			expected: true,
		},
		{
			name: "Non-match with boolean attributes",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"defer"},
			},
			rule:     &rule_attrs{boolean: []string{"async"}},
			expected: false,
		},
		{
			name: "Match with mixed attribute types",
			element: &htmlutil.Element{
				Tag:        "meta",
				Attributes: map[string]string{"name": "viewport"},
				AttributesKnownSafe: map[string]string{
					"content": "width=device-width",
				},
			},
			rule: &rule_attrs{
				attrs:   map[string]string{"name": "viewport"},
				trusted: map[string]string{"content": "width=device-width"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matches_rule(tt.element, tt.rule)
			if result != tt.expected {
				t.Errorf(
					"matches_rule() = %v, expected %v",
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestInitUniqueRules(t *testing.T) {
	inst := NewInstance("test")

	e := New()
	e.Add(Tag("title"))
	e.Meta(e.Name("description"))
	e.Meta(e.Name("viewport"), e.Content("width=device-width"))
	e.Link(e.Rel("stylesheet"), e.Href("/style.css"))

	inst.InitUniqueRules(e)

	if len(inst.unique_rules_by_tag) == 0 {
		t.Error("Expected unique_rules_by_tag to be populated")
	}
	if rules, ok := inst.unique_rules_by_tag["title"]; !ok || len(rules) == 0 {
		t.Error("Expected title rules to be present")
	}
	if rules, ok := inst.unique_rules_by_tag["meta"]; !ok || len(rules) != 2 {
		for _, rule := range rules {
			t.Logf("Meta rule: %+v", rule)
		}
		t.Errorf("Expected 2 meta rules, got %d", len(rules))
	}
	if rules, ok := inst.unique_rules_by_tag["link"]; !ok || len(rules) == 0 {
		t.Error("Expected link rules to be present")
	}

	// Call again to verify once.Do works.
	prev_rules := inst.unique_rules_by_tag
	inst.InitUniqueRules(nil)
	if !reflect.DeepEqual(prev_rules, inst.unique_rules_by_tag) {
		t.Error(
			"InitUniqueRules did not respect once.Do; rules were reinitialized",
		)
	}
}

func TestHighLevelAPI(t *testing.T) {
	h := New()
	h.Add(Tag("meta"), h.Name("description"), h.Content("Test Description"))

	if len(h.els) != 1 {
		t.Fatalf("Expected 1 element, got %d", len(h.els))
	}

	el := h.els[0]
	if el.Tag != "meta" {
		t.Errorf("Expected tag 'meta', got '%s'", el.Tag)
	}
	if el.Attributes["name"] != "description" ||
		el.Attributes["content"] != "Test Description" {
		t.Errorf("Attributes not set correctly: %v", el.Attributes)
	}

	h.Title("Test Title")
	h.Description("Test Description")

	if len(h.els) != 3 {
		t.Fatalf("Expected 3 elements, got %d", len(h.els))
	}

	title_el := h.els[1]
	if title_el.Tag != "title" || title_el.TextContent != "Test Title" {
		t.Errorf("Title not set correctly: %+v", title_el)
	}

	h.Add(Tag("link"), h.Href("/style.css").KnownSafe(), h.Rel("stylesheet"))

	if len(h.els) != 4 {
		t.Fatalf("Expected 4 elements, got %d", len(h.els))
	}

	link_el := h.els[3]
	if _, ok := link_el.AttributesKnownSafe["href"]; !ok {
		t.Errorf("Expected href in AttributesKnownSafe: %+v", link_el)
	}
	if _, ok := link_el.Attributes["rel"]; !ok {
		t.Errorf("Expected rel in Attributes: %+v", link_el)
	}
}

func TestToSortedHeadEls(t *testing.T) {
	inst := NewInstance("test")

	elements := []*htmlutil.Element{
		{Tag: "title", TextContent: "Page Title"},
		{
			Tag: "meta",
			Attributes: map[string]string{
				"name":    "description",
				"content": "Test Description",
			},
		},
		{
			Tag: "link",
			Attributes: map[string]string{
				"rel":  "stylesheet",
				"href": "/style.css",
			},
		},
		{Tag: "script", Attributes: map[string]string{"src": "/script.js"}},
		{
			Tag: "meta",
			Attributes: map[string]string{
				"property": "og:title",
				"content":  "OG Title",
			},
		},
	}

	sorted := inst.ToSortedAndPreEscapedHeadEls(elements)

	if sorted.Title.DangerousInnerHTML != "Page Title" {
		t.Errorf(
			"Expected title 'Page Title', got '%s'",
			sorted.Title.DangerousInnerHTML,
		)
	}
	if len(sorted.Meta) != 2 {
		t.Errorf("Expected 2 meta elements, got %d", len(sorted.Meta))
	}
	if len(sorted.Rest) != 2 {
		t.Errorf("Expected 2 rest elements, got %d", len(sorted.Rest))
	}

	elements_2 := []*htmlutil.Element{
		{Tag: "title", DangerousInnerHTML: "Dangerous <b>Title</b>"},
	}
	sorted_2 := inst.ToSortedAndPreEscapedHeadEls(elements_2)
	if sorted_2.Title.DangerousInnerHTML != "Dangerous <b>Title</b>" {
		t.Errorf(
			"Expected title from DangerousInnerHTML, got '%s'",
			sorted_2.Title.DangerousInnerHTML,
		)
	}
}

func TestEdgeCases(t *testing.T) {
	h := New()
	h.Add(
		Tag("meta"),
		h.Name("viewport"),
		h.Content("width=device-width"),
		SelfClosing(true),
	)

	if len(h.els) != 1 {
		t.Fatalf("Expected 1 element, got %d", len(h.els))
	}
	if !h.els[0].SelfClosing {
		t.Error("Expected SelfClosing to be true")
	}

	h.Add(
		Tag("script"),
		h.Attr("src", "/script.js"),
		BooleanAttribute("async"),
		BooleanAttribute("defer"),
	)

	if len(h.els) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(h.els))
	}
	script_el := h.els[1]
	if len(script_el.BooleanAttributes) != 2 {
		t.Errorf(
			"Expected 2 boolean attributes, got %d",
			len(script_el.BooleanAttributes),
		)
	}

	h.Add(Tag("script"), InnerHTML("console.log('test');"))

	if len(h.els) != 3 {
		t.Fatalf("Expected 3 elements, got %d", len(h.els))
	}
	inner_el := h.els[2]
	if inner_el.DangerousInnerHTML != "console.log('test');" {
		t.Errorf("Expected innerHTML, got '%s'", inner_el.DangerousInnerHTML)
	}

	inst := NewInstance("test")
	invalid_el := &htmlutil.Element{Tag: ""}
	sorted := &SortedAndPreEscapedHeadEls{Meta: []*htmlutil.Element{invalid_el}}
	_, err := inst.Render(sorted)
	if err == nil {
		t.Error("Expected error when rendering element with empty tag")
	}
}

func TestDeduplicationWithMixedContentTypes(t *testing.T) {
	inst := NewInstance("test")

	els := []*htmlutil.Element{
		{Tag: "script", DangerousInnerHTML: "console.log('test1');"},
		{Tag: "script", DangerousInnerHTML: "console.log('test2');"},
		{
			Tag: "meta",
			Attributes: map[string]string{
				"name":    "viewport",
				"content": "width=device-width",
			},
		},
		{
			Tag: "meta",
			Attributes: map[string]string{
				"name":    "viewport",
				"content": "width=device-width",
			},
		},
		{
			Tag: "link",
			Attributes: map[string]string{
				"rel":  "icon",
				"href": "/favicon.ico",
			},
			SelfClosing: true,
		},
		{
			Tag: "link",
			Attributes: map[string]string{
				"rel":  "icon",
				"href": "/favicon.ico",
			},
			SelfClosing: true,
		},
		{
			Tag:               "script",
			Attributes:        map[string]string{"src": "/script.js"},
			BooleanAttributes: []string{"async"},
		},
		{
			Tag:               "script",
			Attributes:        map[string]string{"src": "/script.js"},
			BooleanAttributes: []string{"defer"},
		},
	}

	result := inst.dedup_head_els(els)

	if len(result) != 6 {
		t.Errorf("Expected 6 elements after deduplication, got %d", len(result))
	}

	script_count := 0
	for _, el := range result {
		if el.Tag == "script" && el.DangerousInnerHTML != "" {
			script_count++
		}
	}
	if script_count != 2 {
		t.Errorf("Expected 2 script tags with innerHTML, got %d", script_count)
	}

	viewport_count := 0
	for _, el := range result {
		if el.Tag == "meta" && el.Attributes["name"] == "viewport" {
			viewport_count++
		}
	}
	if viewport_count != 1 {
		t.Errorf("Expected 1 viewport meta tag, got %d", viewport_count)
	}
}

func TestPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when adding element without Tag")
		}
	}()

	h := New()
	h.Add(h.Name("description"))
}

func TestMatchesRuleMultipleBooleanAttributes(t *testing.T) {
	tests := []struct {
		name     string
		element  *htmlutil.Element
		rule     *rule_attrs
		expected bool
	}{
		{
			name: "element has all required boolean attributes",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"async", "defer", "nomodule"},
			},
			rule:     &rule_attrs{boolean: []string{"async", "defer"}},
			expected: true,
		},
		{
			name: "element missing second boolean attribute",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"async"},
			},
			rule:     &rule_attrs{boolean: []string{"async", "defer"}},
			expected: false,
		},
		{
			name: "element missing first boolean attribute",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"defer"},
			},
			rule:     &rule_attrs{boolean: []string{"async", "defer"}},
			expected: false,
		},
		{
			name: "element has none of the required boolean attributes",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"nomodule"},
			},
			rule:     &rule_attrs{boolean: []string{"async", "defer"}},
			expected: false,
		},
		{
			name: "rule with three boolean attributes all present",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"async", "defer", "nomodule"},
			},
			rule: &rule_attrs{
				boolean: []string{"async", "defer", "nomodule"},
			},
			expected: true,
		},
		{
			name: "rule with three boolean attributes missing last",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"async", "defer"},
			},
			rule: &rule_attrs{
				boolean: []string{"async", "defer", "nomodule"},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matches_rule(tt.element, tt.rule)
			if result != tt.expected {
				t.Errorf(
					"matches_rule() = %v, expected %v",
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestInitUniqueRulesDoesNotMutateInput(t *testing.T) {
	inst := NewInstance("test")

	e := New()
	e.Link(e.Rel("stylesheet"), e.Href("/style.css"))

	original_len := len(e.Collect())
	inst.InitUniqueRules(e)

	if len(e.Collect()) != original_len {
		t.Errorf(
			"InitUniqueRules mutated input: had %d elements, now has %d",
			original_len,
			len(e.Collect()),
		)
	}
}

func TestDedupeHeadElsNilElements(t *testing.T) {
	inst := NewInstance("test")
	inst.InitUniqueRules(nil)

	els := []*htmlutil.Element{
		{
			Tag: "meta",
			Attributes: map[string]string{
				"name":    "viewport",
				"content": "width=device-width",
			},
		},
		nil,
		{Tag: "title", TextContent: "Test Title"},
		nil,
		nil,
		{
			Tag: "link",
			Attributes: map[string]string{
				"rel":  "stylesheet",
				"href": "/style.css",
			},
		},
	}

	result := inst.dedup_head_els(els)

	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}
	for i, el := range result {
		if el == nil {
			t.Errorf("result[%d] is nil", i)
		}
	}
}

func TestAddElements_NilSourceIsNoOp(t *testing.T) {
	h := New()
	h.Add(Tag("title"), TextContent("Original"))

	h.AddElements(nil)

	collected := h.Collect()
	if len(collected) != 1 {
		t.Fatalf(
			"expected 1 element after AddElements(nil), got %d",
			len(collected),
		)
	}
	if collected[0].Tag != "title" || collected[0].TextContent != "Original" {
		t.Fatalf("unexpected element after AddElements(nil): %+v", collected[0])
	}
}
