package headels

import (
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/htmlutil"
)

var testInstance = NewInstance("bob")

func TestGetHeadElements(t *testing.T) {
	routeData := &SortedAndPreEscapedHeadEls{
		Title: &htmlutil.Element{Tag: "title", TextContent: "Test Title"},
		Meta: []*htmlutil.Element{
			{Tag: "meta", Attributes: map[string]string{"name": "description", "content": "Test Description"}},
		},
		Rest: []*htmlutil.Element{
			{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
		},
	}

	html, err := testInstance.Render(routeData)
	if err != nil {
		t.Errorf("Expected no error, but got %v", err)
	}
	if !strings.Contains(string(html), "<title>Test Title</title>") {
		t.Errorf("Expected title tag, but it's missing")
	}
	if !strings.Contains(string(html), `name="description"`) || !strings.Contains(string(html), `content="Test Description"`) {
		t.Errorf("Expected meta description tag, but it's missing")
	}
	if !strings.Contains(string(html), `rel="stylesheet"`) || !strings.Contains(string(html), `href="/style.css"`) {
		t.Errorf("Expected link tag, but it's missing")
	}
}

const (
	testTitle         = "Test Title"
	testTitle_2       = "Different Test Title"
	testDescription   = "This is a test description."
	testDescription_2 = "This is a different test description."
)

// Test cases for dedupeHeadEls
func TestDedupeHeadEls(t *testing.T) {
	tests := []struct {
		name     string
		input    []*htmlutil.Element
		expected []*htmlutil.Element
	}{
		{
			name: "No duplicates, with title and description",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": testDescription}},
				{Tag: "meta", Attributes: map[string]string{"name": "og:image", "content": "image.webp"}},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": testDescription}},
				{Tag: "meta", Attributes: map[string]string{"name": "og:image", "content": "image.webp"}},
			},
		},
		{
			name: "With duplicates",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": testDescription}},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": testDescription_2}},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": testDescription_2}},
			},
		},
		{
			name: "With duplicates TrustedAttributes",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "meta", AttributesKnownSafe: map[string]string{"name": "description", "content": testDescription}},
				{Tag: "meta", AttributesKnownSafe: map[string]string{"name": "description", "content": testDescription_2}},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "meta", AttributesKnownSafe: map[string]string{"name": "description", "content": testDescription_2}},
			},
		},
		{
			name: "With duplicates mixed",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": testDescription}},
				{Tag: "meta", AttributesKnownSafe: map[string]string{"name": "description", "content": testDescription_2}},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "meta", AttributesKnownSafe: map[string]string{"name": "description", "content": testDescription_2}},
			},
		},
		{
			name: "No title or description",
			input: []*htmlutil.Element{
				{Tag: "meta", Attributes: map[string]string{"name": "keywords", "content": "go, test"}},
				{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
			},
			expected: []*htmlutil.Element{
				{Tag: "meta", Attributes: map[string]string{"name": "keywords", "content": "go, test"}},
				{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
			},
		},
		{
			name: "Multiple titles and descriptions",
			input: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle, Attributes: nil},
				{Tag: "title", TextContent: testTitle_2, Attributes: nil},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": "Description 1"}},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": "Description 2"}},
			},
			expected: []*htmlutil.Element{
				{Tag: "title", TextContent: testTitle_2, Attributes: nil},
				{Tag: "meta", Attributes: map[string]string{"name": "description", "content": "Description 2"}},
			},
		},
		{
			name: "Different tags with same attributes",
			input: []*htmlutil.Element{
				{Tag: "meta", Attributes: map[string]string{"name": "viewport", "content": "width=device-width, initial-scale=1"}},
				{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
				{Tag: "meta", Attributes: map[string]string{"name": "viewport", "content": "width=device-width, initial-scale=1"}},
			},
			expected: []*htmlutil.Element{
				{Tag: "meta", Attributes: map[string]string{"name": "viewport", "content": "width=device-width, initial-scale=1"}},
				{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testInstance.dedupeHeadEls(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Log("Result:")
				for _, block := range result {
					t.Logf("%+v", block)
				}

				t.Log("Expected:")
				for _, block := range tt.expected {
					t.Logf("%+v", block)
				}

				t.Errorf("dedupeHeadEls() = %v, expected %v", result, tt.expected)
			}
		})
	}
}

func TestDedupeHeadElsEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    []*htmlutil.Element
		expected []*htmlutil.Element
	}{
		{
			name: "Same tag different attributes",
			input: []*htmlutil.Element{
				{Tag: "meta", Attributes: map[string]string{"name": "viewport", "content": "width=device-width, initial-scale=1"}},
				{Tag: "meta", Attributes: map[string]string{"name": "charset", "content": "UTF-8"}},
			},
			expected: []*htmlutil.Element{
				{Tag: "meta", Attributes: map[string]string{"name": "viewport", "content": "width=device-width, initial-scale=1"}},
				{Tag: "meta", Attributes: map[string]string{"name": "charset", "content": "UTF-8"}},
			},
		},
		{
			name: "Script and link tags",
			input: []*htmlutil.Element{
				{Tag: "script", Attributes: map[string]string{"src": "/script.js"}},
				{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
			},
			expected: []*htmlutil.Element{
				{Tag: "script", Attributes: map[string]string{"src": "/script.js"}},
				{Tag: "link", Attributes: map[string]string{"rel": "stylesheet", "href": "/style.css"}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := testInstance.dedupeHeadEls(tt.input)
			if !reflect.DeepEqual(result, tt.expected) {
				t.Log("Result:")
				for _, block := range result {
					t.Logf("%+v", block)
				}

				t.Log("Expected:")
				for _, block := range tt.expected {
					t.Logf("%+v", block)
				}

				t.Errorf("dedupeHeadEls() = %v, expected %v", result, tt.expected)
			}
		})
	}
}
