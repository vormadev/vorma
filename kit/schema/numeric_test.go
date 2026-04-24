package schema_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/vormadev/vorma/kit/schema"
)

/////////////////////////////////////////////////////////////////////
/////// FIXTURES
/////////////////////////////////////////////////////////////////////

type int_holder struct{ V int }
type int8_holder struct{ V int8 }
type int16_holder struct{ V int16 }
type int32_holder struct{ V int32 }
type int64_holder struct{ V int64 }

type uint_holder struct{ V uint }
type u8_holder struct{ V uint8 }
type u16_holder struct{ V uint16 }
type u32_holder struct{ V uint32 }
type u64_holder struct{ V uint64 }

type float_holder struct{ V float64 }
type f32_holder struct{ V float32 }
type f64_holder struct{ V float64 }

type string_value_holder struct{ V string }
type signed_value_holder struct{ V int }

type user_id int

func (user_id) Schema() schema.Schema {
	return schema.Int{Min: 1}
}

/////////////////////////////////////////////////////////////////////
/////// INT
/////////////////////////////////////////////////////////////////////

func TestInt_MustNotBeZero_Zero_Fails(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 0}, schema.Object{
		"V": schema.Int{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestInt_MustNotBeZero_NonZero_Passes(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 5}, schema.Object{
		"V": schema.Int{MustNotBeZero: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInt_DefaultIfZero_Fires(t *testing.T) {
	res, err := schema.Enforce("s", int_holder{V: 0}, schema.Object{
		"V": schema.Int{DefaultIfZero: 42},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != 42 {
		t.Fatalf("got %d", res.Value.V)
	}
}

func TestInt_Min_BelowFails(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 5}, schema.Object{
		"V": schema.Int{Min: 10},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestInt_Min_EqualPasses(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 10}, schema.Object{
		"V": schema.Int{Min: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInt_Max_AboveFails(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 15}, schema.Object{
		"V": schema.Int{Max: 10},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestInt_MinMax_Combined(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 3}, schema.Object{
		"V": schema.Int{Min: 5, Max: 10},
	})
	if err == nil {
		t.Fatalf("expected error for below Min")
	}
	_, err = schema.Enforce("s", int_holder{V: 12}, schema.Object{
		"V": schema.Int{Min: 5, Max: 10},
	})
	if err == nil {
		t.Fatalf("expected error for above Max")
	}
	_, err = schema.Enforce("s", int_holder{V: 7}, schema.Object{
		"V": schema.Int{Min: 5, Max: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error in range: %v", err)
	}
}

func TestInt_ContradictoryBounds_AreSchemaErrors(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 3}, schema.Object{
		"V": schema.Int{Min: 5, Max: 1},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

func TestInt_MustBeIn_Valid(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 2}, schema.Object{
		"V": schema.Int{MustBeIn: []int{1, 2, 3}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInt_MustBeIn_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 9}, schema.Object{
		"V": schema.Int{MustBeIn: []int{1, 2, 3}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestInt_MustNotBeIn_Invalid(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 2}, schema.Object{
		"V": schema.Int{MustNotBeIn: []int{1, 2, 3}},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestInt_AppliedToNonInt_SchemaError(t *testing.T) {
	_, err := schema.Enforce("s", string_value_holder{V: "x"}, schema.Object{
		"V": schema.Int{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

func TestInt_Newtype_ValueMode(t *testing.T) {
	_, err := schema.Enforce("uid", user_id(5))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = schema.Enforce("uid", user_id(0))
	if err == nil {
		t.Fatalf("expected error for zero")
	}
}

func TestInt_AllSignedKinds(t *testing.T) {
	cases := []struct {
		name  string
		apply func() error
	}{
		{"int8", func() error {
			_, err := schema.Enforce(
				"s",
				int8_holder{V: 5},
				schema.Object{"V": schema.Int{MustNotBeZero: true, Min: 1, Max: 100}},
			)
			return err
		}},
		{"int16", func() error {
			_, err := schema.Enforce(
				"s",
				int16_holder{V: 5},
				schema.Object{"V": schema.Int{MustNotBeZero: true, Min: 1, Max: 100}},
			)
			return err
		}},
		{"int32", func() error {
			_, err := schema.Enforce(
				"s",
				int32_holder{V: 5},
				schema.Object{"V": schema.Int{MustNotBeZero: true, Min: 1, Max: 100}},
			)
			return err
		}},
		{"int64", func() error {
			_, err := schema.Enforce(
				"s",
				int64_holder{V: 5},
				schema.Object{"V": schema.Int{MustNotBeZero: true, Min: 1, Max: 100}},
			)
			return err
		}},
	}
	for _, c := range cases {
		if err := c.apply(); err != nil {
			t.Errorf("%s: unexpected error: %v", c.name, err)
		}
	}
}

func TestInt_ValidateFunc(t *testing.T) {
	_, err := schema.Enforce("s", int_holder{V: 3}, schema.Object{
		"V": schema.Int{
			ValidateFunc: func(v int) error {
				if v%2 != 0 {
					return errors.New("must be even")
				}
				return nil
			},
		},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
	if !strings.Contains(err.Error(), "must be even") {
		t.Fatalf("got %q", err.Error())
	}

	_, err = schema.Enforce("s", int_holder{V: 4}, schema.Object{
		"V": schema.Int{
			ValidateFunc: func(v int) error {
				if v%2 != 0 {
					return errors.New("must be even")
				}
				return nil
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

/////////////////////////////////////////////////////////////////////
/////// UINT
/////////////////////////////////////////////////////////////////////

func TestUint_MustNotBeZero_Zero_Fails(t *testing.T) {
	_, err := schema.Enforce("s", uint_holder{V: 0}, schema.Object{
		"V": schema.Uint{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestUint_DefaultIfZero(t *testing.T) {
	res, err := schema.Enforce("s", uint_holder{V: 0}, schema.Object{
		"V": schema.Uint{DefaultIfZero: uint(100)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != 100 {
		t.Fatalf("got %d", res.Value.V)
	}
}

func TestUint_MinMax(t *testing.T) {
	_, err := schema.Enforce("s", uint_holder{V: 3}, schema.Object{
		"V": schema.Uint{Min: 5, Max: 10},
	})
	if err == nil {
		t.Fatalf("expected error below Min")
	}
	_, err = schema.Enforce("s", uint_holder{V: 15}, schema.Object{
		"V": schema.Uint{Min: 5, Max: 10},
	})
	if err == nil {
		t.Fatalf("expected error above Max")
	}
	_, err = schema.Enforce("s", uint_holder{V: 7}, schema.Object{
		"V": schema.Uint{Min: 5, Max: 10},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUint_MustBeInAndMustNotBeIn(t *testing.T) {
	_, err := schema.Enforce("s", uint_holder{V: 2}, schema.Object{
		"V": schema.Uint{
			MustBeIn:    []uint{1, 2, 3},
			MustNotBeIn: []uint{99},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = schema.Enforce("s", uint_holder{V: 99}, schema.Object{
		"V": schema.Uint{
			MustBeIn:    []uint{1, 2, 3},
			MustNotBeIn: []uint{99},
		},
	})
	if err == nil {
		t.Fatalf("expected error for prohibited value")
	}
	_, err = schema.Enforce("s", uint_holder{V: 7}, schema.Object{
		"V": schema.Uint{
			MustBeIn:    []uint{1, 2, 3},
			MustNotBeIn: []uint{99},
		},
	})
	if err == nil {
		t.Fatalf("expected error for non-permitted value")
	}
}

func TestUint_AllUnsignedKinds(t *testing.T) {
	cases := []func() error{
		func() error {
			_, err := schema.Enforce(
				"s",
				u8_holder{V: 5},
				schema.Object{"V": schema.Uint{MustNotBeZero: true, Min: 1, Max: 100}},
			)
			return err
		},
		func() error {
			_, err := schema.Enforce(
				"s",
				u16_holder{V: 5},
				schema.Object{"V": schema.Uint{MustNotBeZero: true, Min: 1, Max: 100}},
			)
			return err
		},
		func() error {
			_, err := schema.Enforce(
				"s",
				u32_holder{V: 5},
				schema.Object{"V": schema.Uint{MustNotBeZero: true, Min: 1, Max: 100}},
			)
			return err
		},
		func() error {
			_, err := schema.Enforce(
				"s",
				u64_holder{V: 5},
				schema.Object{"V": schema.Uint{MustNotBeZero: true, Min: 1, Max: 100}},
			)
			return err
		},
	}
	for i, c := range cases {
		if err := c(); err != nil {
			t.Errorf("case %d: unexpected error: %v", i, err)
		}
	}
}

func TestUint_AppliedToSignedInt_SchemaError(t *testing.T) {
	_, err := schema.Enforce("s", signed_value_holder{V: 5}, schema.Object{
		"V": schema.Uint{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}

/////////////////////////////////////////////////////////////////////
/////// FLOAT
/////////////////////////////////////////////////////////////////////

func TestFloat_MustNotBeZero_Zero_Fails(t *testing.T) {
	_, err := schema.Enforce("s", float_holder{V: 0}, schema.Object{
		"V": schema.Float{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestFloat_DefaultIfZero(t *testing.T) {
	res, err := schema.Enforce("s", float_holder{V: 0}, schema.Object{
		"V": schema.Float{DefaultIfZero: 3.14},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Value.V != 3.14 {
		t.Fatalf("got %v", res.Value.V)
	}
}

func TestFloat_MinMax(t *testing.T) {
	_, err := schema.Enforce("s", float_holder{V: 0.5}, schema.Object{
		"V": schema.Float{Min: 1.0, Max: 10.0},
	})
	if err == nil {
		t.Fatalf("expected error below Min")
	}
	_, err = schema.Enforce("s", float_holder{V: 11.0}, schema.Object{
		"V": schema.Float{Min: 1.0, Max: 10.0},
	})
	if err == nil {
		t.Fatalf("expected error above Max")
	}
	_, err = schema.Enforce("s", float_holder{V: 5.0}, schema.Object{
		"V": schema.Float{Min: 1.0, Max: 10.0},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestFloat_MustBeInAndMustNotBeIn(t *testing.T) {
	_, err := schema.Enforce("s", float_holder{V: 2.5}, schema.Object{
		"V": schema.Float{
			MustBeIn:    []float64{1.5, 2.5, 3.5},
			MustNotBeIn: []float64{99.9},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err = schema.Enforce("s", float_holder{V: 99.9}, schema.Object{
		"V": schema.Float{
			MustBeIn:    []float64{1.5, 2.5, 3.5},
			MustNotBeIn: []float64{99.9},
		},
	})
	if err == nil {
		t.Fatalf("expected error for prohibited value")
	}
	_, err = schema.Enforce("s", float_holder{V: 1.0}, schema.Object{
		"V": schema.Float{
			MustBeIn:    []float64{1.5, 2.5, 3.5},
			MustNotBeIn: []float64{99.9},
		},
	})
	if err == nil {
		t.Fatalf("expected error for non-permitted value")
	}
}

func TestFloat_Float32_And_Float64(t *testing.T) {
	_, err := schema.Enforce("s", f32_holder{V: 5.5}, schema.Object{
		"V": schema.Float{MustNotBeZero: true, Min: 1.0},
	})
	if err != nil {
		t.Fatalf("float32: unexpected error: %v", err)
	}
	_, err = schema.Enforce("s", f64_holder{V: 5.5}, schema.Object{
		"V": schema.Float{MustNotBeZero: true, Min: 1.0},
	})
	if err != nil {
		t.Fatalf("float64: unexpected error: %v", err)
	}
}

func TestFloat_AppliedToInt_SchemaError(t *testing.T) {
	_, err := schema.Enforce("s", signed_value_holder{V: 5}, schema.Object{
		"V": schema.Float{MustNotBeZero: true},
	})
	if err == nil {
		t.Fatalf("expected SchemaError")
	}
	if !schema.IsSchemaError(err) {
		t.Fatalf("expected SchemaError, got %T: %v", err, err)
	}
}
