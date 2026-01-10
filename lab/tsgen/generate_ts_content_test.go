package tsgen

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestGenerateTypeScript(t *testing.T) {
	// Test case without AdHocTypes
	opts := Opts{
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{
					"type": "query",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomInputType": {
						TypeInstance: struct{ Name string }{"TestName"},
						TSTypeName:   "TestQueryInput",
					},
					"phantomOutputType": {
						TypeInstance: struct{ Result string }{"TestResult"},
						TSTypeName:   "TestQueryOutput",
					},
				},
			},
			{
				ArbitraryProperties: map[string]any{
					"type": "mutation",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomInputType": {
						TypeInstance: struct{ ID int }{1},
						TSTypeName:   "TestMutationInput",
					},
					"phantomOutputType": {
						TypeInstance: struct{ Success bool }{true},
						TSTypeName:   "TestMutationOutput",
					},
				},
			},
		},
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %s", err)
	}

	if len(content) == 0 {
		t.Fatal("Generated TypeScript content is empty")
	}

	// Check that the output contains specific strings
	contentStrMinimized := whiteSpaceToSingleSpace(content)

	var expectedStrs = []string{mainTypes, items}

	for _, expectedStr := range expectedStrs {
		if !strings.Contains(contentStrMinimized, whiteSpaceToSingleSpace(expectedStr)) {
			t.Errorf("Expected string not found in generated TypeScript content: %q", expectedStr)
		}
	}

	// Check for the presence of TypeScript types
	if !strings.Contains(content, "export type TestQueryInput = {") {
		t.Error("Expected TypeScript type for TestQueryInput not found")
	}

	if !strings.Contains(content, "export type TestQueryOutput = {") {
		t.Error("Expected TypeScript type for TestQueryOutput not found")
	}

	if !strings.Contains(content, "export type TestMutationInput = {") {
		t.Error("Expected TypeScript type for TestMutationInput not found")
	}

	if !strings.Contains(content, "export type TestMutationOutput = {") {
		t.Error("Expected TypeScript type for TestMutationOutput not found")
	}

	// Check if AdHocTypes are correctly handled when not provided
	if strings.Contains(content, "export type TestAdHocType = ") {
		t.Error("TypeScript type for TestAdHocType found, but AdHocTypes were not provided")
	}

	// Test case with AdHocTypes
	opts.AdHocTypes = []*AdHocType{
		{
			TypeInstance: struct{ Data string }{"TestData"},
			TSTypeName:   "TestAdHocType",
		},
	}

	content, err = GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %s", err)
	}

	if len(content) == 0 {
		t.Fatal("Generated TypeScript content is empty")
	}

	contentStrMinimized = whiteSpaceToSingleSpace(content)

	expectedStrs = append(expectedStrs, adHocTypes)

	for _, expectedStr := range expectedStrs {
		if !strings.Contains(contentStrMinimized, whiteSpaceToSingleSpace(expectedStr)) {
			t.Errorf("Expected string not found in generated TypeScript content: %q", expectedStr)
		}
	}

	// Check for the presence of TypeScript types again
	if !strings.Contains(content, "export type TestQueryInput = {") {
		t.Error("Expected TypeScript types for TestQueryInput not found")
	}

	if !strings.Contains(content, "export type TestQueryOutput = {") {
		t.Error("Expected TypeScript types for TestQueryOutput not found")
	}

	if !strings.Contains(content, "export type TestMutationInput = {") {
		t.Error("Expected TypeScript types for TestMutationInput not found")
	}

	if !strings.Contains(content, "export type TestMutationOutput = {") {
		t.Error("Expected TypeScript types for TestMutationOutput not found")
	}

	// Now check for the presence of AdHocTypes
	if !strings.Contains(content, "export type TestAdHocType = {") {
		t.Error("Expected TypeScript types for TestAdHocType not found")
	}
}

func TestGenerateTypeScriptNoItems(t *testing.T) {
	opts := Opts{
		Collection: []CollectionItem{},
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %s", err)
	}

	if len(content) == 0 {
		t.Fatal("Generated TypeScript content is empty")
	}
}

func TestExtraTS(t *testing.T) {
	opts := Opts{
		ExtraTSCode: "export const extraCode = 'extra';",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %s", err)
	}

	if len(content) == 0 {
		t.Fatal("Generated TypeScript content is empty")
	}

	if !strings.Contains(content, "export const extraCode = 'extra';") {
		t.Error("Expected extra TypeScript code not found")
	}
}

