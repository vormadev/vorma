package wave

import (
	"errors"
	"os"
	"testing"

	"github.com/vormadev/vorma/wave/internal/wavecore"
)

func stubGetFreePortForTest(
	t *testing.T,
	getFreePortFunc func(int) (int, error),
) {
	t.Helper()

	restore := wavecore.SetGetFreePortForTest(getFreePortFunc)
	t.Cleanup(restore)
}

func TestGetIsDevAndSetModeToDev(t *testing.T) {
	t.Setenv(envMode, "production")
	if GetIsDev() {
		t.Fatal("expected GetIsDev=false when __WAVE_MODE is not development")
	}

	SetModeToDev()
	if !GetIsDev() {
		t.Fatal("expected GetIsDev=true after SetModeToDev")
	}
}

func TestEnvPort(t *testing.T) {
	t.Setenv(envPort, "")
	if got := parseEnvPort(); got != 0 {
		t.Fatalf("expected empty PORT to return 0, got %d", got)
	}

	t.Setenv(envPort, "4242")
	if got := parseEnvPort(); got != 4242 {
		t.Fatalf("expected PORT=4242 to parse to 4242, got %d", got)
	}

	t.Setenv(envPort, "not-a-number")
	if got := parseEnvPort(); got != 0 {
		t.Fatalf("expected invalid PORT to return 0, got %d", got)
	}

	t.Setenv(envPort, "-1")
	if got := parseEnvPort(); got != 0 {
		t.Fatalf("expected negative PORT to return 0, got %d", got)
	}

	t.Setenv(envPort, "70000")
	if got := parseEnvPort(); got != 0 {
		t.Fatalf("expected out-of-range PORT to return 0, got %d", got)
	}
}

func TestMustGetPortNonDevPanicsWhenPortMissing(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "")

	defer func() {
		if recover() == nil {
			t.Fatal(
				"expected MustGetPort to panic when PORT is missing in production mode",
			)
		}
	}()
	_ = MustGetPort()
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

func TestMustGetPortNonDevPanicsWhenPortInvalid(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "not-a-number")

	defer func() {
		if recover() == nil {
			t.Fatal(
				"expected MustGetPort to panic when PORT is invalid in production mode",
			)
		}
	}()
	_ = MustGetPort()
}

func TestMustGetPortDevChoosesPortAndMarksSet(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, envModeDev)
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "32123")
	stubGetFreePortForTest(t, func(port int) (int, error) {
		if port != 32123 {
			t.Fatalf("expected free-port lookup from 32123, got %d", port)
		}
		return 32124, nil
	})

	got := MustGetPort()
	if got != 32124 {
		t.Fatalf("expected deterministic free-port result 32124, got %d", got)
	}
	if env := parseEnvPort(); env != got {
		t.Fatalf(
			"expected PORT env to match returned port (%d), got %d",
			got,
			env,
		)
	}
	if flag := os.Getenv(envPortSet); flag != "true" {
		t.Fatalf("expected %s to be true, got %q", envPortSet, flag)
	}
}

func TestMustGetPortDevUsesFallbackBasePortWhenPortMissing(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, envModeDev)
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "")

	stubGetFreePortForTest(t, func(port int) (int, error) {
		if port != 0 {
			t.Fatalf(
				"expected free-port lookup from fallback base port 0, got %d",
				port,
			)
		}
		return 38080, nil
	})

	got := MustGetPort()
	if got != 38080 {
		t.Fatalf("expected fallback free-port result 38080, got %d", got)
	}
	if env := parseEnvPort(); env != got {
		t.Fatalf(
			"expected PORT env to match returned port (%d), got %d",
			got,
			env,
		)
	}
	if flag := os.Getenv(envPortSet); flag != "true" {
		t.Fatalf("expected %s to be true, got %q", envPortSet, flag)
	}
}

func TestMustGetPortDevPanicsWhenRequestedPortIsInvalid(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, envModeDev)
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "0")

	defer func() {
		if recover() == nil {
			t.Fatal("expected MustGetPort to panic when dev PORT is invalid")
		}
	}()
	_ = MustGetPort()
}

func TestMustGetPortDevPanicsWhenFreePortResolutionFails(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, envModeDev)
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "32123")

	stubGetFreePortForTest(t, func(int) (int, error) {
		return 0, errors.New("boom")
	})

	defer func() {
		if recover() == nil {
			t.Fatal("expected Port to panic when free-port resolution fails")
		}
	}()
	_ = MustGetPort()
}

func TestMustGetPortCachesResultAfterFirstCall(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPortSet, "")
	t.Setenv(envPort, "5001")

	first := MustGetPort()
	t.Setenv(envPort, "5002")
	second := MustGetPort()

	if first != 5001 || second != 5001 {
		t.Fatalf(
			"expected Port to cache first result, got first=%d second=%d",
			first,
			second,
		)
	}
}

func TestPortResolverInstancesCacheIndependently(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, "production")
	t.Setenv(envPort, "5001")

	firstResolver := newPortResolver()
	if got := firstResolver.MustGetPort(); got != 5001 {
		t.Fatalf("expected first resolver to return 5001, got %d", got)
	}

	t.Setenv(envPort, "5002")
	secondResolver := newPortResolver()
	if got := secondResolver.MustGetPort(); got != 5002 {
		t.Fatalf("expected second resolver to return 5002, got %d", got)
	}

	if got := firstResolver.MustGetPort(); got != 5001 {
		t.Fatalf("expected first resolver cache to remain 5001, got %d", got)
	}
}

func TestMustGetPortDevHonorsPortWhenAlreadySet(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, envModeDev)
	t.Setenv(envPortSet, "true")
	t.Setenv(envPort, "3333")

	if got := MustGetPort(); got != 3333 {
		t.Fatalf("expected Port to honor already-set dev port, got %d", got)
	}
}

func TestMustGetPortDevAlreadySetPanicsForInvalidPort(t *testing.T) {
	resetPortCacheForTest()
	t.Setenv(envMode, envModeDev)
	t.Setenv(envPortSet, "true")
	t.Setenv(envPort, "")

	defer func() {
		if recover() == nil {
			t.Fatal(
				"expected MustGetPort to panic for invalid pre-set dev port",
			)
		}
	}()
	_ = MustGetPort()
}

func TestGetAndSetRefreshServerPort(t *testing.T) {
	t.Setenv(envRefreshServerPort, "")
	if got := getRefreshServerPort(); got != 0 {
		t.Fatalf("expected empty refresh port to return 0, got %d", got)
	}

	setRefreshServerPort(10999)
	if got := getRefreshServerPort(); got != 10999 {
		t.Fatalf("expected refresh port 10999, got %d", got)
	}

	t.Setenv(envRefreshServerPort, "invalid")
	if got := getRefreshServerPort(); got != 0 {
		t.Fatalf("expected invalid refresh port to return 0, got %d", got)
	}

	t.Setenv(envRefreshServerPort, "-10")
	if got := getRefreshServerPort(); got != 0 {
		t.Fatalf("expected negative refresh port to return 0, got %d", got)
	}

	t.Setenv(envRefreshServerPort, "70000")
	if got := getRefreshServerPort(); got != 0 {
		t.Fatalf("expected out-of-range refresh port to return 0, got %d", got)
	}
}
