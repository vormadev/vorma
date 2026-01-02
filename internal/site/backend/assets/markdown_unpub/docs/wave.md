---
title: Wave Build Tool API Documentation
description: API documentation for the Wave build tool
---

```
import "github.com/vormadev/vorma/wave"
```

## Overview

Wave is a Go-based build system designed for web applications that handles
static asset processing, CSS compilation, file watching, hot reloading, and
integration with Vite for JavaScript/TypeScript builds. It provides a unified
development and production build pipeline with automatic browser refresh
capabilities.

Wave is the lower-level build tool used by the Vorma framework, but Wave itself
can be used without Vorma (and without Vite, for that matter). In fact, it can
be used without any frontend at all as a simple file watcher that restarts any
server (or really, does anything at all in response to file changes).

You can use Wave with any Go server framework or router you choose, or just use
the standard library. Unlike some alternatives, Wave doesn't require you to
install any tooling on your machine — it is orchestrated solely from inside your
repo and its dependencies.

### Key Features

**Development Mode:**

- Automatic smart rebuilds and browser refreshes
- Instant CSS hot reloading (without full page refresh)
- Granular file watching with glob patterns
- Configurable build hooks with timing strategies
- Debounced file watching (30ms default)
- Optional Vite integration for full hot module reloading

**Production Mode:**

- Static asset hashing and embedding
- CSS bundling and minification via esbuild
- Critical CSS inlining
- Safe serving of public assets with immutable cache headers
- Optional Vite integration for building a fully optimized frontend static asset
  suite

## Technical Architecture

### CSS Processing with esbuild

Wave uses **esbuild** internally for CSS processing, providing:

- CSS bundling with `@import` resolution
- Automatic minification in production
- URL resolution for referenced assets (images, fonts, etc.)
- Separate handling for critical and non-critical CSS

### Build Caching

Wave employs several caching strategies:

- **Runtime caches** for filesystem access, CSS content, and public file maps
- **Granular rebuilds** in development that only process changed files
- **Debounced file watching** (30ms default) to batch rapid changes

### WebSocket Communication

The development server uses **WebSocket connections** for browser communication:

- Real-time push of CSS updates without page reload
- Different message types for different update scenarios (CSS, full reload,
  revalidation)
- Rebuilding overlay management

## Quick Start Tutorial (~5 minutes)

### 1. Initialize Your Project

```sh
go mod init your-module-name
```

### 2. Create Directory Structure

```sh
# Scaffold directories
mkdir -p cmd/app cmd/build cmd/dev
mkdir -p static/private static/public/prehashed
mkdir -p dist styles internal/platform

# Create placeholder files
touch cmd/app/main.go cmd/build/main.go cmd/dev/main.go
touch static/private/index.go.html
touch styles/critical.css styles/main.css
touch wave.config.json
touch dist/embed.go internal/platform/wave.go
```

### 3. Configure Wave

Create `wave.config.json`:

```json
{
	"$schema": "dist/static/internal/schema.json",
	"Core": {
		"MainAppEntry": "./cmd/app/main.go",
		"DistDir": "./dist",
		"StaticAssetDirs": {
			"Private": "./static/private",
			"Public": "./static/public"
		},
		"CSSEntryFiles": {
			"Critical": "./styles/critical.css",
			"NonCritical": "./styles/main.css"
		},
		"PublicPathPrefix": "/public/"
	},
	"Watch": {
		"HealthcheckEndpoint": "/healthz",
		"Include": [
			{
				"Pattern": "**/*.go.html",
				"RestartApp": true
			}
		]
	}
}
```

### 4. Setup Embedding (Optional but Recommended for Production)

Create `dist/embed.go`:

```go
package dist

import "embed"

//go:embed static
var StaticFS embed.FS
```

### 5. Initialize Wave

Create `internal/platform/wave.go`:

```go
package platform

import (
    _ "embed"
    "your-module-name/dist"
    "github.com/vormadev/vorma/wave"
)

//go:embed ../../wave.config.json
var configBytes []byte

var Wave = wave.New(&wave.Config{
    ConfigBytes: configBytes,
    StaticFS: dist.StaticFS,
    StaticFSEmbedDirective: "static",
})
```

