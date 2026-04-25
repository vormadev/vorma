package searchparams

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

type BasicTypes struct {
	Name   string  `json:"name"`
	Age    int     `json:"age"`
	Active bool    `json:"active"`
	Height float64 `json:"height"`
}

type BasicTypesPtrs struct {
	Name   *string  `json:"name"`
	Age    *int     `json:"age"`
	Active *bool    `json:"active"`
	Height *float64 `json:"height"`
}

type StringSlice struct {
	Tags []string `json:"tags"`
}

type MixedTypes struct {
	Name   string `json:"name"`
	Age    int    `json:"age"`
	Scores []int  `json:"scores"`
}

type PointerFields struct {
	Name       *string  `json:"name"`
	Age        *int     `json:"age"`
	Salary     *float64 `json:"salary"`
	IsEmployee *bool    `json:"isEmployee"`
}

type SliceOfPointers struct {
	Scores []*int `json:"scores"`
}

type NestedStruct struct {
	Name    string `json:"name"`
	Address struct {
		City string `json:"city"`
		Zip  int    `json:"zip"`
	} `json:"address"`
}

type DoubleNestedStruct struct {
	Name    string `json:"name"`
	Address struct {
		City     string `json:"city"`
		Zip      int    `json:"zip"`
		Location struct {
			Lat float64 `json:"lat"`
			Lng float64 `json:"lng"`
		} `json:"location"`
	} `json:"address"`
}

type NestedWithSlice struct {
	Name    string `json:"name"`
	Address struct {
		City   string   `json:"city"`
		Zip    int      `json:"zip"`
		Phones []string `json:"phones"`
	} `json:"address"`
}

type Embedded struct {
	EmbeddedField string `json:"embeddedField"`
}

type DoubleEmbedded struct {
	Embedded
	EmbeddedField2 string `json:"embeddedField2"`
}

type EmbeddedContainerDirect struct {
	Embedded
}

type EmbeddedContainerPtr struct {
	*Embedded
}

type ExplicitInline struct {
	Inline ExplicitInlineInner `json:",inline"`
}

type ExplicitInlineInner struct {
	Value string `json:"value"`
}

type DominatedEmbedded struct {
	Embedded
	EmbeddedField string `json:"embeddedField"`
}

type StringMap struct {
	Data map[string]string `json:"data"`
}

type StringSliceMap struct {
	Data map[string][]string `json:"data"`
}

type TripleNestedStruct struct {
	Level1 struct {
		Level2 struct {
			Level3 struct {
				Field string `json:"field"`
			} `json:"level3"`
		} `json:"level2"`
	} `json:"level1"`
}

type DoubleNestedWithPointers struct {
	Level1 struct {
		Level2 *struct {
			Field string `json:"field"`
		} `json:"level2"`
	} `json:"level1"`
}

type MixedMaps struct {
	StringMap map[string]string `json:"stringMap"`
	IntMap    map[string]int    `json:"intMap"`
	BoolMap   map[string]bool   `json:"boolMap"`
}

type MapPointer struct {
	Data *map[string]string `json:"data"`
}

type StructPointer struct {
	Data *struct {
		Key1 string `json:"key1"`
		Key2 string `json:"key2"`
	} `json:"data"`
}

type SlicePointer struct {
	Data *[]string `json:"data"`
}

type ComplexPointerMix struct {
	NamePtr       *string   `json:"name_ptr"`
	Name          string    `json:"name"`
	AgePtr        *int      `json:"age_ptr"`
	Age           int       `json:"age"`
	IsFunPtr      *bool     `json:"isFun_ptr"`
	IsFun         bool      `json:"isFun"`
	TagsPtr       *[]string `json:"tags_ptr"`
	Tags          []string  `json:"tags"`
	SomeStructPtr *struct {
		Field string `json:"field"`
	} `json:"someStruct_ptr"`
	SomeStruct struct {
		Field string `json:"field"`
	} `json:"someStruct"`
	SomeMapPtr *map[string]string `json:"someMap_ptr"`
	SomeMap    map[string]string  `json:"someMap"`
}

type UnsupportedType struct {
	ChanField chan int `json:"chanValue"`
}

