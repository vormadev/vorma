package tooling

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"github.com/vormadev/vorma/lab/jsonschema"
)

func writeConfigSchema(b *Builder) error {
	schema := jsonschema.Entry{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Type:        jsonschema.TypeObject,
		Description: "Wave configuration schema.",
		Required:    []string{"Core"},
		Properties: map[string]jsonschema.Entry{
			"Core":  coreSchema,
			"Vite":  viteSchema,
			"Watch": watchSchema,
		},
	}

	// Use schema extensions from config (set via Wave.RegisterConfigSchemaSection)
	if len(b.cfg.FrameworkSchemaExtensions) > 0 {
		props := schema.Properties.(map[string]jsonschema.Entry)
		maps.Copy(props, b.cfg.FrameworkSchemaExtensions)
	}

	jsonBytes, err := json.MarshalIndent(schema, "", "\t")
	if err != nil {
		return fmt.Errorf("configschema.Write: failed to marshal JSON schema: %w", err)
	}

	jsonBytes = append(jsonBytes, '\n')

	target := filepath.Join(b.cfg.Dist.Internal(), "schema.json")
	if err = os.WriteFile(target, jsonBytes, 0644); err != nil {
		return fmt.Errorf("configschema.Write: failed to write JSON schema: %w", err)
	}

	return nil
}

var coreSchema = jsonschema.RequiredObject(jsonschema.Def{
	Description:      `Core Wave configuration. All paths should be set relative to the directory from which you run commands.`,
	RequiredChildren: []string{"MainAppEntry", "DistDir"},
	AllOf: []any{jsonschema.IfThen{
		If: map[string]any{
			"not": map[string]any{
				"properties": map[string]any{
					"ServerOnlyMode": map[string]any{"const": true},
				},
			},
		},
		Then: map[string]any{
			"required": []string{"StaticAssetDirs"},
		},
	}},
	Properties: struct {
		ConfigLocation   jsonschema.Entry
		DevBuildHook     jsonschema.Entry
		ProdBuildHook    jsonschema.Entry
		MainAppEntry     jsonschema.Entry
		DistDir          jsonschema.Entry
		StaticAssetDirs  jsonschema.Entry
		CSSEntryFiles    jsonschema.Entry
		PublicPathPrefix jsonschema.Entry
		ServerOnlyMode   jsonschema.Entry
	}{
		ConfigLocation:   configLocationSchema,
		DevBuildHook:     devBuildHookSchema,
		ProdBuildHook:    prodBuildHookSchema,
		MainAppEntry:     mainAppEntrySchema,
		DistDir:          distDirSchema,
		StaticAssetDirs:  staticAssetDirsSchema,
		CSSEntryFiles:    cssEntryFilesSchema,
		PublicPathPrefix: publicPathPrefixSchema,
		ServerOnlyMode:   serverOnlyModeSchema,
	},
})

var configLocationSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to the Wave configuration file.
This enables restarting the server when you update the Wave config.`,
	Examples: []string{"./wave.json", "./config/wave.json"},
})

var devBuildHookSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Command to run to build your app in dev mode. This runs before Wave's build process and typically generates routes or other code.`,
	Examples:    []string{"go run ./backend/cmd/build -dev", "make dev-generate"},
})

var prodBuildHookSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Command to run to build your app in production mode. This runs before Wave's build process and typically generates routes or other code.`,
	Examples:    []string{"go run ./backend/cmd/build", "make prod-generate"},
})

var mainAppEntrySchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your app's main.go entry file (or its parent directory).`,
	Examples:    []string{"./cmd/app/main.go", "./cmd/app"},
})

var distDirSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.StaticAssetDirs.Private", "Core.StaticAssetDirs.Public") + ` This is where Wave outputs the compiled binary and processed static assets.`,
	Examples:    []string{"./dist"},
})

var staticAssetDirsSchema = jsonschema.ObjectWithOverride(`This object is required unless you are in ServerOnlyMode.
Defines where your static assets are located.`, jsonschema.Def{
	RequiredChildren: []string{"Private", "Public"},
	Properties: struct {
		Private jsonschema.Entry
		Public  jsonschema.Entry
	}{
		Private: privateSchema,
		Public:  publicSchema,
	},
})

var privateSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.DistDir", "Core.StaticAssetDirs.Public") + ` Private assets are only accessible from your Go code (e.g., templates, server-side files).`,
	Examples:    []string{"./static/private"},
})

var publicSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.DistDir", "Core.StaticAssetDirs.Private") + ` Public assets are served directly to the browser and get content-addressed hashing for cache busting. Files in a "prehashed" subdirectory will keep their original names.`,
	Examples:    []string{"./static/public"},
})

var cssEntryFilesSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Use this if you are using Wave's CSS features.
Wave will bundle and optimize your CSS files.`,
	Properties: struct {
		Critical    jsonschema.Entry
		NonCritical jsonschema.Entry
	}{
		Critical:    criticalSchema,
		NonCritical: nonCriticalSchema,
	},
})

var criticalSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your critical CSS entry file. This CSS will be inlined in the HTML head for faster initial rendering.`,
	Examples:    []string{"./styles/main.critical.css"},
})

var nonCriticalSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your non-critical CSS entry file. This CSS will be loaded asynchronously after page load.`,
	Examples:    []string{"./styles/main.css"},
})

var publicPathPrefixSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path prefix for your public assets. Must both start and end with a "/".`,
	Examples:    []string{"/public/", "/assets/", "/"},
	Default:     "/",
})

var serverOnlyModeSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, skips static asset processing/serving and browser reloading.
Use this for API-only servers without frontend assets.`,
	Default: false,
})

var viteSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Vite integration settings. Configure these to use Vite for frontend asset bundling.`,
	Properties: struct {
		JSPackageManagerBaseCmd jsonschema.Entry
		JSPackageManagerCmdDir  jsonschema.Entry
		DefaultPort             jsonschema.Entry
		ViteConfigFile          jsonschema.Entry
	}{
		JSPackageManagerBaseCmd: jsPackageManagerBaseCmdSchema,
		JSPackageManagerCmdDir:  jsPackageManagerCmdDirSchema,
		DefaultPort:             defaultPortSchema,
		ViteConfigFile:          viteConfigFileSchema,
	},
	RequiredChildren: []string{"JSPackageManagerBaseCmd"},
})

var jsPackageManagerBaseCmdSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Base command to run Vite using your preferred package manager.
This is the command to run standalone CLIs (e.g., "npx", not "npm run").`,
	Examples: []string{"npx", "pnpm", "yarn", "bunx"},
})

var jsPackageManagerCmdDirSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Directory to run the package manager command from.
For example, if you're running commands from ".", but you want to run Vite from "./web", set this to "./web".`,
	Examples: []string{"./web", "./client"},
	Default:  ".",
})

var defaultPortSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Default port to use for Vite dev server. This is used when you run "wave dev" without specifying a port.`,
	Default:     5173,
})

var viteConfigFileSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your Vite config file if it is in a non-standard location.
Should be set relative to the JSPackageManagerCmdDir, if set, otherwise your current working directory.`,
	Examples: []string{"./configs/vite.config.ts", "vite.custom.js"},
})

var watchSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `File watching configuration for development mode. Controls which files trigger rebuilds and how.`,
	Properties: struct {
		WatchRoot           jsonschema.Entry
		HealthcheckEndpoint jsonschema.Entry
		Include             jsonschema.Entry
		Exclude             jsonschema.Entry
	}{
		WatchRoot:           watchRootSchema,
		HealthcheckEndpoint: healthcheckEndpointSchema,
		Include:             includeSchema,
		Exclude:             excludeSchema,
	},
})

var watchRootSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `The directory against which all watch settings paths are relative.
If not set, all paths are relative to the directory from which you run commands.`,
	Default: ".",
})

var healthcheckEndpointSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your app's healthcheck endpoint. Must return 200 OK when healthy. During dev-time rebuilds and restarts, this endpoint will be polled to determine when your app is ready to begin serving normal requests.`,
	Examples:    []string{"/healthz", "/health", "/api/health"},
	Default:     "/",
})

var includeSchema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Files and patterns to watch for changes. Each pattern can specify what actions to take when matching files change.`,
	Items:       includeItemsSchema,
})

var includeItemsSchema = jsonschema.OptionalObject(jsonschema.Def{
	RequiredChildren: []string{"Pattern"},
	Properties: struct {
		Pattern                            jsonschema.Entry
		OnChangeHooks                      jsonschema.Entry
		RecompileGoBinary                  jsonschema.Entry
		RestartApp                         jsonschema.Entry
		OnlyRunClientDefinedRevalidateFunc jsonschema.Entry
		RunOnChangeOnly                    jsonschema.Entry
		SkipRebuildingNotification         jsonschema.Entry
		TreatAsNonGo                       jsonschema.Entry
	}{
		Pattern:                            patternSchema,
		OnChangeHooks:                      onChangeHooksSchema,
		RecompileGoBinary:                  recompileGoBinarySchema,
		RestartApp:                         restartAppSchema,
		OnlyRunClientDefinedRevalidateFunc: onlyRunClientDefinedRevalidateFuncSchema,
		RunOnChangeOnly:                    runOnChangeOnlySchema,
		SkipRebuildingNotification:         skipRebuildingNotificationSchema,
		TreatAsNonGo:                       treatAsNonGoSchema,
	},
})

var patternSchema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Glob pattern for matching files (set relative to WatchRoot).
Supports ** for recursive matching.`,
	Examples: []string{"**/*.go", "frontend/src/**/*.ts", "templates/*.html"},
})

