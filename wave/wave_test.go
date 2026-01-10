package wave

import (
	"os"
	"testing"
)

func resetEnv() {
	os.Unsetenv("WAVE_MODE")
	os.Unsetenv("PORT")
	os.Unsetenv("WAVE_PORT_HAS_BEEN_SET")
	os.Unsetenv("WAVE_REFRESH_SERVER_PORT")
}

func TestGetIsDev(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		want     bool
	}{
		{"DevMode", "development", true},
		{"ProdMode", "production", false},
		{"EmptyMode", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resetEnv()
			if tt.envValue != "" {
				os.Setenv("WAVE_MODE", tt.envValue)
			}
			if got := GetIsDev(); got != tt.want {
				t.Errorf("GetIsDev() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSetModeToDev(t *testing.T) {
	resetEnv()

	// Verify not in dev mode initially
	if GetIsDev() {
		t.Errorf("GetIsDev() = true before SetModeToDev(), want false")
	}

	// Set to dev mode
	SetModeToDev()

	// Verify now in dev mode
	if !GetIsDev() {
		t.Errorf("GetIsDev() = false after SetModeToDev(), want true")
	}

	// Clean up
	resetEnv()
}

func TestMustGetPortInvalidValue(t *testing.T) {
	resetEnv()

	// Test with invalid port value - should return default
	os.Setenv("PORT", "invalid")
	if got := MustGetPort(); got != 8080 {
		t.Errorf("MustGetPort() with invalid value = %v, want default %v", got, 8080)
	}

	// Test with negative port - returns the parsed value (strconv parses it)
	os.Setenv("PORT", "-1")
	// Note: strconv.Atoi("-1") returns -1, which is <= 0, so default is used
	if got := MustGetPort(); got != 8080 {
		t.Errorf("MustGetPort() with negative value = %v, want default %v", got, 8080)
	}

	// Test with zero port
	os.Setenv("PORT", "0")
	if got := MustGetPort(); got != 8080 {
		t.Errorf("MustGetPort() with zero = %v, want default %v", got, 8080)
	}

	// Clean up
	resetEnv()
}

func TestConstants(t *testing.T) {
	// Verify exported constants match expected values
	if OnChangeStrategyPre != "pre" {
		t.Errorf("OnChangeStrategyPre = %v, want %v", OnChangeStrategyPre, "pre")
	}
	if OnChangeStrategyConcurrent != "concurrent" {
		t.Errorf("OnChangeStrategyConcurrent = %v, want %v", OnChangeStrategyConcurrent, "concurrent")
	}
	if OnChangeStrategyConcurrentNoWait != "concurrent-no-wait" {
		t.Errorf("OnChangeStrategyConcurrentNoWait = %v, want %v", OnChangeStrategyConcurrentNoWait, "concurrent-no-wait")
	}
	if OnChangeStrategyPost != "post" {
		t.Errorf("OnChangeStrategyPost = %v, want %v", OnChangeStrategyPost, "post")
	}
	if PrehashedDirname != "prehashed" {
		t.Errorf("PrehashedDirname = %v, want %v", PrehashedDirname, "prehashed")
	}
}