const mainTypes = "export type TestMutationInput = {"

const items = ` = [
	{
		phantomInputType: null as unknown as TestMutationInput,
		phantomOutputType: null as unknown as TestMutationOutput,
		type: "mutation",
	},
	{
		phantomInputType: null as unknown as TestQueryInput,
		phantomOutputType: null as unknown as TestQueryOutput,
		type: "query",
	},
] as const;`

const adHocTypes = "export type TestAdHocType = {"

func whiteSpaceToSingleSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// TestGenerateTSContent_SimpleTypes tests generation of simple types
func TestGenerateTSContent_SimpleTypes(t *testing.T) {
	type SimpleType struct {
		Field string
	}

	opts := Opts{
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/simple",
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &SimpleType{}, TSTypeName: "SimpleOutput"},
				},
			},
		},
		CollectionVarName: "routes",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Verify output
	assertContains(t, content, "export type SimpleOutput = {\n\tField: string;\n}")
	assertContains(t, content, "phantomOutputType: null as unknown as SimpleOutput")
}

// TestGenerateTSContent_DuplicateTypes tests handling of duplicate types
func TestGenerateTSContent_DuplicateTypes(t *testing.T) {
	type Inner struct {
		Name string
	}

	type Outer struct {
		X Inner
	}

	opts := Opts{
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/first",
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &Outer{}, TSTypeName: "FirstOutput"},
				},
			},
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/second",
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &Outer{}, TSTypeName: "SecondOutput"},
				},
			},
		},
		CollectionVarName: "routes",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Verify output - both types should reference the same core type
	assertContains(t, content, "export type FirstOutput = {\n\tX: Inner;\n\n}")
	assertContains(t, content, "export type SecondOutput = { X: Inner; }")
	assertContains(t, content, "type Inner = { Name: string; }")

	// There should only be one core type definition
	occurrences := strings.Count(content, "type Inner =")
	if occurrences != 1 {
		t.Errorf("Expected 1 core type definition, found %d", occurrences)
	}
}

// TestGenerateTSContent_DifferentTypesWithSameName tests handling of different types with same name
func TestGenerateTSContent_DifferentTypesWithSameName(t *testing.T) {
	type Type1 struct {
		Field1 string
	}

	type Type2 struct {
		Field2 int
	}

	type Type3ForAdHoc struct {
		Field3 int
	}

	opts := Opts{
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/path",
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &Type1{}, TSTypeName: "SameNameOutput"},
				},
			},
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/path/$",
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &Type2{}, TSTypeName: "SameNameOutput"},
				},
			},
		},
		AdHocTypes: []*AdHocType{
			{TSTypeName: "SameNameOutput", TypeInstance: &Type3ForAdHoc{}},
		},
		CollectionVarName: "routes",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Verify that the second type got a numeric suffix
	assertContains(t, content, "export type SameNameOutput = {")
	assertContains(t, content, "export type SameNameOutput_2 = {")
	assertContains(t, content, "export type SameNameOutput_3 = {")

	// Verify both type definitions exist
	assertContains(t, content, "Field1: string;")
	assertContains(t, content, "Field2: number;")
	assertContains(t, content, "Field3: number;")
}

// TestGenerateTSContent_ComplexNestedTypes tests handling of complex nested types
func TestGenerateTSContent_ComplexNestedTypes(t *testing.T) {
	type NestedType struct {
		Nested string
	}

	type ParentType struct {
		Name  string
		Child NestedType
	}

	opts := Opts{
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/complex",
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &ParentType{}, TSTypeName: "ComplexOutput"},
				},
			},
		},
		CollectionVarName: "routes",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Verify nested type was handled correctly
	assertContains(t, content, "export type ComplexOutput = {")
	assertContains(t, content, "Child: NestedType;")
}

// TestGenerateTSContent_AdHocTypes tests handling of ad-hoc types
func TestGenerateTSContent_AdHocTypes(t *testing.T) {
	type SomeType struct {
		Field string
	}

	opts := Opts{
		Collection: []CollectionItem{},
		AdHocTypes: []*AdHocType{
			{TSTypeName: "CustomType", TypeInstance: &SomeType{}},
		},
		CollectionVarName: "routes",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Verify ad-hoc type was included
	assertContains(t, content, "export type CustomType = { Field: string; };")
}

// TestGenerateTSContent_TypesWithTimeField tests handling of time.Time fields
func TestGenerateTSContent_TypesWithTimeField(t *testing.T) {
	type TypeWithTime struct {
		Created time.Time
	}

	opts := Opts{
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/with-time",
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &TypeWithTime{}, TSTypeName: "TimeOutput"},
				},
			},
		},
		CollectionVarName: "routes",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Verify time.Time was handled correctly (implementation-dependent)
	assertContains(t, content, "export type TimeOutput = {")
	assertContains(t, content, "Created: ")
}

