package wavecfg

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/vormadev/vorma/wave"
)

const (
	reservedSectionNameCore  = "Core"
	reservedSectionNameVite  = "Vite"
	reservedSectionNameWatch = "Watch"
)

type Document struct {
	Core  wave.CoreConfig
	Vite  *wave.ViteConfig
	Watch *wave.WatchConfig

	ConfigDependencies wave.ConfigProviderDependencies
	Custom             map[string]any
}

func (document *Document) MarshalConfigJSON() ([]byte, error) {
	if document == nil {
		return nil, fmt.Errorf("config document is nil")
	}

	return json.Marshal(document)
}

func (document *Document) MustMarshalConfigJSON() []byte {
	configJSON, err := document.MarshalConfigJSON()
	if err != nil {
		panic(err)
	}
	return configJSON
}

func (document *Document) BuildProviderPayloadJSON() ([]byte, error) {
	configJSON, err := document.MarshalConfigJSON()
	if err != nil {
		return nil, fmt.Errorf("marshal config document: %w", err)
	}

	return wave.MarshalConfigProviderPayload(configJSON, document.ConfigDependencies)
}

func (document *Document) MustBuildProviderPayloadJSON() []byte {
	providerPayloadJSON, err := document.BuildProviderPayloadJSON()
	if err != nil {
		panic(err)
	}
	return providerPayloadJSON
}

func (document *Document) EmitProviderPayloadToStdout() {
	providerPayloadJSON := document.MustBuildProviderPayloadJSON()
	if _, err := os.Stdout.Write(providerPayloadJSON); err != nil {
		panic(err)
	}
}

func (document *Document) MarshalJSON() ([]byte, error) {
	if document == nil {
		return nil, fmt.Errorf("config document is nil")
	}

	rootDocument := make(map[string]json.RawMessage, 3+len(document.Custom))

	coreJSON, err := json.Marshal(document.Core)
	if err != nil {
		return nil, fmt.Errorf("marshal core config: %w", err)
	}
	rootDocument[reservedSectionNameCore] = coreJSON

	if document.Vite != nil {
		viteJSON, err := json.Marshal(document.Vite)
		if err != nil {
			return nil, fmt.Errorf("marshal Vite config: %w", err)
		}
		rootDocument[reservedSectionNameVite] = viteJSON
	}

	if document.Watch != nil {
		watchJSON, err := json.Marshal(document.Watch)
		if err != nil {
			return nil, fmt.Errorf("marshal watch config: %w", err)
		}
		rootDocument[reservedSectionNameWatch] = watchJSON
	}

	for sectionName, sectionValue := range document.Custom {
		normalizedSectionName, err := normalizeCustomSectionName(sectionName)
		if err != nil {
			return nil, err
		}

		sectionJSON, err := json.Marshal(sectionValue)
		if err != nil {
			return nil, fmt.Errorf("marshal section %q: %w", normalizedSectionName, err)
		}
		rootDocument[normalizedSectionName] = sectionJSON
	}

	marshaledDocumentJSON, err := json.Marshal(rootDocument)
	if err != nil {
		return nil, fmt.Errorf("marshal config document: %w", err)
	}

	return marshaledDocumentJSON, nil
}

func normalizeCustomSectionName(sectionName string) (string, error) {
	normalizedSectionName := strings.TrimSpace(sectionName)
	if normalizedSectionName == "" {
		return "", fmt.Errorf("section name is required")
	}

	switch normalizedSectionName {
	case reservedSectionNameCore, reservedSectionNameVite, reservedSectionNameWatch:
		return "", fmt.Errorf("section name %q is reserved", normalizedSectionName)
	}

	return normalizedSectionName, nil
}
