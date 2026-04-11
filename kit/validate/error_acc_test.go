package validate

import (
	"errors"
	"strings"
	"testing"
)

// Validators for error accumulation test
type SimpleValidator struct {
	Value string
}

func (v SimpleValidator) Validate() error {
	if v.Value == "" {
		return errors.New("value cannot be empty")
	}
	return nil
}

type NestedMapKey string

func (k NestedMapKey) Validate() error {
	if strings.Contains(string(k), " ") {
		return errors.New("key cannot contain spaces")
	}
	return nil
}

type NestedMapValue struct {
	Name string
}

func (v NestedMapValue) Validate() error {
	if v.Name == "" {
		return errors.New("name cannot be empty")
	}
	return nil
}

type NestedSliceItem struct {
	ID int
}

func (i NestedSliceItem) Validate() error {
	if i.ID <= 0 {
		return errors.New("ID must be positive")
	}
	return nil
}

// Complex structure with multiple nesting levels, all capable of producing validation errors
type MultiLevelStruct struct {
	// Top level field
	Name string

	// Nested struct
	Config SimpleValidator

	// Map with validators at both key and value level
	Properties map[NestedMapKey]NestedMapValue

	// Slice with validator elements
	Items []NestedSliceItem

	// Double-nested structure
	Nested struct {
		Active   bool
		Settings map[NestedMapKey]NestedMapValue
		Records  []NestedSliceItem
	}
}

func TestMultiLevelErrorAccumulation(t *testing.T) {
	t.Run("ErrorAccumulationWithAny", func(t *testing.T) {
		// Create a structure with validation errors at every level
		m := MultiLevelStruct{
			// Top level field intentionally empty

			// Nested struct with validation error
			Config: SimpleValidator{Value: ""},

			// Map with invalid key and value
			Properties: map[NestedMapKey]NestedMapValue{
				"valid_key":   {Name: "Valid"},
				"invalid key": {Name: "Invalid Key"}, // Key has a space
				"another_key": {Name: ""},            // Value has empty name
			},

			// Slice with invalid elements
			Items: []NestedSliceItem{
				{ID: 1}, // Valid
				{ID: 0}, // Invalid ID
			},

			// Nested struct with its own nested invalid elements
			Nested: struct {
				Active   bool
				Settings map[NestedMapKey]NestedMapValue
				Records  []NestedSliceItem
			}{
				Active: true,
				Settings: map[NestedMapKey]NestedMapValue{
					"nested invalid": {Name: "Nested Value"}, // Invalid key
				},
				Records: []NestedSliceItem{
					{ID: -1}, // Invalid ID
				},
			},
		}

		// Use Any() to trigger automatic recursive validation
		err := Any("multilevel", &m).Required().Error()

		if err == nil {
			t.Fatal("expected multiple validation errors from Any()")
		}

		// Define all the expected errors we should find
		errorChecks := map[string]string{
			"nested struct validator":   "value cannot be empty",
			"map key validation":        "key cannot contain spaces",
			"map value validation":      "name cannot be empty",
			"slice element validation":  "ID must be positive",
			"nested map key validation": "key cannot contain spaces",
			"nested slice validation":   "ID must be positive",
		}

		// Check each expected error is present
		errorString := err.Error()
		for description, expectedMsg := range errorChecks {
			if !strings.Contains(errorString, expectedMsg) {
				t.Errorf("Missing %s error: expected to find '%s' in:\n%s",
					description, expectedMsg, errorString)
			}
		}
	})

	t.Run("ErrorAccumulationWithObject", func(t *testing.T) {
		// Create a structure with validation errors at multiple levels
		m := MultiLevelStruct{
			// Fields with errors at multiple levels
			Config: SimpleValidator{Value: ""},
			Properties: map[NestedMapKey]NestedMapValue{
				"invalid key": {Name: "Value"},
			},
			Items: []NestedSliceItem{{ID: 0}},
		}

		// Test explicit validation with Object()
		v := Object(&m)
		v.Required("Name")       // This field is empty - should fail
		v.Required("Config")     // This has a failing Validator
		v.Required("Properties") // This has invalid contents
		v.Required("Items")      // This has invalid contents
		v.Optional("Nested")     // This is optional and won't be validated

		err := v.Error()

		if err == nil {
			t.Fatal("expected multiple validation errors from Object()")
		}

		errorString := err.Error()

		// Check for specific errors
		requiredChecks := []string{
			"Name is required",              // From direct field validation
			"Config: value cannot be empty", // From nested validator
		}

		for _, expected := range requiredChecks {
			if !strings.Contains(errorString, expected) {
				t.Errorf(
					"Missing required error: expected to find '%s' in:\n%s",
					expected,
					errorString,
				)
			}
		}

		// Verify that validation doesn't happen on the optional field
		if strings.Contains(errorString, "Records") {
			t.Errorf(
				"Found validation error for optional field that shouldn't be validated: %s",
				errorString,
			)
		}
	})

	t.Run("CombiningObjectAndAny", func(t *testing.T) {
		propertyField := map[NestedMapKey]NestedMapValue{
			"invalid key": {Name: "Value"},
		}

		// Create a simple test struct to demonstrate combining approaches
		m := MultiLevelStruct{
			Properties: propertyField,
		}

		// Approach 1: Object with selective validation
		obj := Object(&m)
		obj.Optional("Name") // This won't fail as it's optional
		obj.Required(
			"Properties",
		) // This will validate the field exists but not contents

		// Approach 2: Use Any to specifically validate property contents
		propCheck := Any("Properties contents", propertyField)

		// Accumulate errors from both approaches
		var allErrors []error

		if err := obj.Error(); err != nil {
			allErrors = append(allErrors, err)
		}

		if err := propCheck.Required().Error(); err != nil {
			allErrors = append(allErrors, err)
		}

		// Combine errors
		var finalError error
		if len(allErrors) > 0 {
			finalError = errors.Join(allErrors...)
		}

		// Verify expected errors
		if finalError == nil {
			t.Fatal("expected validation errors when combining approaches")
		}

		errorString := finalError.Error()
		if !strings.Contains(errorString, "key cannot contain spaces") {
			t.Errorf("Missing map key validation error: %s", errorString)
		}
	})
}
