package schema_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type mux_input struct {
	Email    string
	Username string
	Age      int
}

type signup_form struct {
	Email    string
	Username string
	Password string
	Age      int
	Bio      string
	Country  string
}

type pipeline_form struct {
	Country   string
	StateCode string
	Priority  int
}

type signup_confirmation_form struct {
	Email           string
	Password        string
	PasswordConfirm string
}

/////////////////////////////////////////////////////////////////////
/////// CASES
/////////////////////////////////////////////////////////////////////

// Case 207: the primary real-world scenario. A framework like kit/mux
// receives a typed pointer wrapped in `any` and calls EnforceAny with
// an explicit root schema.
func TestIntegration_TypedInputFlow_MuxStyle(t *testing.T) {
	input := &mux_input{
		Email:    "  ALICE@Example.COM  ",
		Username: "  alice  ",
		Age:      30,
	}
	var type_erased any = input
	_, err := schema.EnforceAny("input", type_erased, schema.Object{
		"Email": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MustBeEmail:   true,
		},
		"Username": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			MinLen:        3,
			MaxLen:        20,
		},
		"Age": schema.Int{
			MustNotBeZero: true,
			Min:           13,
			Max:           150,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if input.Email != "alice@example.com" {
		t.Fatalf("Email not normalized: %q", input.Email)
	}
	if input.Username != "alice" {
		t.Fatalf("Username not normalized: %q", input.Username)
	}
}

// Case 207b: a type-erased invalid input produces a ValidationError.
func TestIntegration_TypedInputFlow_InvalidProducesError(t *testing.T) {
	input := &mux_input{Email: "bad", Username: "a", Age: 5}
	var type_erased any = input
	_, err := schema.EnforceAny("input", type_erased, schema.Object{
		"Email": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MustBeEmail:   true,
		},
		"Username": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			MinLen:        3,
			MaxLen:        20,
		},
		"Age": schema.Int{
			MustNotBeZero: true,
			Min:           13,
			Max:           150,
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsValidationError(err) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	msg := err.Error()
	for _, want := range []string{"Email", "Username", "Age"} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in error: %q", want, msg)
		}
	}
}

