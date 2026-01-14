# Architecture Decisions: Wave and Vorma

This document explains the fundamental constraints and design decisions behind
Wave and Vorma. It exists to provide context for future development and to
prevent well-intentioned but misguided "simplification" attempts.

## Core Premise

**Wave** is a Go-centric development toolkit that provides file watching, asset
processing, and live reload capabilities--with zero Node.js dependency.

**Vorma** is a full-stack TypeScript/Go framework built on top of Wave that uses
Vite for frontend bundling.

These serve different use cases:

- Wave alone: HTMX apps, Go template sites, internal tools--anywhere you want
  Go + browser without Node
- Vorma: Full SSR applications with React/Solid/Preact and type-safe data
  loading

---

## Fundamental Constraint: Go Cannot Hot Reload

Go is a compiled language. When Go source code changes, you must:

1. Recompile the binary
2. Stop the old process
3. Start the new process

There is no way around this. You cannot patch a running Go process with new
code.

This single constraint drives most of the architectural complexity.

### Implications

**Two processes are unavoidable in development:**

- Process A: The dev server (Wave) that watches files and orchestrates rebuilds
- Process B: The application process that serves requests

When Go code changes, Process B must die and restart. Process A survives to
coordinate this.

**Subprocess architecture is required:** The build hook
(`go run ./backend/cmd/build --dev --hook`) runs as a subprocess because it
needs to reflect on Go types that may have just changed. The dev server process
has stale type information; only freshly compiled code has the new types.

---

## Build vs Runtime: Binary Size Matters

Production server binaries should not include build-time dependencies like
esbuild or JavaScript parsers. These add several megabytes to binary size and
are never used at runtime.

### Package Structure

```
fw/
  types/      # Pure data types -- no dependencies
  runtime/    # Production server code -- stdlib + kit only
  build/      # Build/dev time only -- esbuild, JS parser
```

**Key rule:** `runtime` never imports `build`. Ever.

### Why This Matters for Fast Rebuilds

The fast rebuild path for route changes involves two processes:

| Process        | Has esbuild? | Has handlers? | Role                                       |
| -------------- | ------------ | ------------- | ------------------------------------------ |
| A (dev server) | Yes          | Yes           | Parse routes, generate TS, write artifacts |
| B (app server) | No           | Yes           | Reload artifacts from disk                 |

Process A can generate TypeScript because it runs the same app initialization
code as Process B--it has the same handlers registered for type reflection. The
key insight: when only `vorma.routes.ts` changes (no Go changes), Process A's
type information is still current.

### The Reload Endpoints

- `/__vorma/reload-routes` - Process B reads updated paths JSON from disk
- `/__vorma/reload-template` - Process B re-parses the HTML template file

Note the verb is **"reload" not "rebuild"**--Process B does no generation. All
generation happens in Process A before the HTTP call.

---

## Wave's Design: Why It Has Its Own Asset Pipeline

A common suggestion is "just use Vite for everything." This misunderstands
Wave's purpose.

### Wave Supports Zero-Node Development

Consider these legitimate use cases:

- An HTMX application with Go templates and zero JavaScript
- An internal admin tool with vanilla JS
- A marketing site with static CSS
- Any project where installing Node.js is undesirable

For these, Wave provides:

| Feature             | Purpose                                                          |
| ------------------- | ---------------------------------------------------------------- |
| Static file hashing | Go templates can reference `/style.abc123.css` for cache busting |
| CSS bundling        | Via esbuild's Go API--no Node required                           |
| WebSocket reload    | Tell the browser "Go rebuilt, please refresh"                    |
| Process management  | Compile, restart, health check the Go app                        |

### The WebSocket Server Is Not Duplicating Vite

Wave's WebSocket (`devserver/broadcast.go`) handles events Vite cannot know
about:

- Go code changed --> binary recompiled --> app restarted
- Go template changed --> app may need restart or reload
- Any server-side change that affects rendered HTML

When Vite is present, it handles JS/CSS HMR through its own channel. Wave's
WebSocket handles Go-level events. They coexist, serving different purposes.