// TestGenerateTSContent_EmptyNameHandling tests handling of empty or anonymous names
func TestGenerateTSContent_AnonAndUnnamedShouldBeSkipped(t *testing.T) {
	opts := Opts{
		AdHocTypes: []*AdHocType{{TypeInstance: struct{ Field string }{}, TSTypeName: ""}},
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Verify empty name was handled with default
	assertNotContains(t, content, "export type")
}

// Helper functions for assertions
func assertContains(t *testing.T, outer, inner string) {
	cleanOuter := normalizeWhiteSpace(outer)
	cleanInner := normalizeWhiteSpace(inner)

	t.Helper()
	if !strings.Contains(cleanOuter, cleanInner) {
		t.Errorf("Expected content to contain '%s' but it didn't.\nContent: %s", cleanInner, cleanOuter)
	}
}

func assertNotContains(t *testing.T, outer, inner string) {
	cleanOuter := normalizeWhiteSpace(outer)
	cleanInner := normalizeWhiteSpace(inner)

	t.Helper()
	if strings.Contains(cleanOuter, cleanInner) {
		t.Errorf("Expected content to NOT contain '%s' but it did.\nContent: %s", cleanInner, cleanOuter)
	}
}

// TestGenerateTSContent_WithCustomSorting tests the custom sorting feature
func TestGenerateTSContent_WithCustomSorting(t *testing.T) {
	type SimpleType struct {
		Field string
	}

	opts := Opts{
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/c",
					"order":     3,
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &SimpleType{}, TSTypeName: "COutput"},
				},
			},
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/a",
					"order":     1,
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &SimpleType{}, TSTypeName: "AOutput"},
				},
			},
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/b",
					"order":     2,
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: &SimpleType{}, TSTypeName: "BOutput"},
				},
			},
		},
		CollectionVarName: "routes",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Find positions of patterns in the output
	aPos := strings.Index(content, `pattern: "/a"`)
	bPos := strings.Index(content, `pattern: "/b"`)
	cPos := strings.Index(content, `pattern: "/c"`)

	// Verify they're in the correct order
	if !(aPos < bPos && bPos < cPos) {
		t.Errorf("Items not sorted correctly by 'order' property")
	}
}

// TestGenerateTSContent_NullTypes tests handling of nil type instances
func TestGenerateTSContent_NullTypes(t *testing.T) {
	opts := Opts{
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{
					"pattern":   "/null-type",
					"routeType": "loader",
				},
				PhantomTypes: map[string]AdHocType{
					"phantomOutputType": {TypeInstance: nil, TSTypeName: "ShouldBeNull"},
				},
			},
		},
		CollectionVarName: "routes",
	}

	content, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("GenerateTSContent failed: %v", err)
	}

	// Verify nil type instance handled correctly
	assertContains(t, content, "phantomOutputType: null")
	assertNotContains(t, content, "export type ShouldBeNull")
}

// Test structs with identical field structures but different types
type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

type Animal struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// Test different primitive types
type AllPrimitives struct {
	BoolField    bool    `json:"boolField"`
	IntField     int     `json:"intField"`
	Int8Field    int8    `json:"int8Field"`
	Int16Field   int16   `json:"int16Field"`
	Int32Field   int32   `json:"int32Field"`
	Int64Field   int64   `json:"int64Field"`
	UintField    uint    `json:"uintField"`
	Uint8Field   uint8   `json:"uint8Field"`
	Uint16Field  uint16  `json:"uint16Field"`
	Uint32Field  uint32  `json:"uint32Field"`
	Uint64Field  uint64  `json:"uint64Field"`
	Float32Field float32 `json:"float32Field"`
	Float64Field float64 `json:"float64Field"`
	StringField  string  `json:"stringField"`
	ByteField    byte    `json:"byteField"`
	RuneField    rune    `json:"runeField"`
}

