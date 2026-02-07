package validate

import (
	"regexp"
	"testing"
)

func TestIfCondition(t *testing.T) {
	t.Run("True condition executes function", func(t *testing.T) {
		executed := false
		Any("test", "value").If(true, func(c *AnyChecker) *AnyChecker {
			executed = true
			return c
		})

		if !executed {
			t.Error("function should have been executed for true condition")
		}
	})

	t.Run("False condition skips function", func(t *testing.T) {
		executed := false
		Any("test", "value").If(false, func(c *AnyChecker) *AnyChecker {
			executed = true
			return c
		})

		if executed {
			t.Error("function should not have been executed for false condition")
		}
	})

	t.Run("Already done checker skips function", func(t *testing.T) {
		executed := false
		checker := Any("test", nil).Required()

		checker.If(true, func(c *AnyChecker) *AnyChecker {
			executed = true
			return c
		})

		if executed {
			t.Error("function should not have been executed when checker is done")
		}
	})
}

func TestInValidation(t *testing.T) {
	t.Run("Valid value in slice", func(t *testing.T) {
		allowed := []string{"apple", "banana", "cherry"}
		err := Any("fruit", "banana").In(allowed).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid value not in slice", func(t *testing.T) {
		allowed := []string{"apple", "banana", "cherry"}
		err := Any("fruit", "orange").In(allowed).Error()

		if err == nil {
			t.Error("expected error for value not in allowed list")
		}
	})

	t.Run("Non-slice value", func(t *testing.T) {
		err := Any("fruit", "orange").In("not-a-slice").Error()

		if err == nil {
			t.Error("expected error for non-slice argument")
		}
	})

	t.Run("Empty allowed slice", func(t *testing.T) {
		err := Any("fruit", "orange").In([]string{}).Error()

		if err == nil {
			t.Error("expected error for empty allowed slice")
		}
	})

	t.Run("Nil allowed slice", func(t *testing.T) {
		err := Any("fruit", "orange").In(nil).Error()
		if err == nil {
			t.Error("expected error for nil allowed slice")
		}
	})

	t.Run("Nil candidate value", func(t *testing.T) {
		err := Any("fruit", nil).In([]string{"apple"}).Error()
		if err == nil {
			t.Error("expected error for nil candidate value")
		}
	})
}

func TestNotInValidation(t *testing.T) {
	t.Run("Valid value not in prohibited slice", func(t *testing.T) {
		prohibited := []string{"apple", "banana", "cherry"}
		err := Any("fruit", "orange").NotIn(prohibited).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid value in prohibited slice", func(t *testing.T) {
		prohibited := []string{"apple", "banana", "cherry"}
		err := Any("fruit", "banana").NotIn(prohibited).Error()

		if err == nil {
			t.Error("expected error for value in prohibited list")
		}
	})

	t.Run("Non-slice value", func(t *testing.T) {
		err := Any("fruit", "orange").NotIn("not-a-slice").Error()

		if err == nil {
			t.Error("expected error for non-slice argument")
		}
	})

	t.Run("Empty prohibited slice", func(t *testing.T) {
		err := Any("fruit", "orange").NotIn([]string{}).Error()

		if err == nil {
			t.Error("expected error for empty prohibited slice")
		}
	})

	t.Run("Nil prohibited slice", func(t *testing.T) {
		err := Any("fruit", "orange").NotIn(nil).Error()
		if err == nil {
			t.Error("expected error for nil prohibited slice")
		}
	})
}