var onChangeHooksSchema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Commands or strategies to run when a file matching the pattern changes. Each hook can either specify a Cmd (shell command) or a Strategy (declarative behavior).`,
	Items:       onChangeHooksItemsSchema,
})

var onChangeHooksItemsSchema = jsonschema.OptionalObject(jsonschema.Def{
	Properties: struct {
		Cmd      jsonschema.Entry
		Strategy jsonschema.Entry
		Timing   jsonschema.Entry
		Exclude  jsonschema.Entry
	}{
		Cmd:      cmdSchema,
		Strategy: strategySchema,
		Timing:   timingSchema,
		Exclude:  onChangeHooksExcludeSchema,
	},
})

var cmdSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Shell command to run when a file matching the pattern changes.
Can be any shell command or "DevBuildHook" to run the configured dev build hook.
Ignored if Strategy is set.`,
	Examples: []string{"echo 'File changed!'", "make generate", "DevBuildHook", "npm run lint"},
})

var strategySchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Declarative strategy for handling file changes.
Use this instead of Cmd for complex behaviors like calling HTTP endpoints on the running app.`,
	Properties: struct {
		HttpEndpoint   jsonschema.Entry
		SkipDevHook    jsonschema.Entry
		SkipGoCompile  jsonschema.Entry
		WaitForApp     jsonschema.Entry
		WaitForVite    jsonschema.Entry
		ReloadBrowser  jsonschema.Entry
		FallbackAction jsonschema.Entry
	}{
		HttpEndpoint:   httpEndpointSchema,
		SkipDevHook:    skipDevHookSchema,
		SkipGoCompile:  skipGoCompileSchema,
		WaitForApp:     waitForAppSchema,
		WaitForVite:    waitForViteSchema,
		ReloadBrowser:  reloadBrowserSchema,
		FallbackAction: fallbackActionSchema,
	},
})

var httpEndpointSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `HTTP endpoint to call on the running app (e.g., "/__vorma/reload-routes").
If the call fails, FallbackAction is executed.`,
	Examples: []string{"/__vorma/reload-routes", "/__vorma/reload-template", "/__my-app/refresh-cache"},
})

var skipDevHookSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, skips running the DevBuildHook for this change.`,
	Default:     false,
})

var skipGoCompileSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, skips Go binary recompilation for this change.`,
	Default:     false,
})

var waitForAppSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, waits for the app's healthcheck before reloading the browser.`,
	Default:     false,
})

var waitForViteSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, waits for Vite dev server before reloading the browser.`,
	Default:     false,
})

var reloadBrowserSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, triggers a browser reload after successful execution.`,
	Default:     false,
})

var fallbackActionSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Action to take if HttpEndpoint fails.
"restart" does a full restart with Go recompile, "restart-no-go" restarts without Go recompile, "none" does nothing.`,
	Enum:    []string{"restart", "restart-no-go", "none"},
	Default: "none",
})

var timingSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Timing of the given command relative to Wave's rebuild process. Only applies to Cmd hooks, not Strategy hooks.`,
	Enum:        []string{"pre", "post", "concurrent", "concurrent-no-wait"},
	Default:     "pre",
})

var onChangeHooksExcludeSchema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Glob patterns for files to exclude from triggering this onchange hook (set relative to WatchRoot).`,
	Items:       jsonschema.OptionalString(jsonschema.Def{}),
	Examples:    []string{"**/*_test.go", "**/*.gen.ts"},
})

var recompileGoBinarySchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, the Go binary will be recompiled when this file changes.
Use for non-Go files that affect the Go build (e.g., embedded files).`,
	Default: false,
})

var restartAppSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, the app will be restarted when this file changes.
Use for files that are cached on startup (e.g., templates that are parsed once).`,
	Default: false,
})

var onlyRunClientDefinedRevalidateFuncSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, Wave will call window.__waveRevalidate() instead of reloading the page. Use with frameworks that support hot module replacement or client-side revalidation.`,
	Default:     false,
})

var runOnChangeOnlySchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, only the OnChangeHooks will run - Wave won't reload the browser.
Use when your onChange hook triggers its own reload process.
Note: OnChangeHooks must use "pre" timing (the default) with this option.`,
	Default: false,
})

var skipRebuildingNotificationSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, Wave won't show the "Rebuilding..." overlay in the browser. Use with RunOnChangeOnly if your onChange doesn't trigger a rebuild, or for changes that don't need user notification.`,
	Default:     false,
})

var treatAsNonGoSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, .go files matching this pattern won't trigger Go recompilation.
Use for Go files that are independent from your main app (e.g., WASM files with separate build processes).`,
	Default: false,
})

var excludeSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Patterns for files and directories to exclude from watching.
Use to prevent unnecessary rebuilds from vendor files, build outputs, etc.`,
	Properties: struct {
		Dirs  jsonschema.Entry
		Files jsonschema.Entry
	}{
		Dirs:  excludeDirsSchema,
		Files: excludeFilesSchema,
	},
})

var excludeDirsSchema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Glob patterns for directories to exclude from the watcher (set relative to WatchRoot). Wave automatically excludes .git, node_modules, and the dist directory.`,
	Items:       jsonschema.OptionalString(jsonschema.Def{}),
	Examples:    []string{"vendor", "tmp", ".cache", "coverage"},
})

var excludeFilesSchema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Glob patterns for files to exclude from the watcher (set relative to WatchRoot).`,
	Items:       jsonschema.OptionalString(jsonschema.Def{}),
	Examples:    []string{"**/*.log", "**/.DS_Store", "**/*~"},
})