// Test pointer types
type WithPointers struct {
	StringPtr *string         `json:"stringPtr"`
	IntPtr    *int            `json:"intPtr"`
	BoolPtr   *bool           `json:"boolPtr"`
	StructPtr *Person         `json:"structPtr"`
	SlicePtr  *[]int          `json:"slicePtr"`
	MapPtr    *map[string]int `json:"mapPtr"`
}

// Test nested structs
type Address struct {
	Street  string `json:"street"`
	City    string `json:"city"`
	Country string `json:"country"`
}

type Customer struct {
	ID      int     `json:"id"`
	Name    string  `json:"name"`
	Address Address `json:"address"`
}

// Test slices and arrays
type WithCollections struct {
	IntSlice    []int     `json:"intSlice"`
	StringSlice []string  `json:"stringSlice"`
	StructSlice []Person  `json:"structSlice"`
	IntArray    [3]int    `json:"intArray"`
	StringArray [2]string `json:"stringArray"`
	StructArray [2]Person `json:"structArray"`
}

// Test maps
type WithMaps struct {
	StringToInt    map[string]int    `json:"stringToInt"`
	StringToString map[string]string `json:"stringToString"`
	IntToString    map[int]string    `json:"intToString"`
	StringToPerson map[string]Person `json:"stringToPerson"`
}

// Test empty structs
type Empty struct{}

// Test time.Time fields
type WithTime struct {
	Created time.Time  `json:"created"`
	Updated *time.Time `json:"updated"`
}

// Test interfaces
type WithInterfaces struct {
	EmptyInterface    any          `json:"emptyInterface"`
	StringerInterface fmt.Stringer `json:"stringerInterface"`
}

// Test optional fields with json:"omitempty"
type WithOptionalFields struct {
	Required string `json:"required"`
	Optional string `json:"optional,omitempty"`
}

