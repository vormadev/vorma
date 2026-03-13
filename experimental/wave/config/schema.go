package config

import (
	"fmt"

	"github.com/vormadev/vorma/lab/jsonschema"
)

var reserved_schema_keys = map[string]bool{
	"RootDir":        true,
	"Core":           true,
	"Vite":           true,
	"LifecycleHooks": true,
}

type BuildSchemaOpts struct {
	// ContributeProperties lets a framework plugin inject additional
	// root-level properties into the schema. Keys that collide with
	// Wave's reserved root keys are rejected.
	ContributeProperties map[string]jsonschema.Entry
}

func BuildSchema(opts BuildSchemaOpts) (jsonschema.Entry, error) {
	props := map[string]jsonschema.Entry{
		"RootDir": jsonschema.RequiredString(jsonschema.Def{
			Description: "Root directory for resolving all other paths. Relative to the config file directory.",
		}),
		"Core":           build_core_schema(),
		"Vite":           build_vite_schema(),
		"LifecycleHooks": build_lifecycle_hooks_schema(),
	}

	for k, v := range opts.ContributeProperties {
		if reserved_schema_keys[k] {
			return jsonschema.Entry{}, fmt.Errorf(
				"framework plugin tried to register reserved schema key: %s", k,
			)
		}
		props[k] = v
	}

	return jsonschema.Entry{
		Schema:     "https://json-schema.org/draft/2020-12/schema",
		Type:       jsonschema.TypeObject,
		Properties: props,
		Required:   []string{"RootDir", "Core"},
	}, nil
}

func build_core_schema() jsonschema.Entry {
	return jsonschema.RequiredObject(jsonschema.Def{
		Description:      "Core build and runtime configuration.",
		RequiredChildren: []string{"MainAppEntry"},
		Properties: map[string]jsonschema.Entry{
			"MainAppEntry": jsonschema.RequiredString(jsonschema.Def{
				Description: "Go application entry path. If a directory is given, main.go is assumed. Relative to RootDir.",
				Examples:    []string{"cmd/serve", "cmd/serve/main.go"},
			}),
			"HealthcheckEndpoint": jsonschema.OptionalString(jsonschema.Def{
				Description: "HTTP endpoint path used for app readiness polling after restart.",
				Default:     "/",
				Examples:    []string{"/", "/healthz"},
			}),
			"GlobalWatchExcludePatterns": jsonschema.OptionalArray(
				jsonschema.Def{
					Description: "Glob patterns excluded from all watch event processing. Relative to RootDir. Directories are auto-expanded to include all descendants.",
					Items:       jsonschema.Entry{Type: jsonschema.TypeString},
				},
			),
			"PreventImplicitGoBuildPatterns": jsonschema.OptionalArray(
				jsonschema.Def{
					Description: "Glob patterns for .go files that should not implicitly trigger Go recompilation. Relative to RootDir. Directories are auto-expanded to include all descendants.",
					Items:       jsonschema.Entry{Type: jsonschema.TypeString},
				},
			),
			"ServerOnlyMode": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, run in server-only mode. This takes precedence over any static asset or CSS configuration that may also be present.",
			}),
			"PublicPathPrefix": jsonschema.OptionalString(jsonschema.Def{
				Description: "URL prefix for public static assets. Normalized to have leading and trailing slashes.",
				Default:     "/",
				Examples:    []string{"/", "/public/"},
			}),
			"StaticAssetDirs": jsonschema.OptionalObject(jsonschema.Def{
				Description: "Source directories for static assets. Both fields are optional and independent.",
				Properties: map[string]jsonschema.Entry{
					"Private": jsonschema.OptionalString(jsonschema.Def{
						Description: "Source directory for private static assets. Relative to RootDir.",
						Examples:    []string{"backend/assets"},
					}),
					"Public": jsonschema.OptionalString(jsonschema.Def{
						Description: "Source directory for public static assets. Relative to RootDir.",
						Examples:    []string{"frontend/assets"},
					}),
				},
			}),
			"CSSEntryFiles": jsonschema.OptionalObject(jsonschema.Def{
				Description: "CSS entry points for critical and non-critical style bundles. Both fields are optional and independent.",
				Properties: map[string]jsonschema.Entry{
					"Critical": jsonschema.OptionalString(jsonschema.Def{
						Description: "Critical CSS entry file. Relative to RootDir.",
						Examples: []string{
							"frontend/src/styles/main.critical.css",
						},
					}),
					"NonCritical": jsonschema.OptionalString(jsonschema.Def{
						Description: "Non-critical CSS entry file. Relative to RootDir.",
						Examples:    []string{"frontend/src/styles/main.css"},
					}),
				},
			}),
		},
	})
}

