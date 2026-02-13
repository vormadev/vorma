package wavecfg

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestMustMarshalConfigJSON(t *testing.T) {
	configJSON := (&Document{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      "backend/dist",
		},
	}).MustMarshalConfigJSON()

	parsedConfig, err := wave.ParseConfig(configJSON)
	if err != nil {
		t.Fatalf("ParseConfig returned error: %v", err)
	}

	if parsedConfig.Core.MainAppEntry != "backend/cmd/serve" {
		t.Fatalf("main app entry = %q, want backend/cmd/serve", parsedConfig.Core.MainAppEntry)
	}
}

func TestNewWaveConfig(t *testing.T) {
	configDocument := &Document{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      "backend/dist",
		},
	}

	newWaveConfig := NewWaveConfig(configDocument)

	if newWaveConfig.ConfigSource != nil {
		loadedConfig, err := newWaveConfig.ConfigSource.LoadConfig(context.Background())
		if err != nil {
			t.Fatalf("expected config source to load successfully, got %v", err)
		}
		if len(loadedConfig.ConfigJSON) == 0 {
			t.Fatal("expected config source to return non-empty config JSON")
		}

		if len(loadedConfig.Dependencies.Globs) != 0 {
			t.Fatalf(
				"expected no implicit dependencies from NewWaveConfig, got %#v",
				loadedConfig.Dependencies.Globs,
			)
		}

		if newWaveConfig.DistStaticFS == nil {
			t.Fatal("expected dist static filesystem to be configured")
		}
		return
	}
	t.Fatal("expected non-nil config source")
}

func TestNewWaveConfigRejectsNilDocument(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when config document is nil")
		}
	}()

	_ = NewWaveConfig(nil)
}

func TestNewWaveConfigRejectsEmptyDistDir(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic when dist dir is empty")
		}
	}()

	_ = NewWaveConfig(&Document{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
		},
	})
}

func TestDocumentBuildProviderPayloadJSON(t *testing.T) {
	providerPayloadJSON, err := (&Document{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      "backend/dist",
		},
		ConfigDependencies: wave.ConfigProviderDependencies{
			Files: []string{"backend/config/config.go"},
		},
	}).BuildProviderPayloadJSON()
	if err != nil {
		t.Fatalf("document.BuildProviderPayloadJSON returned error: %v", err)
	}

	providerPayload, err := wave.ParseConfigProviderPayload(providerPayloadJSON)
	if err != nil {
		t.Fatalf("ParseConfigProviderPayload returned error: %v", err)
	}

	if got, want := providerPayload.Dependencies.Files, []string{"backend/config/config.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dependency files = %v, want %v", got, want)
	}
}

func TestConfigDependenciesForGoConfigPackage(t *testing.T) {
	configDependencies := ConfigDependenciesForGoConfigPackage("backend/config")
	if got, want := configDependencies.Globs, []string{"backend/config/**/*.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dependency globs = %v, want %v", got, want)
	}
}

func TestConfigDependenciesForGoConfigPackage_NormalizesDotSlashPrefix(t *testing.T) {
	configDependencies := ConfigDependenciesForGoConfigPackage("./backend/config")
	if got, want := configDependencies.Globs, []string{"backend/config/**/*.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("dependency globs = %v, want %v", got, want)
	}
}

func TestConfigDependenciesForGoConfigPackage_RejectsAbsolutePath(t *testing.T) {
	absolutePath := filepath.Join(t.TempDir(), "backend", "config")
	defer func() {
		if recover() == nil {
			t.Fatal("expected absolute config package path to panic")
		}
	}()
	_ = ConfigDependenciesForGoConfigPackage(absolutePath)
}