// TestAllPrimitiveTypes ensures all primitive types are handled correctly
func TestAllPrimitiveTypes(t *testing.T) {
	opts := Opts{
		AdHocTypes: []*AdHocType{
			{TypeInstance: AllPrimitives{}, TSTypeName: "Primitives"},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check for boolean type
	if !strings.Contains(output, "boolField: boolean") {
		t.Error("Boolean field not properly typed")
	}

	// Check for number types
	numericFields := []string{"intField", "int8Field", "int16Field", "int32Field",
		"int64Field", "uintField", "uint8Field", "uint16Field", "uint32Field",
		"uint64Field", "float32Field", "float64Field", "byteField", "runeField"}

	for _, field := range numericFields {
		if !strings.Contains(output, field+": number") {
			t.Errorf("Numeric field %s not properly typed", field)
		}
	}

	// Check string type
	if !strings.Contains(output, "stringField: string") {
		t.Error("String field not properly typed")
	}
}

// TestPointerTypes ensures pointer types are handled correctly
func TestPointerTypes(t *testing.T) {
	stringVal := "test"
	intVal := 42
	boolVal := true
	sliceVal := []int{1, 2, 3}
	mapVal := map[string]int{"one": 1}

	opts := Opts{
		AdHocTypes: []*AdHocType{
			{TypeInstance: WithPointers{
				StringPtr: &stringVal,
				IntPtr:    &intVal,
				BoolPtr:   &boolVal,
				StructPtr: &Person{Name: "Test", Age: 30},
				SlicePtr:  &sliceVal,
				MapPtr:    &mapVal,
			}, TSTypeName: "PointerTest"},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check optional marking for pointer types
	if !strings.Contains(output, "stringPtr?: string") {
		t.Error("String pointer not marked as optional")
	}
	if !strings.Contains(output, "intPtr?: number") {
		t.Error("Int pointer not marked as optional")
	}
	if !strings.Contains(output, "boolPtr?: boolean") {
		t.Error("Bool pointer not marked as optional")
	}

	// Check that struct pointers are handled correctly
	if !strings.Contains(output, "structPtr?:") {
		t.Error("Struct pointer not marked as optional")
	}

	// Check slice and map pointers
	if !strings.Contains(output, "slicePtr?: Array<number>") {
		t.Error("Slice pointer not handled correctly")
	}
	if !strings.Contains(output, "mapPtr?: Record<string, number>") {
		t.Error("Map pointer not handled correctly")
	}
}

// TestNestedStructs ensures nested struct types are handled correctly
func TestNestedStructs(t *testing.T) {
	opts := Opts{
		AdHocTypes: []*AdHocType{
			{TypeInstance: Customer{}, TSTypeName: "Customer"},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check that Customer type is defined
	if !strings.Contains(output, "export type Customer = ") {
		t.Error("Customer type not found in output")
	}

	// Check that Address type is defined or inlined
	addr := `export type Address = { street: string; city: string; country: string; };`

	if !strings.Contains(normalizeWhiteSpace(output), normalizeWhiteSpace(addr)) {
		t.Log(output)
		t.Log(addr)
		t.Error("Nested Address struct not properly formatted")
	}
}

// Test embedded structs
// HERE IS WHAT WE WANT (MIRRORS BEHAVIOR OF NATIVE JSON MARSHALING)
// If embedded direct with no json tag: just as though it were top-level
// If embedded pointer with no json tag: just as though top-level, but fields optional
// If embedded direct with json tag: tag becomes a required root field under which its fields are nested
// If embedded pointer with json tag: tag becomes an optional root field under which its fields are nested
type Base struct{ Name string }
type BasePtr struct{ NamePtr string }
type Wrapper struct {
	BuiltIn string
	Base
	*BasePtr
}
type WrapperWithJSONTags struct {
	BuiltIn  string
	Base     `json:"base"`
	*BasePtr `json:"basePtr"`
}
type WrapperMix struct {
	BuiltIn string
	Base
	*BasePtr `json:"basePtr"`
}

/*
// Wrapper SHOULD PRODUCE:
export type Wrapper = {
	BuiltIn: string;
	Name: string;
	NamePtr?: string;
}
// WrapperWithJSONTags SHOULD PRODUCE:
export type Base = {
	Name: string;
}
export type BasePtr = {
	NamePtr: string;
}
export type WrapperWithJSONTags = {
	BuiltIn: string;
	base: Base;
	basePtr?: BasePtr;
}
// WrapperMix SHOULD PRODUCE:
export type BasePtr = {
	NamePtr: string;
}
export type WrapperMix = {
	BuiltIn: string;
	Name: string;
	basePtr?: BasePtr;
}
*/

// TestEmbeddedStructs ensures embedded struct fields are handled correctly
func TestEmbeddedStructs(t *testing.T) {
	opts1 := Opts{AdHocTypes: []*AdHocType{{TypeInstance: Wrapper{}}}}
	opts2 := Opts{AdHocTypes: []*AdHocType{{TypeInstance: WrapperWithJSONTags{}}}}
	opts3 := Opts{AdHocTypes: []*AdHocType{{TypeInstance: WrapperMix{}}}}

	output1, err := GenerateTSContent(opts1)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// output1 should only export 1 type
	if strings.Count(output1, "export type") != 1 {
		t.Error("Expected 1 type definition in output1")
	}

	expected := normalizeWhiteSpace("export type Wrapper = { BuiltIn: string; Name: string; NamePtr?: string; };")
	if !strings.Contains(normalizeWhiteSpace(output1), expected) {
		t.Errorf("Expected output1 to contain %q", expected)
	}

	output2, err := GenerateTSContent(opts2)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// output2 should export 3 types
	if strings.Count(output2, "export type") != 3 {
		t.Error("Expected 3 type definitions in output2")
	}

	expected_2_a := normalizeWhiteSpace("export type Base = { Name: string; };")
	expected_2_b := normalizeWhiteSpace("export type BasePtr = { NamePtr: string; };")
	expected_2_c := normalizeWhiteSpace("export type WrapperWithJSONTags = { BuiltIn: string; base: Base; basePtr?: BasePtr; };")

	output2_normalized := normalizeWhiteSpace(output2)
	if !strings.Contains(output2_normalized, expected_2_a) {
		t.Errorf("Expected output2 to contain %q", expected_2_a)
	}
	if !strings.Contains(output2_normalized, expected_2_b) {
		t.Errorf("Expected output2 to contain %q", expected_2_b)
	}
	if !strings.Contains(output2_normalized, expected_2_c) {
		t.Errorf("Expected output2 to contain %q", expected_2_c)
	}

	output3, err := GenerateTSContent(opts3)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// output3 should export 2 types
	if strings.Count(output3, "export type") != 2 {
		t.Error("Expected 2 type definitions in output3")
	}

	expected_3_a := normalizeWhiteSpace("export type BasePtr = { NamePtr: string; };")
	expected_3_b := normalizeWhiteSpace("export type WrapperMix = { BuiltIn: string; Name: string; basePtr?: BasePtr; };")

	output3_normalized := normalizeWhiteSpace(output3)
	if !strings.Contains(output3_normalized, expected_3_a) {
		t.Errorf("Expected output3 to contain %q", expected_3_a)
	}
	if !strings.Contains(output3_normalized, expected_3_b) {
		t.Errorf("Expected output3 to contain %q", expected_3_b)
	}
}

// TestCollectionTypes ensures slices and arrays are handled correctly
func TestCollectionTypes(t *testing.T) {
	opts := Opts{
		AdHocTypes: []*AdHocType{
			{TypeInstance: WithCollections{}, TSTypeName: "CollectionTest"},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check slice types
	if !strings.Contains(output, "intSlice: Array<number>") {
		t.Error("Int slice not properly typed")
	}
	if !strings.Contains(output, "stringSlice: Array<string>") {
		t.Error("String slice not properly typed")
	}

	// Check array types (should be the same as slices in TypeScript)
	if !strings.Contains(output, "intArray: Array<number>") {
		t.Error("Int array not properly typed")
	}
	if !strings.Contains(output, "stringArray: Array<string>") {
		t.Error("String array not properly typed")
	}

	// Check struct collections
	if !strings.Contains(output, "structSlice: Array<") {
		t.Error("Struct slice not properly typed")
	}
	if !strings.Contains(output, "structArray: Array<") {
		t.Error("Struct array not properly typed")
	}
}

// TestMapTypes ensures maps are handled correctly
func TestMapTypes(t *testing.T) {
	opts := Opts{
		AdHocTypes: []*AdHocType{
			{TypeInstance: WithMaps{}, TSTypeName: "MapTest"},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check map types
	if !strings.Contains(output, "stringToInt: Record<string, number>") {
		t.Error("String to int map not properly typed")
	}
	if !strings.Contains(output, "stringToString: Record<string, string>") {
		t.Error("String to string map not properly typed")
	}
	if !strings.Contains(output, "intToString: Record<number, string>") {
		t.Error("Int to string map not properly typed")
	}
	if !strings.Contains(output, "stringToPerson: Record<string,") {
		t.Error("String to struct map not properly typed")
	}
}

// TestTimeType ensures time.Time is handled correctly
func TestTimeType(t *testing.T) {
	opts := Opts{
		AdHocTypes: []*AdHocType{
			{TypeInstance: WithTime{}, TSTypeName: "TimeTest"},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check that time.Time is mapped to string
	if !strings.Contains(output, "created: string") {
		t.Log("Output:", output)
		t.Error("time.Time not mapped to string")
	}

	// Check that *time.Time is optional and mapped to string
	if !strings.Contains(output, "updated?: string") {
		t.Error("*time.Time not mapped to optional string")
	}
}

// TestInterfaceTypes ensures interface types are handled correctly
func TestInterfaceTypes(t *testing.T) {
	opts := Opts{
		AdHocTypes: []*AdHocType{
			{TypeInstance: WithInterfaces{}, TSTypeName: "InterfaceTest"},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check that empty interface is mapped to any
	if !strings.Contains(output, "emptyInterface: unknown") {
		t.Log(output)
		t.Error("Empty interface not mapped to 'unknown'")
	}

	// Check that non-empty interface is mapped to unknown
	if !strings.Contains(output, "stringerInterface: unknown") {
		t.Error("Non-empty interface not mapped to 'unknown'")
	}
}

// TestOptionalFields ensures fields with omitempty are marked as optional
func TestOptionalFields(t *testing.T) {
	opts := Opts{
		AdHocTypes: []*AdHocType{
			{TypeInstance: WithOptionalFields{}, TSTypeName: "OptionalTest"},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check required and optional fields
	if !strings.Contains(output, "required: string") {
		t.Error("Required field not properly typed")
	}
	if !strings.Contains(output, "optional?: string") {
		t.Error("Optional field not marked with ?")
	}
}

/////////////////
// TEST TS TYPE IMPLEMENTATION (STRUCT TAGS AND TsType METHODS)

// @document: TSType() methods must be on the struct directly containing the field(s) it refers to,
// and not a wrapper parent struct, even if the wrapper directly embeds the inner struct.

// Test custom TS type annotations
type TSType_Base struct {
	A int    `ts_type:"qwer"` // Override int with string
	B string `ts_type:"tyui"`
	X string
}

func (b TSType_Base) TSType() map[string]string { return map[string]string{"X": "asdf"} }

type TSType_BaseWithPtrMethod struct {
	A int    `ts_type:"qwer"` // Override int with string
	B string `ts_type:"tyui"`
	X string
}

func (b *TSType_BaseWithPtrMethod) TSType() map[string]string { return map[string]string{"X": "asdf"} }

type TSType_Base_Wrapped struct{ TSType_Base }
type TSType_Base_WrappedPtr struct{ *TSType_Base }
type TSType_BaseWithPtrMethod_Wrapped struct{ TSType_BaseWithPtrMethod }
type TSType_BaseWithPtrMethod_WrappedPtr struct{ *TSType_BaseWithPtrMethod }

type TSType_Base_Json struct {
	A int    `json:"a" ts_type:"qwer"` // Override int with string
	B string `json:"b" ts_type:"tyui"`
	X string `json:"x"`
}

func (b TSType_Base_Json) TSType() map[string]string { return map[string]string{"X": "asdf"} }

type TSType_BaseWithPtrMethod_Json struct {
	A int    `json:"a" ts_type:"qwer"` // Override int with string
	B string `json:"b" ts_type:"tyui"`
	X string `json:"x"`
}

func (b *TSType_BaseWithPtrMethod_Json) TSType() map[string]string {
	return map[string]string{"X": "asdf"}
}

type TSType_Base_Wrapped_Json struct{ TSType_Base_Json }
type TSType_Base_WrappedPtr_Json struct{ *TSType_Base_Json }
type TSType_BaseWithPtrMethod_Wrapped_Json struct{ TSType_BaseWithPtrMethod_Json }
type TSType_BaseWithPtrMethod_WrappedPtr_Json struct{ *TSType_BaseWithPtrMethod_Json }

// TestCustomTSTypes ensures custom ts_type tags are honored
func TestCustomTSTypes(t *testing.T) {
	typesToCheck := []*AdHocType{
		// without json tags
		{TypeInstance: TSType_Base{}},
		{TypeInstance: &TSType_Base{}},
		{TypeInstance: TSType_BaseWithPtrMethod{}},
		{TypeInstance: &TSType_BaseWithPtrMethod{}},
		{TypeInstance: TSType_Base_Wrapped{}},
		{TypeInstance: &TSType_Base_Wrapped{}},
		{TypeInstance: TSType_Base_WrappedPtr{}},
		{TypeInstance: &TSType_Base_WrappedPtr{}},
		{TypeInstance: TSType_BaseWithPtrMethod_Wrapped{}},
		{TypeInstance: &TSType_BaseWithPtrMethod_Wrapped{}},
		{TypeInstance: TSType_BaseWithPtrMethod_WrappedPtr{}},
		{TypeInstance: &TSType_BaseWithPtrMethod_WrappedPtr{}},

		// with json tags
		{TypeInstance: TSType_Base_Json{}},
		{TypeInstance: &TSType_Base_Json{}},
		{TypeInstance: TSType_BaseWithPtrMethod_Json{}},
		{TypeInstance: &TSType_BaseWithPtrMethod_Json{}},
		{TypeInstance: TSType_Base_Wrapped_Json{}},
		{TypeInstance: &TSType_Base_Wrapped_Json{}},
		{TypeInstance: TSType_Base_WrappedPtr_Json{}},
		{TypeInstance: &TSType_Base_WrappedPtr_Json{}},
		{TypeInstance: TSType_BaseWithPtrMethod_Wrapped_Json{}},
		{TypeInstance: &TSType_BaseWithPtrMethod_Wrapped_Json{}},
		{TypeInstance: TSType_BaseWithPtrMethod_WrappedPtr_Json{}},
		{TypeInstance: &TSType_BaseWithPtrMethod_WrappedPtr_Json{}},
	}

	for _, adHocType := range typesToCheck {
		reflectType := reflect.TypeOf(adHocType.TypeInstance)
		name := reflectType.Name()
		if reflectType.Kind() == reflect.Ptr {
			name = "*" + reflectType.Elem().Name()
		}

		output, err := GenerateTSContent(Opts{AdHocTypes: []*AdHocType{adHocType}})
		if err != nil {
			t.Log(name)
			t.Fatalf("Failed to generate TypeScript: %v", err)
		}

		// Check custom type overrides
		if !containsAny(output, "a: qwer", "a?: qwer", "A: qwer", "A?: qwer") {
			t.Log()
			t.Log(name, output)
			t.Log()
			t.Error("^^^ ts_type override for A not honored")
			t.Log()
		}
		if !containsAny(output, "b: tyui", "b?: tyui", "B: tyui", "B?: tyui") {
			t.Log()
			t.Log(name, output)
			t.Log()
			t.Error("^^^ ts_type for B not honored")
			t.Log()
		}
		if !containsAny(output, "x: asdf", "x?: asdf", "X: asdf", "X?: asdf") {
			t.Log()
			t.Log(name, output)
			t.Log()
			t.Error("^^^ ts_type method for X not honored")
			t.Log()
		}
	}
}

type StructTagShouldBeOverridden struct {
	X string `json:"x" ts_type:"asdf"`
}

func (s StructTagShouldBeOverridden) TSType() map[string]string {
	return map[string]string{"X": "qwer"}
}

func TestThatTSTypeOverridesStructTag(t *testing.T) {
	output, err := GenerateTSContent(Opts{AdHocTypes: []*AdHocType{{TypeInstance: StructTagShouldBeOverridden{}}}})
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	if containsAny(output, "x: asdf", "x?: asdf", "X: asdf", "X?: asdf") {
		t.Log(output)
		t.Error("ts_type method should override struct tag")
	}

	if !containsAny(output, "x: qwer", "x?: qwer", "X: qwer", "X?: qwer") {
		t.Log(output)
		t.Error("ts_type method should override struct tag")
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, substr := range substrs {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// TestTypeGeneration tests that the generator produces valid TypeScript
func TestTypeGeneration(t *testing.T) {
	// Create a complex set of types to test many features at once
	opts := Opts{
		CollectionVarName: "complexTest",
		AdHocTypes: []*AdHocType{
			{TypeInstance: Person{}, TSTypeName: "Person"},
			{TypeInstance: Animal{}, TSTypeName: "Animal"},
			{TypeInstance: Customer{}, TSTypeName: "Customer"},
			{TypeInstance: WithCollections{}, TSTypeName: "Collections"},
			{TypeInstance: WithMaps{}, TSTypeName: "Maps"},
			{TypeInstance: WithPointers{}, TSTypeName: "Pointers"},
			{TypeInstance: WithTime{}, TSTypeName: "TimeFields"},
			{TypeInstance: WithInterfaces{}, TSTypeName: "Interfaces"},
			{TypeInstance: WithOptionalFields{}, TSTypeName: "Optionals"},
			{TypeInstance: Empty{}, TSTypeName: "Empty"},
		},
		Collection: []CollectionItem{
			{
				ArbitraryProperties: map[string]any{"strProp": "string value", "numProp": 42, "boolProp": true},
				PhantomTypes: map[string]AdHocType{
					"personRef": {TSTypeName: "PersonRef", TypeInstance: Person{Name: "Jane", Age: 25}},
					"animalRef": {TSTypeName: "AnimalRef", TypeInstance: Animal{Name: "Rex", Age: 3}},
				},
			},
		},
	}

	output, err := GenerateTSContent(opts)
	if err != nil {
		t.Fatalf("Failed to generate TypeScript: %v", err)
	}

	// Check the output has all expected exported types
	expectedTypes := []string{
		"Person", "Animal", "Customer", "Collections",
		"Maps", "Pointers", "TimeFields", "Interfaces", "Optionals",
		"Empty", "PersonRef", "AnimalRef",
	}

	for _, typeName := range expectedTypes {
		if !strings.Contains(output, "export type "+typeName+" = ") {
			t.Errorf("Expected type %s not found in output", typeName)
		}
	}

	// Check that items array is generated
	if !strings.Contains(output, "const complexTest = [") {
		t.Error("Items array not generated")
	}

	// Check arbitrary properties in items array
	if !strings.Contains(output, "strProp: \"string value\"") {
		t.Error("String property not correctly added to items array")
	}
	if !strings.Contains(output, "numProp: 42") {
		t.Error("Number property not correctly added to items array")
	}
	if !strings.Contains(output, "boolProp: true") {
		t.Error("Boolean property not correctly added to items array")
	}

	// Check phantom types in items array
	if !strings.Contains(output, "personRef: null as unknown as PersonRef") {
		t.Error("Person phantom type not correctly added to items array")
	}
	if !strings.Contains(output, "animalRef: null as unknown as AnimalRef") {
		t.Error("Animal phantom type not correctly added to items array")
	}
}

func normalizeWhiteSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
