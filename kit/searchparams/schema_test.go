package searchparams

import (
	"errors"
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

func TestSchemaFromValue(t *testing.T) {
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
		//lint:ignore SA5008 .
		DashName string `json:"'-'"`
		//lint:ignore U1000 .
		private string
	}

	schema, err := SchemaFromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]Schema{
		"embedded":        schema_code_string,
		"pointerEmbedded": schema_code_number,
		"q":               schema_code_string,
		"DefaultName":     schema_code_string,
		"page":            schema_code_number,
		"enabled":         schema_code_bool,
		"tags":            []Schema{schema_code_string},
		"pointerTags":     []Schema{schema_code_string},
		"pointerScores":   []Schema{"?" + schema_code_number},
		"score":           "?" + schema_code_number,
		"address": map[string]Schema{
			"city": schema_code_string,
			"zip":  schema_code_number,
		},
		"pointerAddress": map[string]Schema{
			"city": schema_code_string,
			"zip":  schema_code_number,
		},
		"flags":  []Schema{schema_code_map, schema_code_bool},
		"labels": []Schema{schema_code_map, schema_code_string},
		"groups": []Schema{schema_code_map, []Schema{schema_code_number}},
		"-":      schema_code_string,
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestSchemaFromValueScalarKinds(t *testing.T) {
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

	schema, err := SchemaFromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]Schema{
		"bool":       schema_code_bool,
		"boolPtr":    "?" + schema_code_bool,
		"int":        schema_code_number,
		"int8":       schema_code_number,
		"int16":      schema_code_number,
		"int32":      schema_code_number,
		"int64":      schema_code_number,
		"uint":       schema_code_number,
		"uint8":      schema_code_number,
		"uint16":     schema_code_number,
		"uint32":     schema_code_number,
		"uint64":     schema_code_number,
		"float32":    schema_code_number,
		"float64":    schema_code_number,
		"string":     schema_code_string,
		"stringPtr":  "?" + schema_code_string,
		"alias":      schema_code_string,
		"aliasPtr":   "?" + schema_code_number,
		"numberPtr":  "?" + schema_code_number,
		"boolPtr2":   "?" + schema_code_bool,
		"stringPtr2": "?" + schema_code_string,
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestSchemaFromValueContainers(t *testing.T) {
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

	schema, err := SchemaFromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]Schema{
		"stringArray":      []Schema{schema_code_string},
		"boolSlice":        []Schema{schema_code_bool},
		"pointerArray":     []Schema{schema_code_number},
		"optionalNumbers":  []Schema{"?" + schema_code_number},
		"mapStrings":       []Schema{schema_code_map, schema_code_string},
		"mapOptionalBools": []Schema{schema_code_map, "?" + schema_code_bool},
		"mapNumberSlices":  []Schema{schema_code_map, []Schema{schema_code_number}},
		"mapPointerSlices": []Schema{schema_code_map, []Schema{"?" + schema_code_number}},
		"pointerMap":       []Schema{schema_code_map, "?" + schema_code_string},
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestSchemaFromValueDeepNesting(t *testing.T) {
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

	schema, err := SchemaFromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]Schema{
		"level1": map[string]Schema{
			"level2": map[string]Schema{
				"level3": map[string]Schema{
					"field": schema_code_string,
				},
			},
		},
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestSchemaFromValueFieldNames(t *testing.T) {
	type input struct {
		DefaultName string `json:",omitempty"`
		ExtraOpts   int    `json:"extra,string,omitempty"`
		Raw         bool
		Skip        string `json:"-"`
		//lint:ignore SA5008 .
		DashName string `json:"'-'"`
		//lint:ignore U1000 .
		private string
	}

	schema, err := SchemaFromValue(input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]Schema{
		"DefaultName": schema_code_string,
		"Raw":         schema_code_bool,
		"extra":       schema_code_number,
		"-":           schema_code_string,
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestSchemaFromValueEmptyStruct(t *testing.T) {
	schema, err := SchemaFromValue(struct{}{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !reflect.DeepEqual(schema, map[string]Schema{}) {
		t.Fatalf("schema mismatch: %#v", schema)
	}
}

func TestSchemaFromValueRootPointer(t *testing.T) {
	type input struct {
		Query string `json:"q"`
	}

	schema, err := SchemaFromValue(&input{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]Schema{
		"q": schema_code_string,
	}
	if !reflect.DeepEqual(schema, expected) {
		t.Fatalf("schema mismatch:\n got: %#v\nwant: %#v", schema, expected)
	}
}

func TestSchemaFromValueNil(t *testing.T) {
	_, err := SchemaFromValue(nil)
	if !errors.Is(err, ErrNilValueInSchema) {
		t.Fatalf("expected SchemaNilValueError, got %v", err)
	}
}

func TestSchemaFromValueRejectsUnsupportedShapes(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
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
			name:  "root scalar",
			value: 42,
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
			_, err := SchemaFromValue(tt.value)
			if err == nil {
				t.Fatal("expected error")
			}
		})
	}
}