// Case 208: a realistic signup form with normalizations, defaults,
// and validators, applied through a pointer and checked end-to-end.
func TestIntegration_UserSignupForm(t *testing.T) {
	form := &signup_form{
		Email:    "  JOE@Example.com  ",
		Username: "  joe_123  ",
		Password: "supersecret",
		Age:      25,
		Country:  "us",
	}
	_, err := schema.Enforce("s", form, schema.Object{
		"Email": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MustBeEmail:   true,
		},
		"Username": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			MinLen:        3,
			MaxLen:        20,
			AllowedChars:  "abcdefghijklmnopqrstuvwxyz0123456789_",
		},
		"Password": schema.String{
			MustNotBeZero: true,
			MinLen:        8,
			MaxLen:        128,
		},
		"Age": schema.Int{
			MustNotBeZero: true,
			Min:           13,
			Max:           150,
		},
		"Bio": schema.String{
			TrimSpace:     true,
			MaxLen:        500,
			DefaultIfZero: "no bio provided",
		},
		"Country": schema.String{
			MustNotBeZero: true,
			ToUpper:       true,
			MustBeIn:      []string{"US", "CA", "MX", "GB"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if form.Email != "joe@example.com" {
		t.Errorf("Email: got %q", form.Email)
	}
	if form.Username != "joe_123" {
		t.Errorf("Username: got %q", form.Username)
	}
	if form.Bio != "no bio provided" {
		t.Errorf("Bio default did not fire: got %q", form.Bio)
	}
	if form.Country != "US" {
		t.Errorf("Country: got %q", form.Country)
	}
}

// Case 208b: same signup form with multiple errors at once.
func TestIntegration_UserSignupForm_MultipleErrors(t *testing.T) {
	form := signup_form{
		Email:    "not-an-email",
		Username: "!!",
		Password: "short",
		Age:      200,
		Country:  "ZZ",
	}
	_, err := schema.Enforce("s", form, schema.Object{
		"Email": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MustBeEmail:   true,
		},
		"Username": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			MinLen:        3,
			MaxLen:        20,
			AllowedChars:  "abcdefghijklmnopqrstuvwxyz0123456789_",
		},
		"Password": schema.String{
			MustNotBeZero: true,
			MinLen:        8,
			MaxLen:        128,
		},
		"Age": schema.Int{
			MustNotBeZero: true,
			Min:           13,
			Max:           150,
		},
		"Bio": schema.String{
			TrimSpace:     true,
			MaxLen:        500,
			DefaultIfZero: "no bio provided",
		},
		"Country": schema.String{
			MustNotBeZero: true,
			ToUpper:       true,
			MustBeIn:      []string{"US", "CA", "MX", "GB"},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	for _, want := range []string{
		"Email",
		"Username",
		"Password",
		"Age",
		"Country",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("missing %q in error: %q", want, msg)
		}
	}
}

// Case 209: normalize + default + validate + object-level logic all
// in one schema, exercised together.
func TestIntegration_FullTransformPipeline(t *testing.T) {
	form := &pipeline_form{
		Country:   "us",
		StateCode: "cal",
	}
	_, err := schema.Enforce("p", form, schema.Object{
		"Country": schema.String{
			MustNotBeZero: true,
			ToUpper:       true,
			MustBeIn:      []string{"US", "CA"},
		},
		"StateCode": schema.String{
			ToUpper: true,
		},
		"Priority": schema.Int{
			DefaultIfZero: 3,
			Min:           1,
			Max:           5,
		},
		schema.ValidateFunc: func(v pipeline_form) error {
			if v.Country == "US" && len(v.StateCode) != 2 {
				return errors.New("state code must be 2 characters for US")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatalf("expected error for invalid StateCode")
	}
	if !strings.Contains(err.Error(), "state code must be 2 characters for US") {
		t.Fatalf("expected cross-field error, got %q", err.Error())
	}

	form = &pipeline_form{
		Country:   "us",
		StateCode: "ca",
		Priority:  0,
	}
	_, err = schema.Enforce("p", form, schema.Object{
		"Country": schema.String{
			MustNotBeZero: true,
			ToUpper:       true,
			MustBeIn:      []string{"US", "CA"},
		},
		"StateCode": schema.String{
			ToUpper: true,
		},
		"Priority": schema.Int{
			DefaultIfZero: 3,
			Min:           1,
			Max:           5,
		},
		schema.ValidateFunc: func(v pipeline_form) error {
			if v.Country == "US" && len(v.StateCode) != 2 {
				return errors.New("state code must be 2 characters for US")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if form.Country != "US" {
		t.Errorf("Country not uppercased: %q", form.Country)
	}
	if form.StateCode != "CA" {
		t.Errorf("StateCode not uppercased: %q", form.StateCode)
	}
	if form.Priority != 3 {
		t.Errorf("Priority default did not fire: %d", form.Priority)
	}
}

func TestIntegration_ObjectTransformRunsBeforeObjectValidate(t *testing.T) {
	form := &signup_confirmation_form{
		Email:           "  USER@EXAMPLE.COM ",
		Password:        "password",
		PasswordConfirm: " password ",
	}
	_, err := schema.Enforce("signup", form, schema.Object{
		"Email": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MustBeEmail:   true,
		},
		"Password": schema.String{
			MustNotBeZero: true,
			MinLen:        8,
		},
		"PasswordConfirm": schema.String{
			MustNotBeZero: true,
		},
		schema.TransformFunc: func(v signup_confirmation_form) (signup_confirmation_form, error) {
			v.PasswordConfirm = strings.TrimSpace(v.PasswordConfirm)
			return v, nil
		},
		schema.ValidateFunc: func(v signup_confirmation_form) error {
			if v.PasswordConfirm != v.Password {
				return errors.New("passwords must match")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if form.Email != "user@example.com" {
		t.Fatalf("expected normalized email, got %q", form.Email)
	}
	if form.PasswordConfirm != "password" {
		t.Fatalf("expected transformed confirmation, got %q", form.PasswordConfirm)
	}
}

func TestIntegration_NilObjectCallbacksAreSchemaErrors(t *testing.T) {
	form := signup_confirmation_form{}

	var nil_transform func(signup_confirmation_form) (signup_confirmation_form, error)
	_, err := schema.Enforce("signup", form, schema.Object{
		schema.TransformFunc: nil_transform,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "object TransformFunc is nil") {
		t.Fatalf("unexpected error: %v", err)
	}

	var nil_validate func(signup_confirmation_form) error
	_, err = schema.Enforce("signup", form, schema.Object{
		schema.ValidateFunc: nil_validate,
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "object ValidateFunc is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Case 210: applying the same transformations twice through a pointer
// is idempotent.
func TestIntegration_RepeatedApply_PointerIdempotent(t *testing.T) {
	form := &signup_form{
		Email:    "  JOE@Example.com  ",
		Username: "joe_123",
		Password: "supersecret",
		Age:      25,
		Bio:      "hello",
		Country:  "us",
	}
	_, err := schema.Enforce("s", form, schema.Object{
		"Email": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MustBeEmail:   true,
		},
		"Username": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			MinLen:        3,
			MaxLen:        20,
			AllowedChars:  "abcdefghijklmnopqrstuvwxyz0123456789_",
		},
		"Password": schema.String{
			MustNotBeZero: true,
			MinLen:        8,
			MaxLen:        128,
		},
		"Age": schema.Int{
			MustNotBeZero: true,
			Min:           13,
			Max:           150,
		},
		"Bio": schema.String{
			TrimSpace:     true,
			MaxLen:        500,
			DefaultIfZero: "no bio provided",
		},
		"Country": schema.String{
			MustNotBeZero: true,
			ToUpper:       true,
			MustBeIn:      []string{"US", "CA", "MX", "GB"},
		},
	})
	if err != nil {
		t.Fatalf("first enforce error: %v", err)
	}
	first := *form
	_, err = schema.Enforce("s", form, schema.Object{
		"Email": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			ToLower:       true,
			MustBeEmail:   true,
		},
		"Username": schema.String{
			MustNotBeZero: true,
			TrimSpace:     true,
			MinLen:        3,
			MaxLen:        20,
			AllowedChars:  "abcdefghijklmnopqrstuvwxyz0123456789_",
		},
		"Password": schema.String{
			MustNotBeZero: true,
			MinLen:        8,
			MaxLen:        128,
		},
		"Age": schema.Int{
			MustNotBeZero: true,
			Min:           13,
			Max:           150,
		},
		"Bio": schema.String{
			TrimSpace:     true,
			MaxLen:        500,
			DefaultIfZero: "no bio provided",
		},
		"Country": schema.String{
			MustNotBeZero: true,
			ToUpper:       true,
			MustBeIn:      []string{"US", "CA", "MX", "GB"},
		},
	})
	if err != nil {
		t.Fatalf("second enforce error: %v", err)
	}
	second := *form
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("non-idempotent:\n1: %+v\n2: %+v", first, second)
	}
}
