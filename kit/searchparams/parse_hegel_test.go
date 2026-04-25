package searchparams

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_slice_fallback_form struct {
	Tags    []string `json:"tags"`
	Address struct {
		Phones []string `json:"phones"`
	} `json:"address"`
}

type property_slice_fallback_case struct {
	tags_direct_mode   int
	tags_prefixed      map[string][]string
	phones_direct_mode int
	phones_prefixed    map[string][]string
}

type property_slice_fallback_case_space struct{}

var property_slice_fallback_values = []string{
	"",
	"  one  ",
	"two",
	"",
	"three",
}

var property_slice_fallback_child_keys = []string{
	"alpha",
	"beta",
	"gamma",
	"x.y",
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestParseSliceFallbackMatchesDeterministicModel(t *testing.T) {
	t.Run("generated_prefix_fallback_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_slice_fallback_case_space{}).draw_case(ht)
		tc.assert_matches_model(ht)
	}, hegel.WithTestCases(750)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_slice_fallback_case_space) draw_case(
	ht *hegel.T,
) property_slice_fallback_case {
	return property_slice_fallback_case{
		tags_direct_mode:   hegel.Draw(ht, hegel.Integers(0, 2)),
		tags_prefixed:      (property_slice_fallback_case_space{}).draw_prefixed_values(ht, 4),
		phones_direct_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
		phones_prefixed:    (property_slice_fallback_case_space{}).draw_prefixed_values(ht, 4),
	}
}

func (property_slice_fallback_case_space) draw_prefixed_values(
	ht *hegel.T,
	max_keys int,
) map[string][]string {
	key_count := hegel.Draw(ht, hegel.Integers(0, max_keys))
	out := make(map[string][]string, key_count)
	for range key_count {
		key := hegel.Draw(ht, hegel.SampledFrom(property_slice_fallback_child_keys))
		value_count := hegel.Draw(ht, hegel.Integers(0, 3))
		values := make([]string, 0, value_count)
		for range value_count {
			values = append(values, hegel.Draw(ht, hegel.SampledFrom(property_slice_fallback_values)))
		}
		out[key] = values
	}
	return out
}

func (tc property_slice_fallback_case) assert_matches_model(ht *hegel.T) {
	tc.note(ht)

	req := tc.request()

	expected := tc.expected_form()

	actual, err := ParseToStruct[property_slice_fallback_form](req)
	if err != nil {
		ht.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(actual, expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}

	first, err := ParseToStruct[property_slice_fallback_form](req)
	if err != nil {
		ht.Fatalf("unexpected error on repeated parse: %v", err)
	}
	second, err := ParseToStruct[property_slice_fallback_form](req)
	if err != nil {
		ht.Fatalf("unexpected error on repeated parse: %v", err)
	}
	if !reflect.DeepEqual(first, second) {
		ht.Fatalf("first = %#v, second = %#v", first, second)
	}
}

func (tc property_slice_fallback_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {tags_direct_mode:%d tags_prefixed:%#v phones_direct_mode:%d phones_prefixed:%#v}",
			tc.tags_direct_mode,
			tc.tags_prefixed,
			tc.phones_direct_mode,
			tc.phones_prefixed,
		),
	)
}

func (tc property_slice_fallback_case) request() *http.Request {
	query := url.Values{}
	for _, value := range tc.direct_values(tc.tags_direct_mode) {
		query.Add("tags", value)
	}
	for key, values := range tc.tags_prefixed {
		for _, value := range values {
			query.Add("tags."+key, value)
		}
	}
	for _, value := range tc.direct_values(tc.phones_direct_mode) {
		query.Add("address.phones", value)
	}
	for key, values := range tc.phones_prefixed {
		for _, value := range values {
			query.Add("address.phones."+key, value)
		}
	}
	query.Add("ignored", "value")
	query.Add("ignored.child", "value")

	req := &http.Request{URL: &url.URL{}}
	req.URL.RawQuery = query.Encode()
	return req
}

func (tc property_slice_fallback_case) expected_form() property_slice_fallback_form {
	expected := property_slice_fallback_form{}
	expected.Tags = tc.expected_slice(tc.tags_direct_mode, tc.tags_prefixed)
	expected.Address.Phones = tc.expected_slice(
		tc.phones_direct_mode,
		tc.phones_prefixed,
	)
	return expected
}

func (tc property_slice_fallback_case) expected_slice(
	direct_mode int,
	prefixed map[string][]string,
) []string {
	direct_values := tc.direct_values(direct_mode)
	if direct_mode != 0 {
		return tc.filtered_values(direct_values)
	}

	keys := make([]string, 0, len(prefixed))
	for key := range prefixed {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	var out []string
	for _, key := range keys {
		for _, value := range prefixed[key] {
			if value != "" {
				out = append(out, value)
			}
		}
	}
	if out == nil {
		return []string{}
	}
	return out
}

func (tc property_slice_fallback_case) direct_values(mode int) []string {
	switch mode {
	case 1:
		return []string{"", "two", "", "three"}
	case 2:
		return []string{"  one  ", "", "two"}
	default:
		return nil
	}
}

func (tc property_slice_fallback_case) filtered_values(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
