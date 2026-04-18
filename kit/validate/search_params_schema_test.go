package validate

import (
	"reflect"
	"testing"
)

type SchemaEmbedded struct {
	Embedded string `json:"embedded"`
}

type SchemaPointerEmbedded struct {
	PointerEmbedded int `json:"pointerEmbedded"`
}

type SchemaNested struct {
	City string `json:"city"`
	Zip  int    `json:"zip"`
}

type SchemaString string

type schema_int int

func TestURLSearchParamsSchemaBuilder(t *testing.T) {
	type input struct {
		SchemaEmbedded
		*SchemaPointerEmbedded

		Query         string             `json:"q"`
		DefaultName   string             `json:",omitempty"`
		Page          int                `json:"page"`
		Enabled       bool               `json:"enabled"`
		Tags          []string           `json:"tags"`
		PointerTags   *[]string          `json:"pointerTags"`
		PointerScores []*float64         `json:"pointerScores"`
		Score         *float64           `json:"score"`
		Address       SchemaNested       `json:"address"`
		PointerAddr   *SchemaNested      `json:"pointerAddress"`
		Flags         map[string]bool    `json:"flags"`
		Labels        *map[string]string `json:"labels"`
		Groups        map[string][]uint  `json:"groups"`
		Skip          string             `json:"-"`
		SkipComma     string             `json:"-,"`
		private       string
	}

	schema, err := (URLSearchParamsSchemaBuilder{}).FromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]URLSearchParamsSchema{
		"embedded":        url_search_params_schema_string,
		"pointerEmbedded": url_search_params_schema_number,
		"q":               url_search_params_schema_string,
		"DefaultName":     url_search_params_schema_string,
		"page":            url_search_params_schema_number,
		"enabled":         url_search_params_schema_bool,
		"tags": []URLSearchParamsSchema{
			url_search_params_schema_string,
		},
		"pointerTags": []URLSearchParamsSchema{
			url_search_params_schema_string,
		},
		"pointerScores": []URLSearchParamsSchema{
			"?" + url_search_params_schema_number,
		},
		"score": "?" + url_search_params_schema_number,
		"address": map[string]URLSearchParamsSchema{
			"city": url_search_params_schema_string,
			"zip":  url_search_params_schema_number,
		},
		"pointerAddress": map[string]URLSearchParamsSchema{
			"city": url_search_params_schema_string,
			"zip":  url_search_params_schema_number,
		},
		"flags": []URLSearchParamsSchema{
			url_search_params_schema_map,
			url_search_params_schema_bool,
		},
		"labels": []URLSearchParamsSchema{
			url_search_params_schema_map,
			url_search_params_schema_string,
		},
		"groups": []URLSearchParamsSchema{
			url_search_params_schema_map,
			[]URLSearchParamsSchema{url_search_params_schema_number},
		},
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestURLSearchParamsSchemaBuilderScalarKinds(t *testing.T) {
	type input struct {
		Bool       bool           `json:"bool"`
		BoolPtr    *bool          `json:"boolPtr"`
		Int        int            `json:"int"`
		Int8       int8           `json:"int8"`
		Int16      int16          `json:"int16"`
		Int32      int32          `json:"int32"`
		Int64      int64          `json:"int64"`
		Uint       uint           `json:"uint"`
		Uint8      uint8          `json:"uint8"`
		Uint16     uint16         `json:"uint16"`
		Uint32     uint32         `json:"uint32"`
		Uint64     uint64         `json:"uint64"`
		Float32    float32        `json:"float32"`
		Float64    float64        `json:"float64"`
		String     string         `json:"string"`
		StringPtr  *string        `json:"stringPtr"`
		Alias      SchemaString   `json:"alias"`
		AliasPtr   *schema_int    `json:"aliasPtr"`
		NumberPtr  **schema_int   `json:"numberPtr"`
		BoolPtr2   **bool         `json:"boolPtr2"`
		StringPtr2 **SchemaString `json:"stringPtr2"`
	}

	schema, err := (URLSearchParamsSchemaBuilder{}).FromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]URLSearchParamsSchema{
		"bool":       url_search_params_schema_bool,
		"boolPtr":    "?" + url_search_params_schema_bool,
		"int":        url_search_params_schema_number,
		"int8":       url_search_params_schema_number,
		"int16":      url_search_params_schema_number,
		"int32":      url_search_params_schema_number,
		"int64":      url_search_params_schema_number,
		"uint":       url_search_params_schema_number,
		"uint8":      url_search_params_schema_number,
		"uint16":     url_search_params_schema_number,
		"uint32":     url_search_params_schema_number,
		"uint64":     url_search_params_schema_number,
		"float32":    url_search_params_schema_number,
		"float64":    url_search_params_schema_number,
		"string":     url_search_params_schema_string,
		"stringPtr":  "?" + url_search_params_schema_string,
		"alias":      url_search_params_schema_string,
		"aliasPtr":   "?" + url_search_params_schema_number,
		"numberPtr":  "?" + url_search_params_schema_number,
		"boolPtr2":   "?" + url_search_params_schema_bool,
		"stringPtr2": "?" + url_search_params_schema_string,
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestURLSearchParamsSchemaBuilderContainers(t *testing.T) {
	type input struct {
		StringArray      [2]string             `json:"stringArray"`
		BoolSlice        []bool                `json:"boolSlice"`
		PointerArray     *[2]int               `json:"pointerArray"`
		OptionalNumbers  []*int                `json:"optionalNumbers"`
		MapStrings       map[string]string     `json:"mapStrings"`
		MapOptionalBools map[string]*bool      `json:"mapOptionalBools"`
		MapNumberSlices  map[string][]uint     `json:"mapNumberSlices"`
		MapPointerSlices map[string][]*float64 `json:"mapPointerSlices"`
		PointerMap       *map[string]*string   `json:"pointerMap"`
	}

	schema, err := (URLSearchParamsSchemaBuilder{}).FromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]URLSearchParamsSchema{
		"stringArray": []URLSearchParamsSchema{
			url_search_params_schema_string,
		},
		"boolSlice": []URLSearchParamsSchema{
			url_search_params_schema_bool,
		},
		"pointerArray": []URLSearchParamsSchema{
			url_search_params_schema_number,
		},
		"optionalNumbers": []URLSearchParamsSchema{
			"?" + url_search_params_schema_number,
		},
		"mapStrings": []URLSearchParamsSchema{
			url_search_params_schema_map,
			url_search_params_schema_string,
		},
		"mapOptionalBools": []URLSearchParamsSchema{
			url_search_params_schema_map,
			"?" + url_search_params_schema_bool,
		},
		"mapNumberSlices": []URLSearchParamsSchema{
			url_search_params_schema_map,
			[]URLSearchParamsSchema{url_search_params_schema_number},
		},
		"mapPointerSlices": []URLSearchParamsSchema{
			url_search_params_schema_map,
			[]URLSearchParamsSchema{"?" + url_search_params_schema_number},
		},
		"pointerMap": []URLSearchParamsSchema{
			url_search_params_schema_map,
			"?" + url_search_params_schema_string,
		},
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestURLSearchParamsSchemaBuilderDeepNesting(t *testing.T) {
	type level3 struct {
		Field string `json:"field"`
	}
	type level2 struct {
		Level3 *level3 `json:"level3"`
	}
	type level1 struct {
		Level2 level2 `json:"level2"`
	}
	type input struct {
		Level1 *level1 `json:"level1"`
	}

	schema, err := (URLSearchParamsSchemaBuilder{}).FromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]URLSearchParamsSchema{
		"level1": map[string]URLSearchParamsSchema{
			"level2": map[string]URLSearchParamsSchema{
				"level3": map[string]URLSearchParamsSchema{
					"field": url_search_params_schema_string,
				},
			},
		},
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestURLSearchParamsSchemaBuilderFieldNames(t *testing.T) {
	type input struct {
		DefaultName string `json:",omitempty"`
		ExtraOpts   int    `json:"extra,string,omitempty"`
		Raw         bool
		Skip        string `json:"-"`
		SkipComma   string `json:"-,"`
		private     string
	}

	schema, err := (URLSearchParamsSchemaBuilder{}).FromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]URLSearchParamsSchema{
		"DefaultName": url_search_params_schema_string,
		"Raw":         url_search_params_schema_bool,
		"extra":       url_search_params_schema_number,
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestURLSearchParamsSchemaBuilderEmptyStruct(t *testing.T) {
	schema, err := (URLSearchParamsSchemaBuilder{}).FromValue(struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(schema, map[string]URLSearchParamsSchema{}) {
		t.Fatalf("schema mismatch: %#v", schema)
	}
}

func TestURLSearchParamsSchemaBuilderRootPointer(t *testing.T) {
	type input struct {
		Query string `json:"q"`
	}

	schema, err := (URLSearchParamsSchemaBuilder{}).FromValue(&input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]URLSearchParamsSchema{
		"q": url_search_params_schema_string,
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestURLSearchParamsSchemaBuilderRejectsUnsupportedShapes(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{
			name:  "nil",
			value: nil,
		},
		{
			name: "non string map key",
			value: struct {
				Data map[int]string `json:"data"`
			}{},
		},
		{
			name:  "root map",
			value: map[string]string{},
		},
		{
			name:  "root slice",
			value: []string{},
		},
		{
			name: "channel field",
			value: struct {
				Data chan string `json:"data"`
			}{},
		},
		{
			name: "complex field",
			value: struct {
				Data complex64 `json:"data"`
			}{},
		},
		{
			name: "function field",
			value: struct {
				Data func() `json:"data"`
			}{},
		},
		{
			name: "slice of structs",
			value: struct {
				Data []SchemaNested `json:"data"`
			}{},
		},
		{
			name: "slice of maps",
			value: struct {
				Data []map[string]string `json:"data"`
			}{},
		},
		{
			name: "map of structs",
			value: struct {
				Data map[string]SchemaNested `json:"data"`
			}{},
		},
		{
			name: "map of struct slices",
			value: struct {
				Data map[string][]SchemaNested `json:"data"`
			}{},
		},
		{
			name: "map of maps",
			value: struct {
				Data map[string]map[string]string `json:"data"`
			}{},
		},
		{
			name: "anonymous non struct",
			value: struct {
				SchemaString
			}{},
		},
		{
			name: "interface field",
			value: struct {
				Data any `json:"data"`
			}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := (URLSearchParamsSchemaBuilder{}).FromValue(tt.value)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
