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

type property_agreement_case struct {
	kind      int
	name_mode int
}

type property_agreement_case_space struct{}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

func TestParseAndSchemaAgreeOnSupportedAddressableShapes(t *testing.T) {
	t.Run("generated_supported_shape_space", hegel.Case(func(ht *hegel.T) {
		tc := (property_agreement_case_space{}).draw_case(ht)
		tc.assert_matches_model(ht)
	}, hegel.WithTestCases(500)))
}

/////////////////////////////////////////////////////////////////////
/////// HELPERS
/////////////////////////////////////////////////////////////////////

func (property_agreement_case_space) draw_case(ht *hegel.T) property_agreement_case {
	return property_agreement_case{
		kind:      hegel.Draw(ht, hegel.Integers(0, 8)),
		name_mode: hegel.Draw(ht, hegel.Integers(0, 4)),
	}
}

func (tc property_agreement_case) assert_matches_model(ht *hegel.T) {
	tc.note(ht)

	root_type, expected_value := tc.root_type_and_expected_value()
	dest := reflect.New(root_type)

	schema_value, err := SchemaFromValue(dest.Elem().Interface())
	if err != nil {
		ht.Fatalf("unexpected schema error: %v", err)
	}
	tc.assert_schema_key(ht, schema_value)

	err = ParseIntoStructPtr(tc.request(), dest.Interface())
	if err != nil {
		ht.Fatalf("unexpected parse error: %v", err)
	}

	actual := dest.Elem().Field(0).Interface()
	if !reflect.DeepEqual(actual, expected_value) {
		ht.Fatalf("actual = %#v, expected = %#v", actual, expected_value)
	}
}

func (tc property_agreement_case) note(ht *hegel.T) {
	ht.Note(fmt.Sprintf("case = %#v", tc))
}

func (tc property_agreement_case) assert_schema_key(ht *hegel.T, schema_value Schema) {
	schema_map, ok := schema_value.(map[string]Schema)
	if !ok {
		ht.Fatalf("expected root schema map, got %#v", schema_value)
	}
	_, exists := schema_map[field_name(0, tc.name_mode)]
	if tc.name_mode == 4 {
		if exists {
			ht.Fatalf("expected skipped field to be absent from schema, got %#v", schema_map)
		}
		return
	}
	if !exists {
		ht.Fatalf("expected schema key %q in %#v", field_name(0, tc.name_mode), schema_map)
	}
}

func (tc property_agreement_case) root_type_and_expected_value() (reflect.Type, any) {
	field := reflect.StructField{
		Name: "Field0",
		Type: tc.field_type(),
		Tag:  reflect.StructTag(tc.json_tag()),
	}
	return reflect.StructOf([]reflect.StructField{field}), tc.expected_value()
}

func (tc property_agreement_case) field_type() reflect.Type {
	switch tc.kind {
	case 0:
		return reflect.TypeFor[string]()
	case 1:
		return reflect.TypeFor[*string]()
	case 2:
		return reflect.TypeFor[**int]()
	case 3:
		return reflect.TypeFor[[]string]()
	case 4:
		return reflect.TypeFor[[2]string]()
	case 5:
		return reflect.TypeFor[*[2]int]()
	case 6:
		return reflect.TypeFor[map[string]string]()
	case 7:
		return reflect.TypeFor[map[string][]bool]()
	default:
		nested_field := reflect.StructField{
			Name: "Child",
			Type: reflect.TypeFor[string](),
			Tag:  `json:"child"`,
		}
		return reflect.PointerTo(reflect.StructOf([]reflect.StructField{nested_field}))
	}
}

func (tc property_agreement_case) expected_value() any {
	if tc.name_mode == 4 {
		return reflect.Zero(tc.field_type()).Interface()
	}

	switch tc.kind {
	case 0:
		return "hello"
	case 1:
		value := "hello"
		return &value
	case 2:
		value := 7
		ptr := &value
		return &ptr
	case 3:
		return []string{"a", "b"}
	case 4:
		return [2]string{"a", "b"}
	case 5:
		value := [2]int{1, 2}
		return &value
	case 6:
		return map[string]string{"k": "v"}
	case 7:
		return map[string][]bool{"k": {true, false}}
	default:
		nested := reflect.New(tc.field_type().Elem()).Elem()
		nested.Field(0).SetString("child")
		ptr := reflect.New(nested.Type())
		ptr.Elem().Set(nested)
		return ptr.Interface()
	}
}

func (tc property_agreement_case) json_tag() string {
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

func (tc property_agreement_case) request() *http.Request {
	query := url.Values{}
	name := field_name(0, tc.name_mode)

	if tc.name_mode != 4 {
		switch tc.kind {
		case 0, 1:
			query.Add(name, "hello")
		case 2:
			query.Add(name, "7")
		case 3, 4:
			query.Add(name, "a")
			query.Add(name, "b")
		case 5:
			query.Add(name, "1")
			query.Add(name, "2")
		case 6:
			query.Add(name+".k", "v")
		case 7:
			query.Add(name+".k", "true")
			query.Add(name+".k", "false")
		default:
			query.Add(name+".child", "child")
		}
	} else {
		query.Add("Field0", "ignored")
		query.Add("Field0.k", "ignored")
		query.Add("Field0.child", "ignored")
	}

	req := &http.Request{URL: &url.URL{}}
	req.URL.RawQuery = query.Encode()
	return req
}