### CSS Pipeline Rationale

Wave's CSS processing (`builder/css.go`) handles CSS entry points defined in
`wave.config.json`--files outside the JavaScript module graph.

- CSS imported in JavaScript --> Vite handles it
- CSS referenced from Go templates, no JS involved --> Wave handles it

If you require Vite for all CSS, you lose the Node-free use case.

### Static File Hashing: What Vite Doesn't Do

Vite handles asset hashing differently depending on how assets enter the build:

| Asset Source                                | Vite Behavior               | Cache Headers     |
| ------------------------------------------- | --------------------------- | ----------------- |
| `import logo from './logo.svg'` (JS import) | Hashed: `logo-abc123.svg`   | Immutable ✓       |
| `public/favicon.svg` (public folder)        | Copied as-is: `favicon.svg` | Must revalidate ✗ |

**The problem:** Assets referenced from Go templates don't go through JavaScript
imports.

```html
<!-- Go template -->
<link rel="icon" href="{{ .App.GetPublicURL "favicon.svg" }}">
<img src="{{ .App.GetPublicURL "hero.png" }}">
```

With Vite alone, these would come from the `public/` folder--unhashed, requiring
cache revalidation on every request.

Wave solves this. `GetPublicURL("favicon.svg")` returns
`vorma_out_favicon_abc123.svg`. The content hash is in the filename, enabling:

```
Cache-Control: public, max-age=31536000, immutable
```

This matters for production performance. Without content-addressed filenames,
browsers must revalidate assets on every page load. With them, assets are cached
forever until they actually change.

**Even Vorma apps benefit from Wave's hashing**--any asset referenced from Go
templates (favicons, OG images, fonts loaded via CSS `url()`, etc.) gets proper
cache busting that Vite's `public/` folder cannot provide.

---

## Vorma's Design: Speed on Top of Wave

Vorma requires Vite. This is a deliberate choice--Vorma targets rich frontend
applications where Vite's capabilities matter.

### The Two-Stage Build Problem

Vorma needs information that only exists at different times:

**Before Vite runs:**

