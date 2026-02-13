package wave

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestBuildConfigProviderPayload(t *testing.T) {
	configJSON := []byte(`{
		"Core": {
			"MainAppEntry": "backend/cmd/serve",
			"DistDir": "backend/dist"
		}
	}`)

	configDependencies := ConfigProviderDependencies{
		Files: []string{
			"backend/config/config.go",
			"backend/config/config.go",
			" backend/config/settings.go ",
		},
		Globs: []string{"backend/config/**/*.go"},
		Env:   []string{"PORT"},
	}

	payload, err := BuildConfigProviderPayload(configJSON, configDependencies)
	if err != nil {
		t.Fatalf("BuildConfigProviderPayload returned error: %v", err)
	}

	if payload.Version != 1 {
		t.Fatalf("payload version = %d, want 1", payload.Version)
	}
	if got, want := string(payload.Config), string(configJSON); got != want {
		t.Fatalf("payload config = %q, want %q", got, want)
	}
	if got, want := payload.Dependencies.Files, []string{"backend/config/config.go", "backend/config/settings.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("payload dependency files = %v, want %v", got, want)
	}
	if payload.Fingerprint == "" {
		t.Fatal("expected non-empty payload fingerprint")
	}
}

func TestBuildConfigProviderPayloadFingerprintDeterministic(t *testing.T) {
	configJSONA := []byte(`{"Core":{"MainAppEntry":"backend/cmd/serve","DistDir":"backend/dist"}}`)
	configJSONB := []byte(`{
		"Core": {
			"DistDir": "backend/dist",
			"MainAppEntry": "backend/cmd/serve"
		}
	}`)

	configDependenciesA := ConfigProviderDependencies{
		Files: []string{"backend/config/config.go", "backend/config/settings.go"},
		Globs: []string{"backend/config/**/*.go"},
		Env:   []string{"PORT"},
	}
	configDependenciesB := ConfigProviderDependencies{
		Files: []string{" backend/config/settings.go ", "backend/config/config.go"},
		Globs: []string{"backend/config/**/*.go"},
		Env:   []string{"PORT"},
	}

	payloadA, err := BuildConfigProviderPayload(configJSONA, configDependenciesA)
	if err != nil {
		t.Fatalf("BuildConfigProviderPayload returned error: %v", err)
	}

	payloadB, err := BuildConfigProviderPayload(configJSONB, configDependenciesB)
	if err != nil {
		t.Fatalf("BuildConfigProviderPayload returned error: %v", err)
	}

	if payloadA.Fingerprint != payloadB.Fingerprint {
		t.Fatalf("fingerprint A %q != fingerprint B %q", payloadA.Fingerprint, payloadB.Fingerprint)
	}
}

func TestMarshalConfigProviderPayload(t *testing.T) {
	payloadJSON, err := MarshalConfigProviderPayload(
		[]byte(`{"Core":{"MainAppEntry":"backend/cmd/serve","DistDir":"backend/dist"}}`),
		ConfigProviderDependencies{
			Files: []string{"backend/config/config.go"},
		},
	)
	if err != nil {
		t.Fatalf("MarshalConfigProviderPayload returned error: %v", err)
	}

	var payload ConfigProviderPayload
	if err := json.Unmarshal(payloadJSON, &payload); err != nil {
		t.Fatalf("failed parsing payload JSON: %v", err)
	}

	if payload.Version != 1 {
		t.Fatalf("payload version = %d, want 1", payload.Version)
	}
	if payload.Fingerprint == "" {
		t.Fatal("expected non-empty payload fingerprint")
	}
}

func TestBuildConfigProviderPayloadRejectsInvalidConfig(t *testing.T) {
	_, err := BuildConfigProviderPayload([]byte(`{"Vite":{"DefaultPort":5173}}`), ConfigProviderDependencies{})
	if err == nil {
		t.Fatal("expected BuildConfigProviderPayload to fail for invalid config")
	}
}
