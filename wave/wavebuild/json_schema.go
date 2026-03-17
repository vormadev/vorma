package wavebuild

import "github.com/vormadev/vorma/lab/jsonschema"

var (
	reserved_json_cfg_keys = []string{
		"RootDir",
		"Core",
		"Vite",
		"LifecycleHooks",
	}
	json_cfg_key_root_dir        = reserved_json_cfg_keys[0] // "RootDir"
	json_cfg_key_core            = reserved_json_cfg_keys[1] // "Core"
	json_cfg_key_vite            = reserved_json_cfg_keys[2] // "Vite"
	json_cfg_key_lifecycle_hooks = reserved_json_cfg_keys[3] // "LifecycleHooks"
)

func build_schema(plugin_cfgs []*validated_plugin_config) jsonschema.Entry {
	props := map[string]jsonschema.Entry{
		json_cfg_key_root_dir: jsonschema.RequiredString(jsonschema.Def{
			Description: "Root directory for resolving all other paths. Relative to the config file directory.",
		}),
		json_cfg_key_core:            build_core_schema(),
		json_cfg_key_vite:            build_vite_schema(),
		json_cfg_key_lifecycle_hooks: build_lifecycle_hooks_schema(),
	}

	for _, plugin_cfg := range plugin_cfgs {
		if plugin_cfg == nil || plugin_cfg.json_key == "" {
			continue
		}
		if plugin_cfg.json_schema.IsZero() {
			continue
		}
		props[plugin_cfg.json_key] = plugin_cfg.json_schema
	}

	return jsonschema.Entry{
		Schema:     "https://json-schema.org/draft/2020-12/schema",
		Type:       jsonschema.TypeObject,
		Properties: props,
		Required:   []string{"RootDir", "Core"},
	}
}

func build_core_schema() jsonschema.Entry {
	return jsonschema.RequiredObject(jsonschema.Def{
		Description:      "Core build and runtime configuration.",
		RequiredChildren: []string{"BinaryName"},
		Properties: map[string]jsonschema.Entry{
			"BinaryName": jsonschema.RequiredString(jsonschema.Def{
				Description: "Name of the compiled binary (plain name, not a path). Wave outputs it to waveout/<n> and exposes the full path to hooks as WAVE_BIN_OUTPUT_PATH. The .exe suffix is added automatically on Windows.",
				Examples:    []string{"myapp", "server"},
			}),
			"HealthcheckEndpoint": jsonschema.OptionalString(jsonschema.Def{
				Description: "HTTP endpoint path used for app readiness polling after restart. Must return HTTP 200 when the app is ready.",
				Default:     "/",
				Examples:    []string{"/", "/healthz"},
			}),
			"GlobalWatchExcludePatterns": jsonschema.OptionalArray(
				jsonschema.Def{
					Description: "Glob patterns excluded from all watch event processing. Relative to RootDir. Directories are auto-expanded to include all descendants.",
					Items:       jsonschema.Entry{Type: jsonschema.TypeString},
				},
			),
			"ServerOnlyMode": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, disables the Vite dev server integration and browser sync (live reload, CSS hot reload, rebuilding overlay). Static asset processing and CSS bundling are unaffected — those are controlled by their own config fields.",
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
		Description:      "Vite dev server configuration. When present, JSPackageManagerBaseCmd is required. Ignored when ServerOnlyMode is true.",
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
		RequiredChildren:    []string{"WatchIncludePatterns"},
		Properties: map[string]jsonschema.Entry{
			"Name": jsonschema.OptionalString(jsonschema.Def{
				Description: "A human-readable name for this hook, used in build logs. If not set, defaults to the hook's index in the array.",
				Examples: []string{
					"go-compile",
					"generate-routes",
					"tailwind",
				},
			}),
			"WatchIncludePatterns": jsonschema.RequiredArray(jsonschema.Def{
				Description: "Glob patterns that activate this hook when matched. Relative to RootDir. Directories are auto-expanded. Must have at least one pattern.",
				Items:       jsonschema.Entry{Type: jsonschema.TypeString},
				MinItems:    1,
			}),
			"WatchExcludePatterns": jsonschema.OptionalArray(jsonschema.Def{
				Description: "Glob patterns excluded from this hook's watch scope. Relative to RootDir. Directories are auto-expanded.",
				Items:       jsonschema.Entry{Type: jsonschema.TypeString},
			}),
			"Cmd": jsonschema.OptionalString(jsonschema.Def{
				Description: "Shell command to execute when the hook fires.",
			}),
			"IsGoCompile": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "Convenience bool that, when true, sets StartAt and FinishBy to 'go_compile' and implies 'restart_app'. Cannot be combined with StartAt, FinishBy, or Effects.",
			}),
			"StartAt": jsonschema.OptionalString(jsonschema.Def{
				Description: "Earliest checkpoint at which the hook may begin execution.",
				Enum: []string{
					Checkpoint_1_CycleStart.Str(),
					Checkpoint_2_UserlandPublicFilemapReady.Str(),
					Checkpoint_3_FullPublicFilemapFinalized.Str(),
					Checkpoint_4_GoCompile.Str(),
					Checkpoint_5_GoCompileComplete.Str(),
					Checkpoint_6_ServiceRestarted.Str(),
					Checkpoint_7_CycleEnd.Str(),
				},
			}),
			"FinishBy": jsonschema.OptionalString(jsonschema.Def{
				Description: "Latest checkpoint by which the hook must have completed.",
				Enum: []string{
					Checkpoint_1_CycleStart.Str(),
					Checkpoint_2_UserlandPublicFilemapReady.Str(),
					Checkpoint_3_FullPublicFilemapFinalized.Str(),
					Checkpoint_4_GoCompile.Str(),
					Checkpoint_5_GoCompileComplete.Str(),
					Checkpoint_6_ServiceRestarted.Str(),
					Checkpoint_7_CycleEnd.Str(),
				},
			}),
			"Effects": jsonschema.OptionalArray(jsonschema.Def{
				Description: "What effects this hook causes in the current build cycle.",
				Items: jsonschema.Entry{
					Type: jsonschema.TypeString,
					Enum: []string{
						EffectRestartApp.Str(),
						EffectHardReloadBrowser.Str(),
						EffectRevalidateClientData.Str(),
						EffectProcessPrivateStatic.Str(),
						EffectShowFrontendRebuildingOverlay.Str(),
						EffectNoFrontendSettling.Str(),
					},
				},
			}),
			"DevOnly": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, this hook only runs in dev mode.",
			}),
			"ProdOnly": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, this hook only runs in prod mode.",
			}),
		},
	})
}
