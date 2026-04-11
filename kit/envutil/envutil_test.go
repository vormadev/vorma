package envutil

import (
	"os"
	"strconv"
	"testing"
)

func TestGetStr(t *testing.T) {
	// Set up
	key := "TEST_STR"
	defaultValue := "default"

	// Test when environment variable is not set
	if value := GetStr(key, defaultValue); value != defaultValue {
		t.Fatalf("expected %s, got %s", defaultValue, value)
	}

	// Test when environment variable is set
	expectedValue := "test_value"
	os.Setenv(key, expectedValue)
	if value := GetStr(key, defaultValue); value != expectedValue {
		t.Fatalf("expected %s, got %s", expectedValue, value)
	}

	// Clean up
	os.Unsetenv(key)
}

func TestGetInt(t *testing.T) {
	// Set up
	key := "TEST_INT"
	defaultValue := 42

	// Test when environment variable is not set
	if value := GetInt(key, defaultValue); value != defaultValue {
		t.Fatalf("expected %d, got %d", defaultValue, value)
	}

	// Test when environment variable is set to a valid integer
	expectedValue := 100
	os.Setenv(key, strconv.Itoa(expectedValue))
	if value := GetInt(key, defaultValue); value != expectedValue {
		t.Fatalf("expected %d, got %d", expectedValue, value)
	}

	// Test when environment variable is set to an invalid integer
	os.Setenv(key, "invalid")
	if value := GetInt(key, defaultValue); value != defaultValue {
		t.Fatalf("expected %d, got %d (when env var is invalid)", defaultValue, value)
	}

	// Clean up
	os.Unsetenv(key)
}

func TestGetBool(t *testing.T) {
	// Set up
	key := "TEST_BOOL"
	defaultValue := true

	// Test when environment variable is not set
	if value := GetBool(key, defaultValue); value != defaultValue {
		t.Fatalf("expected %v, got %v", defaultValue, value)
	}

	// Test when environment variable is set to a valid boolean
	os.Setenv(key, "false")
	if value := GetBool(key, defaultValue); value != false {
		t.Fatalf("expected false, got %v", value)
	}

	// Test when environment variable is set to an invalid boolean
	os.Setenv(key, "invalid")
	if value := GetBool(key, defaultValue); value != defaultValue {
		t.Fatalf("expected %v, got %v (when env var is invalid)", defaultValue, value)
	}

	// Clean up
	os.Unsetenv(key)
}

func TestEdgeCases(t *testing.T) {
	// Test Int with extreme values
	key := "TEST_INT_EXTREME"
	os.Setenv(key, "2147483647") // Max int32
	if value := GetInt(key, 0); value != 2147483647 {
		t.Fatalf("expected 2147483647, got %d", value)
	}

	os.Setenv(key, "-2147483648") // Min int32
	if value := GetInt(key, 0); value != -2147483648 {
		t.Fatalf("expected -2147483648, got %d", value)
	}

	// Test Bool with various valid representations
	key = "TEST_BOOL_REPRESENTATION"
	os.Setenv(key, "1")
	if value := GetBool(key, false); value != true {
		t.Fatalf("expected true for '1', got %v", value)
	}

	os.Setenv(key, "0")
	if value := GetBool(key, true); value != false {
		t.Fatalf("expected false for '0', got %v", value)
	}

	os.Setenv(key, "t")
	if value := GetBool(key, false); value != true {
		t.Fatalf("expected true for 't', got %v", value)
	}

	os.Setenv(key, "f")
	if value := GetBool(key, true); value != false {
		t.Fatalf("expected false for 'f', got %v", value)
	}

	// Clean up
	os.Unsetenv("TEST_INT_EXTREME")
	os.Unsetenv("TEST_BOOL_REPRESENTATION")
}
