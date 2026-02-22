package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/wave"
	"github.com/vormadev/vorma/wave/tooling/internal/shared"
)

// Processor builds and writes Wave configuration schema documents.
type Processor struct {
	cfg *wave.ParsedConfig
	log *slog.Logger
}

// NewProcessor creates a schema processor for one parsed config.
func NewProcessor(cfg *wave.ParsedConfig, log *slog.Logger) *Processor {
	if log == nil {
		log = slog.Default()
	}
	return &Processor{cfg: cfg, log: log}
}

// WriteSchema writes schema.json into dist internal directory.
func (processor *Processor) WriteSchema() error {
	if processor == nil || processor.cfg == nil {
		return errors.New("schema processor config is nil")
	}

	schemaDocument, buildSchemaError := processor.BuildSchemaDocument()
	if buildSchemaError != nil {
		return buildSchemaError
	}

	schemaBytes, marshalError := json.MarshalIndent(schemaDocument, "", "  ")
	if marshalError != nil {
		return fmt.Errorf("marshal schema document: %w", marshalError)
	}

	targetPath := filepath.Join(processor.cfg.Dist.Internal(), "schema.json")
	if writeError := shared.WriteFileAtomically(targetPath, schemaBytes, 0o644); writeError != nil {
		return writeError
	}
	processor.log.Info("wrote wave config schema", "path", targetPath)
	return nil
}

// BuildSchemaDocument builds schema object for wave config JSON.
func (processor *Processor) BuildSchemaDocument() (map[string]any, error) {
	if processor == nil || processor.cfg == nil {
		return nil, errors.New("schema processor config is nil")
	}

	coreSchema := processor.buildCoreSchema()
	watchSchema := processor.buildWatchSchema()
	viteSchema := processor.buildViteSchema()

	properties := map[string]any{
		"Core":  coreSchema,
		"Watch": watchSchema,
		"Vite":  viteSchema,
	}

	if len(processor.cfg.FrameworkSchemaExtensions) > 0 {
		frameworkExtensionProperties, extensionError := processor.buildFrameworkExtensionProperties()
		if extensionError != nil {
			return nil, extensionError
		}
		for extensionName, extensionProperty := range frameworkExtensionProperties {
			properties[extensionName] = extensionProperty
		}
	}

	document := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
		"$id":                  "https://vorma.dev/schemas/wave.json",
		"title":                "Wave Configuration",
		"type":                 "object",
		"properties":           properties,
		"required":             []string{"Core"},
		"additionalProperties": false,
	}

	return document, nil
}

// buildCoreSchema builds schema for Core section.
func (processor *Processor) buildCoreSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required": []string{
			"MainAppEntry",
			"DistDir",
			"StaticAssetDirs",
		},
		"properties": map[string]any{
			"ConfigLocation": stringSchema(
				"Configuration file path used for reload-aware tooling workflows.",
			),
			"MainAppEntry": stringSchema(
				"Go application entry file path for build execution.",
			),
			"DistDir": stringSchema(
				"Build output directory root for binaries and static artifacts.",
			),
			"PublicPathPrefix": map[string]any{
				"type":        "string",
				"description": "Public URL prefix for static assets. Must be slash-rooted when provided.",
			},
			"ServerOnlyMode": boolSchema(
				"When true, browser runtime integrations are disabled.",
			),
			"SequentialGoBuild": boolSchema(
				"When true, go compilation waits for hook/build stages to finish.",
			),
			"DevBuildHook": stringSchema(
				"Optional shell command run before dev build output is finalized.",
			),
			"DevBuildHookTimeoutMilliseconds": numberSchema(
				"Timeout for dev build hook command in milliseconds.",
			),
			"ProdBuildHook": stringSchema(
				"Optional shell command run before production build output is finalized.",
			),
			"ProdBuildHookTimeoutMilliseconds": numberSchema(
				"Timeout for production build hook command in milliseconds.",
			),
			"StaticAssetDirs": map[string]any{
				"type":                 "object",
				"required":             []string{"Public", "Private"},
				"additionalProperties": false,
				"properties": map[string]any{
					"Public": stringSchema(
						"Source directory for public static assets.",
					),
					"Private": stringSchema(
						"Source directory for private static assets.",
					),
				},
			},
			"CSSEntryFiles": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"Critical": stringSchema(
						"Critical CSS entry file path.",
					),
					"NonCritical": stringSchema(
						"Non-critical CSS entry file path.",
					),
				},
			},
		},
	}
}

