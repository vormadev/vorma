package validate

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestAnyChecker(t *testing.T) {
	t.Run("Required", func(t *testing.T) {
		var i int
		if err := Any("int", i).Required().Error(); err == nil {
			t.Error("expected error for zero value")
		}

		i = 1
		if err := Any("int", i).Required().Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var s *string
		if err := Any("string", s).Required().Error(); err == nil {
			t.Error("expected error for nil pointer")
		}

		str := "test"
		s = &str
		if err := Any("string", s).Required().Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Optional", func(t *testing.T) {
		var i int
		if err := Any("int", i).Optional().Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		i = 1
		if err := Any("int", i).Optional().Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		var s *string
		if err := Any("string", s).Optional().Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		str := "test"
		s = &str
		if err := Any("string", s).Optional().Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestObjectChecker(t *testing.T) {
	t.Run("RequiredFields", func(t *testing.T) {
		type Person struct {
			Name    string
			Age     int
			Address *string
		}

		// All fields zero
		p := Person{}
		v := Object(&p)
		v.Required("Name")
		v.Required("Age")
		v.Required("Address")
		err := v.Error()

		if err == nil {
			t.Error("expected error for all zero fields")
		} else {
			if !strings.Contains(err.Error(), "Name is required") {
				t.Error("expected 'Name is required' error")
			}
			if !strings.Contains(err.Error(), "Age is required") {
				t.Error("expected 'Age is required' error")
			}
			if !strings.Contains(err.Error(), "Address is required") {
				t.Error("expected 'Address is required' error")
			}
		}

		// Filled fields
		addr := "123 Main St"
		p = Person{Name: "John", Age: 30, Address: &addr}
		v = Object(&p)
		v.Required("Name")
		v.Required("Age")
		v.Required("Address")
		err = v.Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("OptionalFields", func(t *testing.T) {
		type Person struct {
			Name    string
			Age     int
			Address *string
		}

		// All fields zero
		p := Person{}
		v := Object(&p)
		v.Optional("Name")
		v.Optional("Age")
		v.Optional("Address")
		err := v.Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Mix of required and optional
		v = Object(&p)
		v.Required("Name")
		v.Optional("Age")
		v.Optional("Address")
		err = v.Error()

		if err == nil {
			t.Error("expected error for required zero field")
		} else {
			if !strings.Contains(err.Error(), "Name is required") {
				t.Error("expected 'Name is required' error")
			}
		}

		p.Name = "John"
		v = Object(&p)
		v.Required("Name")
		v.Optional("Age")
		v.Optional("Address")
		err = v.Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("MapWithStringKeys", func(t *testing.T) {
		// Empty map
		m := map[string]any{}
		v := Object(m)
		v.Required("name")
		v.Required("age")
		err := v.Error()

		if err == nil {
			t.Error("expected error for missing map keys")
		} else {
			if !strings.Contains(err.Error(), "name is required") {
				t.Error("expected 'name is required' error")
			}
			if !strings.Contains(err.Error(), "age is required") {
				t.Error("expected 'age is required' error")
			}
		}

		// Filled map
		m = map[string]any{
			"name": "John",
			"age":  30,
		}
		v = Object(m)
		v.Required("name")
		v.Required("age")
		v.Optional("address")
		err = v.Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("NestedStructs", func(t *testing.T) {
		type Address struct {
			Street string
			City   string
		}
		type Person struct {
			Name    string
			Address Address
		}

		p := Person{}
		v := Object(&p)
		v.Required("Name")
		v.Required("Address")
		err := v.Error()

		if err == nil {
			t.Error("expected error for zero name field")
		} else {
			if !strings.Contains(err.Error(), "Name is required") {
				t.Error("expected 'Name is required' error")
			}
		}

		p.Name = "John"
		v = Object(&p)
		v.Required("Name")
		v.Required("Address")
		err = v.Error()

		if err != nil {
			t.Errorf("unexpected error for Address: %v", err)
		}
	})
}

func TestObjectCheckerUnknownFieldFailsFast(t *testing.T) {
	type Person struct {
		Name string
	}

	err := Object(Person{}).Required("Nmae").Error()
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
	if !strings.Contains(err.Error(), "unknown field Nmae") {
		t.Fatalf("expected unknown-field error, got %v", err)
	}

	err = Object(Person{}).Optional("Nmae").Error()
	if err == nil {
		t.Fatal("expected error for unknown optional field")
	}
}

func TestObjectCheckerErrorIdempotent(t *testing.T) {
	type Person struct {
		Name string
	}

	checker := Object(Person{})
	checker.Required("Name")

	err1 := checker.Error()
	err2 := checker.Error()
	if err1 == nil || err2 == nil {
		t.Fatal("expected validation errors")
	}
	if err1.Error() != err2.Error() {
		t.Fatalf(
			"expected idempotent errors, got %q then %q",
			err1.Error(),
			err2.Error(),
		)
	}
	if strings.Count(err2.Error(), "Name is required") != 1 {
		t.Fatalf("expected exactly one Name error, got %q", err2.Error())
	}
}

// Validator implementation tests
type User struct {
	Username string
	Password string
	Profile  Profile
}

func (u User) Validate() error {
	if len(u.Username) < 3 {
		return errors.New("username must be at least 3 characters")
	}
	if len(u.Password) < 8 {
		return errors.New("password must be at least 8 characters")
	}
	return nil
}

type Profile struct {
	Bio    string
	Email  string
	Status *Status
}

func (p Profile) Validate() error {
	if p.Email != "" && !strings.Contains(p.Email, "@") {
		return errors.New("invalid email format")
	}
	return nil
}

type Status struct {
	Active string // "true" or "false"
}

const status_err = "status must be 'true' or 'false'"

func (s *Status) Validate() error {
	if s.Active != "true" && s.Active != "false" {
		return errors.New(status_err)
	}
	return nil
}

func TestValidatorInterface(t *testing.T) {
	// Invalid user
	u1 := User{Username: "ab", Password: "short"}
	err := Any("user", u1).Required().Error()
	if err == nil {
		t.Error("expected validation error")
	} else {
		if !strings.Contains(err.Error(), "username must be at least 3 characters") {
			t.Error("expected username validation error")
		}
	}

	// Bad profile status
	u2 := User{
		Username: "john",
		Password: "password123",
		Profile:  Profile{Status: &Status{}},
	}
	err = Any("user", u2).Required().Error()
	if err == nil {
		t.Error("expected validation error")
	} else {
		if !strings.Contains(err.Error(), status_err) {
			t.Error("expected status validation error")
		}
	}

	// Bad profile email
	u3 := User{
		Username: "john",
		Password: "password123",
		Profile:  Profile{Email: "invalid-email"},
	}
	err = Any("user", u3).Required().Error()
	if err == nil {
		t.Error("expected validation error")
	} else {
		if !strings.Contains(err.Error(), "invalid email format") {
			t.Error("expected email validation error")
		}
	}

	// Good profile email and status
	u3.Profile.Email = "john@example.com"
	u3.Profile.Status = &Status{Active: "true"}
	err = Any("user", u3).Required().Error()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

// Complex nested structures tests
type Company struct {
	Name      string
	Employees map[string]*Employee
	Offices   []Office
	Config    *Config
}

type Employee struct {
	ID   int
	Name string
	Tags []string
}

type Office struct {
	Location string
	Capacity int
}

type Config struct {
	Settings map[string]any
}

func (c *Company) Validate() error {
	if c.Name == "" {
		return errors.New("company name is required")
	}
	return nil
}

func (e *Employee) Validate() error {
	if e.ID <= 0 {
		return errors.New("employee ID must be positive")
	}
	if e.Name == "" {
		return errors.New("employee name is required")
	}
	return nil
}

func TestComplexNestedStructures(t *testing.T) {
	t.Run("MapOfPointersValidation", func(t *testing.T) {
		// Invalid employee
		company := Company{
			Name: "Acme Corp",
			Employees: map[string]*Employee{
				"emp1": {ID: 0, Name: ""}, // Invalid employee
			},
		}
		err := Any("company", &company).Required().Error()
		if err == nil {
			t.Error("expected validation error")
		} else {
			if !strings.Contains(err.Error(), "employee ID must be positive") {
				t.Error("expected employee ID validation error")
			}
		}

		// Valid structure
		company.Employees["emp1"].ID = 1
		company.Employees["emp1"].Name = "John"
		err = Any("company", &company).Required().Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("SliceValidation", func(t *testing.T) {
		company := Company{
			Name: "Acme Corp",
			Employees: map[string]*Employee{
				"emp1": {ID: 1, Name: "John"},
			},
			Offices: []Office{
				{
					Location: "",
					Capacity: 0,
				}, // We don't have validation for this yet
			},
		}
		err := Any("company", &company).Required().Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Test with Object checker for specific office validation
		v := Object(&company)
		v.Required("Name")
		v.Optional("Employees")
		v.Optional("Offices")
		err = v.Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("NilPointerInNestedStruct", func(t *testing.T) {
		// Config is nil but optional
		company := Company{
			Name:   "Acme Corp",
			Config: nil,
		}
		v := Object(&company)
		v.Required("Name")
		v.Optional("Config")
		err := v.Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Config is nil but required
		v = Object(&company)
		v.Required("Name")
		v.Required("Config")
		err = v.Error()
		if err == nil {
			t.Error("expected error for nil Config")
		} else {
			if !strings.Contains(err.Error(), "Config is required") {
				t.Error("expected 'Config is required' error")
			}
		}
	})

	t.Run("ItemResource", func(t *testing.T) {
		/////// PTR VERSIONS

		// should work because Child is not marked as required
		p1 := &ParentOptionalChild{Child: nil}
		if err := attempt_validation("", p1); err != nil {
			t.Errorf("p1: unexpected error: %v", err)
		}

		// should fail because Child is marked as required
		p2 := &ParentRequiredChild{Child: nil}
		if err := attempt_validation("", p2); err == nil {
			t.Error("p2: expected error for nil Child")
		}

		// should fail because Child value is invalid
		p3 := &ParentOptionalChild{Child: &Child{A: "b"}}
		if err := attempt_validation("", p3); err == nil {
			t.Error("p3: expected error for invalid Child value")
		}

		/////// DIRECT VERSIONS

		// should work because Child is not marked as required
		p12 := ParentOptionalChild{Child: nil}
		if err := attempt_validation("", p12); err != nil {
			t.Errorf("p12: unexpected error: %v", err)
		}

		// should fail because Child is marked as required
		p22 := ParentRequiredChild{Child: nil}
		if err := attempt_validation("", p22); err == nil {
			t.Error("p22: expected error for nil Child")
		}

		// should fail because Child value is invalid
		p32 := ParentOptionalChild{Child: &Child{A: "b"}}
		if err := attempt_validation("", p32); err == nil {
			t.Error("p32: expected error for invalid Child value")
		}
	})
}

type ParentOptionalChild struct {
	Child *Child `json:"child,omitempty"`
}
type ParentRequiredChild struct {
	Child *Child `json:"child,omitempty"`
}

func (c *ParentOptionalChild) Validate() error {
	v := Object(c)
	return v.Error()
}
func (c *ParentRequiredChild) Validate() error {
	v := Object(c)
	v.Required("Child")
	return v.Error()
}

type Child struct {
	A string `json:"a"`
}

func (c *Child) Validate() error {
	v := Object(c)
	v.Required("A").In([]string{"a"})
	return v.Error()
}

// Test safe_deref and get_type_state
func TestSafeDeref(t *testing.T) {
	str := "test"
	ptr_value := reflect.ValueOf(&str)
	deref_value := safe_deref(ptr_value)

	if deref_value.Kind() != reflect.String {
		t.Errorf("expected string, got %v", deref_value.Kind())
	}

	non_ptr_value := reflect.ValueOf(str)
	deref_value = safe_deref(non_ptr_value)

	if deref_value.Kind() != reflect.String {
		t.Errorf("expected string, got %v", deref_value.Kind())
	}
}

// Test field group constraints
func TestFieldGroupConstraint(t *testing.T) {
	type TestForm struct {
		Email    string
		Phone    string
		Username string
	}

	// Create a custom constraint function for testing
	require_at_least_one := func(truthy_count, total_fields int) string {
		if truthy_count == 0 {
			return "at least one of %s fields is required"
		}
		return ""
	}

	require_exactly_one := func(truthy_count, total_fields int) string {
		if truthy_count != 1 {
			return "exactly one of %s fields is required"
		}
		return ""
	}

	require_all := func(truthy_count, total_fields int) string {
		if truthy_count != total_fields {
			return "all %s fields are required"
		}
		return ""
	}

	t.Run("RequireAtLeastOne", func(t *testing.T) {
		// No fields provided
		form := TestForm{}
		oc := Object(&form)
		oc = oc.validate_field_group_constraint(
			"contact",
			[]string{"Email", "Phone"},
			require_at_least_one,
		)
		if err := oc.Error(); err == nil {
			t.Error("expected error when no fields provided")
		} else if !strings.Contains(err.Error(), "at least one of contact fields is required") {
			t.Errorf("unexpected error: %v", err)
		}

		// One field provided
		form.Email = "test@example.com"
		oc = Object(&form)
		oc = oc.validate_field_group_constraint(
			"contact",
			[]string{"Email", "Phone"},
			require_at_least_one,
		)
		if err := oc.Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("RequireExactlyOne", func(t *testing.T) {
		// No fields provided
		form := TestForm{}
		oc := Object(&form)
		oc = oc.validate_field_group_constraint(
			"identifier",
			[]string{"Email", "Phone", "Username"},
			require_exactly_one,
		)
		if err := oc.Error(); err == nil {
			t.Error("expected error when no fields provided")
		}

		// One field provided
		form.Email = "test@example.com"
		oc = Object(&form)
		oc = oc.validate_field_group_constraint(
			"identifier",
			[]string{"Email", "Phone", "Username"},
			require_exactly_one,
		)
		if err := oc.Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Multiple fields provided
		form.Phone = "555-1234"
		oc = Object(&form)
		oc = oc.validate_field_group_constraint(
			"identifier",
			[]string{"Email", "Phone", "Username"},
			require_exactly_one,
		)
		if err := oc.Error(); err == nil {
			t.Error("expected error when multiple fields provided")
		}
	})

	t.Run("RequireAll", func(t *testing.T) {
		// No fields provided
		form := TestForm{}
		oc := Object(&form)
		oc = oc.validate_field_group_constraint(
			"user info",
			[]string{"Email", "Phone", "Username"},
			require_all,
		)
		if err := oc.Error(); err == nil {
			t.Error("expected error when no fields provided")
		}

		// Some fields provided
		form.Email = "test@example.com"
		form.Phone = "555-1234"
		oc = Object(&form)
		oc = oc.validate_field_group_constraint(
			"user info",
			[]string{"Email", "Phone", "Username"},
			require_all,
		)
		if err := oc.Error(); err == nil {
			t.Error("expected error when not all fields provided")
		}

		// All fields provided
		form.Username = "testuser"
		oc = Object(&form)
		oc = oc.validate_field_group_constraint(
			"user info",
			[]string{"Email", "Phone", "Username"},
			require_all,
		)
		if err := oc.Error(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

// Test error collector edge cases
func TestECEdgeCases(t *testing.T) {
	t.Run("EmptyMap", func(t *testing.T) {
		m := map[string]any{}
		err := Object(m).Error()
		if err != nil {
			t.Errorf("unexpected error for empty map: %v", err)
		}
	})

	t.Run("NilMap", func(t *testing.T) {
		var m map[string]any
		err := Object(m).Error()
		if err != nil {
			t.Errorf("unexpected error for nil map: %v", err)
		}
	})

	t.Run("InvalidObject", func(t *testing.T) {
		err := Object(42).Error()
		if err == nil {
			t.Error("expected error for invalid object")
		}
	})

	t.Run("RecursiveNesting", func(t *testing.T) {
		type Node struct {
			Value    string
			Children []*Node
		}

		// Create a simple tree
		leaf1 := &Node{Value: "Leaf 1"}
		leaf2 := &Node{Value: "Leaf 2"}
		root := &Node{
			Value:    "Root",
			Children: []*Node{leaf1, leaf2},
		}

		err := Any("node", root).Required().Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		// Add a nil child
		root.Children = append(root.Children, nil)
		err = Any("node", root).Required().Error()
		if err != nil {
			t.Errorf("unexpected error with nil child: %v", err)
		}

		// Empty value in non-nil child
		leaf1.Value = ""
		v := Object(root)
		v.Required("Value")
		v.Optional("Children")
		err = v.Error()
		if err != nil {
			t.Errorf(
				"unexpected error for empty but non-required child field: %v",
				err,
			)
		}
	})
}

// Test custom validators
type CustomValidator struct {
	Value       string
	MinLength   int
	MaxLength   int
	CustomError string
}

func (cv CustomValidator) Validate() error {
	if len(cv.Value) < cv.MinLength {
		return fmt.Errorf("minimum length is %d", cv.MinLength)
	}
	if cv.MaxLength > 0 && len(cv.Value) > cv.MaxLength {
		return fmt.Errorf("maximum length is %d", cv.MaxLength)
	}
	if cv.CustomError != "" {
		return errors.New(cv.CustomError)
	}
	return nil
}

func TestCustomValidators(t *testing.T) {
	t.Run("TooShort", func(t *testing.T) {
		cv := CustomValidator{
			Value:     "ab",
			MinLength: 3,
		}
		err := Any("custom", cv).Required().Error()
		if err == nil {
			t.Error("expected validation error for too short")
		} else {
			if !strings.Contains(err.Error(), "minimum length is 3") {
				t.Error("expected 'minimum length is 3' error")
			}
		}
	})

	t.Run("TooLong", func(t *testing.T) {
		cv := CustomValidator{
			Value:     "abcdefg",
			MinLength: 3,
			MaxLength: 5,
		}
		err := Any("custom", cv).Required().Error()
		if err == nil {
			t.Error("expected validation error for too long")
		} else {
			if !strings.Contains(err.Error(), "maximum length is 5") {
				t.Error("expected 'maximum length is 5' error")
			}
		}
	})

	t.Run("CustomError", func(t *testing.T) {
		cv := CustomValidator{
			Value:       "abcde",
			MinLength:   3,
			MaxLength:   10,
			CustomError: "this is a custom error",
		}
		err := Any("custom", cv).Required().Error()
		if err == nil {
			t.Error("expected validation error for custom error")
		} else {
			if !strings.Contains(err.Error(), "this is a custom error") {
				t.Error("expected custom error message")
			}
		}
	})

	t.Run("Valid", func(t *testing.T) {
		cv := CustomValidator{
			Value:     "abcde",
			MinLength: 3,
			MaxLength: 10,
		}
		err := Any("custom", cv).Required().Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestValidationError(t *testing.T) {
	t.Run("IsValidationError", func(t *testing.T) {
		// Test direct validation error
		user := User{Username: "ab", Password: "short"}
		err := Any("user", user).Required().Error()

		if err == nil {
			t.Fatal("expected validation error")
		}

		if !IsValidationError(err) {
			t.Error("expected err to be a ValidationError")
		}

		// Test wrapped validation error
		wrapped_err := fmt.Errorf("wrapped: %w", err)
		if !IsValidationError(wrapped_err) {
			t.Error("expected wrapped err to be detected as ValidationError")
		}

		// Test non-validation error
		regular_err := errors.New("regular error")
		if IsValidationError(regular_err) {
			t.Error(
				"non-validation error incorrectly identified as ValidationError",
			)
		}
	})

	t.Run("ValidationErrorWrapping", func(t *testing.T) {
		// Ensure validation error preserves original message
		user := User{Username: "ab", Password: "short"}
		err := Any("user", user).Required().Error()

		if err == nil {
			t.Fatal("expected validation error")
		}

		original_msg := err.Error()
		if !strings.Contains(
			original_msg,
			"username must be at least 3 characters",
		) {
			t.Error("validation error should contain original error message")
		}
	})
}

type MyStruct struct {
	Name string
}

func (s *MyStruct) Validate() error {
	if s.Name == "" {
		return errors.New("name is required")
	}
	return nil
}

type MySlice []*MyStruct

func TestSliceWithNilItems(t *testing.T) {
	bob := MySlice{&MyStruct{Name: "Bob"}}
	empty := MySlice{&MyStruct{}}
	nil_item := MySlice{nil}

	err := attempt_validation("bob", bob)
	if err != nil {
		t.Errorf("unexpected error for non-empty slice: %v", err)
	}

	err = attempt_validation("empty", empty)
	if err == nil {
		t.Error("expected error for empty struct in slice")
	}

	err = attempt_validation("nil_item", nil_item)
	if err != nil {
		// because when an item is nil, we do not try to call Validate on it
		t.Error("expected no error for nil item in slice")
	}
}
