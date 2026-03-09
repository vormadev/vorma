package config

import "github.com/vormadev/vorma/lab/jsonschema"

func BuildSchema() jsonschema.Entry {
	return jsonschema.Entry{
		Schema: "https://json-schema.org/draft/2020-12/schema",
		Type:   jsonschema.TypeObject,
		Properties: map[string]jsonschema.Entry{
			"Core":  build_core_schema(),
			"Vite":  build_vite_schema(),
			"Watch": build_watch_schema(),
		},
		Required: []string{"Core"},
	}
}

func build_core_schema() jsonschema.Entry {
	return jsonschema.RequiredObject(jsonschema.Def{
		Description: "Core build and runtime configuration.",
		RequiredChildren: []string{
			"MainAppEntry",
		},
		AllOf: []any{
			jsonschema.IfThen{
				If: map[string]any{
					"not": map[string]any{
						"properties": map[string]any{
							"ServerOnlyMode": map[string]any{"const": true},
						},
					},
				},
				Then: map[string]any{
					"required": []string{
						"StaticAssetDirs",
						"CSSEntryFiles",
					},
				},
			},
		},
		Properties: map[string]jsonschema.Entry{
			"ResolveRoot": jsonschema.OptionalString(jsonschema.Def{
				Description: "Base directory for resolving all other paths. Relative to the config file directory. Defaults to the config file directory when omitted.",
				Examples:    []string{"../"},
			}),
			"MainAppEntry": jsonschema.RequiredString(jsonschema.Def{
				Description: "Go application entry path. Relative to ResolveRoot.",
				Examples:    []string{"backend/cmd/serve"},
			}),
			"DevBuildHook": jsonschema.OptionalString(jsonschema.Def{
				Description: "Shell command run before dev build output is finalized.",
			}),
			"ProdBuildHook": jsonschema.OptionalString(jsonschema.Def{
				Description: "Shell command run before production build output is finalized.",
			}),
			"ServerOnlyMode": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, browser runtime integrations are disabled and full-stack fields are ignored.",
			}),
			"SequentialGoBuild": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, hook calls and Wave build will finish before Go compilation starts.",
			}),
			"PublicPathPrefix": jsonschema.OptionalString(jsonschema.Def{
				Description: "Public URL prefix for static assets. Normalized to have leading and trailing slashes.",
				Default:     "/",
				Examples:    []string{"/", "/public/"},
			}),
			"StaticAssetDirs": jsonschema.OptionalObject(jsonschema.Def{
				Description: "Source directories for static assets. Required when ServerOnlyMode is false.",
				RequiredChildren: []string{
					"Private",
					"Public",
				},
				Properties: map[string]jsonschema.Entry{
					"Private": jsonschema.RequiredString(jsonschema.Def{
						Description: "Source directory for private static assets. Relative to ResolveRoot.",
						Examples:    []string{"backend/assets"},
					}),
					"Public": jsonschema.RequiredString(jsonschema.Def{
						Description: "Source directory for public static assets. Relative to ResolveRoot.",
						Examples:    []string{"frontend/assets"},
					}),
				},
			}),
			"CSSEntryFiles": jsonschema.OptionalObject(jsonschema.Def{
				Description: "CSS entrypoints for critical and non-critical style outputs.",
				Properties: map[string]jsonschema.Entry{
					"Critical": jsonschema.OptionalString(jsonschema.Def{
						Description: "Critical CSS entry file path. Relative to ResolveRoot.",
						Examples: []string{
							"frontend/src/styles/main.critical.css",
						},
					}),
					"NonCritical": jsonschema.OptionalString(jsonschema.Def{
						Description: "Non-critical CSS entry file path. Relative to ResolveRoot.",
						Examples:    []string{"frontend/src/styles/main.css"},
					}),
				},
			}),
		},
	})
}