func TestInValidationWithPointers(t *testing.T) {
	t.Run("Valid pointer value in slice of values", func(t *testing.T) {
		allowed := []string{"apple", "banana", "cherry"}
		value := "banana"
		err := Any("fruit", &value).In(allowed).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Valid value in slice of pointers", func(t *testing.T) {
		apple, banana, cherry := "apple", "banana", "cherry"
		allowed := []*string{&apple, &banana, &cherry}
		err := Any("fruit", "banana").In(allowed).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Valid pointer value in slice of pointers", func(t *testing.T) {
		apple, banana, cherry := "apple", "banana", "cherry"
		allowed := []*string{&apple, &banana, &cherry}
		value := "banana"
		err := Any("fruit", &value).In(allowed).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestNotInValidationWithPointers(t *testing.T) {
	t.Run("Valid pointer value not in prohibited slice", func(t *testing.T) {
		prohibited := []string{"apple", "banana", "cherry"}
		value := "orange"
		err := Any("fruit", &value).NotIn(prohibited).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Valid value not in prohibited slice of pointers", func(t *testing.T) {
		apple, banana, cherry := "apple", "banana", "cherry"
		prohibited := []*string{&apple, &banana, &cherry}
		err := Any("fruit", "orange").NotIn(prohibited).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Valid pointer value not in prohibited slice of pointers", func(t *testing.T) {
		apple, banana, cherry := "apple", "banana", "cherry"
		prohibited := []*string{&apple, &banana, &cherry}
		value := "orange"
		err := Any("fruit", &value).NotIn(prohibited).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestSimpleCustomTypeValidation(t *testing.T) {
	type CustomType string

	a := CustomType("a")
	b := CustomType("b")

	var customType *CustomType = &a

	allowed := []string{"a", "b", "c"}

	err := Any("CustomType", customType).In(allowed).Error()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	allowedTypes := []CustomType{a, b}
	err = Any("CustomType", customType).In(allowedTypes).Error()

	if err != nil {
		t.Errorf("unexpected error with typed slice: %v", err)
	}
}

func TestCustomTypeValidation(t *testing.T) {
	// String-based custom types
	type CustomString string
	type AnotherCustomString string

	// Int-based custom types
	type CustomInt int
	type AnotherCustomInt int

	// Float-based custom types
	type CustomFloat float64

	t.Run("Custom string type against string slice", func(t *testing.T) {
		cs := CustomString("test")
		allowed := []string{"foo", "test", "bar"}
		err := Any("value", cs).In(allowed).Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("String against custom string type slice", func(t *testing.T) {
		allowed := []CustomString{"foo", "test", "bar"}
		err := Any("value", "test").In(allowed).Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Different custom string types", func(t *testing.T) {
		cs := CustomString("test")
		allowed := []AnotherCustomString{"foo", "test", "bar"}
		err := Any("value", cs).In(allowed).Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Custom int type against int slice", func(t *testing.T) {
		ci := CustomInt(42)
		allowed := []int{10, 42, 100}
		err := Any("value", ci).In(allowed).Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Int against custom int type slice", func(t *testing.T) {
		allowed := []CustomInt{10, 42, 100}
		err := Any("value", 42).In(allowed).Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Different custom int types", func(t *testing.T) {
		ci := CustomInt(42)
		allowed := []AnotherCustomInt{10, 42, 100}
		err := Any("value", ci).In(allowed).Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Custom float type against float slice", func(t *testing.T) {
		cf := CustomFloat(3.14)
		allowed := []float64{1.23, 3.14, 5.67}
		err := Any("value", cf).In(allowed).Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	// NotIn tests
	t.Run("NotIn with custom types", func(t *testing.T) {
		cs := CustomString("test")
		prohibited := []string{"foo", "bar", "baz"}
		err := Any("value", cs).NotIn(prohibited).Error()
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("NotIn with custom types - should fail", func(t *testing.T) {
		cs := CustomString("foo")
		prohibited := []string{"foo", "bar", "baz"}
		err := Any("value", cs).NotIn(prohibited).Error()
		if err == nil {
			t.Errorf("expected error but got none")
		}
	})
}

func TestMutuallyExclusiveFields(t *testing.T) {
	type TestStruct struct {
		Field1 string
		Field2 string
		Field3 string
	}

	t.Run("No fields provided", func(t *testing.T) {
		obj := TestStruct{}
		err := Object(obj).MutuallyExclusive("test_group", "Field1", "Field2", "Field3").Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("One field provided", func(t *testing.T) {
		obj := TestStruct{Field1: "value"}
		err := Object(obj).MutuallyExclusive("test_group", "Field1", "Field2", "Field3").Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Multiple fields provided", func(t *testing.T) {
		obj := TestStruct{Field1: "value1", Field2: "value2"}
		err := Object(obj).MutuallyExclusive("test_group", "Field1", "Field2", "Field3").Error()

		if err == nil {
			t.Error("expected error for multiple fields in mutually exclusive group")
		}
	})
}

func TestMutuallyRequiredFields(t *testing.T) {
	type TestStruct struct {
		Field1 string
		Field2 string
		Field3 string
	}

	t.Run("No fields provided", func(t *testing.T) {
		obj := TestStruct{}
		err := Object(obj).MutuallyRequired("test_group", "Field1", "Field2", "Field3").Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("All fields provided", func(t *testing.T) {
		obj := TestStruct{Field1: "value1", Field2: "value2", Field3: "value3"}
		err := Object(obj).MutuallyRequired("test_group", "Field1", "Field2", "Field3").Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Some fields missing", func(t *testing.T) {
		obj := TestStruct{Field1: "value1"}
		err := Object(obj).MutuallyRequired("test_group", "Field1", "Field2", "Field3").Error()

		if err == nil {
			t.Error("expected error for some fields missing in mutually required group")
		}
	})
}

func TestPermittedChars(t *testing.T) {
	t.Run("Valid string with permitted chars", func(t *testing.T) {
		err := Any("test", "abc123").PermittedChars("abcdefghijklmnopqrstuvwxyz0123456789").Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid string with disallowed chars", func(t *testing.T) {
		err := Any("test", "abc!").PermittedChars("abcdefghijklmnopqrstuvwxyz0123456789").Error()

		if err == nil {
			t.Error("expected error for string with disallowed characters")
		}
	})

	t.Run("Non-string value", func(t *testing.T) {
		err := Any("test", 123).PermittedChars("0123456789").Error()

		if err == nil {
			t.Error("expected error for non-string value")
		}
	})
}

func TestEmailValidation(t *testing.T) {
	t.Run("Valid email", func(t *testing.T) {
		err := Any("email", "test@example.com").Email().Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid email", func(t *testing.T) {
		err := Any("email", "not-an-email").Email().Error()

		if err == nil {
			t.Error("expected error for invalid email")
		}
	})

	t.Run("Empty email", func(t *testing.T) {
		err := Any("email", "").Email().Error()

		if err == nil {
			t.Error("expected error for empty email")
		}
	})

	t.Run("Non-string value", func(t *testing.T) {
		err := Any("email", 123).Email().Error()

		if err == nil {
			t.Error("expected error for non-string value")
		}
	})
}

func TestRegexValidation(t *testing.T) {
	t.Run("Valid string matching regex", func(t *testing.T) {
		re := regexp.MustCompile(`^[a-z]+\d+$`)
		err := Any("test", "abc123").Regex(re).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid string not matching regex", func(t *testing.T) {
		re := regexp.MustCompile(`^[a-z]+\d+$`)
		err := Any("test", "123abc").Regex(re).Error()

		if err == nil {
			t.Error("expected error for string not matching regex")
		}
	})

	t.Run("Nil regex", func(t *testing.T) {
		err := Any("test", "abc123").Regex(nil).Error()

		if err == nil {
			t.Error("expected error for nil regex")
		}
	})

	t.Run("Non-string value", func(t *testing.T) {
		re := regexp.MustCompile(`^[a-z]+\d+$`)
		err := Any("test", 123).Regex(re).Error()

		if err == nil {
			t.Error("expected error for non-string value")
		}
	})
}

func TestStartsWithValidation(t *testing.T) {
	t.Run("Valid string with prefix", func(t *testing.T) {
		err := Any("test", "prefixedString").StartsWith("prefix").Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid string without prefix", func(t *testing.T) {
		err := Any("test", "string").StartsWith("prefix").Error()

		if err == nil {
			t.Error("expected error for string without required prefix")
		}
	})

	t.Run("Non-string value", func(t *testing.T) {
		err := Any("test", 123).StartsWith("prefix").Error()

		if err == nil {
			t.Error("expected error for non-string value")
		}
	})
}

func TestEndsWithValidation(t *testing.T) {
	t.Run("Valid string with suffix", func(t *testing.T) {
		err := Any("test", "stringSuffixed").EndsWith("Suffixed").Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid string without suffix", func(t *testing.T) {
		err := Any("test", "string").EndsWith("Suffix").Error()

		if err == nil {
			t.Error("expected error for string without required suffix")
		}
	})

	t.Run("Non-string value", func(t *testing.T) {
		err := Any("test", 123).EndsWith("Suffix").Error()

		if err == nil {
			t.Error("expected error for non-string value")
		}
	})
}

func TestURLValidation(t *testing.T) {
	t.Run("Valid URL", func(t *testing.T) {
		err := Any("url", "https://example.com/path?query=1").URL().Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid URL", func(t *testing.T) {
		err := Any("url", "not a url").URL().Error()

		if err == nil {
			t.Error("expected error for invalid URL")
		}
	})

	t.Run("Non-string value", func(t *testing.T) {
		err := Any("url", 123).URL().Error()

		if err == nil {
			t.Error("expected error for non-string value")
		}
	})
}

func TestNumericMin(t *testing.T) {
	t.Run("Integer above minimum", func(t *testing.T) {
		err := Any("number", 10).Min(5).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Integer equal to minimum", func(t *testing.T) {
		err := Any("number", 5).Min(5).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Integer below minimum", func(t *testing.T) {
		err := Any("number", 3).Min(5).Error()

		if err == nil {
			t.Error("expected error for value below minimum")
		}
	})

	t.Run("Float above minimum", func(t *testing.T) {
		err := Any("number", 5.5).Min(5.0).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("String length above minimum", func(t *testing.T) {
		err := Any("string", "hello").Min(3).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("String length below minimum", func(t *testing.T) {
		err := Any("string", "hi").Min(3).Error()

		if err == nil {
			t.Error("expected error for string length below minimum")
		}
	})

	t.Run("Slice length above minimum", func(t *testing.T) {
		err := Any("slice", []int{1, 2, 3, 4}).Min(3).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Map length below minimum", func(t *testing.T) {
		err := Any("map", map[string]int{"a": 1}).Min(3).Error()

		if err == nil {
			t.Error("expected error for map length below minimum")
		}
	})

	t.Run("Non-numeric type", func(t *testing.T) {
		err := Any("complex", complex(1, 2)).Min(3).Error()

		if err == nil {
			t.Error("expected error for non-numeric type")
		}
	})
}

func TestNumericMax(t *testing.T) {
	t.Run("Integer below maximum", func(t *testing.T) {
		err := Any("number", 3).Max(5).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Integer equal to maximum", func(t *testing.T) {
		err := Any("number", 5).Max(5).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Integer above maximum", func(t *testing.T) {
		err := Any("number", 10).Max(5).Error()

		if err == nil {
			t.Error("expected error for value above maximum")
		}
	})

	t.Run("Float below maximum", func(t *testing.T) {
		err := Any("number", 4.5).Max(5.0).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("String length below maximum", func(t *testing.T) {
		err := Any("string", "hello").Max(10).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("String length above maximum", func(t *testing.T) {
		err := Any("string", "hello world").Max(5).Error()

		if err == nil {
			t.Error("expected error for string length above maximum")
		}
	})

	t.Run("Slice length below maximum", func(t *testing.T) {
		err := Any("slice", []int{1, 2}).Max(5).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Map length above maximum", func(t *testing.T) {
		err := Any("map", map[string]int{"a": 1, "b": 2, "c": 3, "d": 4}).Max(3).Error()

		if err == nil {
			t.Error("expected error for map length above maximum")
		}
	})
}

func TestMinMaxMixed(t *testing.T) {
	t.Run("Valid integer within min and max", func(t *testing.T) {
		err := Any("number", 7).Min(5).Max(10).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Invalid integer below min", func(t *testing.T) {
		err := Any("number", 3).Min(5).Max(10).Error()

		if err == nil {
			t.Error("expected error for value below minimum")
		}
	})

	t.Run("Invalid integer above max", func(t *testing.T) {
		err := Any("number", 12).Min(5).Max(10).Error()

		if err == nil {
			t.Error("expected error for value above maximum")
		}
	})
}

func TestRangeInclusive(t *testing.T) {
	t.Run("Integer within range", func(t *testing.T) {
		err := Any("number", 7).RangeInclusive(5, 10).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Integer at lower bound", func(t *testing.T) {
		err := Any("number", 5).RangeInclusive(5, 10).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Integer at upper bound", func(t *testing.T) {
		err := Any("number", 10).RangeInclusive(5, 10).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Integer below range", func(t *testing.T) {
		err := Any("number", 3).RangeInclusive(5, 10).Error()

		if err == nil {
			t.Error("expected error for value below range")
		}
	})

	t.Run("Integer above range", func(t *testing.T) {
		err := Any("number", 12).RangeInclusive(5, 10).Error()

		if err == nil {
			t.Error("expected error for value above range")
		}
	})

	t.Run("String length within range", func(t *testing.T) {
		err := Any("string", "hello").RangeInclusive(3, 10).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Slice length outside range", func(t *testing.T) {
		err := Any("slice", []int{1, 2}).RangeInclusive(3, 10).Error()

		if err == nil {
			t.Error("expected error for slice length outside range")
		}
	})
}

func TestRangeExclusive(t *testing.T) {
	t.Run("Integer within range", func(t *testing.T) {
		err := Any("number", 7).RangeExclusive(5, 10).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Integer at lower bound", func(t *testing.T) {
		err := Any("number", 5).RangeExclusive(5, 10).Error()

		if err == nil {
			t.Error("expected error for value at lower bound")
		}
	})

	t.Run("Integer at upper bound", func(t *testing.T) {
		err := Any("number", 10).RangeExclusive(5, 10).Error()

		if err == nil {
			t.Error("expected error for value at upper bound")
		}
	})

	t.Run("Integer below range", func(t *testing.T) {
		err := Any("number", 3).RangeExclusive(5, 10).Error()

		if err == nil {
			t.Error("expected error for value below range")
		}
	})

	t.Run("Integer above range", func(t *testing.T) {
		err := Any("number", 12).RangeExclusive(5, 10).Error()

		if err == nil {
			t.Error("expected error for value above range")
		}
	})

	t.Run("String length within range", func(t *testing.T) {
		err := Any("string", "hello").RangeExclusive(2, 10).Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Map length outside range", func(t *testing.T) {
		err := Any("map", map[string]int{"a": 1, "b": 2}).RangeExclusive(2, 5).Error()

		if err == nil {
			t.Error("expected error for map length at lower bound of exclusive range")
		}
	})
}

// Test chain validations
func TestChainValidations(t *testing.T) {
	t.Run("Multiple validations pass", func(t *testing.T) {
		err := Any("username", "user123").
			Required().
			PermittedChars("abcdefghijklmnopqrstuvwxyz0123456789").
			Min(5).
			Max(20).
			Error()

		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("Multiple validations with failure", func(t *testing.T) {
		err := Any("username", "user@123").
			Required().
			PermittedChars("abcdefghijklmnopqrstuvwxyz0123456789").
			Min(5).
			Max(20).
			Error()

		if err == nil {
			t.Error("expected error for invalid character in string")
		}
	})

	t.Run("Validation stops after failure", func(t *testing.T) {
		executed := false
		checker := Any("username", "").Required()
		checker.Error() // This will fail

		checker.If(true, func(c *AnyChecker) *AnyChecker {
			executed = true
			return c
		})

		if executed {
			t.Error("further validation should not execute after failure")
		}

		if err := checker.Error(); err == nil {
			t.Error("expected error for required check")
		}
	})
}
