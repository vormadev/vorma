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

type property_unsupported_parse_case struct {
	kind      int
	name_mode int
}

type property_unsupported_parse_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestParseUnsupportedAddressedShapesMatchModel(t *testing.T) {
	t.Run("generated_unsupported_shape_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_unsupported_parse_case_space{}).draw_case(ht)
		tc.assert_matches_model(ht)
	}, hegel.WithTestCases(250)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_unsupported_parse_case_space) draw_case(
	ht *hegel.T,
) property_unsupported_parse_case {
	return property_unsupported_parse_case{
		kind:      hegel.Draw(ht, hegel.Integers(0, 5)),
		name_mode: hegel.Draw(ht, hegel.Integers(0, 4)),
	}
}

func (tc property_unsupported_parse_case) assert_matches_model(ht *hegel.T) {
	tc.note(ht)

	dest := reflect.New(tc.root_type()).Interface()
	err := ParseIntoStructPtr(tc.request(), dest)

	if tc.name_mode == 4 {
		if err != nil {
			ht.Fatalf("expected skipped field to be ignored, got %v", err)
		}
		return
	}

	if err == nil {
		ht.Fatalf("expected error for unsupported addressed shape")
	}
	if !errors.Is(err, ErrCannotParse) {
		ht.Fatalf("expected ParseError, got %v", err)
	}
}

func (tc property_unsupported_parse_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))
}

func (tc property_unsupported_parse_case) root_type() reflect.Type {
	field := reflect.StructField{
		Name: "Field0",
		Type: tc.field_type(),
		Tag:  reflect.StructTag(tc.json_tag()),
	}
	return reflect.StructOf([]reflect.StructField{field})
}

func (tc property_unsupported_parse_case) field_type() reflect.Type {
	switch tc.kind {
	case 0:
		nested := reflect.StructOf([]reflect.StructField{{
			Name: "Inner",
			Type: reflect.TypeFor[string](),
			Tag:  `json:"inner"`,
		}})
		return reflect.SliceOf(nested)
	case 1:
		return reflect.TypeFor[map[string]map[string]string]()
	case 2:
		nested := reflect.StructOf([]reflect.StructField{{
			Name: "Inner",
			Type: reflect.TypeFor[string](),
			Tag:  `json:"inner"`,
		}})
		return reflect.MapOf(
			reflect.TypeFor[string](),
			reflect.SliceOf(nested),
		)
	case 3:
		return reflect.TypeFor[chan int]()
	case 4:
		return reflect.TypeFor[any]()
	default:
		return reflect.TypeFor[func()]()
	}
}

func (tc property_unsupported_parse_case) json_tag() string {
	switch tc.name_mode {
	case 1:
		return `json:"named_0"`
	case 2:
		return `json:",omitempty"`
	case 3:
		return `json:"named_0,omitempty"`
	case 4:
		return `json:"-"`
	default:
		return ""
	}
}

func (tc property_unsupported_parse_case) field_name() string {
	switch tc.name_mode {
	case 1, 3:
		return "named_0"
	default:
		return "Field0"
	}
}

func (tc property_unsupported_parse_case) request() *http.Request {
	query := url.Values{}
	name := tc.field_name()
	switch tc.kind {
	case 1:
		query.Add(name+".outer.inner", "value")
	case 2:
		query.Add(name+".outer", "value")
	default:
		query.Add(name, "value")
	}
	req := &http.Request{URL: &url.URL{}}
	req.URL.RawQuery = query.Encode()
	return req
}
