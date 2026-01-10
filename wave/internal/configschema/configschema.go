package configschema

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"

	"github.com/vormadev/vorma/lab/jsonschema"
)

func Write(target string, additional map[string]jsonschema.Entry) error {
	schema := jsonschema.Entry{
		Schema:      "http://json-schema.org/draft-07/schema#",
		Type:        jsonschema.TypeObject,
		Description: "Wave configuration schema.",
		Required:    []string{"Core"},
		Properties: map[string]jsonschema.Entry{
			"Core":  Core_Schema,
			"Vite":  Vite_Schema,
			"Watch": Watch_Schema,
		},
	}

	if len(additional) > 0 {
		props := schema.Properties.(map[string]jsonschema.Entry)
		maps.Copy(props, additional)
	}

	jsonBytes, err := json.MarshalIndent(schema, "", "\t")
	if err != nil {
		return fmt.Errorf("configschema.Write: failed to marshal JSON schema: %w", err)
	}

	jsonBytes = append(jsonBytes, []byte("\n")...)

	if err = os.WriteFile(target, jsonBytes, 0644); err != nil {
		return fmt.Errorf("configschema.Write: failed to write JSON schema: %w", err)
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS
/////////////////////////////////////////////////////////////////////

var Core_Schema = jsonschema.RequiredObject(jsonschema.Def{
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
		ConfigLocation:   ConfigLocation_Schema,
		DevBuildHook:     DevBuildHook_Schema,
		ProdBuildHook:    ProdBuildHook_Schema,
		MainAppEntry:     MainAppEntry_Schema,
		DistDir:          DistDir_Schema,
		StaticAssetDirs:  StaticAssetDirs_Schema,
		CSSEntryFiles:    CSSEntryFiles_Schema,
		PublicPathPrefix: PublicPathPrefix_Schema,
		ServerOnlyMode:   ServerOnlyMode_Schema,
	},
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- CONFIG LOCATION
/////////////////////////////////////////////////////////////////////

var ConfigLocation_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to the Wave configuration file.
This enables restarting the server when you update the Wave config.`,
	Examples: []string{"./wave.json", "./config/wave.json"},
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- DEV BUILD HOOK
/////////////////////////////////////////////////////////////////////

var DevBuildHook_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Command to run to build your app in dev mode. This runs before Wave's build process and typically generates routes or other code.`,
	Examples:    []string{"go run ./backend/cmd/build -dev", "make dev-generate"},
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- PROD BUILD HOOK
/////////////////////////////////////////////////////////////////////

var ProdBuildHook_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Command to run to build your app in production mode. This runs before Wave's build process and typically generates routes or other code.`,
	Examples:    []string{"go run ./backend/cmd/build", "make prod-generate"},
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- APP ENTRY
/////////////////////////////////////////////////////////////////////

var MainAppEntry_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Path to your app's main.go entry file (or its parent directory).`,
	Examples:    []string{"./cmd/app/main.go", "./cmd/app"},
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- DIST DIR
/////////////////////////////////////////////////////////////////////

var DistDir_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.StaticAssetDirs.Private", "Core.StaticAssetDirs.Public") + ` This is where Wave outputs the compiled binary and processed static assets.`,
	Examples:    []string{"./dist"},
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- STATIC DIRS
/////////////////////////////////////////////////////////////////////

var StaticAssetDirs_Schema = jsonschema.ObjectWithOverride(`This object is required unless you are in ServerOnlyMode.
Defines where your static assets are located.`, jsonschema.Def{
	RequiredChildren: []string{"Private", "Public"},
	Properties: struct {
		Private jsonschema.Entry
		Public  jsonschema.Entry
	}{
		Private: Private_Schema,
		Public:  Public_Schema,
	},
})

var Private_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.DistDir", "Core.StaticAssetDirs.Public") + ` Private assets are only accessible from your Go code (e.g., templates, server-side files).`,
	Examples:    []string{"./static/private"},
})

var Public_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: jsonschema.UniqueFrom("Core.DistDir", "Core.StaticAssetDirs.Private") + ` Public assets are served directly to the browser and get content-addressed hashing for cache busting. Files in a "prehashed" subdirectory will keep their original names.`,
	Examples:    []string{"./static/public"},
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- CSS ENTRY FILES
/////////////////////////////////////////////////////////////////////

var CSSEntryFiles_Schema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Use this if you are using Wave's CSS features.
Wave will bundle and optimize your CSS files.`,
	Properties: struct {
		Critical    jsonschema.Entry
		NonCritical jsonschema.Entry
	}{
		Critical:    Critical_Schema,
		NonCritical: NonCritical_Schema,
	},
})

var Critical_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your critical CSS entry file. This CSS will be inlined in the HTML head for faster initial rendering.`,
	Examples:    []string{"./styles/main.critical.css"},
})