func build_vite_schema() jsonschema.Entry {
	return jsonschema.OptionalObject(jsonschema.Def{
		Description:      "Vite dev server configuration. When present, JSPackageManagerBaseCmd is required.",
		RequiredChildren: []string{"JSPackageManagerBaseCmd"},
		Properties: map[string]jsonschema.Entry{
			"JSPackageManagerBaseCmd": jsonschema.RequiredString(jsonschema.Def{
				Description: "Base command used to run Vite.",
				Examples:    []string{"pnpm", "npx", "yarn", "bunx"},
			}),
			"JSPackageManagerCmdDir": jsonschema.OptionalString(jsonschema.Def{
				Description: "Directory where the JS package manager command should run. Relative to RootDir.",
			}),
			"DefaultPort": jsonschema.OptionalNumber(jsonschema.Def{
				Description: "Preferred Vite dev server port.",
				Default:     5173,
			}),
			"ViteConfigFile": jsonschema.OptionalString(jsonschema.Def{
				Description: "Explicit Vite config file path. Relative to RootDir.",
				Examples:    []string{"vite.config.ts"},
			}),
		},
	})
}

func build_lifecycle_hooks_schema() jsonschema.Entry {
	return jsonschema.OptionalArray(jsonschema.Def{
		Description: "Lifecycle hooks that run shell commands at defined points in the build cycle.",
		Items:       build_lifecycle_hook_entry_schema(),
	})
}

func build_lifecycle_hook_entry_schema() jsonschema.Entry {
	return jsonschema.RequiredObject(jsonschema.Def{
		DescriptionOverride: "One lifecycle hook definition.",
		RequiredChildren: []string{
			"WatchIncludePatterns",
			"Cmd",
			"StartAt",
			"FinishBy",
			"DownstreamEffect",
		},
		Properties: map[string]jsonschema.Entry{
			"WatchIncludePatterns": jsonschema.RequiredArray(jsonschema.Def{
				Description: "Glob patterns that activate this hook when matched. Relative to RootDir. Directories are auto-expanded.",
				Items:       jsonschema.Entry{Type: jsonschema.TypeString},
			}),
			"WatchExcludePatterns": jsonschema.OptionalArray(jsonschema.Def{
				Description: "Glob patterns excluded from this hook's watch scope. Relative to RootDir. Directories are auto-expanded.",
				Items:       jsonschema.Entry{Type: jsonschema.TypeString},
			}),
			"Cmd": jsonschema.RequiredString(jsonschema.Def{
				Description: "Shell command to execute when the hook fires.",
			}),
			"StartAt": jsonschema.RequiredString(jsonschema.Def{
				Description: "Earliest checkpoint at which the hook may begin execution.",
				Enum: []string{
					string(Checkpoint_1_CycleStart),
					string(Checkpoint_2_BuildStart),
					string(Checkpoint_3_GoCompileStart),
					string(Checkpoint_4_BuildEnd),
					string(Checkpoint_5_CycleEnd),
				},
			}),
			"FinishBy": jsonschema.RequiredString(jsonschema.Def{
				Description: "Latest checkpoint by which the hook must have completed.",
				Enum: []string{
					string(Checkpoint_1_CycleStart),
					string(Checkpoint_2_BuildStart),
					string(Checkpoint_3_GoCompileStart),
					string(Checkpoint_4_BuildEnd),
					string(Checkpoint_5_CycleEnd),
				},
			}),
			"DownstreamEffect": jsonschema.RequiredString(jsonschema.Def{
				Description: "What downstream action this hook's completion triggers.",
				Enum: []string{
					string(DownstreamEffectGoCompile),
					string(DownstreamEffectAppRestart),
					string(DownstreamEffectFrontendRevalidate),
				},
			}),
			"IncludeFrontendRebuildingOverlay": jsonschema.OptionalBoolean(
				jsonschema.Def{
					Description: "When true, a rebuilding overlay is shown in the browser while this hook runs.",
				},
			),
			"DevOnly": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, this hook only runs in dev mode.",
			}),
			"ProdOnly": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, this hook only runs in prod mode.",
			}),
		},
	})
}
