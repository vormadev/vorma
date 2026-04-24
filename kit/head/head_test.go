package head

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/htmlutil"
)

var test_renderer = NewRenderer("bob")

func TestGetHeadElements(t *testing.T) {
	route_data := &Prepared{
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

	html, err := test_renderer.Render(route_data)
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

func TestRender_NilInputPanics(t *testing.T) {
	renderer := NewRenderer("nil-input")

	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Render(nil) should panic")
		}
	}()

	renderer.Render(nil)
}

func TestRender_EmptyInputDoesNotPanic(t *testing.T) {
	renderer := NewRenderer("empty-input")
	if _, err := renderer.Render(&Prepared{}); err != nil {
		t.Fatalf("Render(empty) returned error: %v", err)
	}
}

const (
	test_title         = "Test Title"
	test_title_2       = "Different Test Title"
	test_description   = "This is a test description."
	test_description_2 = "This is a different test description."
)

func TestDedupeHead(t *testing.T) {
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
			result := test_renderer.dedup_head_els(tt.input)
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

func TestInitDedupeRules(t *testing.T) {
	renderer := NewRenderer("test")

	b := NewBuilder()
	b.Add(Tag("title"))
	b.Meta(b.Name("keywords"))
	b.Link(b.Rel("stylesheet"), b.Href("/style.css"))

	renderer.InitDedupeRules(b)

	if len(renderer.unique_rules_by_tag) == 0 {
		t.Error("Expected unique_rules_by_tag to be populated")
	}
	if rules, ok := renderer.unique_rules_by_tag["title"]; !ok || len(rules) == 0 {
		t.Error("Expected title rules to be present")
	}
	meta_rules, ok := renderer.unique_rules_by_tag["meta"]
	if !ok || len(meta_rules) == 0 {
		t.Fatal("Expected meta rules to be present")
	}
	if !slices.ContainsFunc(meta_rules, func(rule *rule_attrs) bool {
		return matches_rule(&htmlutil.Element{
			Tag: "meta",
			Attributes: map[string]string{
				"name": "description",
			},
		}, rule)
	}) {
		t.Error("Expected default description meta rule to be present")
	}
	if !slices.ContainsFunc(meta_rules, func(rule *rule_attrs) bool {
		return matches_rule(&htmlutil.Element{
			Tag: "meta",
			Attributes: map[string]string{
				"name": "keywords",
			},
		}, rule)
	}) {
		t.Error("Expected caller-provided keywords meta rule to be present")
	}
	if !slices.ContainsFunc(meta_rules, func(rule *rule_attrs) bool {
		return matches_rule(&htmlutil.Element{
			Tag: "meta",
			Attributes: map[string]string{
				"charset": "utf-8",
			},
		}, rule)
	}) {
		t.Error("Expected default charset meta rule to be present")
	}
	link_rules, ok := renderer.unique_rules_by_tag["link"]
	if !ok || len(link_rules) == 0 {
		t.Fatal("Expected link rules to be present")
	}
	if !slices.ContainsFunc(link_rules, func(rule *rule_attrs) bool {
		return matches_rule(&htmlutil.Element{
			Tag: "link",
			Attributes: map[string]string{
				"rel":  "stylesheet",
				"href": "/style.css",
			},
		}, rule)
	}) {
		t.Error("Expected caller-provided stylesheet link rule to be present")
	}

	// Call again to verify once.Do works.
	prev_rules := renderer.unique_rules_by_tag
	renderer.InitDedupeRules(nil)
	if !reflect.DeepEqual(prev_rules, renderer.unique_rules_by_tag) {
		t.Error(
			"InitDedupeRules did not respect once.Do; rules were reinitialized",
		)
	}
}

func TestHighLevelAPI(t *testing.T) {
	b := NewBuilder()
	b.Add(Tag("meta"), b.Name("description"), b.Content("Test Description"))

	if len(b.els) != 1 {
		t.Fatalf("Expected 1 element, got %d", len(b.els))
	}

	el := b.els[0]
	if el.Tag != "meta" {
		t.Errorf("Expected tag 'meta', got '%s'", el.Tag)
	}
	if el.Attributes["name"] != "description" ||
		el.Attributes["content"] != "Test Description" {
		t.Errorf("Attributes not set correctly: %v", el.Attributes)
	}

	b.Title("Test Title")
	b.Description("Test Description")

	if len(b.els) != 3 {
		t.Fatalf("Expected 3 elements, got %d", len(b.els))
	}

	title_el := b.els[1]
	if title_el.Tag != "title" || title_el.TextContent != "Test Title" {
		t.Errorf("Title not set correctly: %+v", title_el)
	}

	b.Add(Tag("link"), b.Href("/style.css").KnownSafe(), b.Rel("stylesheet"))

	if len(b.els) != 4 {
		t.Fatalf("Expected 4 elements, got %d", len(b.els))
	}

	link_el := b.els[3]
	if _, ok := link_el.AttributesKnownSafe["href"]; !ok {
		t.Errorf("Expected href in AttributesKnownSafe: %+v", link_el)
	}
	if _, ok := link_el.Attributes["rel"]; !ok {
		t.Errorf("Expected rel in Attributes: %+v", link_el)
	}
}

func TestPrepare(t *testing.T) {
	renderer := NewRenderer("test")

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

	prepared := renderer.Prepare(elements)

	if prepared.Title.DangerousInnerHTML != "Page Title" {
		t.Errorf(
			"Expected title 'Page Title', got '%s'",
			prepared.Title.DangerousInnerHTML,
		)
	}
	if len(prepared.Meta) != 2 {
		t.Errorf("Expected 2 meta elements, got %d", len(prepared.Meta))
	}
	if len(prepared.Rest) != 2 {
		t.Errorf("Expected 2 rest elements, got %d", len(prepared.Rest))
	}

	elements_2 := []*htmlutil.Element{
		{Tag: "title", DangerousInnerHTML: "Dangerous <b>Title</b>"},
	}
	prepared_2 := renderer.Prepare(elements_2)
	if prepared_2.Title.DangerousInnerHTML != "Dangerous <b>Title</b>" {
		t.Errorf(
			"Expected title from DangerousInnerHTML, got '%s'",
			prepared_2.Title.DangerousInnerHTML,
		)
	}
}

func TestEdgeCases(t *testing.T) {
	b := NewBuilder()
	b.Add(
		Tag("meta"),
		b.Name("viewport"),
		b.Content("width=device-width"),
		SelfClosing(true),
	)

	if len(b.els) != 1 {
		t.Fatalf("Expected 1 element, got %d", len(b.els))
	}
	if !b.els[0].SelfClosing {
		t.Error("Expected SelfClosing to be true")
	}

	b.Add(
		Tag("script"),
		b.Attr("src", "/script.js"),
		BooleanAttribute("async"),
		BooleanAttribute("defer"),
	)

	if len(b.els) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(b.els))
	}
	script_el := b.els[1]
	if len(script_el.BooleanAttributes) != 2 {
		t.Errorf(
			"Expected 2 boolean attributes, got %d",
			len(script_el.BooleanAttributes),
		)
	}

	b.Add(Tag("script"), InnerHTML("console.log('test');"))

	if len(b.els) != 3 {
		t.Fatalf("Expected 3 elements, got %d", len(b.els))
	}
	inner_el := b.els[2]
	if inner_el.DangerousInnerHTML != "console.log('test');" {
		t.Errorf("Expected innerHTML, got '%s'", inner_el.DangerousInnerHTML)
	}

	renderer := NewRenderer("test")
	invalid_el := &htmlutil.Element{Tag: ""}
	prepared := &Prepared{Meta: []*htmlutil.Element{invalid_el}}
	_, err := renderer.Render(prepared)
	if err == nil {
		t.Error("Expected error when rendering element with empty tag")
	}
}

