package searchparams

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_array_form struct {
	Tags   [2]string `json:"tags"`
	Scores *[2]int   `json:"scores"`
}

type property_array_case struct {
	tags_mode   int
	scores_mode int
}

type property_array_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestParseArraysMatchModel(t *testing.T) {
	t.Run("generated_array_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_array_case_space{}).draw_case(ht)
		tc.assert_matches_model(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_array_case_space) draw_case(ht *hegel.T) property_array_case {
	return property_array_case{
		tags_mode:   hegel.Draw(ht, hegel.Integers(0, 3)),
		scores_mode: hegel.Draw(ht, hegel.Integers(0, 3)),
	}
}

func (tc property_array_case) assert_matches_model(ht *hegel.T) {
	tc.note(ht)

	actual, err := ParseToStruct[property_array_form](tc.request())
	expected, expected_err := tc.expected_form()

	if expected_err != "" {
		if err == nil {
			ht.Fatalf("expected error %q, got nil", expected_err)
		}
		if !errors.Is(err, ErrCannotParse) {
			ht.Fatalf("expected ParseError, got %v", err)
		}
		if !reflect.DeepEqual(actual, expected) {
			ht.Fatalf("expected partial value %#v on error, got %#v", expected, actual)
		}
		return
	}

	if err != nil {
		ht.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}
}

func (tc property_array_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))
}

func (tc property_array_case) request() *http.Request {
	query := url.Values{}
	for _, value := range tc.raw_values(tc.tags_mode, false) {
		query.Add("tags", value)
	}
	for _, value := range tc.raw_values(tc.scores_mode, true) {
		query.Add("scores", value)
	}

	req := &http.Request{URL: &url.URL{}}
	req.URL.RawQuery = query.Encode()
	return req
}

func (tc property_array_case) expected_form() (property_array_form, string) {
	expected := property_array_form{}

	tag_values := tc.filtered_values(tc.raw_values(tc.tags_mode, false))
	if len(tag_values) > len(expected.Tags) {
		return property_array_form{}, "tags overflow"
	}
	copy(expected.Tags[:], tag_values)

	score_values := tc.filtered_values(tc.raw_values(tc.scores_mode, true))
	if len(score_values) > 2 {
		scores := [2]int{}
		expected.Scores = &scores
		return expected, "scores overflow"
	}
	if tc.scores_mode != 0 {
		scores := [2]int{}
		for i, value := range score_values {
			var parsed int
			fmt.Sscanf(value, "%d", &parsed)
			scores[i] = parsed
		}
		expected.Scores = &scores
	}

	return expected, ""
}

func (tc property_array_case) raw_values(mode int, numeric bool) []string {
	switch mode {
	case 1:
		if numeric {
			return []string{"1", "2"}
		}
		return []string{"a", "b"}
	case 2:
		if numeric {
			return []string{"", "2"}
		}
		return []string{"", "b"}
	case 3:
		if numeric {
			return []string{"1", "2", "3"}
		}
		return []string{"a", "b", "c"}
	default:
		return nil
	}
}

func (tc property_array_case) filtered_values(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