// buildWatchSchema builds schema for Watch section.
func (processor *Processor) buildWatchSchema() map[string]any {
	hookSchema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"Cmd": stringSchema(
				"Shell command to execute for this hook.",
			),
			"Timing": stringSchema(
				"Hook stage name: pre, concurrent, concurrent_no_wait, or post.",
			),
			"RunCombinedDevBuildHookCommands": boolSchema(
				"When true, explicit command is combined with configured dev build hooks.",
			),
			"CommandTimeoutMilliseconds": numberSchema(
				"Command timeout override for this hook execution.",
			),
			"DisableStageCommandTimeout": boolSchema(
				"When true, stage-level command timeout is ignored.",
			),
			"CallbackTimeoutMilliseconds": numberSchema(
				"Callback timeout override for this hook execution.",
			),
			"DisableStageCallbackTimeout": boolSchema(
				"When true, stage-level callback timeout is ignored.",
			),
			"Exclude": arrayOfStringsSchema(
				"Glob patterns excluded from this hook.",
			),
		},
	}

	watchedFileSchema := map[string]any{
		"type":                 "object",
		"required":             []string{"Pattern"},
		"additionalProperties": false,
		"properties": map[string]any{
			"Pattern": stringSchema(
				"Glob pattern for watched file selection.",
			),
			"OnChangeHooks": map[string]any{
				"type":  "array",
				"items": hookSchema,
			},
			"RecompileGoBinary": boolSchema(
				"When true, go binary recompilation is requested.",
			),
			"RestartApp": boolSchema(
				"When true, app process restart is requested.",
			),
			"OnlyRunClientDefinedRevalidateFunc": boolSchema(
				"When true, browser action prefers framework revalidate callback.",
			),
			"RunOnChangeOnly": boolSchema(
				"When true, skip implicit build and run hooks only.",
			),
			"SkipRebuildingNotification": boolSchema(
				"When true, rebuilding overlay notification is suppressed.",
			),
			"TreatAsNonGo": boolSchema(
				"When true, .go files matching this pattern are not treated as go recompilation triggers.",
			),
		},
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"WatchRoot": stringSchema(
				"Root path used by recursive watch registration.",
			),
			"HealthcheckEndpoint": stringSchema(
				"HTTP endpoint path used for app readiness polling.",
			),
			"HookStageFailurePolicy": stringSchema(
				"Pipeline policy when hook stages report execution errors.",
			),
			"Include": map[string]any{
				"type":  "array",
				"items": watchedFileSchema,
			},
			"Exclude": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"Dirs": arrayOfStringsSchema(
						"Directory glob patterns excluded from watch registration.",
					),
					"Files": arrayOfStringsSchema(
						"File glob patterns excluded from event processing.",
					),
				},
			},
		},
	}
}

// buildViteSchema builds schema for Vite section.
func (processor *Processor) buildViteSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"JSPackageManagerBaseCmd": stringSchema(
				"Base command used to run Vite (for example, pnpm).",
			),
			"JSPackageManagerCmdDir": stringSchema(
				"Directory where JS package manager command should run.",
			),
			"DefaultPort": numberSchema("Preferred Vite port."),
			"ViteConfigFile": stringSchema(
				"Optional explicit Vite config file path.",
			),
		},
	}
}

// buildFrameworkExtensionProperties converts framework extension entries to root properties.
func (processor *Processor) buildFrameworkExtensionProperties() (map[string]any, error) {
	serializedExtensionEntries := make(map[string]any)
	for fieldName, extensionEntry := range processor.cfg.FrameworkSchemaExtensions {
		entryJSON, marshalError := json.Marshal(extensionEntry)
		if marshalError != nil {
			return nil, fmt.Errorf(
				"marshal framework schema extension %q: %w",
				fieldName,
				marshalError,
			)
		}
		entryMap := make(map[string]any)
		if unmarshalError := json.Unmarshal(entryJSON, &entryMap); unmarshalError != nil {
			return nil, fmt.Errorf(
				"decode framework schema extension %q: %w",
				fieldName,
				unmarshalError,
			)
		}
		serializedExtensionEntries[fieldName] = entryMap
	}
	return serializedExtensionEntries, nil
}

// stringSchema builds a basic string schema entry with description.
func stringSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": strings.TrimSpace(description),
	}
}

// boolSchema builds a basic boolean schema entry with description.
func boolSchema(description string) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": strings.TrimSpace(description),
	}
}

// numberSchema builds a basic integer schema entry with description.
func numberSchema(description string) map[string]any {
	return map[string]any{
		"type":        "integer",
		"minimum":     0,
		"description": strings.TrimSpace(description),
	}
}

// arrayOfStringsSchema builds a string-array schema entry with description.
func arrayOfStringsSchema(description string) map[string]any {
	return map[string]any{
		"type":        "array",
		"items":       map[string]any{"type": "string"},
		"description": strings.TrimSpace(description),
	}
}

// EnsureSchemaDirectoryExists ensures schema output directory exists.
func EnsureSchemaDirectoryExists(cfg *wave.ParsedConfig) error {
	if cfg == nil {
		return errors.New("config is nil")
	}
	targetDirectory := cfg.Dist.Internal()
	if mkdirError := os.MkdirAll(targetDirectory, 0o755); mkdirError != nil {
		return fmt.Errorf(
			"create schema output directory %q: %w",
			targetDirectory,
			mkdirError,
		)
	}
	return nil
}