var NonCritical_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your non-critical CSS entry file. This CSS will be loaded asynchronously after page load.`,
	Examples:    []string{"./styles/main.css"},
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- PUBLIC PATH PREFIX
/////////////////////////////////////////////////////////////////////

var PublicPathPrefix_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path prefix for your public assets. Must both start and end with a "/".`,
	Examples:    []string{"/public/", "/assets/", "/"},
	Default:     "/",
})

/////////////////////////////////////////////////////////////////////
/////// CORE SETTINGS -- SERVER ONLY
/////////////////////////////////////////////////////////////////////

var ServerOnlyMode_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, skips static asset processing/serving and browser reloading.
Use this for API-only servers without frontend assets.`,
	Default: false,
})

/////////////////////////////////////////////////////////////////////
/////// VITE SETTINGS
/////////////////////////////////////////////////////////////////////

var Vite_Schema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Vite integration settings. Configure these to use Vite for frontend asset bundling.`,
	Properties: struct {
		JSPackageManagerBaseCmd jsonschema.Entry
		JSPackageManagerCmdDir  jsonschema.Entry
		DefaultPort             jsonschema.Entry
		ViteConfigFile          jsonschema.Entry
	}{
		JSPackageManagerBaseCmd: JSPackageManagerBaseCmd_Schema,
		JSPackageManagerCmdDir:  JSPackageManagerCmdDir_Schema,
		DefaultPort:             DefaultPort_Schema,
		ViteConfigFile:          ViteConfigFile_Schema,
	},
	RequiredChildren: []string{"JSPackageManagerBaseCmd"},
})

/////////////////////////////////////////////////////////////////////
/////// VITE SETTINGS -- JS PACKAGE MANAGER BASE CMD
/////////////////////////////////////////////////////////////////////

var JSPackageManagerBaseCmd_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Base command to run Vite using your preferred package manager.
This is the command to run standalone CLIs (e.g., "npx", not "npm run").`,
	Examples: []string{"npx", "pnpm", "yarn", "bunx"},
})

/////////////////////////////////////////////////////////////////////
/////// VITE SETTINGS -- JS PACKAGE MANAGER CMD DIR
/////////////////////////////////////////////////////////////////////

var JSPackageManagerCmdDir_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Directory to run the package manager command from. For example, if you're running commands from ".", but you want to run Vite from "./web", set this to "./web".`,
	Examples:    []string{"./web", "./client"},
	Default:     ".",
})

/////////////////////////////////////////////////////////////////////
/////// VITE SETTINGS -- DEFAULT PORT
/////////////////////////////////////////////////////////////////////

var DefaultPort_Schema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Default port to use for Vite dev server. This is used when you run "wave dev" without specifying a port.`,
	Default:     5173,
})

/////////////////////////////////////////////////////////////////////
/////// VITE SETTINGS -- CONFIG FILE
/////////////////////////////////////////////////////////////////////

var ViteConfigFile_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your Vite config file if it is in a non-standard location.
Should be set relative to the JSPackageManagerCmdDir, if set, otherwise your current working directory.`,
	Examples: []string{"./configs/vite.config.ts", "vite.custom.js"},
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS
/////////////////////////////////////////////////////////////////////

var Watch_Schema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `File watching configuration for development mode. Controls which files trigger rebuilds and how.`,
	Properties: struct {
		WatchRoot           jsonschema.Entry
		HealthcheckEndpoint jsonschema.Entry
		Include             jsonschema.Entry
		Exclude             jsonschema.Entry
	}{
		WatchRoot:           WatchRoot_Schema,
		HealthcheckEndpoint: HealthcheckEndpoint_Schema,
		Include:             Include_Schema,
		Exclude:             Exclude_Schema,
	},
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- ROOT DIR
/////////////////////////////////////////////////////////////////////

