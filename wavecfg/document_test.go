package wavecfg

import (
	"encoding/json"
	"io"
	"os"
	"reflect"
	"testing"

	"github.com/vormadev/vorma/wave"
)

func TestDocumentMarshalConfigJSON(t *testing.T) {
	document := &Document{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      "backend/dist",
		},
		Vite: &wave.ViteConfig{
			JSPackageManagerBaseCmd: "pnpm",
		},
		Watch: &wave.WatchConfig{
			HealthcheckEndpoint: "/healthz",
		},
		ConfigDependencies: wave.ConfigProviderDependencies{
			Files: []string{"backend/config/config.go"},
		},
		Custom: map[string]any{
			"Vorma": map[string]any{"UIVariant": "solid"},
		},
	}

	configJSON, err := document.MarshalConfigJSON()
	if err != nil {
		t.Fatalf("document.MarshalConfigJSON returned error: %v", err)
	}

	var root map[string]any
	if err := json.Unmarshal(configJSON, &root); err != nil {
		t.Fatalf("failed to parse config JSON: %v", err)
	}

	if _, exists := root["Core"]; !exists {
		t.Fatal("missing Core section")
	}
	if _, exists := root["Vite"]; !exists {
		t.Fatal("missing Vite section")
	}
	if _, exists := root["Watch"]; !exists {
		t.Fatal("missing Watch section")
	}
	if _, exists := root["Vorma"]; !exists {
		t.Fatal("missing custom Vorma section")
	}
	if _, exists := root["ConfigDependencies"]; exists {
		t.Fatal("ConfigDependencies should not be serialized into config JSON")
	}
}

func TestDocumentMarshalConfigJSONRejectsReservedSectionName(t *testing.T) {
	document := &Document{
		Core: wave.CoreConfig{},
		Custom: map[string]any{
			"Core": map[string]any{},
		},
	}
	if _, err := document.MarshalConfigJSON(); err == nil {
		t.Fatal("expected reserved section name to fail")
	}
}

func TestBuildProviderPayloadJSON(t *testing.T) {
	document := &Document{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      "backend/dist",
		},
		ConfigDependencies: wave.ConfigProviderDependencies{
			Files: []string{"backend/config/config.go"},
		},
	}

	payloadJSON, err := document.BuildProviderPayloadJSON()
	if err != nil {
		t.Fatalf("document.BuildProviderPayloadJSON returned error: %v", err)
	}

	payload, err := wave.ParseConfigProviderPayload(payloadJSON)
	if err != nil {
		t.Fatalf("failed parsing payload JSON: %v", err)
	}

	if payload.Version != 1 {
		t.Fatalf("payload version = %d, want 1", payload.Version)
	}
	if got, want := payload.Dependencies.Files, []string{"backend/config/config.go"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("payload dependency files = %v, want %v", got, want)
	}
}

func TestDocumentEmitProviderPayloadToStdout(t *testing.T) {
	document := &Document{
		Core: wave.CoreConfig{
			MainAppEntry: "backend/cmd/serve",
			DistDir:      "backend/dist",
		},
	}

	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed creating stdout pipe: %v", err)
	}
	defer stdoutReader.Close()

	originalStdout := os.Stdout
	os.Stdout = stdoutWriter
	t.Cleanup(func() {
		os.Stdout = originalStdout
	})

	document.EmitProviderPayloadToStdout()

	if err := stdoutWriter.Close(); err != nil {
		t.Fatalf("failed closing stdout writer: %v", err)
	}

	outputBytes, err := io.ReadAll(stdoutReader)
	if err != nil {
		t.Fatalf("failed reading stdout output: %v", err)
	}

	payload, err := wave.ParseConfigProviderPayload(outputBytes)
	if err != nil {
		t.Fatalf("failed parsing provider payload output: %v", err)
	}

	if payload.Version != 1 {
		t.Fatalf("payload version = %d, want 1", payload.Version)
	}
}