func TestParse(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		parse      func(*http.Request) (any, error)
		check      func(any) bool
		shouldFail bool
	}{
		{
			name:  "basic types",
			url:   "http://example.com?name=John&age=30&active=true&height=1.75",
			parse: parser[BasicTypes](),
			check: func(v any) bool {
				d := v.(BasicTypes)
				return d.Name == "John" && d.Age == 30 && d.Active && d.Height == 1.75
			},
		},
		{
			name:  "slice of strings",
			url:   "http://example.com?tags=go&tags=programming&tags=test",
			parse: parser[StringSlice](),
			check: func(v any) bool {
				return reflect.DeepEqual(
					v.(StringSlice).Tags,
					[]string{"go", "programming", "test"},
				)
			},
		},
		{
			name:  "mixed types",
			url:   "http://example.com?name=Alice&age=25&scores=90&scores=85&scores=95",
			parse: parser[MixedTypes](),
			check: func(v any) bool {
				d := v.(MixedTypes)
				return d.Name == "Alice" && d.Age == 25 &&
					reflect.DeepEqual(d.Scores, []int{90, 85, 95})
			},
		},
		{
			name:  "pointer fields",
			url:   "http://example.com?name=Jane&age=28&salary=50000.50&isEmployee=true",
			parse: parser[PointerFields](),
			check: func(v any) bool {
				d := v.(PointerFields)
				return d.Name != nil && *d.Name == "Jane" &&
					d.Age != nil && *d.Age == 28 &&
					d.Salary != nil && *d.Salary == 50000.50 &&
					d.IsEmployee != nil && *d.IsEmployee
			},
		},
		{
			name:  "missing pointer fields remain nil",
			url:   "http://example.com?name=John&age=30",
			parse: parser[PointerFields](),
			check: func(v any) bool {
				d := v.(PointerFields)
				return d.Name != nil && *d.Name == "John" &&
					d.Age != nil && *d.Age == 30 &&
					d.Salary == nil && d.IsEmployee == nil
			},
		},
		{
			name:  "slice of pointers",
			url:   "http://example.com?scores=90&scores=85&scores=95",
			parse: parser[SliceOfPointers](),
			check: func(v any) bool {
				d := v.(SliceOfPointers)
				want := []int{90, 85, 95}
				if len(d.Scores) != len(want) {
					return false
				}
				for i, s := range d.Scores {
					if *s != want[i] {
						return false
					}
				}
				return true
			},
		},
		{
			name:  "empty values yield nil for pointer fields",
			url:   "http://example.com?name=&age=&active=",
			parse: parser[BasicTypesPtrs](),
			check: func(v any) bool {
				d := v.(BasicTypesPtrs)
				return d.Name == nil && d.Age == nil && d.Active == nil
			},
		},
		{
			name:  "empty values yield zero for non-pointer fields",
			url:   "http://example.com?name=&age=&active=",
			parse: parser[BasicTypes](),
			check: func(v any) bool {
				d := v.(BasicTypes)
				return d.Name == "" && d.Age == 0 && !d.Active
			},
		},
		{
			name: "type mismatch fails",
			url:  "http://example.com?age=notanumber",
			parse: parser[struct {
				Age int `json:"age"`
			}](),
			shouldFail: true,
		},
		{
			name:  "nested structs",
			url:   "http://example.com?name=John&address.city=NewYork&address.zip=10001",
			parse: parser[NestedStruct](),
			check: func(v any) bool {
				d := v.(NestedStruct)
				return d.Name == "John" &&
					d.Address.City == "NewYork" &&
					d.Address.Zip == 10001
			},
		},
		{
			name:  "double nested structs",
			url:   "http://example.com?name=John&address.city=NewYork&address.zip=10001&address.location.lat=40.7128&address.location.lng=-74.0060",
			parse: parser[DoubleNestedStruct](),
			check: func(v any) bool {
				d := v.(DoubleNestedStruct)
				return d.Name == "John" &&
					d.Address.City == "NewYork" &&
					d.Address.Zip == 10001 &&
					d.Address.Location.Lat == 40.7128 &&
					d.Address.Location.Lng == -74.0060
			},
		},
		{
			name:  "nested struct with slice",
			url:   "http://example.com?name=John&address.city=NewYork&address.zip=10001&address.phones=1234567890&address.phones=0987654321",
			parse: parser[NestedWithSlice](),
			check: func(v any) bool {
				d := v.(NestedWithSlice)
				return d.Name == "John" &&
					d.Address.City == "NewYork" &&
					d.Address.Zip == 10001 &&
					reflect.DeepEqual(d.Address.Phones, []string{"1234567890", "0987654321"})
			},
		},
		{
			name:  "embedded struct -- direct",
			url:   "http://example.com?embeddedField=embeddedValue",
			parse: parser[EmbeddedContainerDirect](),
			check: func(v any) bool {
				return v.(EmbeddedContainerDirect).EmbeddedField == "embeddedValue"
			},
		},
		{
			name:  "embedded struct -- pointer",
			url:   "http://example.com?embeddedField=embeddedValue",
			parse: parser[EmbeddedContainerPtr](),
			check: func(v any) bool {
				return v.(EmbeddedContainerPtr).EmbeddedField == "embeddedValue"
			},
		},
		{
			name:  "double embedded",
			url:   "http://example.com?embeddedField=embeddedValue&embeddedField2=embeddedValue2",
			parse: parser[DoubleEmbedded](),
			check: func(v any) bool {
				d := v.(DoubleEmbedded)
				return d.EmbeddedField == "embeddedValue" &&
					d.EmbeddedField2 == "embeddedValue2"
			},
		},
		{
			name:  "explicit inline",
			url:   "http://example.com?value=inlineValue",
			parse: parser[ExplicitInline](),
			check: func(v any) bool {
				return v.(ExplicitInline).Inline.Value == "inlineValue"
			},
		},
		{
			name:  "embedded field dominance",
			url:   "http://example.com?embeddedField=topValue",
			parse: parser[DominatedEmbedded](),
			check: func(v any) bool {
				d := v.(DominatedEmbedded)
				return d.EmbeddedField == "topValue" &&
					d.Embedded.EmbeddedField == ""
			},
		},
		{
			name:  "basic map",
			url:   "http://example.com?data.key1=value1&data.key2=value2",
			parse: parser[StringMap](),
			check: func(v any) bool {
				d := v.(StringMap)
				return d.Data["key1"] == "value1" && d.Data["key2"] == "value2"
			},
		},
		{
			name:  "map with slice values",
			url:   "http://example.com?data.tags=go&data.tags=programming&data.scores=85&data.scores=90",
			parse: parser[StringSliceMap](),
			check: func(v any) bool {
				d := v.(StringSliceMap)
				return reflect.DeepEqual(d.Data["tags"], []string{"go", "programming"}) &&
					reflect.DeepEqual(d.Data["scores"], []string{"85", "90"})
			},
		},
		{
			name:  "empty map",
			url:   "http://example.com",
			parse: parser[StringMap](),
			check: func(v any) bool {
				return len(v.(StringMap).Data) == 0
			},
		},
		{
			name:  "map with empty values",
			url:   "http://example.com?data.key1=&data.key2=",
			parse: parser[StringMap](),
			check: func(v any) bool {
				d := v.(StringMap)
				return d.Data["key1"] == "" && d.Data["key2"] == ""
			},
		},
		{
			name: "map with pointer values",
			url:  "http://example.com?data.name=John&data.age=30&data.active=true",
			parse: parser[struct {
				Data map[string]*string `json:"data"`
			}](),
			check: func(v any) bool {
				d := v.(struct {
					Data map[string]*string `json:"data"`
				})
				return *d.Data["name"] == "John" &&
					*d.Data["age"] == "30" &&
					*d.Data["active"] == "true"
			},
		},
		{
			name:  "multiple maps of different value types",
			url:   "http://example.com?stringMap.key1=value1&intMap.key2=42&boolMap.key3=true",
			parse: parser[MixedMaps](),
			check: func(v any) bool {
				d := v.(MixedMaps)
				return d.StringMap["key1"] == "value1" &&
					d.IntMap["key2"] == 42 &&
					d.BoolMap["key3"]
			},
		},
		{
			name: "map with invalid type conversion fails",
			url:  "http://example.com?data.key=notanumber",
			parse: parser[struct {
				Data map[string]int `json:"data"`
			}](),
			shouldFail: true,
		},
		{
			name: "map with mixed valid and invalid values fails",
			url:  "http://example.com?data.valid=42&data.invalid=notanumber",
			parse: parser[struct {
				Data map[string]int `json:"data"`
			}](),
			shouldFail: true,
		},
		{
			name:  "map key with dot treated as flat key",
			url:   "http://example.com?data.key.with.dot=value",
			parse: parser[StringMap](),
			check: func(v any) bool {
				return v.(StringMap).Data["key.with.dot"] == "value"
			},
		},
		{
			name:  "pointer to map",
			url:   "http://example.com?data.key1=value1&data.key2=value2",
			parse: parser[MapPointer](),
			check: func(v any) bool {
				d := v.(MapPointer)
				return (*d.Data)["key1"] == "value1" && (*d.Data)["key2"] == "value2"
			},
		},
		{
			name:  "pointer to struct",
			url:   "http://example.com?data.key1=value1&data.key2=value2",
			parse: parser[StructPointer](),
			check: func(v any) bool {
				d := v.(StructPointer)
				return d.Data.Key1 == "value1" && d.Data.Key2 == "value2"
			},
		},
		{
			name:  "pointer to slice",
			url:   "http://example.com?data=value1&data=value2",
			parse: parser[SlicePointer](),
			check: func(v any) bool {
				d := v.(SlicePointer)
				return len(*d.Data) == 2 &&
					(*d.Data)[0] == "value1" &&
					(*d.Data)[1] == "value2"
			},
		},
		{
			name:       "unsupported type fails",
			url:        "http://example.com?chanValue=something",
			parse:      parser[UnsupportedType](),
			shouldFail: true,
		},
		{
			name:  "triple nested structs",
			url:   "http://example.com?level1.level2.level3.field=value",
			parse: parser[TripleNestedStruct](),
			check: func(v any) bool {
				return v.(TripleNestedStruct).Level1.Level2.Level3.Field == "value"
			},
		},
		{
			name:  "double nested with pointer",
			url:   "http://example.com?level1.level2.field=value",
			parse: parser[DoubleNestedWithPointers](),
			check: func(v any) bool {
				return v.(DoubleNestedWithPointers).Level1.Level2.Field == "value"
			},
		},
		{
			// Primitive types should be nil for pointers and zero values for non-pointers.
			// Complex types are initialized to their zero values regardless.
			name:  "empty query parameters across mixed shapes",
			url:   "http://example.com?name_ptr=&name=&age_ptr=&age=&tags_ptr=&tags=&someStruct_ptr=&someStruct=&someMap_ptr=&someMap=",
			parse: parser[ComplexPointerMix](),
			check: func(v any) bool {
				d := v.(ComplexPointerMix)
				return d.NamePtr == nil && d.Name == "" &&
					d.AgePtr == nil && d.Age == 0 &&
					d.IsFunPtr == nil && !d.IsFun &&
					d.TagsPtr != nil && len(*d.TagsPtr) == 0 &&
					len(d.Tags) == 0 &&
					d.SomeStructPtr != nil && d.SomeStructPtr.Field == "" &&
					d.SomeStruct.Field == "" &&
					d.SomeMapPtr != nil && len(*d.SomeMapPtr) == 0 &&
					len(d.SomeMap) == 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", tt.url, nil)
			got, err := tt.parse(r)

			if tt.shouldFail {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if !errors.Is(err, ParseError) {
					t.Errorf("expected ParseError, got %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.check != nil && !tt.check(got) {
				t.Errorf("check failed for %+v", got)
			}
		})
	}
}

func TestParseNilRequest(t *testing.T) {
	_, err := ParseToStruct[BasicTypes](nil)
	if !errors.Is(err, ParseNilRequestError) {
		t.Fatalf("expected ParseNilRequestError, got %v", err)
	}
}

func TestParseNilURL(t *testing.T) {
	_, err := ParseToStruct[BasicTypes](&http.Request{})
	if !errors.Is(err, ParseNilURLError) {
		t.Fatalf("expected ParseNilURLError, got %v", err)
	}
}

func TestParseNonStructType(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	_, err := ParseToStruct[int](r)
	if !errors.Is(err, ParseNonStructDestError) {
		t.Fatalf("expected ParseNonStructDestError, got %v", err)
	}
}

func TestParseIntoHappyPath(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com?name=John&age=30", nil)
	var dest BasicTypes
	if err := ParseIntoStructPtr(r, &dest); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dest.Name != "John" || dest.Age != 30 {
		t.Errorf("unexpected values: %+v", dest)
	}
}

func TestParseIntoNilRequest(t *testing.T) {
	var dest BasicTypes
	if err := ParseIntoStructPtr(nil, &dest); !errors.Is(err, ParseNilRequestError) {
		t.Fatalf("expected ParseNilRequestError, got %v", err)
	}
}

func TestParseIntoNilURL(t *testing.T) {
	var dest BasicTypes
	if err := ParseIntoStructPtr(&http.Request{}, &dest); !errors.Is(err, ParseNilURLError) {
		t.Fatalf("expected ParseNilURLError, got %v", err)
	}
}

func TestParseIntoNilDest(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	if err := ParseIntoStructPtr(r, nil); !errors.Is(err, ParseNilDestError) {
		t.Fatalf("expected ParseNilDestError, got %v", err)
	}
}

func TestParseIntoNilPointerDest(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	var dest *BasicTypes
	if err := ParseIntoStructPtr(r, dest); !errors.Is(err, ParseNilDestError) {
		t.Fatalf("expected ParseNilDestError, got %v", err)
	}
}

func TestParseIntoNonPointerDest(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	var dest BasicTypes
	if err := ParseIntoStructPtr(r, dest); !errors.Is(err, ParseNilDestError) {
		t.Fatalf("expected ParseNilDestError, got %v", err)
	}
}

func TestParseIntoPointerToNonStruct(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com", nil)
	var dest int
	if err := ParseIntoStructPtr(r, &dest); !errors.Is(err, ParseNonStructDestError) {
		t.Fatalf("expected ParseNonStructDestError, got %v", err)
	}
}

func TestParseAbsentPointerCompositesRemainNil(t *testing.T) {
	r, _ := http.NewRequest("GET", "http://example.com?name=John", nil)

	got, err := ParseToStruct[struct {
		Name    string             `json:"name"`
		Scores  *[]int             `json:"scores"`
		Meta    *map[string]string `json:"meta"`
		Address *struct {
			City string `json:"city"`
		} `json:"address"`
	}](r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Scores != nil {
		t.Fatalf("expected nil scores pointer, got %#v", *got.Scores)
	}
	if got.Meta != nil {
		t.Fatalf("expected nil meta pointer, got %#v", *got.Meta)
	}
	if got.Address != nil {
		t.Fatalf("expected nil address pointer, got %#v", *got.Address)
	}
}

func TestParseMapOfMapsFails(t *testing.T) {
	r, _ := http.NewRequest(
		"GET",
		"http://example.com?data.outer.inner=value",
		nil,
	)

	_, err := ParseToStruct[struct {
		Data map[string]map[string]string `json:"data"`
	}](r)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ParseError) {
		t.Fatalf("expected ParseError, got %v", err)
	}
}

func TestParseArrays(t *testing.T) {
	r, _ := http.NewRequest(
		"GET",
		"http://example.com?tags=a&tags=b&scores=1&scores=2",
		nil,
	)

	got, err := ParseToStruct[struct {
		Tags   [2]string `json:"tags"`
		Scores *[2]int   `json:"scores"`
	}](r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.Tags != [2]string{"a", "b"} {
		t.Fatalf("unexpected tags: %#v", got.Tags)
	}
	if got.Scores == nil || *got.Scores != [2]int{1, 2} {
		t.Fatalf("unexpected scores: %#v", got.Scores)
	}
}

func TestParseArrayOverflowFails(t *testing.T) {
	r, _ := http.NewRequest(
		"GET",
		"http://example.com?tags=a&tags=b&tags=c",
		nil,
	)

	_, err := ParseToStruct[struct {
		Tags [2]string `json:"tags"`
	}](r)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ParseError) {
		t.Fatalf("expected ParseError, got %v", err)
	}
}

func TestParseMultiPointerScalars(t *testing.T) {
	r, _ := http.NewRequest(
		"GET",
		"http://example.com?stringPtr=hello&numberPtr=7&boolPtr=true&stringPtr3=deep",
		nil,
	)

	got, err := ParseToStruct[struct {
		StringPtr  **string  `json:"stringPtr"`
		NumberPtr  **int     `json:"numberPtr"`
		BoolPtr    **bool    `json:"boolPtr"`
		StringPtr3 ***string `json:"stringPtr3"`
	}](r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.StringPtr == nil || **got.StringPtr != "hello" {
		t.Fatalf("unexpected StringPtr: %#v", got.StringPtr)
	}
	if got.NumberPtr == nil || **got.NumberPtr != 7 {
		t.Fatalf("unexpected NumberPtr: %#v", got.NumberPtr)
	}
	if got.BoolPtr == nil || !**got.BoolPtr {
		t.Fatalf("unexpected BoolPtr: %#v", got.BoolPtr)
	}
	if got.StringPtr3 == nil || ***got.StringPtr3 != "deep" {
		t.Fatalf("unexpected StringPtr3: %#v", got.StringPtr3)
	}
}

func TestParsePrefixedSliceFallbackUsesSortedChildKeys(t *testing.T) {
	r, _ := http.NewRequest(
		"GET",
		"http://example.com?tags.beta=two&tags.alpha=one&address.phones.gamma=333&address.phones.alpha=111&address.phones.beta=222",
		nil,
	)

	got, err := ParseToStruct[property_slice_fallback_form](r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !reflect.DeepEqual(got.Tags, []string{"one", "two"}) {
		t.Fatalf("unexpected tags: %#v", got.Tags)
	}
	if !reflect.DeepEqual(got.Address.Phones, []string{"111", "222", "333"}) {
		t.Fatalf("unexpected phones: %#v", got.Address.Phones)
	}
}

// parser returns a closure that invokes Parse[T] and returns the result as any.
// This lets table-driven tests with different T values share a single field type.
func parser[T any]() func(*http.Request) (any, error) {
	return func(r *http.Request) (any, error) {
		v, err := ParseToStruct[T](r)
		if err != nil {
			return nil, err
		}
		return v, nil
	}
}