### 6. Create Your Application

Create `cmd/app/main.go`:

```go
package main

import (
    "fmt"
    "html/template"
    "net/http"
    "your-module-name/internal/platform"
    "github.com/vormadev/vorma/wave"
)

func main() {
    // Health check endpoint
    http.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    })

    // Serve static files
    handler, err := platform.Wave.GetServeStaticHandler(true)
    if err != nil {
        panic(err)
    }
    http.Handle("/public/", handler)

    // Serve HTML template
    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        privateFS, err := platform.Wave.GetPrivateFS()
        if err != nil {
            http.Error(w, "Error loading filesystem", http.StatusInternalServerError)
            return
        }

        tmpl, err := template.ParseFS(privateFS, "index.go.html")
        if err != nil {
            http.Error(w, "Error loading template", http.StatusInternalServerError)
            return
        }

        err = tmpl.Execute(w, map[string]any{
            "Wave": platform.Wave,
        })
        if err != nil {
            http.Error(w, "Error executing template", http.StatusInternalServerError)
        }
    })

    port := wave.MustGetPort()
    fmt.Printf("Starting server on: http://localhost:%d\n", port)
    http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
}
```

### 7. Create Your Template

Create `static/private/index.go.html`:

```html
<!DOCTYPE html>
<html lang="en">
	<head>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1" />
		{{.Wave.GetCriticalCSSStyleElement}} {{.Wave.GetStyleSheetLinkElement}}
	</head>
	<body>
		<h1>Hello, Wave!</h1>
		{{.Wave.GetRefreshScript}}
	</body>
</html>
```

### 8. Build and Dev Commands

Create `cmd/build/main.go`:

```go
package main

import "your-module-name/internal/platform"

func main() {
    if err := platform.Wave.Build(); err != nil {
        panic(err)
    }
}
```

Running `go run ./cmd/build` will build your project and save your binary to
`dist/main` (or `dist/main.exe` on Windows).

**With embedded assets (recommended)**: You can run your binary from anywhere,
and it will serve static assets from the embedded filesystem.

**Without embedding**: The binary must be a sibling of the `static` directory
(copy `dist/static` to `static` in your deployment directory).

> **Note**: Often you'll want to handle Go binary compilation yourself. Use
> `platform.Wave.BuildWithoutCompilingGo()` instead of `platform.Wave.Build()`
> to run Wave's processing (asset hashing, etc.) without producing an
> executable.

Create `cmd/dev/main.go`:

```go
package main

import "your-module-name/internal/platform"

func main() {
    platform.Wave.MustStartDev()
}
```

### 9. Run Your Dev Server

```sh
go run ./cmd/dev
```

The dev server will start on port 8080 or find an available fallback port.

### 10. Setup .gitignore

```sh
echo "dist/*\n!dist/embed.go" > .gitignore
```

## When to Use Wave vs Alternatives

