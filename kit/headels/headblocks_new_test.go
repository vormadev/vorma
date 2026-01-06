package headels

import (
	"reflect"
	"testing"

	"github.com/vormadev/vorma/kit/htmlutil"
)

// TestHashElement tests that the hashElement function generates different hashes for different elements
func TestHashElement(t *testing.T) {
	elements := []*htmlutil.Element{
		{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
		{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/other.css"}},
		{Tag: "meta", Attributes: map[string]string{"name": "viewport", "content": "width=device-width"}},
		{Tag: "title", TextContent: "Page Title"},
		{Tag: "link", AttributesKnownSafe: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
		{Tag: "meta", BooleanAttributes: []string{"async"}},
		{Tag: "script", DangerousInnerHTML: "console.log('test');"},
	}

	// Create a map to check for hash collisions
	hashes := make(map[uint64]int)

	for i, el := range elements {
		hash := hashElement(el)
		if existingIndex, exists := hashes[hash]; exists {
			t.Errorf("Hash collision between elements %d and %d", existingIndex, i)
		}
		hashes[hash] = i
	}
}

// TestMatchesRule tests the matchesRule function with various scenarios
func TestMatchesRule(t *testing.T) {
	tests := []struct {
		name     string
		element  *htmlutil.Element
		rule     *ruleAttrs
		expected bool
	}{
		{
			name: "Exact match with regular attributes",
			element: &htmlutil.Element{
				Tag:        "meta",
				Attributes: map[string]string{"name": "description", "content": "Test"},
			},
			rule: &ruleAttrs{
				attrs: map[string]string{"name": "description"},
			},
			expected: true,
		},
		{
			name: "Non-match with regular attributes",
			element: &htmlutil.Element{
				Tag:        "meta",
				Attributes: map[string]string{"name": "keywords", "content": "Test"},
			},
			rule: &ruleAttrs{
				attrs: map[string]string{"name": "description"},
			},
			expected: false,
		},
		{
			name: "Match with trusted attributes",
			element: &htmlutil.Element{
				Tag:                 "meta",
				AttributesKnownSafe: map[string]string{"name": "description", "content": "Test"},
			},
			rule: &ruleAttrs{
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
			rule: &ruleAttrs{
				boolean: []string{"async"},
			},
			expected: true,
		},
		{
			name: "Non-match with boolean attributes",
			element: &htmlutil.Element{
				Tag:               "script",
				BooleanAttributes: []string{"defer"},
			},
			rule: &ruleAttrs{
				boolean: []string{"async"},
			},
			expected: false,
		},
		{
			name: "Match with mixed attribute types",
			element: &htmlutil.Element{
				Tag:                 "meta",
				Attributes:          map[string]string{"name": "viewport"},
				AttributesKnownSafe: map[string]string{"content": "width=device-width"},
			},
			rule: &ruleAttrs{
				attrs:   map[string]string{"name": "viewport"},
				trusted: map[string]string{"content": "width=device-width"},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesRule(tt.element, tt.rule)
			if result != tt.expected {
				t.Errorf("matchesRule() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

// TestInitUniqueRules tests the InitUniqueRules function
func TestInitUniqueRules(t *testing.T) {
	inst := NewInstance("test")

	// Create HeadEls with some rules
	e := New()
	e.Add(Tag("title"))
	e.Meta(e.Name("description"))
	e.Meta(e.Name("viewport"), e.Content("width=device-width"))
	e.Link(e.Rel("stylesheet"), e.Href("/style.css"))

	// Initialize unique rules
	inst.InitUniqueRules(e)

	// Check that rules were properly initialized
	if len(inst.uniqueRulesByTag) == 0 {
		t.Error("Expected uniqueRulesByTag to be populated, but it's empty")
	}

	// Check specific tags
	if rules, ok := inst.uniqueRulesByTag["title"]; !ok || len(rules) == 0 {
		t.Error("Expected title rules to be present")
	}

	if rules, ok := inst.uniqueRulesByTag["meta"]; !ok || len(rules) != 2 {
		for _, rule := range rules {
			t.Logf("Meta rule: %+v", rule)
		}
		t.Errorf("Expected 2 meta rules, got %d", len(rules))
	}

	if rules, ok := inst.uniqueRulesByTag["link"]; !ok || len(rules) == 0 {
		t.Error("Expected link rules to be present")
	}

	// Call InitUniqueRules again to test that once.Do works
	prevRules := inst.uniqueRulesByTag
	inst.InitUniqueRules(nil)

	// The rules should be the same object (not reinitialized)
	if !reflect.DeepEqual(prevRules, inst.uniqueRulesByTag) {
		t.Error("InitUniqueRules did not respect once.Do; rules were reinitialized")
	}
}

// TestHighLevelAPI tests the high level API functions
func TestHighLevelAPI(t *testing.T) {
	// Test Add with various types
	h := New()
	h.Add(Tag("meta"), h.Name("description"), h.Content("Test Description"))

	if len(h.els) != 1 {
		t.Fatalf("Expected 1 element, got %d", len(h.els))
	}

	el := h.els[0]
	if el.Tag != "meta" {
		t.Errorf("Expected tag 'meta', got '%s'", el.Tag)
	}

	if el.Attributes["name"] != "description" || el.Attributes["content"] != "Test Description" {
		t.Errorf("Attributes not set correctly: %v", el.Attributes)
	}

	// Test helper methods
	h.Title("Test Title")
	h.Description("Test Description")

	if len(h.els) != 3 {
		t.Fatalf("Expected 3 elements, got %d", len(h.els))
	}

	// Test that Title element was created correctly
	titleEl := h.els[1]
	if titleEl.Tag != "title" || titleEl.TextContent != "Test Title" {
		t.Errorf("Title not set correctly: %+v", titleEl)
	}

	// Test KnownSafe
	h.Add(Tag("link"), h.Href("/style.css").KnownSafe(), h.Rel("stylesheet"))

	if len(h.els) != 4 {
		t.Fatalf("Expected 4 elements, got %d", len(h.els))
	}

	linkEl := h.els[3]
	if _, ok := linkEl.AttributesKnownSafe["href"]; !ok {
		t.Errorf("Expected href to be in AttributesKnownSafe, but it's not: %+v", linkEl)
	}

	if _, ok := linkEl.Attributes["rel"]; !ok {
		t.Errorf("Expected rel to be in Attributes, but it's not: %+v", linkEl)
	}
}

// TestToSortedHeadEls tests the ToSortedHeadEls function
func TestToSortedHeadEls(t *testing.T) {
	inst := NewInstance("test")

	elements := []*htmlutil.Element{
		{Tag: "title", TextContent: "Page Title"},
		{Tag: "meta", Attributes: map[string]string{"name": "description", "content": "Test Description"}},
		{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
		{Tag: "script", Attributes: map[string]string{"src": "/script.js"}},
		{Tag: "meta", Attributes: map[string]string{"property": "og:title", "content": "OG Title"}},
	}

	sorted := inst.ToSortedAndPreEscapedHeadEls(elements)

	// Check that the title was extracted correctly
	if sorted.Title.DangerousInnerHTML != "Page Title" {
		t.Errorf("Expected title 'Page Title', got '%s'", sorted.Title.TextContent)
	}

	// Check that meta elements are in Meta
	if len(sorted.Meta) != 2 {
		t.Errorf("Expected 2 meta elements, got %d", len(sorted.Meta))
	}

	// Check that other elements are in Rest
	if len(sorted.Rest) != 2 {
		t.Errorf("Expected 2 rest elements, got %d", len(sorted.Rest))
	}

	// Test title with DangerousInnerHTML instead of TextContent
	elements2 := []*htmlutil.Element{
		{Tag: "title", DangerousInnerHTML: "Dangerous <b>Title</b>"},
	}

	sorted2 := inst.ToSortedAndPreEscapedHeadEls(elements2)

	// Check that the title was extracted correctly from DangerousInnerHTML
	if sorted2.Title.DangerousInnerHTML != "Dangerous <b>Title</b>" {
		t.Errorf("Expected title from DangerousInnerHTML, got '%s'", sorted2.Title.TextContent)
	}
}

// TestEdgeCases tests various edge cases not covered by other tests
func TestEdgeCases(t *testing.T) {
	// Test SelfClosing
	h := New()
	h.Add(Tag("meta"), h.Name("viewport"), h.Content("width=device-width"), SelfClosing(true))

	if len(h.els) != 1 {
		t.Fatalf("Expected 1 element, got %d", len(h.els))
	}

	if !h.els[0].SelfClosing {
		t.Error("Expected SelfClosing to be true")
	}

	// Test BooleanAttribute
	h.Add(Tag("script"), h.Attr("src", "/script.js"), BooleanAttribute("async"), BooleanAttribute("defer"))

	if len(h.els) != 2 {
		t.Fatalf("Expected 2 elements, got %d", len(h.els))
	}

	scriptEl := h.els[1]
	if len(scriptEl.BooleanAttributes) != 2 {
		t.Errorf("Expected 2 boolean attributes, got %d", len(scriptEl.BooleanAttributes))
	}

	// Test InnerHTML
	h.Add(Tag("script"), InnerHTML("console.log('test');"))

	if len(h.els) != 3 {
		t.Fatalf("Expected 3 elements, got %d", len(h.els))
	}

	scriptWithInnerEl := h.els[2]
	if scriptWithInnerEl.DangerousInnerHTML != "console.log('test');" {
		t.Errorf("Expected innerHTML, got '%s'", scriptWithInnerEl.DangerousInnerHTML)
	}

	// Test rendering error handling
	inst := NewInstance("test")

	// Test rendering error handling
	invalidEl := &htmlutil.Element{
		Tag: "", // Empty tag should cause rendering error
	}

	sorted := &SortedAndPreEscapedHeadEls{
		Meta: []*htmlutil.Element{invalidEl},
	}

	_, err := inst.Render(sorted)
	if err == nil {
		t.Error("Expected error when rendering element with empty tag, but got nil")
	}
}

// TestDeduplicationWithMixedContentTypes tests deduplication with a mix of content types
func TestDeduplicationWithMixedContentTypes(t *testing.T) {
	inst := NewInstance("test")

	els := []*htmlutil.Element{
		// Two script tags with different inner content
		{Tag: "script", DangerousInnerHTML: "console.log('test1');"},
		{Tag: "script", DangerousInnerHTML: "console.log('test2');"},

		// Two identical meta tags
		{Tag: "meta", Attributes: map[string]string{"name": "viewport", "content": "width=device-width"}},
		{Tag: "meta", Attributes: map[string]string{"name": "viewport", "content": "width=device-width"}},

		// Self-closing tags
		{Tag: "link", Attributes: map[string]string{"rel": "icon", "href": "/favicon.ico"}, SelfClosing: true},
		{Tag: "link", Attributes: map[string]string{"rel": "icon", "href": "/favicon.ico"}, SelfClosing: true},

		// Mix of boolean attributes
		{Tag: "script", Attributes: map[string]string{"src": "/script.js"}, BooleanAttributes: []string{"async"}},
		{Tag: "script", Attributes: map[string]string{"src": "/script.js"}, BooleanAttributes: []string{"defer"}},
	}

	result := inst.dedupeHeadEls(els)

	// Should have 5 elements after deduplication (not 8)
	if len(result) != 6 {
		t.Errorf("Expected 6 elements after deduplication, got %d", len(result))
	}

	// Check that different inner HTML scripts are preserved
	scriptCount := 0
	for _, el := range result {
		if el.Tag == "script" && el.DangerousInnerHTML != "" {
			scriptCount++
		}
	}

	if scriptCount != 2 {
		t.Errorf("Expected 2 script tags with innerHTML, got %d", scriptCount)
	}

	// Check that only one viewport meta tag remains
	viewportCount := 0
	for _, el := range result {
		if el.Tag == "meta" && el.Attributes["name"] == "viewport" {
			viewportCount++
		}
	}

	if viewportCount != 1 {
		t.Errorf("Expected 1 viewport meta tag, got %d", viewportCount)
	}
}

// TestPanic tests that Add panics when no Tag is provided
func TestPanic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("Expected panic when adding element without Tag, but it didn't happen")
		}
	}()

	h := New()
	h.Add(h.Name("description")) // Should panic since no Tag provided
}
