package wave

import (
	"os"
	"testing"
)

func TestGetIsDevAndSetModeToDev(t *testing.T) {
	t.Setenv(envMode, "production")
	if GetIsDev() {
		t.Fatal("expected GetIsDev=false when WAVE_MODE is not development")
	}

	SetModeToDev()
	if !GetIsDev() {
		t.Fatal("expected GetIsDev=true after SetModeToDev")
	}
}

func TestGetPortAndSetPort(t *testing.T) {
	t.Setenv(envPort, "")
	if got := GetPort(); got != 0 {
		t.Fatalf("expected empty PORT to return 0, got %d", got)
	}

	SetPort(4242)
	if got := GetPort(); got != 4242 {
		t.Fatalf("expected SetPort to update PORT, got %d", got)
	}

	t.Setenv(envPort, "not-a-number")
	if got := GetPort(); got != 0 {
		t.Fatalf("expected invalid PORT to return 0, got %d", got)
	}
}

func TestMustGetPortNonDevDefaultsTo8080(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "")

	if got := MustGetPort(); got != 8080 {
		t.Fatalf("expected default non-dev port 8080, got %d", got)
	}
}

func TestMustGetPortNonDevUsesConfiguredPort(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "9090")

	if got := MustGetPort(); got != 9090 {
		t.Fatalf("expected configured non-dev port 9090, got %d", got)
	}
}

func TestMustGetPortDevChoosesPortAndMarksSet(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, envModeDev)
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "32123")

	got := MustGetPort()
	if got <= 0 {
		t.Fatalf("expected positive port in dev mode, got %d", got)
	}
	if env := GetPort(); env != got {
		t.Fatalf("expected PORT env to match returned port (%d), got %d", got, env)
	}
	if flag := os.Getenv(envPortSet); flag != "true" {
		t.Fatalf("expected %s to be true, got %q", envPortSet, flag)
	}
}

func TestMustGetPortCachesResultAfterFirstCall(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPortSet, "")
	SetPort(5001)

	first := MustGetPort()
	SetPort(5002)
	second := MustGetPort()

	if first != 5001 || second != 5001 {
		t.Fatalf("expected MustGetPort to cache first result, got first=%d second=%d", first, second)
	}
	if MustGetAppPort() != first {
		t.Fatalf("expected MustGetAppPort alias to match MustGetPort result %d", first)
	}
}

func TestGetAndSetRefreshServerPort(t *testing.T) {
	t.Setenv(envRefreshServerPort, "")
	if got := GetRefreshServerPort(); got != 0 {
		t.Fatalf("expected empty refresh port to return 0, got %d", got)
	}

	SetRefreshServerPort(10999)
	if got := GetRefreshServerPort(); got != 10999 {
		t.Fatalf("expected refresh port 10999, got %d", got)
	}

	t.Setenv(envRefreshServerPort, "invalid")
	if got := GetRefreshServerPort(); got != 0 {
		t.Fatalf("expected invalid refresh port to return 0, got %d", got)
	}
}