var WatchRoot_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `The directory against which all watch settings paths are relative.
If not set, all paths are relative to the directory from which you run commands.`,
	Default: ".",
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- HEALTHCHECK PATH
/////////////////////////////////////////////////////////////////////

var HealthcheckEndpoint_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Path to your app's healthcheck endpoint. Must return 200 OK when healthy. During dev-time rebuilds and restarts, this endpoint will be polled to determine when your app is ready to begin serving normal requests.`,
	Examples:    []string{"/healthz", "/health", "/api/health"},
	Default:     "/",
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE
/////////////////////////////////////////////////////////////////////

var Include_Schema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Files and patterns to watch for changes. Each pattern can specify what actions to take when matching files change.`,
	Items:       IncludeItems_Schema,
})

var IncludeItems_Schema = jsonschema.OptionalObject(jsonschema.Def{
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
		Pattern:                            Pattern_Schema,
		OnChangeHooks:                      OnChangeHooks_Schema,
		RecompileGoBinary:                  RecompileGoBinary_Schema,
		RestartApp:                         RestartApp_Schema,
		OnlyRunClientDefinedRevalidateFunc: OnlyRunClientDefinedRevalidateFunc_Schema,
		RunOnChangeOnly:                    RunOnChangeOnly_Schema,
		SkipRebuildingNotification:         SkipRebuildingNotification_Schema,
		TreatAsNonGo:                       TreatAsNonGo_Schema,
	},
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE -- PATTERN
/////////////////////////////////////////////////////////////////////

var Pattern_Schema = jsonschema.RequiredString(jsonschema.Def{
	Description: `Glob pattern for matching files (set relative to WatchRoot).
Supports ** for recursive matching.`,
	Examples: []string{"**/*.go", "frontend/src/**/*.ts", "templates/*.html"},
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE -- ON CHANGE
/////////////////////////////////////////////////////////////////////

var OnChangeHooks_Schema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Commands or strategies to run when a file matching the pattern changes. Each hook can either specify a Cmd (shell command) or a Strategy (declarative behavior).`,
	Items:       OnChangeHooksItems_Schema,
})

var OnChangeHooksItems_Schema = jsonschema.OptionalObject(jsonschema.Def{
	Properties: struct {
		Cmd      jsonschema.Entry
		Strategy jsonschema.Entry
		Timing   jsonschema.Entry
		Exclude  jsonschema.Entry
	}{
		Cmd:      Cmd_Schema,
		Strategy: Strategy_Schema,
		Timing:   Timing_Schema,
		Exclude:  OnChangeHooksExclude_Schema,
	},
})

var Cmd_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Shell command to run when a file matching the pattern changes.
Can be any shell command or "DevBuildHook" to run the configured dev build hook.
Ignored if Strategy is set.`,
	Examples: []string{"echo 'File changed!'", "make generate", "DevBuildHook", "npm run lint"},
})

var Strategy_Schema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Declarative strategy for handling file changes. Use this instead of Cmd for complex behaviors like calling HTTP endpoints on the running app.`,
	Properties: struct {
		HttpEndpoint   jsonschema.Entry
		SkipDevHook    jsonschema.Entry
		SkipGoCompile  jsonschema.Entry
		WaitForApp     jsonschema.Entry
		WaitForVite    jsonschema.Entry
		ReloadBrowser  jsonschema.Entry
		FallbackAction jsonschema.Entry
	}{
		HttpEndpoint:   HttpEndpoint_Schema,
		SkipDevHook:    SkipDevHook_Schema,
		SkipGoCompile:  SkipGoCompile_Schema,
		WaitForApp:     WaitForApp_Schema,
		WaitForVite:    WaitForVite_Schema,
		ReloadBrowser:  ReloadBrowser_Schema,
		FallbackAction: FallbackAction_Schema,
	},
})

var HttpEndpoint_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `HTTP endpoint to call on the running app (e.g., "/__vorma/rebuild-routes").
If the call fails, FallbackAction is executed.`,
	Examples: []string{"/__vorma/rebuild-routes", "/__vorma/reload-template", "/__my-app/refresh-cache"},
})

