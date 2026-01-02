---
title: Configuring the Wave Build Tool
description: How to configure the Wave build tool
---

Wave uses a JSON configuration file to control build processes, asset handling,
and development workflows. The configuration is organized into four main
sections: `Core`, `Vorma`, `Vite`, and `Watch`.

## Core Settings

The `Core` section contains fundamental Wave configuration that every project
needs.

### Core.MainAppEntry

- **Required**
- Path to your Go application's main entry point
- Refers to your actual application server that calls `http.Handler.ServeHTTP()`
  from a `main()` func

```json
{
	"Core": {
		"MainAppEntry": "./cmd/app/main.go"
	}
}
```

You can specify either the `main.go` file directly or its parent directory (just
like the behavior when you call `go run` manually).

### Core.DistDir

- **Required**
- Where Wave outputs the compiled binary and processed static assets
- Must be unique from your static asset directories

```json
{
	"Core": {
		"DistDir": "./dist"
	}
}
```

### Core.StaticAssetDirs

- **Required** (unless `ServerOnlyMode` is `true`)
- Defines where your static assets are located

```json
{
	"Core": {
		"StaticAssetDirs": {
			"Private": "./static/private", // Server-side only (templates, etc.)
			"Public": "./static/public" // Browser-accessible, gets hashed
		}
	}
}
```

- **Private**: Assets only accessible from Go code (templates, server-side
  files)
- **Public**: Assets served to browsers with content-addressed hashing for cache
  busting
    - Files in a `prehashed` subdirectory keep their original names without
      hashing

### Core.CSSEntryFiles

- **Optional**
- Entry points for CSS bundling and optimization

```json
{
	"Core": {
		"CSSEntryFiles": {
			"Critical": "./styles/main.critical.css", // Inlined in HTML head
			"NonCritical": "./styles/main.css" // Loaded asynchronously
		}
	}
}
```

### Core.PublicPathPrefix

- **Optional**
- Default: `"/"`
- URL path prefix for public assets
- Must start and end with `/`

```json
{
	"Core": {
		"PublicPathPrefix": "/assets/"
	}
}
```

### Core.ServerOnlyMode

- **Optional**
- Default: `false`
- Skip static asset processing and browser reloading for API-only servers

```json
{
	"Core": {
		"ServerOnlyMode": false
	}
}
```

### Core.ConfigLocation

- **Optional**
- Path to the Wave config file itself
- Enables auto-restart on config changes

```json
{
	"Core": {
		"ConfigLocation": "./wave.json"
	}
}
```

### Core.DevBuildHook

- **Optional**
- Command to run before Wave's build in development mode

```json
{
	"Core": {
		"DevBuildHook": "go run ./backend/cmd/build -dev"
	}
}
```

### Core.ProdBuildHook

- **Optional**
- Command to run before Wave's build in production mode

```json
{
	"Core": {
		"ProdBuildHook": "go run ./backend/cmd/build"
	}
}
```

## Vorma Settings

Configure Wave's integration with the Vorma framework.

### Vorma.IncludeDefaults

- **Optional**
- Default: `true`
- Whether to include Vorma's default watch patterns and build hooks

```json
{
	"Vorma": {
		"IncludeDefaults": true
	}
}
```

### Vorma.UIVariant

- **Required** (when using Vorma)
- Which UI library integration to use
- Options: `"react"`, `"preact"`, `"solid"`

```json
{
	"Vorma": {
		"UIVariant": "react"
	}
}
```

### Vorma.HTMLTemplateLocation

- **Required** (when using Vorma)
- Path to HTML template, relative to your private static directory

```json
{
	"Vorma": {
		"HTMLTemplateLocation": "entry.go.html"
	}
}
```

### Vorma.ClientEntry

- **Required** (when using Vorma)
- Client-side TypeScript entry point

```json
{
	"Vorma": {
		"ClientEntry": "frontend/src/vorma.entry.tsx"
	}
}
```

### Vorma.ClientRouteDefsFile

- **Required** (when using Vorma)
- Where Vorma writes route definitions

```json
{
	"Vorma": {
		"ClientRouteDefsFile": "frontend/src/vorma.routes.ts"
	}
}
```

### Vorma.TSGenOutPath