func build_vite_schema() jsonschema.Entry {
	return jsonschema.OptionalObject(jsonschema.Def{
		Description: "Vite process configuration. When present, JSPackageManagerBaseCmd is required.",
		RequiredChildren: []string{
			"JSPackageManagerBaseCmd",
		},
		Properties: map[string]jsonschema.Entry{
			"JSPackageManagerBaseCmd": jsonschema.RequiredString(jsonschema.Def{
				Description: "Base command used to run Vite.",
				Examples:    []string{"pnpm", "npx", "yarn", "bunx"},
			}),
			"JSPackageManagerCmdDir": jsonschema.OptionalString(jsonschema.Def{
				Description: "Directory where the JS package manager command should run. Relative to ResolveRoot.",
			}),
			"DefaultPort": jsonschema.OptionalNumber(jsonschema.Def{
				Description: "Preferred Vite dev server port. Must be between 1 and 65535.",
				Default:     5173,
			}),
			"ViteConfigFile": jsonschema.OptionalString(jsonschema.Def{
				Description: "Explicit Vite config file path. Relative to ResolveRoot.",
				Examples:    []string{"vite.config.ts"},
			}),
		},
	})
}

func build_watch_schema() jsonschema.Entry {
	return jsonschema.OptionalObject(jsonschema.Def{
		Description: "Dev watch behavior, hook execution, and restart/revalidate triggers.",
		Properties: map[string]jsonschema.Entry{
			"HealthcheckEndpoint": jsonschema.OptionalString(jsonschema.Def{
				Description: "HTTP endpoint path used for app readiness polling. Normalized to have a leading slash.",
				Examples:    []string{"/healthz"},
			}),
			"Include": jsonschema.OptionalArray(jsonschema.Def{
				Description: "Watched file patterns and their behavior overrides.",
				Items:       build_watch_include_entry_schema(),
			}),
			"Exclude": jsonschema.OptionalArray(jsonschema.Def{
				Description: "Glob patterns excluded from watch event processing. Relative to ResolveRoot.",
				Items: jsonschema.Entry{
					Type: jsonschema.TypeString,
				},
			}),
		},
	})
}

func build_watch_include_entry_schema() jsonschema.Entry {
	return jsonschema.RequiredObject(jsonschema.Def{
		DescriptionOverride: "One watched file pattern and its behavior overrides.",
		RequiredChildren:    []string{"Pattern"},
		Properties: map[string]jsonschema.Entry{
			"Pattern": jsonschema.RequiredString(jsonschema.Def{
				Description: "Glob pattern for watched file selection. Relative to ResolveRoot.",
				Examples:    []string{"src/**/*.go", "assets/**/*.md"},
			}),
			"OnChangeHooks": jsonschema.OptionalArray(jsonschema.Def{
				Description: "Hook definitions that execute when this pattern matches a changed file.",
				Items:       build_on_change_hook_schema(),
			}),
			"RecompileGoBinary": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, Go binary recompilation is triggered.",
			}),
			"RestartApp": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, app process restart is triggered.",
			}),
			"OnlyRunClientDefinedRevalidateFunc": jsonschema.OptionalBoolean(
				jsonschema.Def{
					Description: "When true, browser action prefers framework revalidate callback.",
				},
			),
			"RunOnChangeOnly": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, skip implicit build and run hooks only.",
			}),
			"SkipRebuildingNotification": jsonschema.OptionalBoolean(
				jsonschema.Def{
					Description: "When true, rebuilding overlay notification is suppressed.",
				},
			),
			"TreatAsNonGo": jsonschema.OptionalBoolean(jsonschema.Def{
				Description: "When true, .go files matching this pattern are not treated as Go recompilation triggers.",
			}),
		},
	})
}

func build_on_change_hook_schema() jsonschema.Entry {
	return jsonschema.RequiredObject(jsonschema.Def{
		DescriptionOverride: "One on-change hook definition.",
		RequiredChildren:    []string{"Cmd"},
		Properties: map[string]jsonschema.Entry{
			"Cmd": jsonschema.RequiredString(jsonschema.Def{
				Description: "Shell command to execute.",
			}),
			"Timing": jsonschema.OptionalString(jsonschema.Def{
				Description: "Hook execution timing relative to Wave processing.",
				Default:     "pre",
				Enum: []string{
					string(OnChangeHookTimingPre),
					string(OnChangeHookTimingConcurrent),
					string(OnChangeHookTimingConcurrentNoWait),
					string(OnChangeHookTimingPost),
				},
			}),
			"Exclude": jsonschema.OptionalArray(jsonschema.Def{
				Description: "Glob patterns excluded from this hook. Relative to ResolveRoot.",
				Items: jsonschema.Entry{
					Type: jsonschema.TypeString,
				},
			}),
		},
	})
}
