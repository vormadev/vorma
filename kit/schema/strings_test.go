package schema_test

import (
	"errors"
	"regexp"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type string_holder struct {
	V string
}

type int_field_holder struct {
	N int
}

var test_regex = regexp.MustCompile(`^[a-z]+\d+$`)

type email_newtype string

func (email_newtype) Schema() schema.Schema {
	return schema.String{
		MustNotBeZero: true,
		TrimSpace:     true,
		ToLower:       true,
		MustBeEmail:   true,
	}
}

type cross_field struct {
	Country   string
	StateCode string
}

/////////////////////////////////////////////////////////////////////
/////// NORMALIZATIONS
/////////////////////////////////////////////////////////////////////

func TestString_TrimSpace(t *testing.T) {
	res, err := schema.Enforce("s", string_holder{V: "  hello  "}, schema.Object{
		"V": schema.String{TrimSpace: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != "hello" {
		t.Fatalf("got %q", res.Value.V)
	}
}

func TestString_ToLower(t *testing.T) {
	res, err := schema.Enforce("s", string_holder{V: "HELLO"}, schema.Object{
		"V": schema.String{ToLower: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != "hello" {
		t.Fatalf("got %q", res.Value.V)
	}
}

func TestString_ToUpper(t *testing.T) {
	res, err := schema.Enforce("s", string_holder{V: "hello"}, schema.Object{
		"V": schema.String{ToUpper: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != "HELLO" {
		t.Fatalf("got %q", res.Value.V)
	}
}

func TestString_ToLower_BeatsToUpper(t *testing.T) {
	res, err := schema.Enforce("s", string_holder{V: "Hello"}, schema.Object{
		"V": schema.String{ToLower: true, ToUpper: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != "hello" {
		t.Fatalf("ToLower should win; got %q", res.Value.V)
	}
}

func TestString_TransformFunc(t *testing.T) {
	res, err := schema.Enforce("s", string_holder{V: "hi"}, schema.Object{
		"V": schema.String{
			TransformFunc: func(v string) (string, error) {
				return v + "!", nil
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != "hi!" {
		t.Fatalf("got %q", res.Value.V)
	}
}

func TestString_TransformFunc_Error(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "hi"}, schema.Object{
		"V": schema.String{
			TransformFunc: func(string) (string, error) {
				return "", errors.New("boom")
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsValidationError(err) {
		t.Fatalf("expected ValidationError, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected callback error message in output, got %q", err.Error())
	}
}

func TestString_NormalizeThenDefaultIfZero_WhitespaceOnlyGetsDefault(t *testing.T) {
	res, err := schema.Enforce("s", string_holder{V: "   "}, schema.Object{
		"V": schema.String{
			TrimSpace:     true,
			DefaultIfZero: "fallback",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != "fallback" {
		t.Fatalf("expected default to fire after trim; got %q", res.Value.V)
	}
}

/////////////////////////////////////////////////////////////////////
/////// LENGTH VALIDATORS
/////////////////////////////////////////////////////////////////////

func TestString_MinLen(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "ab"}, schema.Object{
		"V": schema.String{MinLen: 3},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsValidationError(err) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
}

func TestString_MaxLen(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "abcd"}, schema.Object{
		"V": schema.String{MaxLen: 3},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestString_MinLen_RuneCount(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "日本語"}, schema.Object{
		"V": schema.String{MinLen: 3, MaxLen: 3},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_NegativeLengthLimit_IsSchemaError(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "abc"}, schema.Object{
		"V": schema.String{MinLen: -1},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

func TestString_ContradictoryLengthLimits_AreSchemaErrors(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "abc"}, schema.Object{
		"V": schema.String{MinLen: 5, MaxLen: 3},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

/////////////////////////////////////////////////////////////////////
/////// MEMBERSHIP VALIDATORS
/////////////////////////////////////////////////////////////////////

func TestString_MustBeIn_Valid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "b"}, schema.Object{
		"V": schema.String{MustBeIn: []string{"a", "b", "c"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustBeIn_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "z"}, schema.Object{
		"V": schema.String{MustBeIn: []string{"a", "b", "c"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestString_MustBeIn_EmptySlice_NoCheck(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "anything"}, schema.Object{
		"V": schema.String{MustBeIn: []string{}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustNotBeIn_Valid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "ok"}, schema.Object{
		"V": schema.String{MustNotBeIn: []string{"bad", "worse"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustNotBeIn_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "bad"}, schema.Object{
		"V": schema.String{MustNotBeIn: []string{"bad", "worse"}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

/////////////////////////////////////////////////////////////////////
/////// CONTENT VALIDATORS
/////////////////////////////////////////////////////////////////////

func TestString_MustBeEmail_Valid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "foo@bar.com"}, schema.Object{
		"V": schema.String{MustBeEmail: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustBeEmail_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "not-an-email"}, schema.Object{
		"V": schema.String{MustBeEmail: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestString_MustBeEmail_Empty_WithMustNotBeZero(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: ""}, schema.Object{
		"V": schema.String{MustNotBeZero: true, MustBeEmail: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "must not be empty") {
		t.Fatalf("expected empty-string error, got %q", err.Error())
	}
}

func TestString_MustBeURL_Valid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "https://example.com/path?q=1"}, schema.Object{
		"V": schema.String{MustBeURL: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustBeURL_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "not a url"}, schema.Object{
		"V": schema.String{MustBeURL: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestString_MustMatch_Valid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "abc123"}, schema.Object{
		"V": schema.String{MustMatch: test_regex},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustMatch_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "123abc"}, schema.Object{
		"V": schema.String{MustMatch: test_regex},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestString_MustMatch_Nil_NoCheck(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "anything"}, schema.Object{
		"V": schema.String{MustMatch: nil},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustStartWith_Valid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "prefix"}, schema.Object{
		"V": schema.String{MustStartWith: "pre"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustStartWith_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "nostart"}, schema.Object{
		"V": schema.String{MustStartWith: "pre"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestString_MustEndWith_Valid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "theend"}, schema.Object{
		"V": schema.String{MustEndWith: "end"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_MustEndWith_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "nofinish"}, schema.Object{
		"V": schema.String{MustEndWith: "end"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestString_AllowedChars_Valid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "cab"}, schema.Object{
		"V": schema.String{AllowedChars: "abc"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_AllowedChars_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "abcd"}, schema.Object{
		"V": schema.String{AllowedChars: "abc"},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestString_AppliedToNonString_SchemaError(t *testing.T) {
	_, err := schema.Enforce("s", int_field_holder{N: 5}, schema.Object{
		"N": schema.String{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

/////////////////////////////////////////////////////////////////////
/////// VALUE-MODE (NEWTYPE)
/////////////////////////////////////////////////////////////////////

func TestString_ValueMode_Newtype_Normalize(t *testing.T) {
	res, err := schema.Enforce("e", email_newtype("  FOO@BAR.COM  "))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(res.Value) != "foo@bar.com" {
		t.Fatalf("got %q", res.Value)
	}
}

func TestString_ValueMode_Newtype_Validate(t *testing.T) {
	_, err := schema.Enforce("e", email_newtype("not-an-email"))
	if err == nil {
		t.Fatalf("expected error")
	}
}

/////////////////////////////////////////////////////////////////////
/////// VALIDATE FUNC / OBJECT-LEVEL VALIDATE
/////////////////////////////////////////////////////////////////////

func TestString_ValidateFunc_RunsAfterValidators(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "MIXED"}, schema.Object{
		"V": schema.String{
			ToLower: true,
			ValidateFunc: func(v string) error {
				if v != strings.ToLower(v) {
					return errors.New("expected normalized input in ValidateFunc")
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_ObjectValidate_CrossField(t *testing.T) {
	_, err := schema.Enforce("s", cross_field{Country: "US", StateCode: "CAL"}, schema.Object{
		"Country": schema.String{MustNotBeZero: true, ToUpper: true},
		"StateCode": schema.String{
			ToUpper: true,
		},
		schema.ValidateFunc: func(v cross_field) error {
			if v.Country == "US" && len(v.StateCode) != 2 {
				return errors.New("state must be 2 chars for US")
			}
			return nil
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}

	_, err = schema.Enforce("s", cross_field{Country: "US", StateCode: "CA"}, schema.Object{
		"Country": schema.String{MustNotBeZero: true, ToUpper: true},
		"StateCode": schema.String{
			ToUpper: true,
		},
		schema.ValidateFunc: func(v cross_field) error {
			if v.Country == "US" && len(v.StateCode) != 2 {
				return errors.New("state must be 2 chars for US")
			}
			return nil
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_ValidateFunc_Nil_Omitted(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "x"}, schema.Object{
		"V": schema.String{},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestString_ValidateFunc_CanAccumulateIntent(t *testing.T) {
	_, err := schema.Enforce("s", string_holder{V: "x"}, schema.Object{
		"V": schema.String{
			ValidateFunc: func(v string) error {
				var errs []error
				if !strings.Contains(v, "a") {
					errs = append(errs, errors.New("must contain a"))
				}
				if !strings.Contains(v, "b") {
					errs = append(errs, errors.New("must contain b"))
				}
				if len(errs) == 0 {
					return nil
				}
				return errors.Join(errs...)
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "must contain a") {
		t.Errorf("missing 'a' error: %q", msg)
	}
	if !strings.Contains(msg, "must contain b") {
		t.Errorf("missing 'b' error: %q", msg)
	}
}