func TestDeduplicationWithMixedContentTypes(t *testing.T) {
	renderer := NewRenderer("test")

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

	result := renderer.dedup_head_els(els)

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

	b := NewBuilder()
	b.Add(b.Name("description"))
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

func TestInitDedupeRulesDoesNotMutateInput(t *testing.T) {
	renderer := NewRenderer("test")

	b := NewBuilder()
	b.Link(b.Rel("stylesheet"), b.Href("/style.css"))

	original_len := len(b.Elements())
	renderer.InitDedupeRules(b)

	if len(b.Elements()) != original_len {
		t.Errorf(
			"InitDedupeRules mutated input: had %d elements, now has %d",
			original_len,
			len(b.Elements()),
		)
	}
}

func TestDedupeHeadNilElements(t *testing.T) {
	renderer := NewRenderer("test")
	renderer.InitDedupeRules(nil)

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

	result := renderer.dedup_head_els(els)

	if len(result) != 3 {
		t.Errorf("expected 3 elements, got %d", len(result))
	}
	for i, el := range result {
		if el == nil {
			t.Errorf("result[%d] is nil", i)
		}
	}
}

func TestAppendNilSourceIsNoOp(t *testing.T) {
	b := NewBuilder()
	b.Add(Tag("title"), TextContent("Original"))

	b.Append(nil)

	elements := b.Elements()
	if len(elements) != 1 {
		t.Fatalf(
			"expected 1 element after Append(nil), got %d",
			len(elements),
		)
	}
	if elements[0].Tag != "title" || elements[0].TextContent != "Original" {
		t.Fatalf("unexpected element after Append(nil): %+v", elements[0])
	}
}
