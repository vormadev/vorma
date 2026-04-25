package searchparams

import (
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

type property_scalar_pointer_form struct {
	StringPtr  **string  `json:"stringPtr"`
	NumberPtr  **int     `json:"numberPtr"`
	BoolPtr    **bool    `json:"boolPtr"`
	StringPtr3 ***string `json:"stringPtr3"`
}

type property_scalar_pointer_case struct {
	string_mode  int
	number_mode  int
	bool_mode    int
	string3_mode int
}

type property_scalar_pointer_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestParseMultiPointerScalarsMatchModel(t *testing.T) {
	t.Run("generated_multi_pointer_scalar_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_scalar_pointer_case_space{}).draw_case(ht)
		tc.assert_matches_model(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_scalar_pointer_case_space) draw_case(
	ht *hegel.T,
) property_scalar_pointer_case {
	return property_scalar_pointer_case{
		string_mode:  hegel.Draw(ht, hegel.Integers(0, 2)),
		number_mode:  hegel.Draw(ht, hegel.Integers(0, 2)),
		bool_mode:    hegel.Draw(ht, hegel.Integers(0, 2)),
		string3_mode: hegel.Draw(ht, hegel.Integers(0, 2)),
	}
}

func (tc property_scalar_pointer_case) assert_matches_model(ht *hegel.T) {
	tc.note(ht)

	actual, err := ParseToStruct[property_scalar_pointer_form](tc.request())
	if err != nil {
		ht.Fatalf("unexpected error: %v", err)
	}

	expected := tc.expected_form()
	if !reflect.DeepEqual(actual, expected) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected)
	}
}

func (tc property_scalar_pointer_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))
}

func (tc property_scalar_pointer_case) request() *http.Request {
	query := url.Values{}
	for _, pair := range []struct {
		key   string
		value string
	}{
		{"stringPtr", tc.raw_value(tc.string_mode, "hello")},
		{"numberPtr", tc.raw_value(tc.number_mode, "7")},
		{"boolPtr", tc.raw_value(tc.bool_mode, "true")},
		{"stringPtr3", tc.raw_value(tc.string3_mode, "deep")},
	} {
		if pair.value == "<absent>" {
			continue
		}
		query.Add(pair.key, pair.value)
	}

	req := &http.Request{URL: &url.URL{}}
	req.URL.RawQuery = query.Encode()
	return req
}

func (tc property_scalar_pointer_case) expected_form() property_scalar_pointer_form {
	return property_scalar_pointer_form{
		StringPtr:  tc.expected_string_ptr(tc.string_mode),
		NumberPtr:  tc.expected_int_ptr(tc.number_mode),
		BoolPtr:    tc.expected_bool_ptr(tc.bool_mode),
		StringPtr3: tc.expected_string_ptr3(tc.string3_mode),
	}
}

func (tc property_scalar_pointer_case) raw_value(mode int, value string) string {
	switch mode {
	case 1:
		return ""
	case 2:
		return value
	default:
		return "<absent>"
	}
}

func (tc property_scalar_pointer_case) expected_string_ptr(mode int) **string {
	switch mode {
	case 1:
		return nil
	case 2:
		value := "hello"
		ptr := &value
		return &ptr
	default:
		return nil
	}
}

func (tc property_scalar_pointer_case) expected_int_ptr(mode int) **int {
	switch mode {
	case 1:
		return nil
	case 2:
		value := 7
		ptr := &value
		return &ptr
	default:
		return nil
	}
}

func (tc property_scalar_pointer_case) expected_bool_ptr(mode int) **bool {
	switch mode {
	case 1:
		return nil
	case 2:
		value := true
		ptr := &value
		return &ptr
	default:
		return nil
	}
}

func (tc property_scalar_pointer_case) expected_string_ptr3(mode int) ***string {
	switch mode {
	case 1:
		return nil
	case 2:
		value := "deep"
		ptr := &value
		ptr2 := &ptr
		return &ptr2
	default:
		return nil
	}
}
