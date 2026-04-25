package searchparams

import (
	"fmt"
	"net/http"
	"net/url"
	"reflect"
	"slices"
	"strconv"
	"testing"

	"hegel.dev/go/hegel"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type property_pointer_address struct {
	City string `json:"city"`
	Zip  *int   `json:"zip"`
}

type property_pointer_form struct {
	Scores  *[]int                    `json:"scores"`
	Meta    *map[string]string        `json:"meta"`
	Address *property_pointer_address `json:"address"`
}

type property_pointer_case struct {
	scores_direct_mode int
	scores_prefixed    map[string][]string
	meta_entries       map[string][]string
	address_city_mode  int
	address_zip_mode   int
}

type property_pointer_case_space struct{}

var property_pointer_int_values = []string{
	"",
	"1",
	"2",
	"5",
}

var property_pointer_string_values = []string{
	"",
	"alpha",
	"beta",
}

var property_pointer_child_keys = []string{
	"a",
	"b",
	"c.d",
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestParsePointerCompositeFieldsMatchModel(t *testing.T) {
	t.Run("generated_pointer_composite_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_pointer_case_space{}).draw_case(ht)
		tc.assert_matches_model(ht)
	}, hegel.WithTestCases(750)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_pointer_case_space) draw_case(ht *hegel.T) property_pointer_case {
	return property_pointer_case{
		scores_direct_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
		scores_prefixed:    (property_pointer_case_space{}).draw_values(ht, property_pointer_int_values, 4),
		meta_entries:       (property_pointer_case_space{}).draw_values(ht, property_pointer_string_values, 4),
		address_city_mode:  hegel.Draw(ht, hegel.Integers(0, 2)),
		address_zip_mode:   hegel.Draw(ht, hegel.Integers(0, 2)),
	}
}

func (property_pointer_case_space) draw_values(
	ht *hegel.T,
	value_space []string,
	max_keys int,
) map[string][]string {
	key_count := hegel.Draw(ht, hegel.Integers(0, max_keys))
	out := make(map[string][]string, key_count)
	for range key_count {
		key := hegel.Draw(ht, hegel.SampledFrom(property_pointer_child_keys))
		value_count := hegel.Draw(ht, hegel.Integers(0, 3))
		values := make([]string, 0, value_count)
		for range value_count {
			values = append(values, hegel.Draw(ht, hegel.SampledFrom(value_space)))
		}
		out[key] = values
	}
	return out
}

func (tc property_pointer_case) assert_matches_model(ht *hegel.T) {
	tc.note(ht)

	req := tc.request()

	actual, err := ParseToStruct[property_pointer_form](req)
	if err != nil {
		ht.Fatalf("unexpected error: %v", err)
	}

	expected := tc.expected_form()
	if !reflect.DeepEqual(actual, expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}
}

func (tc property_pointer_case) note(ht *hegel.T) {
	ht.Note(
		fmt.Sprintf(
			"case = {scores_direct_mode:%d scores_prefixed:%#v meta_entries:%#v address_city_mode:%d address_zip_mode:%d}",
			tc.scores_direct_mode,
			tc.scores_prefixed,
			tc.meta_entries,
			tc.address_city_mode,
			tc.address_zip_mode,
		),
	)
}

func (tc property_pointer_case) request() *http.Request {
	query := url.Values{}
	for _, value := range tc.direct_values(tc.scores_direct_mode) {
		query.Add("scores", value)
	}
	for key, values := range tc.scores_prefixed {
		for _, value := range values {
			query.Add("scores."+key, value)
		}
	}
	for key, values := range tc.meta_entries {
		for _, value := range values {
			query.Add("meta."+key, value)
		}
	}
	for _, value := range tc.direct_values(tc.address_city_mode) {
		query.Add("address.city", value)
	}
	for _, value := range tc.direct_values(tc.address_zip_mode) {
		query.Add("address.zip", value)
	}

	req := &http.Request{URL: &url.URL{}}
	req.URL.RawQuery = query.Encode()
	return req
}

func (tc property_pointer_case) expected_form() property_pointer_form {
	expected := property_pointer_form{}

	if tc.scores_present() {
		scores := tc.expected_scores()
		expected.Scores = &scores
	}

	if tc.meta_present() {
		meta := tc.expected_meta()
		expected.Meta = &meta
	}

	if tc.address_present() {
		address := property_pointer_address{
			City: tc.expected_city(),
			Zip:  tc.expected_zip(),
		}
		expected.Address = &address
	}

	return expected
}

func (tc property_pointer_case) scores_present() bool {
	if len(tc.direct_values(tc.scores_direct_mode)) > 0 {
		return true
	}
	for _, values := range tc.scores_prefixed {
		if len(values) > 0 {
			return true
		}
	}
	return false
}

func (tc property_pointer_case) meta_present() bool {
	for _, values := range tc.meta_entries {
		if len(values) > 0 {
			return true
		}
	}
	return false
}

func (tc property_pointer_case) address_present() bool {
	return len(tc.direct_values(tc.address_city_mode)) > 0 ||
		len(tc.direct_values(tc.address_zip_mode)) > 0
}

func (tc property_pointer_case) expected_scores() []int {
	direct_values := tc.direct_values(tc.scores_direct_mode)
	var raw_values []string
	if tc.scores_direct_mode != 0 {
		raw_values = tc.filtered_values(direct_values)
	} else {
		keys := make([]string, 0, len(tc.scores_prefixed))
		for key := range tc.scores_prefixed {
			keys = append(keys, key)
		}
		slices.Sort(keys)
		for _, key := range keys {
			raw_values = append(raw_values, tc.filtered_values(tc.scores_prefixed[key])...)
		}
	}

	out := make([]int, 0, len(raw_values))
	for _, raw := range raw_values {
		n, err := strconv.Atoi(raw)
		if err != nil {
			panic(err)
		}
		out = append(out, n)
	}
	return out
}

func (tc property_pointer_case) expected_meta() map[string]string {
	out := make(map[string]string, len(tc.meta_entries))
	for key, values := range tc.meta_entries {
		if len(values) == 0 {
			continue
		}
		out[key] = values[0]
	}
	return out
}

func (tc property_pointer_case) expected_city() string {
	values := tc.direct_values(tc.address_city_mode)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func (tc property_pointer_case) expected_zip() *int {
	values := tc.direct_values(tc.address_zip_mode)
	if len(values) == 0 || values[0] == "" {
		return nil
	}
	n, err := strconv.Atoi(values[0])
	if err != nil {
		panic(err)
	}
	return &n
}

func (tc property_pointer_case) direct_values(mode int) []string {
	switch mode {
	case 1:
		return []string{"", "1", "2"}
	case 2:
		return []string{"5"}
	default:
		return nil
	}
}

func (tc property_pointer_case) filtered_values(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "" {
			out = append(out, value)
		}
	}
	return out
}