var SkipDevHook_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, skips running the DevBuildHook for this change.`,
	Default:     false,
})

var SkipGoCompile_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, skips Go binary recompilation for this change.`,
	Default:     false,
})

var WaitForApp_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, waits for the app's healthcheck before reloading the browser.`,
	Default:     false,
})

var WaitForVite_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, waits for Vite dev server before reloading the browser.`,
	Default:     false,
})

var ReloadBrowser_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, triggers a browser reload after successful execution.`,
	Default:     false,
})

var FallbackAction_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Action to take if HttpEndpoint fails.
"restart" does a full restart with Go recompile, "restart-no-go" restarts without Go recompile, "none" does nothing.`,
	Enum:    []string{"restart", "restart-no-go", "none"},
	Default: "none",
})

var Timing_Schema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Timing of the given command relative to Wave's rebuild process. Only applies to Cmd hooks, not Strategy hooks.`,
	Enum:        []string{"pre", "post", "concurrent", "concurrent-no-wait"},
	Default:     "pre",
})

var OnChangeHooksExclude_Schema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Glob patterns for files to exclude from triggering this onchange hook (set relative to WatchRoot).`,
	Items:       jsonschema.OptionalString(jsonschema.Def{}),
	Examples:    []string{"**/*_test.go", "**/*.gen.ts"},
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE -- RECOMPILE BINARY
/////////////////////////////////////////////////////////////////////

var RecompileGoBinary_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, the Go binary will be recompiled when this file changes.
Use for non-Go files that affect the Go build (e.g., embedded files).`,
	Default: false,
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE -- RESTART APP
/////////////////////////////////////////////////////////////////////

var RestartApp_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, the app will be restarted when this file changes. Use for files that are cached on startup (e.g., templates that are parsed once).`,
	Default:     false,
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE -- RUN CLIENT DEFINED REVALIDATE FUNC
/////////////////////////////////////////////////////////////////////

var OnlyRunClientDefinedRevalidateFunc_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, Wave will call window.__waveRevalidate() instead of reloading the page. Use with frameworks that support hot module replacement or client-side revalidation.`,
	Default:     false,
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE -- RUN ON CHANGE ONLY
/////////////////////////////////////////////////////////////////////

var RunOnChangeOnly_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, only the OnChangeHooks will run - Wave won't reload the browser.
Use when your onChange hook triggers its own reload process.
Note: OnChangeHooks must use "pre" timing (the default) with this option.`,
	Default: false,
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE -- SKIP REBUILDING NOTIFICATION
/////////////////////////////////////////////////////////////////////

var SkipRebuildingNotification_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, Wave won't show the "Rebuilding..." overlay in the browser. Use with RunOnChangeOnly if your onChange doesn't trigger a rebuild, or for changes that don't need user notification.`,
	Default:     false,
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- INCLUDE -- TREAT AS NON GO
/////////////////////////////////////////////////////////////////////

var TreatAsNonGo_Schema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, .go files matching this pattern won't trigger Go recompilation. Use for Go files that are independent from your main app (e.g., WASM files with separate build processes).`,
	Default:     false,
})

/////////////////////////////////////////////////////////////////////
/////// WATCH SETTINGS -- EXCLUDE
/////////////////////////////////////////////////////////////////////

var Exclude_Schema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Patterns for files and directories to exclude from watching.
Use to prevent unnecessary rebuilds from vendor files, build outputs, etc.`,
	Properties: struct {
		Dirs  jsonschema.Entry
		Files jsonschema.Entry
	}{
		Dirs:  ExcludeDirs_Schema,
		Files: ExcludeFiles_Schema,
	},
})

var ExcludeDirs_Schema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Glob patterns for directories to exclude from the watcher (set relative to WatchRoot). Wave automatically excludes .git, node_modules, and the dist directory.`,
	Items:       jsonschema.OptionalString(jsonschema.Def{}),
	Examples:    []string{"vendor", "tmp", ".cache", "coverage"},
})

var ExcludeFiles_Schema = jsonschema.OptionalArray(jsonschema.Def{
	Description: `Glob patterns for files to exclude from the watcher (set relative to WatchRoot).`,
	Items:       jsonschema.OptionalString(jsonschema.Def{}),
	Examples:    []string{"**/*.log", "**/.DS_Store", "**/*~"},
})