- Route patterns (to generate TypeScript types)
- Entry points (to configure Vite's rollup input)

**After Vite runs:**

- Hashed output filenames
- Dependency graph for preloading
- CSS bundle mappings

This creates the "stage 1" and "stage 2" `PathsFile` structure. Stage 1 has
source paths; stage 2 has output paths. This is not over-engineering--it's the
minimum information needed at each phase.

### Why HTTP Endpoints for Fast Rebuilds

The subprocess build hook takes ~1.5 seconds due to `go run` startup overhead.

But many changes don't affect Go types:

- `vorma.routes.ts` changes --> route patterns changed, but Go loaders are the
  same
- HTML template changes --> just re-parse the file

For these, the running app process (Process B) can reload from disk without a
full restart. The HTTP endpoints let Wave coordinate this:

| Change Type       | Rebuild Path              | Time  |
| ----------------- | ------------------------- | ----- |
| Go code           | Subprocess (full rebuild) | ~1.5s |
| Route definitions | Callback + HTTP reload    | ~50ms |
| HTML template     | HTTP reload               | ~10ms |

This is a 30x improvement for the most common development changes.

### The Strategy System

Wave is framework-agnostic. Vorma needs framework-specific rebuild behavior. The
"Strategy" system bridges this.

**Callbacks and Strategies work together.** They are not mutually exclusive:

```go
OnChangeHooks: []waveconfig.OnChangeHook{{
    // Callback runs first in Process A (dev server)
    Callback: func(string) error {
        return rebuildRoutesOnly(v)  // Parse routes, generate TS, write JSON
    },
    // Strategy runs after callback succeeds
    Strategy: &waveconfig.OnChangeStrategy{
        HttpEndpoint:   "/__vorma/reload-routes",  // Tell Process B to reload
        WaitForApp:     true,
        WaitForVite:    true,
        ReloadBrowser:  true,
        FallbackAction: waveconfig.FallbackRestartNoGo,
    },
}},
```

This tells Wave:

1. Run the Callback first (Process A generates artifacts)
2. Then call the HttpEndpoint (Process B reloads from disk)
3. If the endpoint fails, fall back to restart without Go recompilation

Without this system, either:

- Wave would need Vorma-specific knowledge (breaks separation)
- Every route change would trigger full rebuild (30x slower)

### The Fast Rebuild Flow in Detail

When `vorma.routes.ts` changes:

```
┌─────────────────────────────────────────────────────────────────┐
│ Process A (Dev Server)                                          │
│                                                                 │
│  1. File watcher detects vorma.routes.ts changed                │
│  2. Callback runs: rebuildRoutesOnly()                          │
│     a. Parse routes with esbuild (Process A has esbuild)        │
│     b. Generate TypeScript using handler reflection             │
│        (Process A has same handlers as B - Go unchanged)        │
│     c. Write paths JSON to disk                                 │
│     d. Write route manifest to disk                             │
│  3. Strategy executes: HTTP POST to /__vorma/reload-routes      │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Process B (App Server)                                          │
│                                                                 │
│  4. Receives HTTP request at /__vorma/reload-routes             │
│  5. ReloadRoutesFromDisk():                                     │
│     a. Read paths JSON from disk                                │
│     b. Update in-memory route state                             │
│     c. Rebuild router with new patterns                         │
│  6. Returns 200 OK                                              │
└─────────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────────┐
│ Process A (Dev Server)                                          │
│                                                                 │
│  7. Receives 200 OK from Process B                              │
│  8. Broadcasts reload to browser via WebSocket                  │
└─────────────────────────────────────────────────────────────────┘
```

**Why Process A can generate TypeScript:**

Process A runs the same `cmd/build/main.go` that initializes the Vorma app with
all loaders and actions registered. When Go code hasn't changed, Process A's
handler types are identical to Process B's. The `loader.O()` and `action.I()`
reflection calls return the same types.

---

## What "Simplification" Would Actually Cost

### "Just use Vite for everything"

**Lost:** Zero-Node development. HTMX apps, Go template sites, and anyone who
doesn't want Node.js installed.

### "Delete the WebSocket server, use Vite HMR"

**Lost:** Reload capability when Vite isn't running. Also, no way to signal "Go
just rebuilt" through Vite's HMR channel without additional glue code.

### "Move TypeScript parsing to Node"

**Lost:** Nothing significant for Vorma (it requires Node anyway). But adds
coordination complexity: Node script --> JSON file --> Go reads --> generates
config. Current flow is simpler.

### "Just restart Go on every change, it's fast enough"

**Lost:** 30x iteration speed for route/template changes. "Fast enough" is 1.5
seconds. Current fast path is 50ms. This difference compounds over a development
session.

### "Merge Wave and Vorma"

**Lost:** Wave's utility as a standalone tool. Future frameworks built on Wave.
The ability to use Wave's asset pipeline without Vorma's opinions.

### "Put generation code in runtime"

**Lost:** Several megabytes savings on production binary size. The build package
imports esbuild and a JavaScript parser--neither is needed at runtime.

---

## Summary of Constraints

| Constraint                          | Consequence                                                  |
| ----------------------------------- | ------------------------------------------------------------ |
| Go requires recompilation           | Two-process architecture; subprocess for builds              |
| Type reflection needs fresh code    | Build hook must be subprocess, not in-process call           |
| Wave should work without Node       | Custom CSS/static handling via esbuild Go API                |
| Vite can't know about Go events     | Separate WebSocket for Go-level reload signals               |
| Route changes don't affect Go types | Callback + HTTP endpoint fast path to avoid subprocess       |
| TypeScript needs Go type info       | Two-stage build (pre-Vite and post-Vite)                     |
| Wave should be framework-agnostic   | Strategy system for framework-specific behavior              |
| Production binary size matters      | Separate build/runtime packages; runtime never imports build |
| Fast rebuilds need type reflection  | Process A runs same init code, has handlers for reflection   |
