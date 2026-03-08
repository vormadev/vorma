// Package schema parses and validates Wave configuration documents for build
// and runtime consumers.
//
// It provides one normalization/validation surface so downstream engines do not
// each re-implement config safety checks.
package schema

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"path/filepath"
	"strings"

	"github.com/vormadev/vorma/internal/artifactio"
	"github.com/vormadev/vorma/wave/waveconfig"
	"github.com/vormadev/vorma/wave/waveframework"
)

// Processor builds and writes Wave configuration schema documents.
type Processor struct {
	cfg waveconfig.ParsedConfig
	log *slog.Logger
}

// NewProcessor creates a schema processor for one parsed config.
func NewProcessor(cfg waveconfig.ParsedConfig, log *slog.Logger) *Processor {
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

	targetPath := filepath.Join(processor.cfg.Dist().Internal(), "schema.json")
	if writeError := artifactio.WriteFileAtomically(targetPath, schemaBytes, 0o644); writeError != nil {
		return writeError
	}
	processor.log.Debug("wrote wave config schema", "path", targetPath)
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
		"$schema": stringSchema(
			"Optional JSON schema URI used by editors for Wave config validation.",
		),
		"Core":  coreSchema,
		"Watch": watchSchema,
		"Vite":  viteSchema,
	}

	if len(waveframework.StateForConfig(processor.cfg).SchemaExtensions) > 0 {
		frameworkExtensionProperties, extensionError := processor.buildFrameworkExtensionProperties()
		if extensionError != nil {
			return nil, extensionError
		}
		maps.Copy(properties, frameworkExtensionProperties)
	}

	document := map[string]any{
		"$schema":              "https://json-schema.org/draft/2020-12/schema",
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
		"description":          "Core build graph and runtime wiring used by Wave tooling and startup.",
		"additionalProperties": false,
		"required": []string{
			"ProjectID",
			"MainAppEntry",
			"StaticAssetDirs",
		},
		"properties": map[string]any{
			"ProjectID": stringSchema(
				"Required project identifier used for deterministic config discovery when multiple configs match ConfigPath semantics.",
			),
			"ResolveRoot": stringSchema(
				"Optional base directory for resolving filesystem paths. Relative to the config file directory. When omitted, paths resolve relative to the config file directory. Example: \"../\" resolves paths from the config file's parent directory.",
			),
			"MainAppEntry": stringSchema(
				"Go application entry file path. Must be set relative to your JSON config file.",
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
				"description":          "Source directories for static assets that are copied and hashed during builds.",
				"required":             []string{"Public", "Private"},
				"additionalProperties": false,
				"properties": map[string]any{
					"Public": stringSchema(
						"Source directory for public static assets. Must be set relative to your JSON config file.",
					),
					"Private": stringSchema(
						"Source directory for private static assets. Must be set relative to your JSON config file.",
					),
				},
			},
			"CSSEntryFiles": map[string]any{
				"type":                 "object",
				"description":          "Optional CSS entrypoints used for critical and non-critical style outputs.",
				"additionalProperties": false,
				"properties": map[string]any{
					"Critical": stringSchema(
						"Critical CSS entry file path. Must be set relative to your JSON config file.",
					),
					"NonCritical": stringSchema(
						"Non-critical CSS entry file path. Must be set relative to your JSON config file.",
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
		"description":          "One on-change hook definition for watched-file events.",
		"additionalProperties": false,
		"properties": map[string]any{
			"Cmd": stringSchema(
				"Shell command to execute for this hook.",
			),
			"Timing": stringSchema(
				"Hook stage name: pre, concurrent, concurrent-no-wait, or post.",
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
		"description":          "One watched-file pattern and its behavior overrides.",
		"required":             []string{"Pattern"},
		"additionalProperties": false,
		"properties": map[string]any{
			"Pattern": stringSchema(
				"Glob pattern for watched file selection. Must be set relative to your JSON config file.",
			),
			"OnChangeHooks": map[string]any{
				"type":        "array",
				"description": "Hook definitions that execute when this watched pattern changes.",
				"items":       hookSchema,
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
		"description":          "Dev watch behavior, hook execution policies, and restart/revalidate triggers.",
		"additionalProperties": false,
		"properties": map[string]any{
			"HealthcheckEndpoint": stringSchema(
				"HTTP endpoint path used for app readiness polling.",
			),
			"HookStageFailurePolicy": stringSchema(
				"Pipeline policy when hook stages report execution errors.",
			),
			"HookCommandTimeouts": map[string]any{
				"type":                 "object",
				"description":          "Default per-stage command timeouts used by on-change hooks.",
				"additionalProperties": false,
				"properties": map[string]any{
					"PreCommandTimeoutMilliseconds": numberSchema(
						"Default timeout for pre-stage hook commands in milliseconds.",
					),
					"ConcurrentCommandTimeoutMilliseconds": numberSchema(
						"Default timeout for concurrent-stage hook commands in milliseconds.",
					),
					"ConcurrentNoWaitCommandTimeoutMilliseconds": numberSchema(
						"Default timeout for concurrent-no-wait-stage hook commands in milliseconds.",
					),
					"PostCommandTimeoutMilliseconds": numberSchema(
						"Default timeout for post-stage hook commands in milliseconds.",
					),
				},
			},
			"HookCallbackTimeouts": map[string]any{
				"type":                 "object",
				"description":          "Default per-stage callback timeouts used by on-change hooks.",
				"additionalProperties": false,
				"properties": map[string]any{
					"PreCallbackTimeoutMilliseconds": numberSchema(
						"Default timeout for pre-stage hook callbacks in milliseconds.",
					),
					"ConcurrentCallbackTimeoutMilliseconds": numberSchema(
						"Default timeout for concurrent-stage hook callbacks in milliseconds.",
					),
					"ConcurrentNoWaitCallbackTimeoutMilliseconds": numberSchema(
						"Default timeout for concurrent-no-wait-stage hook callbacks in milliseconds.",
					),
					"PostCallbackTimeoutMilliseconds": numberSchema(
						"Default timeout for post-stage hook callbacks in milliseconds.",
					),
				},
			},
			"Include": map[string]any{
				"type":        "array",
				"description": "Custom watched-file patterns that extend or override default watch behavior.",
				"items":       watchedFileSchema,
			},
			"Exclude": map[string]any{
				"type":                 "object",
				"description":          "Path patterns excluded from watch registration and event processing.",
				"additionalProperties": false,
				"properties": map[string]any{
					"Dirs": arrayOfStringsSchema(
						"Directory glob patterns excluded from watch registration. Must be set relative to your JSON config file.",
					),
					"Files": arrayOfStringsSchema(
						"File glob patterns excluded from event processing. Must be set relative to your JSON config file.",
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
		"description":          "Vite process invocation settings used by Wave devserver workflows.",
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
	for fieldName, extensionEntry := range waveframework.StateForConfig(processor.cfg).SchemaExtensions {
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