If you only need automatic Go application rebuilds without browser refreshes or
static asset build tooling, Wave may be overkill. Consider using
[Air](https://github.com/cosmtrek/air) instead.

However, Wave offers advantages:

- No external tooling installation required (everything runs from your Go
  dependencies)
- `ServerOnlyMode` option for simpler use cases without frontend build steps
- Better dev-prod parity

To enable server-only mode:

```json
{
	"Core": {
		"ServerOnlyMode": true
	}
}
```

## Installation & Setup

### Basic Configuration

Wave requires a configuration file (typically named `wave.config.json`) with the
following structure (many fields are optional, as documented by the included
JSON schema):

```json
{
  "$schema": "dist/static/internal/schema.json",
  "Core": {
    "DevBuildHook": "go run ./cmd/build -dev",
    "ProdBuildHook": "go run ./cmd/build",
    "MainAppEntry": "./cmd/app/main.go",
    "DistDir": "./dist",
    "StaticAssetDirs": {
      "Private": "./static/private",
      "Public": "./static/public"
    },
    "CSSEntryFiles": {
      "Critical": "./styles/critical.css",
      "NonCritical": "./styles/main.css"
    },
    "PublicPathPrefix": "/public/",
    "ServerOnlyMode": false
  },
  "Vite": {
    "JSPackageManagerBaseCmd": "npx",
    "JSPackageManagerCmdDir": "./app",
    "DefaultPort": 5173,
    "ViteConfigFile": "./vite.config.ts"
  },
  "Watch": {
    "WatchRoot": ".",
    "HealthcheckEndpoint": "/healthz",
    "Include": [...],
    "Exclude": {...}
  }
}
```

### Initialization

```go
import "github.com/vormadev/vorma/wave"

// With embedded filesystem (recommended for production)
//go:embed dist/static
var staticFS embed.FS

//go:embed wave.config.json
var configJSON []byte

w := wave.New(&wave.Config{
    ConfigBytes: configJSON,
    StaticFS: staticFS,
    StaticFSEmbedDirective: "dist/static",
    Logger: slog.Default(),
})

// Without embedded filesystem (files served from disk)
w := wave.New(&wave.Config{
    ConfigBytes: configJSON,
})
```

## Core API Methods

### Build Methods

`Build() error`

Builds the entire application including Go binary compilation.

```go
err := w.Build()
```

---

`BuildWithoutCompilingGo() error`

Builds static assets and runs build hooks without compiling the Go binary.

```go
err := w.BuildWithoutCompilingGo()
```

---

`ViteProdBuild() error`

Runs the Vite production build if configured.

```go
err := w.ViteProdBuild()
```

### Development Mode

`MustStartDev()`

Starts the development server with file watching and hot reloading.

```go
w.MustStartDev()
```

Features:

- Automatic smart rebuilds and browser refreshes
- Instant CSS hot reloading (without full page refresh)
- Granular file watching with glob patterns
- Configurable build hooks with timing strategies
- Debounced file watching (30ms)

---

`Builder(hook func(isDev bool) error)`

Helper method for creating build commands with custom hooks.

```go
w.Builder(func(isDev bool) error {
    // Custom build logic
    if isDev {
        // Development-specific logic
    } else {
        // Production-specific logic
    }
    return nil
})
```

### Filesystem Access

`GetPublicFS() (fs.FS, error)`

Returns the filesystem containing public static assets.

```go
publicFS, err := w.GetPublicFS()
```

---

`MustGetPublicFS() fs.FS`

Panics if unable to get public filesystem.

```go
publicFS := w.MustGetPublicFS()
```

---

`GetPrivateFS() (fs.FS, error)`

Returns the filesystem containing private static assets (templates, server-side
files).

```go
privateFS, err := w.GetPrivateFS()
```

---

`MustGetPrivateFS() fs.FS`

Panics if unable to get private filesystem.

```go
privateFS := w.MustGetPrivateFS()
```

---

`GetBaseFS() (fs.FS, error)`

Returns the base filesystem (development or embedded).

```go
baseFS, err := w.GetBaseFS()
```

### URL Generation

Wave automatically hashes public assets for cache busting.

---

`GetPublicURL(originalURL string) string`

Returns the hashed/versioned URL for a public asset at runtime.

```go
// Input: "images/logo.png"
// Output: "/public/vorma_out_images_logo_abc123def456.png"
imageURL := w.GetPublicURL("images/logo.png")
```

---

`MustGetPublicURLBuildtime(originalURL string) string`

Returns the hashed URL for a public asset at build time. Panics on error.

```go
cssURL := w.MustGetPublicURLBuildtime("styles/theme.css")
```

### CSS Management

Wave provides special handling for critical and non-critical CSS using esbuild
for processing.

Critical CSS is intended to be inlined into your document head, whereas
non-critical CSS is intended to be loaded via a normal stylesheet via HTTP
request.

---

`GetCriticalCSS() template.CSS`

Returns the critical CSS content for inline rendering.

```go
criticalCSS := w.GetCriticalCSS()
```

---

`GetCriticalCSSStyleElement() template.HTML`

Returns the complete `<style>` element with critical CSS.

```go
// Returns: <style id="wave-critical-css">...</style>
styleElement := w.GetCriticalCSSStyleElement()
```

---

`GetCriticalCSSStyleElementSha256Hash() string`

Returns the SHA256 hash for CSP headers.

```go
hash := w.GetCriticalCSSStyleElementSha256Hash()
```

---

`GetStyleSheetURL() string`

Returns the URL for the non-critical CSS file.

```go
cssURL := w.GetStyleSheetURL()
```

---

`GetStyleSheetLinkElement() template.HTML`

Returns the complete `<link>` element for non-critical CSS.

```go
// Returns: <link rel="stylesheet" href="/public/normal_abc123.css" id="wave-normal-css" />
linkElement := w.GetStyleSheetLinkElement()
```

### Public File Map

Wave maintains a map of original filenames to their hashed versions, stored as
both Gob (for Go) and JavaScript modules (for client-side).

---

`GetPublicFileMap() (FileMap, error)`

Returns the complete mapping of original to hashed filenames.

```go
fileMap, err := w.GetPublicFileMap()
```

---

`GetPublicFileMapKeysBuildtime() ([]string, error)`

Returns all original filenames in the public file map (build time).

```go
// Returns: ["images/logo.png", "fonts/main.woff2", ...]
keys, err := w.GetPublicFileMapKeysBuildtime()
```

---

`GetSimplePublicFileMapBuildtime() (map[string]string, error)`

Returns a simplified string-to-string mapping (build time). Triggers a build if
the file map doesn't exist.

```go
// Returns: {"images/logo.png": "vorma_out_images_logo_abc123.png", ...}
simpleMap, err := w.GetSimplePublicFileMapBuildtime()
```

---

`GetPublicFileMapElements() template.HTML`

Returns HTML elements for client-side file mapping.

```go
// Returns: <link rel="modulepreload"...><script type="module">...</script>
elements := w.GetPublicFileMapElements()
```

---

`GetPublicFileMapScriptSha256Hash() string`

Returns the SHA256 hash of the file map script for CSP.

```go
hash := w.GetPublicFileMapScriptSha256Hash()
```

---

`GetPublicFileMapURL() string`

Returns the URL of the public file map JavaScript module.

```go
url := w.GetPublicFileMapURL()
```

### Development Tools

`GetRefreshScript() template.HTML`

Returns the hot reload script for development mode.

```go
if wave.GetIsDev() {
    // Inject into HTML head
    refreshScript := w.GetRefreshScript()
}
```

The refresh script:

- Establishes WebSocket connection to Wave's dev server
- Handles different reload types (CSS hot reload, full page reload, client
  revalidation)
- Shows "Rebuilding..." overlay during rebuilds
- Preserves scroll position across reloads

---

`GetRefreshScriptSha256Hash() string`

Returns the SHA256 hash of the refresh script for CSP.

```go
hash := w.GetRefreshScriptSha256Hash()
```

### HTTP Handlers

`GetServeStaticHandler(addImmutableCacheHeaders bool) (http.Handler, error)`

Returns an HTTP handler for serving static files.

```go
handler, err := w.GetServeStaticHandler(true)
http.Handle("/public/", handler)
```

---

`MustGetServeStaticHandler(addImmutableCacheHeaders bool) http.Handler`

Panics if unable to create static handler.

```go
handler := w.MustGetServeStaticHandler(true)
```

---

`ServeStatic(addImmutableCacheHeaders bool) func(http.Handler) http.Handler`

Returns middleware for serving static assets.

```go
// As middleware
router.Use(w.ServeStatic(true))
```

---

`FaviconRedirect() middleware.Middleware`

Returns middleware that redirects `/favicon.ico` to the public path.

```go
router.Use(w.FaviconRedirect())
```

### Configuration Accessors

```go
// Returns: "/public/"
w.GetPublicPathPrefix()

// Returns: "./static/private"
w.GetPrivateStaticDir()

// Returns: "./static/public"
w.GetPublicStaticDir()

// Returns: "./dist"
w.GetDistDir()

// Returns: "./dist/static/assets/private"
w.GetStaticPrivateOutDir()

// Returns: "./dist/static/assets/public"
w.GetStaticPublicOutDir()

// Path to config file
w.GetConfigFile()
```

### Utility Methods

---

`SetupDistDir()`

Creates the necessary directory structure for the distribution folder.

```go
w.SetupDistDir()
```

## Global Functions

`wave.GetIsDev() bool`

Returns true if running in development mode.

```go
if wave.GetIsDev() {
    // Development-specific logic
}
```

---

`wave.SetModeToDev()`

Sets the environment to development mode.

```go
wave.SetModeToDev()
```

---

`wave.MustGetPort() int`

Returns the application port, finding a free port if necessary.

```go
port := wave.MustGetPort()
http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
```

## File Watching Configuration

Wave's file watching system uses the **fsnotify** library for cross-platform
file system notifications.

### WatchedFile Structure

```go
type WatchedFile struct {
    // Glob pattern for matching files
    Pattern string

    // Commands to run on change
    OnChangeHooks []OnChangeHook

    // Recompile Go binary
    RecompileGoBinary bool

    // Restart the application
    RestartApp bool

    // Only run client revalidate
    OnlyRunClientDefinedRevalidateFunc bool

    // Skip reload, only run hooks
    RunOnChangeOnly bool

    // Don't show rebuilding message
    SkipRebuildingNotification bool

    // Treat .go files as non-Go
    TreatAsNonGo bool
}
```

### OnChangeHook Structure

```go
type OnChangeHook struct {
    // Command to execute
    Cmd string

    // When to run (pre/post/concurrent/concurrent-no-wait)
    Timing string

    // Glob patterns to exclude
    Exclude []string
}
```

### Timing Strategies

```go
const (
    // Run before Wave processing
    OnChangeStrategyPre = "pre"

    // Run concurrently with Wave
    OnChangeStrategyConcurrent = "concurrent"

    // Run without waiting
    OnChangeStrategyConcurrentNoWait = "concurrent-no-wait"

    // Run after Wave processing
    OnChangeStrategyPost = "post"
)
```

### Example Watch Configuration

```json
{
	"Watch": {
		"WatchRoot": ".",
		"HealthcheckEndpoint": "/healthz",
		"Include": [
			{
				"Pattern": "**/*.go",
				"RecompileGoBinary": true
			},
			{
				"Pattern": "**/*.sql",
				"OnChangeHooks": [
					{
						"Cmd": "go run ./cmd/sqlgen",
						"Timing": "pre"
					}
				]
			},
			{
				"Pattern": "data/*.json",
				"OnlyRunClientDefinedRevalidateFunc": true
			},
			{
				"Pattern": "templates/*.html",
				"RestartApp": true
			}
		],
		"Exclude": {
			"Dirs": ["vendor", "node_modules", ".git"],
			"Files": ["**/*.test.go", "**/*.gen.go"]
		}
	}
}
```

## Client-Defined Revalidation Function

Wave supports custom client-side revalidation without full page reloads.

### Setting up a Revalidation Function

Define a `__waveRevalidate` function on the window object:

```js
// In your client-side code
window.__waveRevalidate = async () => {
	// Your custom revalidation logic here
	await myFramework.revalidateCurrentRoute();

	console.log("Revalidation complete");
};
```

### Configuring Wave to Use It

```json
{
	"Watch": {
		"Include": [
			{
				"Pattern": "data/*.json",
				"OnlyRunClientDefinedRevalidateFunc": true
			}
		]
	}
}
```

When `OnlyRunClientDefinedRevalidateFunc` is set, Wave will:

1. Detect the file change
2. Send a `revalidate` message to the browser via WebSocket
3. Call `window.__waveRevalidate()` if it exists
4. Show "Rebuilding..." overlay until the Promise resolves

## Special Directories

### Prehashed Directory

Files in `public/prehashed` directory will keep their original names and won't
be hashed.

```go
const PrehashedDirname = "prehashed"
```

Example structure:

```
public/
  prehashed/
    # Will be served as-is
    robots.txt
    # Won't be renamed
    sitemap.xml
  images/
    # Will be hashed
    logo.png
```

## Vite Integration

### Configuration

Add Vite settings to your `wave.config.json`:

```json
{
	"Vite": {
		"JSPackageManagerBaseCmd": "npx",
		"JSPackageManagerCmdDir": "./app",
		"DefaultPort": 5173,
		"ViteConfigFile": "./vite.config.ts"
	}
}
```

Configuration options:

- **JSPackageManagerBaseCmd** (required): Your package manager command (`npx`,
  `pnpm`, `yarn`, `bunx`)
- **JSPackageManagerCmdDir** (optional): Working directory for Vite commands
- **DefaultPort** (optional): Vite dev server port (default: 5173)
- **ViteConfigFile** (optional): Path to custom Vite config

### Development Mode

When you run Wave in development with Vite configured:

1. Wave automatically starts the Vite dev server
2. Vite handles JavaScript/TypeScript with HMR
3. Wave handles Go rebuilds and static assets
4. Both systems coordinate for optimal reloading

### Production Builds

```go
err := w.ViteProdBuild()
```

This will:

- Build your JavaScript/TypeScript with Vite
- Output to Wave's public directory
- Generate a manifest for tracking dependencies
- Integrate with Wave's asset hashing system

### React Fast Refresh

When using Vorma with `UIVariant: "react"` configured, Wave injects React Fast
Refresh runtime in development for component state preservation during edits.

### API Methods

```go
// Get the Vite manifest location
manifestPath := w.GetViteManifestLocation()

// Get the Vite output directory
viteOutDir := w.GetViteOutDir()
```

## Vorma Framework Integration

Wave is the build system that powers the Vorma framework. When used with Vorma,
Wave provides additional functionality.

### Vorma Configuration

Configure Vorma-specific settings in your `wave.config.json`:

```json
{
	"Vorma": {
		"IncludeDefaults": true,
		"UIVariant": "react",
		"HTMLTemplateLocation": "templates/index.html",
		"ClientEntry": "./frontend/src/main.tsx",
		"ClientRouteDefsFile": "./frontend/src/routes.gen.ts",
		"TSGenOutPath": "./frontend/vorma.gen.ts",
		"BuildtimePublicURLFuncName": "waveURL"
	}
}
```

### Vorma-Specific Methods

```go
// Get the frontend UI library variant
variant := w.GetVormaUIVariant()

// Get HTML template location relative to private static dir
templatePath := w.GetVormaHTMLTemplateLocation()

// Get client-side entry point
clientEntry := w.GetVormaClientEntry()

// Get route definitions file path
routeDefsPath := w.GetVormaClientRouteDefsFile()

// Get TypeScript generation output path
tsGenPath := w.GetVormaTSGenOutPath()

// Get build-time public URL function name
funcName := w.GetVormaBuildtimePublicURLFuncName()
```

### Default Watch Patterns

When `IncludeDefaults` is true, Vorma adds these watch patterns:

1. **Go files**: Triggers build hook on changes
2. **HTML templates**: Triggers app restart for template parsing
3. **Route definitions**: Triggers full rebuild when routes change
4. **TypeScript generated files**: Automatically ignored to prevent loops

## File Hashing System

Files are hashed based on their content using SHA256 (truncated to 12
characters):

```
Original: images/logo.png
Hashed: vorma_out_images_logo_abc123def456.png
```

## Production Deployment

### With Embedded Assets (Recommended)

```go
//go:embed dist/static
var staticFS embed.FS

w := wave.New(&wave.Config{
    ConfigBytes: configJSON,
    StaticFS: staticFS,
    StaticFSEmbedDirective: "dist/static",
})
```

With embedded assets, you can run your binary from anywhere on the build
machine.

### Without Embedded Assets

```go
w := wave.New(&wave.Config{
    ConfigBytes: configJSON,
    // StaticFS and StaticFSEmbedDirective are omitted
})
```

When `StaticFS` is nil, Wave looks for a `static` directory next to the
executable.

**Required structure:**

```
your-binary
static/         # Copy dist/static here
  assets/
  internal/
```

**Note:** You must copy `dist/static` to `static` in your deployment directory,
not the entire `dist` folder.

## Element IDs

Wave injects elements with these IDs:

- `wave-critical-css` - Critical CSS style element
- `wave-normal-css` - Non-critical CSS link element
- `wave-refreshscript-rebuilding` - Development mode rebuilding indicator

## Constants

```go
const PrehashedDirname = "prehashed"
```