- **Required** (when using Vorma)
- Where TypeScript type definitions are generated

```json
{
	"Vorma": {
		"TSGenOutPath": "frontend/src/vorma.gen.ts"
	}
}
```

### Vorma.BuildtimePublicURLFuncName

- **Optional**
- Function name for resolving public URLs at build time

```json
{
	"Vorma": {
		"BuildtimePublicURLFuncName": "waveBuildtimeURL"
	}
}
```

## Vite Settings

Configure Vite integration for frontend asset bundling.

### Vite.JSPackageManagerBaseCmd

- **Required** (when using Vite)
- Base command for running standalone CLIs
- Options: `"npx"`, `"pnpm"`, `"yarn"`, `"bunx"`

```json
{
	"Vite": {
		"JSPackageManagerBaseCmd": "npx"
	}
}
```

### Vite.JSPackageManagerCmdDir

- **Optional**
- Default: `"."`
- Directory to run package manager commands from

```json
{
	"Vite": {
		"JSPackageManagerCmdDir": "./app"
	}
}
```

### Vite.DefaultPort

- **Optional**
- Default: `5173`
- Default Vite dev server port

```json
{
	"Vite": {
		"DefaultPort": 5173
	}
}
```

### Vite.ViteConfigFile

- **Optional**
- Path to Vite config if in non-standard location
- Relative to `JSPackageManagerCmdDir`

```json
{
	"Vite": {
		"ViteConfigFile": "./configs/vite.config.ts"
	}
}
```

## Watch Settings

Control file watching behavior in development mode.

### Watch.WatchRoot

- **Optional**
- Default: `"."`
- Base directory for all watch paths

```json
{
	"Watch": {
		"WatchRoot": "."
	}
}
```

### Watch.HealthcheckEndpoint

- **Optional**
- Default: `"/"`
- Endpoint Wave polls to determine when your app is ready after rebuilds

```json
{
	"Watch": {
		"HealthcheckEndpoint": "/healthz"
	}
}
```

### Watch.Include

- **Optional**
- Array of file patterns to watch with specific actions

```json
{
	"Watch": {
		"Include": [
			{
				"Pattern": "**/*.go",
				"RecompileGoBinary": false,
				"RestartApp": false,
				"OnlyRunClientDefinedRevalidateFunc": false,
				"RunOnChangeOnly": false,
				"SkipRebuildingNotification": false,
				"TreatAsNonGo": false,
				"OnChangeHooks": [
					{
						"Cmd": "make generate",
						"Timing": "pre",
						"Exclude": ["**/*_test.go"]
					}
				]
			}
		]
	}
}
```

#### Watch.Include Properties

- **Pattern** (required): Glob pattern for matching files (relative to
  `WatchRoot`)
- **RecompileGoBinary**: Recompile Go binary when this file changes (for non-Go
  files affecting the Go binary)
- **RestartApp**: Restart the app when this file changes (for cached files like
  templates)
- **OnlyRunClientDefinedRevalidateFunc**: Call `window.__waveRevalidate()`
  instead of reloading
- **RunOnChangeOnly**: Only run `OnChangeHooks` without browser reload (requires
  `"pre"` timing)
- **SkipRebuildingNotification**: Don't show "Rebuilding..." overlay
- **TreatAsNonGo**: Don't trigger binary recompilation for `.go` files matching
  this pattern

#### Watch.Include.OnChangeHooks Properties

- **Cmd**: Command to run
    - Any shell command as a string
    - Special value: `"DevBuildHook"` runs the configured `Core.DevBuildHook`
      command
- **Timing**: When to run
    - `"pre"`: Before Wave's processing (default)
    - `"post"`: After Wave's processing
    - `"concurrent"`: During Wave's processing
    - `"concurrent-no-wait"`: During Wave's processing without waiting
- **Exclude**: Array of patterns to exclude from triggering this hook

### Watch.Exclude

- **Optional**
- Patterns for files and directories to exclude from watching

```json
{
	"Watch": {
		"Exclude": {
			"Dirs": ["vendor", "tmp", ".cache"],
			"Files": ["**/*.log", "**/.DS_Store"]
		}
	}
}
```

Wave automatically excludes `.git`, `node_modules`, and the `dist/static`
directory.
