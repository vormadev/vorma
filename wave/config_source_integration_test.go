package wave

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestNewWithConfigSource(t *testing.T) {
	fixture := newWaveTestFixture(t)
	source := &stubConfigSource{
		loadedConfig: &LoadedConfig{
			ConfigJSON:  fixture.configJSON(t),
			Fingerprint: "provider-fingerprint",
			Dependencies: ConfigProviderDependencies{
				Files: []string{"backend/wave.config.go"},
				Globs: []string{"backend/**/*.go"},
				Env:   []string{"WAVE_MODE"},
			},
		},
	}

	w := New(Config{
		ConfigSource: source,
		DistStaticFS: nil,
		Logger:       newDiscardLoggerForWaveTests(),
	})

	if source.callCount != 1 {
		t.Fatalf("config source call count = %d, want 1", source.callCount)
	}
	if got := w.GetConfigFingerprint(); got != "provider-fingerprint" {
		t.Fatalf("fingerprint = %q, want provider-fingerprint", got)
	}
	if got, want := w.GetConfigDependencies().Files, []string{"backend/wave.config.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dependencies.files = %v, want %v", got, want)
	}
	if got := w.RawConfigJSON(); !reflect.DeepEqual(got, fixture.configJSON(t)) {
		t.Fatalf("raw config JSON mismatch")
	}
}

func TestNewPanicsWhenConfigSourceFails(t *testing.T) {
	source := &stubConfigSource{
		err: errors.New("source failed"),
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("expected panic")
		}
		panicText := recovered.(string)
		if !strings.Contains(panicText, "load config source") {
			t.Fatalf("unexpected panic: %v", recovered)
		}
		if !strings.Contains(panicText, "source failed") {
			t.Fatalf("unexpected panic: %v", recovered)
		}
	}()

	_ = New(Config{
		ConfigSource: source,
		Logger:       newDiscardLoggerForWaveTests(),
	})
}

type stubConfigSource struct {
	loadedConfig *LoadedConfig
	err          error
	callCount    int
}

func (source *stubConfigSource) LoadConfig(ctx context.Context) (*LoadedConfig, error) {
	_ = ctx
	source.callCount++
	if source.err != nil {
		return nil, source.err
	}
	return source.loadedConfig, nil
}
