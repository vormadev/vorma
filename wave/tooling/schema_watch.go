package tooling

import "github.com/vormadev/vorma/lab/jsonschema"

var watchSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `File watching configuration for development mode. Controls which files trigger rebuilds and how.`,
	Properties: struct {
		WatchRoot            jsonschema.Entry
		HealthcheckEndpoint  jsonschema.Entry
		HookCommandTimeouts  jsonschema.Entry
		HookCallbackTimeouts jsonschema.Entry
		Include              jsonschema.Entry
		Exclude              jsonschema.Entry
	}{
		WatchRoot:            watchRootSchema,
		HealthcheckEndpoint:  healthcheckEndpointSchema,
		HookCommandTimeouts:  hookCommandTimeoutsSchema,
		HookCallbackTimeouts: hookCallbackTimeoutsSchema,
		Include:              includeSchema,
		Exclude:              excludeSchema,
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

var hookCommandTimeoutsSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Stage-specific command timeouts for onChange hook command execution.
Use these values to prevent a single command hook from stalling watch cycles.
Set to 0 (or omit) to disable timeout for a stage.`,
	Properties: struct {
		PreCommandTimeoutMilliseconds              jsonschema.Entry
		ConcurrentCommandTimeoutMilliseconds       jsonschema.Entry
		ConcurrentNoWaitCommandTimeoutMilliseconds jsonschema.Entry
		PostCommandTimeoutMilliseconds             jsonschema.Entry
	}{
		PreCommandTimeoutMilliseconds:              preCommandTimeoutMillisecondsSchema,
		ConcurrentCommandTimeoutMilliseconds:       concurrentCommandTimeoutMillisecondsSchema,
		ConcurrentNoWaitCommandTimeoutMilliseconds: concurrentNoWaitCommandTimeoutMillisecondsSchema,
		PostCommandTimeoutMilliseconds:             postCommandTimeoutMillisecondsSchema,
	},
})

var hookCallbackTimeoutsSchema = jsonschema.OptionalObject(jsonschema.Def{
	Description: `Stage-specific callback timeouts for onChange hook callback execution.
Callback timeout handling is cooperative and surfaces as deadline/cancellation
through HookContext.ExecutionContext.
Set to 0 (or omit) to disable timeout for a stage.`,
	Properties: struct {
		PreCallbackTimeoutMilliseconds              jsonschema.Entry
		ConcurrentCallbackTimeoutMilliseconds       jsonschema.Entry
		ConcurrentNoWaitCallbackTimeoutMilliseconds jsonschema.Entry
		PostCallbackTimeoutMilliseconds             jsonschema.Entry
	}{
		PreCallbackTimeoutMilliseconds:              preCallbackTimeoutMillisecondsSchema,
		ConcurrentCallbackTimeoutMilliseconds:       concurrentCallbackTimeoutMillisecondsSchema,
		ConcurrentNoWaitCallbackTimeoutMilliseconds: concurrentNoWaitCallbackTimeoutMillisecondsSchema,
		PostCallbackTimeoutMilliseconds:             postCallbackTimeoutMillisecondsSchema,
	},
})

var preCommandTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Timeout in milliseconds for pre-stage hook commands.`,
})

var concurrentCommandTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Timeout in milliseconds for concurrent-stage hook commands.`,
})

var concurrentNoWaitCommandTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Timeout in milliseconds for concurrent-no-wait-stage hook commands.`,
})

var postCommandTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Timeout in milliseconds for post-stage hook commands.`,
})

var preCallbackTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Timeout in milliseconds for pre-stage hook callbacks.`,
})

var concurrentCallbackTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Timeout in milliseconds for concurrent-stage hook callbacks.`,
})

var concurrentNoWaitCallbackTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Timeout in milliseconds for concurrent-no-wait-stage hook callbacks.`,
})

var postCallbackTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Timeout in milliseconds for post-stage hook callbacks.`,
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
	Description: `Commands to run when a file matching the pattern changes.`,
	Items:       onChangeHooksItemsSchema,
})

var onChangeHooksItemsSchema = jsonschema.OptionalObject(jsonschema.Def{
	Properties: struct {
		Cmd                             jsonschema.Entry
		CommandTimeoutMilliseconds      jsonschema.Entry
		DisableStageCommandTimeout      jsonschema.Entry
		CallbackTimeoutMilliseconds     jsonschema.Entry
		DisableStageCallbackTimeout     jsonschema.Entry
		RunCombinedDevBuildHookCommands jsonschema.Entry
		Timing                          jsonschema.Entry
		Exclude                         jsonschema.Entry
	}{
		Cmd:                             cmdSchema,
		CommandTimeoutMilliseconds:      commandTimeoutMillisecondsSchema,
		DisableStageCommandTimeout:      disableStageCommandTimeoutSchema,
		CallbackTimeoutMilliseconds:     callbackTimeoutMillisecondsSchema,
		DisableStageCallbackTimeout:     disableStageCallbackTimeoutSchema,
		RunCombinedDevBuildHookCommands: runCombinedDevBuildHookCommandsSchema,
		Timing:                          timingSchema,
		Exclude:                         onChangeHooksExcludeSchema,
	},
})

var cmdSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Shell command to run when a file matching the pattern changes.
Can be any shell command.`,
	Examples: []string{"echo 'File changed!'", "make generate", "npm run lint"},
})

var commandTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Optional per-hook command timeout in milliseconds. When > 0, this overrides the stage-level timeout for this hook command.`,
})

var disableStageCommandTimeoutSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, disables stage-level command timeout for this hook command.`,
	Default:     false,
})

var callbackTimeoutMillisecondsSchema = jsonschema.OptionalNumber(jsonschema.Def{
	Description: `Optional per-hook callback timeout in milliseconds. When > 0, this overrides the stage-level timeout for this hook callback.`,
})

var disableStageCallbackTimeoutSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, disables stage-level callback timeout for this hook callback.`,
	Default:     false,
})

var runCombinedDevBuildHookCommandsSchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, runs configured development build hooks in sequence.
This runs Core.DevBuildHook first, then framework-injected dev build hooks.`,
	Default: false,
})

var timingSchema = jsonschema.OptionalString(jsonschema.Def{
	Description: `Timing of the given command relative to Wave's rebuild process.`,
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
	Description: `If true, Wave will call the configured browser revalidate function (default: window.__waveRevalidate()) instead of reloading the page. Use with frameworks that support hot module replacement or client-side revalidation.`,
	Default:     false,
})

var runOnChangeOnlySchema = jsonschema.OptionalBoolean(jsonschema.Def{
	Description: `If true, only the OnChangeHooks will run - Wave won't perform standard build/reload.
Use when your onChange hook handles everything including any necessary reload.`,
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
