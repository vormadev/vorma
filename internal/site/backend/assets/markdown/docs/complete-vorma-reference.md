# Vorma Framework -- Complete API Reference

**DISCLAIMER:** This was LLM-generated and may have some hallucinations.

## Introduction

### What is Vorma?

Vorma is a full-stack framework combining Go (server) and TypeScript (client)
with end-to-end type safety and Remix-style nested routing. You define routes,
loaders, and actions in Go, and they automatically generate TypeScript types for
the frontend.

**Core concepts:**

- **Loaders**: Server functions that fetch data to feed into nested UI
  components (GET requests)
- **Actions**: Server functions that handle mutations (POST/PUT/DELETE/etc)
- **Routes**: Client components that receive loader data as props
- **Type safety**: Go types automatically become TypeScript types

**Supported UI libraries**: React, Preact, Solid (nearly identical APIs)

### Architecture Overview

**Server (Go):**

- Define loaders and actions with typed inputs/outputs
- Handle HTTP requests, database queries, business logic
- Automatic TypeScript type generation for full-stack type safety

**Client (TypeScript):**

- Route components receive typed data from loaders
- Type-safe API client for calling actions
- Navigation with prefetching and client-side loaders

---

## Project Structure

### Required Directory Layout

```
project/
├── .gitignore
├── backend/
│   ├── assets/
│   │   └── entry.go.html
│   ├── cmd/
│   │   ├── build/
│   │   │   └── main.go
│   │   └── serve/
│   │       └── main.go
│   ├── dist/
│   │   ├── main                     # Compiled binary (gitignored)
│   │   └── static/
│   │       ├── .keep
│   │       └── internal/            # Build artifacts (gitignored)
│   ├── src/
│   │   └── router/
│   │       └── router.go
│   ├── wave.config.json
│   └── wave.go
├── frontend/
│   ├── assets/                      # Public static files
│   └── src/
│       ├── components/
│       ├── styles/
│       │   ├── main.critical.css
│       │   └── main.css
│       ├── vorma.api.ts
│       ├── vorma.entry.tsx
│       ├── vorma.gen/               # Generated (gitignored)
│       │   └── index.ts
│       ├── vorma.routes.ts
│       └── vorma.utils.tsx
├── go.mod
├── go.sum
├── package.json
├── tsconfig.json
└── vite.config.ts
```

### Required Files - Complete Code

**.gitignore**

```
# Node
**/node_modules/

# Wave
backend/dist/static/*
!backend/dist/static/.keep
backend/dist/main*
```

**backend/dist/static/.keep**

```
//go:embed directives require at least one file to compile
```

**backend/wave.go**

```go
package backend

import (
    "embed"
    "github.com/vormadev/vorma/kit/fsutil"
    "github.com/vormadev/vorma/wave"
)

//go:embed all:dist/static wave.config.json
var embedFS embed.FS

var Wave = wave.New(wave.Config{
    WaveConfigJSON: fsutil.MustReadFile(embedFS, "wave.config.json"),
    DistStaticFS:   fsutil.MustSub(embedFS, "dist", "static"),
})
```

**backend/wave.config.json**

```json
{
	"$schema": "dist/static/internal/schema.json",
	"Core": {
		"ConfigLocation": "backend/wave.config.json",
		"DevBuildHook": "go run ./backend/cmd/build --dev --hook",
		"ProdBuildHook": "go run ./backend/cmd/build --hook",
		"MainAppEntry": "backend/cmd/serve",
		"DistDir": "backend/dist",
		"StaticAssetDirs": {
			"Private": "backend/assets",
			"Public": "frontend/assets"
		},
		"CSSEntryFiles": {
			"Critical": "frontend/src/styles/main.critical.css",
			"NonCritical": "frontend/src/styles/main.css"
		},
		"PublicPathPrefix": "/"
	},
	"Vorma": {
		"UIVariant": "react",
		"HTMLTemplateLocation": "entry.go.html",
		"ClientEntry": "frontend/src/vorma.entry.tsx",
		"ClientRouteDefsFile": "frontend/src/vorma.routes.ts",
		"TSGenOutDir": "frontend/src/vorma.gen/index.ts",
		"BuildtimePublicURLFuncName": "waveBuildtimeURL"
	},
	"Vite": {
		"JSPackageManagerBaseCmd": "npx"
	},
	"Watch": {
		"HealthcheckEndpoint": "/healthz",
		"Include": []
	}
}
```

**backend/assets/entry.go.html**

```html
<!doctype html>
<html lang="en">
	<head>
		<meta charset="utf-8" />
		<meta name="viewport" content="width=device-width, initial-scale=1" />
		{{.VormaHeadEls}} {{.VormaSSRScript}}
	</head>
	<body>
		<div id="{{.VormaRootID}}"></div>
		{{.VormaBodyScripts}}
	</body>
</html>
```

**backend/cmd/serve/main.go**

```go
package main

import (
    "yourmodule/backend/src/router"
    "fmt"
    "net/http"
)

func main() {
    addr, handler := router.Init()
    fmt.Printf("Server starting at http://localhost%s\n", addr)
    http.ListenAndServe(addr, handler)
}
```

**backend/cmd/build/main.go**

```go
package main

import "yourmodule/backend/src/router"

func main() {
    router.App.Build()
}
```

**backend/src/router/router.go** (minimal template)

```go
package router

import (
    "yourmodule/backend"
    "net/http"

    "github.com/vormadev/vorma"
    "github.com/vormadev/vorma/kit/middleware/healthcheck"
)

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
    Wave: backend.Wave,
})

func Init() (addr string, handler http.Handler) {
    r := App.InitWithDefaultRouter()
    r.SetGlobalHTTPMiddleware(App.ServeStatic())
    r.SetGlobalHTTPMiddleware(healthcheck.Healthz)
    return App.ServerAddr(), r
}

func NewLoader[O any](
    pattern string, loader vorma.LoaderFunc[LoaderCtx, O],
) *vorma.Loader[O] {
    return vorma.NewLoader(App, pattern, loader, decorateLoaderCtx)
}

func NewAction[I any, O any](
    method string, pattern string, action vorma.ActionFunc[ActionCtx[I], I, O],
) *vorma.Action[I, O] {
    return vorma.NewAction(App, method, pattern, action, decorateActionCtx)
}

type LoaderCtx struct{ *vorma.LoaderReqData }
type ActionCtx[I any] struct{ *vorma.ActionReqData[I] }

func decorateLoaderCtx(rd *vorma.LoaderReqData) *LoaderCtx {
    return &LoaderCtx{LoaderReqData: rd}
}
func decorateActionCtx[I any](rd *vorma.ActionReqData[I]) *ActionCtx[I] {
    return &ActionCtx[I]{ActionReqData: rd}
}
```

**vite.config.ts**

```typescript
import vorma from "vorma/vite";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react-swc"; // or preact/solid plugin
import { vormaViteConfig } from "./frontend/src/vorma.gen/index.ts";

export default defineConfig({
	plugins: [react(), vorma(vormaViteConfig)],
});
```

**frontend/src/vorma.entry.tsx** (React)

```typescript
import { createRoot } from "react-dom/client";
import { getRootEl, initClient } from "vorma/client";
import { VormaRootOutlet } from "vorma/react";
import { vormaAppConfig } from "./vorma.gen/index.ts";

await initClient({
    vormaAppConfig,
    renderFn: () => {
        createRoot(getRootEl()).render(<VormaRootOutlet />);
    },
});
```

**frontend/src/vorma.entry.tsx** (Preact)

```typescript
import { render } from "preact";
import { getRootEl, initClient } from "vorma/client";
import { VormaRootOutlet } from "vorma/preact";
import { vormaAppConfig } from "./vorma.gen/index.ts";

await initClient({
    vormaAppConfig,
    renderFn: () => {
        render(<VormaRootOutlet />, getRootEl());
    },
});
```

**frontend/src/vorma.entry.tsx** (Solid)

```typescript
import { getRootEl, initClient } from "vorma/client";
import { VormaRootOutlet } from "vorma/solid";
import { render } from "solid-js/web";
import { vormaAppConfig } from "./vorma.gen/index.ts";

await initClient({
    vormaAppConfig,
    renderFn: () => {
        render(() => <VormaRootOutlet />, getRootEl());
    },
});
```

**frontend/src/vorma.routes.ts**

```typescript
import { route } from "vorma/client";

route("/", import("./components/root.tsx"), "Root");
route("/_index", import("./components/home.tsx"), "Home");
```

**frontend/src/vorma.utils.tsx** (React example)

```typescript
import { makeTypedNavigate } from "vorma/client";
import {
	makeTypedAddClientLoader,
	makeTypedLink,
	makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData,
	makeTypedUseRouterData,
} from "vorma/react";
import { vormaAppConfig, type VormaApp } from "./vorma.gen/index.ts";

export const Link = makeTypedLink(vormaAppConfig, { prefetch: "intent" });
export const navigate = makeTypedNavigate(vormaAppConfig);
export const useRouterData = makeTypedUseRouterData<VormaApp>();
export const useLoaderData = makeTypedUseLoaderData<VormaApp>();
export const usePatternLoaderData = makeTypedUsePatternLoaderData<VormaApp>();
export const addClientLoader = makeTypedAddClientLoader<VormaApp>();
```

**frontend/src/vorma.api.ts**

```typescript
import {
	buildMutationURL,
	buildQueryURL,
	resolveBody,
	submit,
} from "vorma/client";
import {
	vormaAppConfig,
	type MutationOutput,
	type MutationPattern,
	type MutationProps,
	type QueryOutput,
	type QueryPattern,
	type QueryProps,
} from "./vorma.gen/index.ts";

export const api = { query, mutate };

async function query<P extends QueryPattern>(props: QueryProps<P>) {
	return await submit<QueryOutput<P>>(
		buildQueryURL(vormaAppConfig, props),
		{ method: "GET", ...props.requestInit },
		props.options,
	);
}

async function mutate<P extends MutationPattern>(props: MutationProps<P>) {
	return await submit<MutationOutput<P>>(
		buildMutationURL(vormaAppConfig, props),
		{
			method: "POST",
			...props.requestInit,
			body: resolveBody(props),
		},
		props.options,
	);
}
```

**package.json**

```json
{
	"type": "module",
	"private": true,
	"scripts": {
		"dev": "go run ./backend/cmd/build --dev",
		"build": "go run ./backend/cmd/build"
	},
	"devDependencies": {
		"typescript": "",
		"vite": "",
		"vorma": "",
		"react": "",
		"react-dom": "",
		"@types/react": "",
		"@types/react-dom": "",
		"@vitejs/plugin-react-swc": ""
	}
}
```

**tsconfig.json**

```json
{
	"compilerOptions": {
		"target": "ES2022",
		"module": "ESNext",
		"moduleResolution": "Bundler",
		"jsx": "react-jsx",
		"jsxImportSource": "react",
		"strict": true,
		"skipLibCheck": true,
		"noEmit": true,
		"esModuleInterop": true,
		"allowImportingTsExtensions": true,
		"verbatimModuleSyntax": true,
		"noUncheckedIndexedAccess": true
	},
	"exclude": ["node_modules"]
}
```

---

## 3. Server API - Complete Reference

### 3.1 Vorma App Creation

```go
import "github.com/vormadev/vorma"

func NewVormaApp(config VormaAppConfig) *Vorma

type VormaAppConfig struct {
    Wave *wave.Wave  // REQUIRED: Wave instance from backend.Wave

    // Optional: Custom head element rules
    GetDefaultHeadEls    func(r *http.Request, app *Vorma, h *headels.HeadEls) error
    GetHeadElUniqueRules func(h *headels.HeadEls)

    // Optional: Custom template data
    GetRootTemplateData func(r *http.Request) (map[string]any, error)

    // Optional: Router configuration
    LoadersRouterOptions LoadersRouterOptions
    ActionsRouterOptions ActionsRouterOptions

    // Optional: TypeScript generation
    AdHocTypes  []*AdHocType
    ExtraTSCode string
}

type LoadersRouterOptions struct {
    DynamicParamPrefix     rune   // default: ':'
    SplatSegmentIdentifier rune   // default: '*'
    IndexSegmentIdentifier string // default: "_index"
}

type ActionsRouterOptions struct {
    DynamicParamPrefix     rune     // default: ':'
    SplatSegmentIdentifier rune     // default: '*'
    MountRoot              string   // default: "/api/"
    SupportedMethods       []string // default: ["GET","POST","PUT","DELETE","PATCH"]
}
```

### 3.2 Vorma Methods

```go
// Initialize with default mux.Router
func (v *Vorma) InitWithDefaultRouter() *mux.Router

// Initialize without router (manual setup)
func (v *Vorma) Init()

// Get server address (e.g., ":8080")
func (v *Vorma) ServerAddr() string

// Build the application (dev or prod)
func (v *Vorma) Build()

// Serve static files middleware
func (v *Vorma) ServeStatic() func(http.Handler) http.Handler

// Get current build ID
func (v *Vorma) GetCurrentBuildID() string

// Check if request is JSON with current build
func (v *Vorma) IsCurrentBuildJSONRequest(r *http.Request) bool

// Get public URL for an asset
func (v *Vorma) GetPublicURL(assetPath string) string

// Access routers
func (v *Vorma) Loaders() *Loaders
func (v *Vorma) Actions() *Actions

// Router access
func (v *Vorma) LoadersRouter() *LoadersRouter
func (v *Vorma) ActionsRouter() *ActionsRouter
```

### 3.3 Loaders (Data Fetching)

Loaders are functions that fetch data for routes. They run on the server for
both SSR and client navigations.

```go
// Create a loader
func NewLoader[O any](
    app *Vorma,
    pattern string,
    loader LoaderFunc[C, O],
    decorateCtx func(*LoaderReqData) *C,
) *Loader[O]

// Loader function signature
type LoaderFunc[C any, O any] func(ctx *C) (O, error)

// LoaderReqData - available in loader context
type LoaderReqData struct {
    // Access methods
    Params() map[string]string
    Param(key string) string
    SplatValues() []string
    TasksCtx() *tasks.Ctx
    Request() *http.Request
    ResponseProxy() *response.Proxy

    // Response manipulation
    HeadEls() *headels.HeadEls
    Redirect(url string, code ...int)
    SetResponseStatus(status int, errorText ...string)
    SetResponseCookie(cookie *http.Cookie)
    SetResponseHeader(key, value string)
    AddResponseHeader(key, value string)
    GetResponseStatus() (int, string)
    GetResponseHeader(key string) string
    GetResponseHeaders(key string) []string
    GetResponseCookies() []*http.Cookie
    GetResponseLocation() string
    IsResponseError() bool
    IsResponseRedirect() bool
    IsResponseSuccess() bool
}
```

**Example loaders:**

```go
// Root layout loader (runs for ALL routes)
var _ = NewLoader("/", func(c *LoaderCtx) (*RootData, error) {
    return &RootData{Message: "Hello"}, nil
})

// Loader with params
var _ = NewLoader("/users/:id", func(c *LoaderCtx) (*User, error) {
    id := c.Param("id")
    user, err := db.GetUser(id)
    if err != nil {
        return nil, err
    }
    return user, nil
})

// Loader with head elements
var _ = NewLoader("/about", func(c *LoaderCtx) (*AboutData, error) {
    head := c.HeadEls()
    head.Title("About Us")
    head.Description("Learn about our company")
    return &AboutData{}, nil
})

// Loader with redirect
var _ = NewLoader("/old-path", func(c *LoaderCtx) (vorma.None, error) {
    c.Redirect("/new-path")
    return vorma.None{}, nil
})

// Loader returning error (shows error boundary on client)
var _ = NewLoader("/protected", func(c *LoaderCtx) (*Data, error) {
    if !isAuthenticated(c.Request()) {
        return nil, &vorma.LoaderError{
            Client: "You must be logged in",
            Server: fmt.Errorf("unauthorized access attempt"),
        }
    }
    return &Data{}, nil
})
```

**Loader error handling:**

```go
type LoaderError struct {
    Client string  // Message sent to client
    Server error   // Logged server-side only
}

func (e *LoaderError) Error() string
func (e *LoaderError) ClientMessage() string
func (e *LoaderError) ServerError() error
```

### 3.4 Actions (Mutations)

Actions handle form submissions and API calls. They support typed inputs and
outputs.

```go
// Create an action
func NewAction[I any, O any](
    app *Vorma,
    method string,      // "POST", "PUT", "DELETE", "PATCH"
    pattern string,
    action ActionFunc[C, I, O],
    decorateCtx func(*ActionReqData[I]) *C,
) *Action[I, O]

// Action function signature
type ActionFunc[C any, I any, O any] func(ctx *C) (O, error)

// ActionReqData - available in action context
type ActionReqData[I any] struct {
    // Same as LoaderReqData, plus:
    Input() I  // Parsed and validated input
}
```

**Example actions:**

```go
// Simple POST action
type CreateUserInput struct {
    Name  string `json:"name" validate:"required"`
    Email string `json:"email" validate:"required,email"`
}

type CreateUserOutput struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

var _ = NewAction("POST", "/users", func(c *ActionCtx[CreateUserInput]) (*CreateUserOutput, error) {
    input := c.Input()
    user, err := db.CreateUser(input.Name, input.Email)
    if err != nil {
        return nil, err
    }
    return &CreateUserOutput{ID: user.ID, Name: user.Name}, nil
})

// Action with params
var _ = NewAction("PUT", "/users/:id", func(c *ActionCtx[UpdateUserInput]) (*User, error) {
    id := c.Param("id")
    input := c.Input()
    return db.UpdateUser(id, input)
})

// Action with no input
var _ = NewAction("POST", "/logout", func(c *ActionCtx[vorma.None]) (vorma.None, error) {
    c.SetResponseCookie(&http.Cookie{
        Name:   "session",
        Value:  "",
        MaxAge: -1,
    })
    return vorma.None{}, nil
})

// Query action (GET with input from query params)
type SearchInput struct {
    Q     string `form:"q"`
    Limit int    `form:"limit"`
}

var _ = NewAction("GET", "/search", func(c *ActionCtx[SearchInput]) (*SearchResults, error) {
    input := c.Input()
    return performSearch(input.Q, input.Limit)
})
```

**Special input type for FormData:**

```go
type FormData struct{}  // Use this type when input is FormData

var _ = NewAction("POST", "/upload", func(c *ActionCtx[vorma.FormData]) (string, error) {
    r := c.Request()
    file, header, err := r.FormFile("file")
    if err != nil {
        return "", err
    }
    defer file.Close()
    // Process file...
    return "File uploaded successfully", nil
})
```

### 3.5 Route Patterns

**Static routes:**

```go
NewLoader("/about", ...)      // Matches /about exactly
NewLoader("/users", ...)      // Matches /users exactly
```

**Dynamic parameters:**

```go
NewLoader("/users/:id", ...)           // Matches /users/123
NewLoader("/posts/:slug", ...)         // Matches /posts/hello-world
NewLoader("/users/:id/posts/:postId", ...) // Multiple params
```

**Splat/catch-all:**

```go
NewLoader("/files/*", ...)    // Matches /files/a, /files/a/b, /files/a/b/c
// Access via c.SplatValues() returns ["a", "b", "c"]
```

**Index routes:**

```go
NewLoader("/", ...)           // Root layout (runs for ALL routes)
NewLoader("/_index", ...)     // Root index (matches / exactly)
NewLoader("/blog/_index", ...) // Index for /blog (matches /blog exactly)
```

**Nested layouts:**

```go
// All these run for /blog/my-post:
NewLoader("/", ...)             // Root layout (runs for ALL routes)
NewLoader("/blog", ...)         // Blog layout
NewLoader("/blog/:slug", ...)   // Post page
```

### 3.6 Head Elements API

```go
import "github.com/vormadev/vorma/kit/headels"

// Get head elements in loader
head := c.HeadEls()

// Title and description
head.Title("Page Title")
head.Description("Page description")

// Meta tags
head.Meta(head.Name("author"), head.Content("John Doe"))
head.Meta(head.Property("og:title"), head.Content("Title"))
head.MetaNameContent("viewport", "width=device-width")
head.MetaPropertyContent("og:image", "https://example.com/image.jpg")

// Links
head.Link(head.Rel("icon"), head.Href("/favicon.ico"))
head.Link(
    head.Rel("stylesheet"),
    head.Href("/custom.css"),
    head.Attr("media", "print"),
)

// Scripts
head.Script(
    head.Src("https://example.com/script.js"),
    head.BoolAttr("async"),
)
head.Script(head.TextContent("console.log('inline script')"))

// Custom elements
head.Add(
    headels.Tag("link"),
    head.Rel("canonical"),
    head.Href("https://example.com/page"),
)

// Dangerous innerHTML
head.Style(head.DangerousInnerHTML(".custom { color: red; }"))

// Self-closing
head.Add(
    headels.Tag("meta"),
    head.Name("custom"),
    head.Content("value"),
    head.SelfClosing(),
)

// Known-safe attributes (pre-escaped, use carefully)
head.Add(
    headels.Tag("meta"),
    head.Attr("data-custom", value).KnownSafe(),
)
```

**Head element deduplication:** Configure unique rules in VormaAppConfig:

```go
GetHeadElUniqueRules: func(h *headels.HeadEls) {
    h.Add(headels.Tag("title"))              // Only one title
    h.Meta(h.Name("description"))            // One description
    h.Meta(h.Property("og:title"))           // One OG title
    h.Meta(h.Property("og:description"))     // One OG description
}
```

**Default head elements:**

```go
GetDefaultHeadEls: func(r *http.Request, app *Vorma, h *headels.HeadEls) error {
    h.Title("Default Title")
    h.Link(
        h.Rel("icon"),
        h.Href(app.GetPublicURL("favicon.svg")),
        h.Type("image/svg+xml"),
    )
    return nil
}
```

### 3.7 Response Manipulation

**In loaders/actions via context:**

```go
// Redirects
c.Redirect("/new-url")                    // 303 by default
c.Redirect("/new-url", http.StatusFound)  // Custom status

// Status codes
c.SetResponseStatus(404, "Not found")
c.SetResponseStatus(500)  // Uses default text

// Headers
c.SetResponseHeader("Cache-Control", "max-age=3600")
c.AddResponseHeader("Set-Cookie", "key=value")

// Cookies
c.SetResponseCookie(&http.Cookie{
    Name:     "session",
    Value:    "abc123",
    MaxAge:   3600,
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteStrictMode,
})

// Query response state
status, text := c.GetResponseStatus()
header := c.GetResponseHeader("Content-Type")
headers := c.GetResponseHeaders("Set-Cookie")
cookies := c.GetResponseCookies()
location := c.GetResponseLocation()

// Checks
if c.IsResponseError() { /* ... */ }
if c.IsResponseRedirect() { /* ... */ }
if c.IsResponseSuccess() { /* ... */ }
```

### 3.8 TypeScript Generation

**Ad-hoc types:** Export custom Go types to TypeScript:

```go
type CustomType struct {
    Field string `json:"field"`
}

App = vorma.NewVormaApp(vorma.VormaAppConfig{
    Wave: backend.Wave,
    AdHocTypes: []*vorma.AdHocType{
        {TypeInstance: CustomType{}},
    },
})
```

**Extra TypeScript code:**

```go
App = vorma.NewVormaApp(vorma.VormaAppConfig{
    Wave: backend.Wave,
    ExtraTSCode: `
export type MyCustomType = {
    field: string;
};

export const MY_CONSTANT = "value";
    `,
})
```

### 3.9 Wave API (Static Files & Build)

```go
import "github.com/vormadev/vorma/wave"

// Wave instance is created in backend/wave.go
var Wave = wave.New(wave.Config{
    WaveConfigJSON: fsutil.MustReadFile(embedFS, "wave.config.json"),
    DistStaticFS:   fsutil.MustSub(embedFS, "dist", "static"),
})

// Get public URL for asset (content-hashed in production)
func (v *Vorma) GetPublicURL(assetPath string) string

// Example usage in loader:
iconURL := app.GetPublicURL("favicon.svg")
// Returns: "/favicon.svg" (dev) or "/vorma_out_favicon_abc123.svg" (prod)
```

**Available in templates:**

```html
<link rel="icon" href="{{ .App.GetPublicURL "favicon.svg" }}">
<img src="{{ .App.GetPublicURL "hero.png" }}">
```

**Available in TypeScript (build-time only):**

```typescript
// In any TypeScript file:
const logoUrl = waveBuildtimeURL("logo.svg");
// Replaced at build time with actual hashed URL
```

### 3.10 Middleware

Vorma uses the standard `kit/mux` router. See mux documentation for complete
middleware API.

**Basic middleware:**

```go
r := App.InitWithDefaultRouter()

// Serve static files (required)
r.SetGlobalHTTPMiddleware(App.ServeStatic())

// Health check
import "github.com/vormadev/vorma/kit/middleware/healthcheck"
r.SetGlobalHTTPMiddleware(healthcheck.Healthz)

// Custom middleware
r.SetGlobalHTTPMiddleware(func(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Before handler
        log.Println("Request:", r.URL.Path)

        next.ServeHTTP(w, r)

        // After handler
        log.Println("Response sent")
    })
})
```

---

## 4. Client API - Complete Reference

### 4.1 Route Definitions

**frontend/src/vorma.routes.ts:**

```typescript
import { route } from "vorma/client";

// Basic route
route("/", import("./components/root.tsx"), "Root");

// With error boundary
route("/", import("./components/root.tsx"), "Root", "RootError");

// Pattern examples:
route("/", import("./root.tsx"), "Root"); // Root layout
route("/_index", import("./home.tsx"), "Home"); // Index
route("/about", import("./about.tsx"), "About"); // Static
route("/users/:id", import("./user.tsx"), "User"); // Dynamic
route("/files/*", import("./files.tsx"), "Files"); // Splat
route("/blog", import("./blog-layout.tsx"), "BlogLayout"); // Layout
route("/blog/:slug", import("./post.tsx"), "Post"); // Nested
```

**Export names:**

- Component export name must match the third argument
- Error boundary export name must match the fourth argument (if provided)

**Example component file:**

```typescript
// components/user.tsx
import type { RouteProps } from "../vorma.gen/index.ts";
import { useLoaderData } from "../vorma.utils.tsx";

export function User(props: RouteProps<"/users/:id">) {
    const user = useLoaderData(props);
    return <div>{user.name}</div>;
}

export function UserError(props: { error: string }) {
    return <div>Error: {props.error}</div>;
}
```

### 4.2 Client Initialization

**React:**

```typescript
import { createRoot } from "react-dom/client";
import { getRootEl, initClient } from "vorma/client";
import { VormaRootOutlet } from "vorma/react";
import { vormaAppConfig } from "./vorma.gen/index.ts";

await initClient({
    vormaAppConfig,
    renderFn: () => {
        createRoot(getRootEl()).render(<VormaRootOutlet />);
    },
    // Optional:
    defaultErrorBoundary: CustomErrorBoundary,
    useViewTransitions: true,
});
```

**Preact:**

```typescript
import { render } from "preact";
import { getRootEl, initClient } from "vorma/client";
import { VormaRootOutlet } from "vorma/preact";
import { vormaAppConfig } from "./vorma.gen/index.ts";

await initClient({
    vormaAppConfig,
    renderFn: () => {
        render(<VormaRootOutlet />, getRootEl());
    },
});
```

**Solid:**

```typescript
import { getRootEl, initClient } from "vorma/client";
import { VormaRootOutlet } from "vorma/solid";
import { render } from "solid-js/web";
import { vormaAppConfig } from "./vorma.gen/index.ts";

await initClient({
    vormaAppConfig,
    renderFn: () => {
        render(() => <VormaRootOutlet />, getRootEl());
    },
});
```

### 4.3 Route Components

**React:**

```typescript
import type { RouteProps } from "../vorma.gen/index.ts";
import { useLoaderData, useRouterData } from "../vorma.utils.tsx";

export function MyRoute(props: RouteProps<"/my-pattern">) {
    const data = useLoaderData(props);
    const router = useRouterData();

    return (
        <div>
            <h1>{data.title}</h1>
            <props.Outlet />
        </div>
    );
}
```

**Preact:**

```typescript
import type { RouteProps } from "../vorma.gen/index.ts";
import { useLoaderData, useRouterData } from "../vorma.utils.tsx";

export function MyRoute(props: RouteProps<"/my-pattern">) {
    const data = useLoaderData(props);
    const router = useRouterData();

    return (
        <div>
            <h1>{data.value.title}</h1>
            <props.Outlet />
        </div>
    );
}
```

**Solid:**

```typescript
import type { RouteProps } from "../vorma.gen/index.ts";
import { useLoaderData, useRouterData } from "../vorma.utils.tsx";

export function MyRoute(props: RouteProps<"/my-pattern">) {
    const data = useLoaderData(props);
    const router = useRouterData();

    return (
        <div>
            <h1>{data().title}</h1>
            <props.Outlet />
        </div>
    );
}
```

### 4.4 Hooks/Signals

**React:**

```typescript
import {
	makeTypedUseRouterData,
	makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData,
	makeTypedAddClientLoader,
} from "vorma/react";
import { vormaAppConfig, type VormaApp } from "./vorma.gen/index.ts";

export const useRouterData = makeTypedUseRouterData<VormaApp>();
export const useLoaderData = makeTypedUseLoaderData<VormaApp>();
export const usePatternLoaderData = makeTypedUsePatternLoaderData<VormaApp>();
export const addClientLoader = makeTypedAddClientLoader<VormaApp>();

// Usage:
function MyComponent(props: RouteProps<"/pattern">) {
	const data = useLoaderData(props); // Type: loader output for /pattern
	const router = useRouterData(); // Type: { buildID, matchedPatterns, params, rootData }

	// Access specific pattern's data
	const otherData = usePatternLoaderData("/other-pattern");
}
```

**Preact:**

```typescript
import {
    makeTypedUseRouterData,
    makeTypedUseLoaderData,
    makeTypedUsePatternLoaderData,
    makeTypedAddClientLoader,
} from "vorma/preact";
import { vormaAppConfig, type VormaApp } from "./vorma.gen/index.ts";

export const useRouterData = makeTypedUseRouterData<VormaApp>();
export const useLoaderData = makeTypedUseLoaderData<VormaApp>();
export const usePatternLoaderData = makeTypedUsePatternLoaderData<VormaApp>();
export const addClientLoader = makeTypedAddClientLoader<VormaApp>();

// Usage: same as React but returns signals
function MyComponent(props: RouteProps<"/pattern">) {
    const data = useLoaderData(props);  // Type: Signal<loader output>
    const router = useRouterData();     // Type: Signal<router data>

    return <div>{data.value.title}</div>;
}
```

**Solid:**

```typescript
import {
    makeTypedUseRouterData,
    makeTypedUseLoaderData,
    makeTypedUsePatternLoaderData,
    makeTypedAddClientLoader,
} from "vorma/solid";
import { vormaAppConfig, type VormaApp } from "./vorma.gen/index.ts";

export const useRouterData = makeTypedUseRouterData<VormaApp>();
export const useLoaderData = makeTypedUseLoaderData<VormaApp>();
export const usePatternLoaderData = makeTypedUsePatternLoaderData<VormaApp>();
export const addClientLoader = makeTypedAddClientLoader<VormaApp>();

// Usage: returns accessors
function MyComponent(props: RouteProps<"/pattern">) {
    const data = useLoaderData(props);  // Type: Accessor<loader output>
    const router = useRouterData();     // Type: Accessor<router data>

    return <div>{data().title}</div>;
}
```

**Router data shape:**

```typescript
type RouterData<RootData, Params> = {
	buildID: string;
	matchedPatterns: string[];
	splatValues: string[];
	params: Params; // e.g., { id: string } for /users/:id
	rootData: RootData; // Data from root loader ("/")
};
```

### 4.5 Navigation

**Link component:**

```typescript
import { makeTypedLink } from "vorma/react"; // or preact/solid
import { vormaAppConfig } from "./vorma.gen/index.ts";

export const Link = makeTypedLink(vormaAppConfig, {
    prefetch: "intent",  // Optional default
});

// Usage:
<Link pattern="/">Home</Link>
<Link pattern="/users/:id" params={{ id: "123" }}>User 123</Link>
<Link pattern="/files/*" splatValues={["a", "b", "c"]}>Files</Link>
<Link pattern="/search" search="?q=hello">Search</Link>
<Link pattern="/about" hash="#section">About</Link>
<Link pattern="/admin" replace>Admin</Link>
<Link pattern="/page" scrollToTop={false}>Page</Link>
<Link pattern="/external" state={{ from: "home" }}>Link</Link>

// Prefetching:
<Link pattern="/profile" prefetch="intent">Profile</Link>
<Link pattern="/dashboard" prefetchDelayMs={500}>Dashboard</Link>

// Callbacks:
<Link
    pattern="/page"
    beforeBegin={(e) => console.log("before navigation starts")}
    beforeRender={(e) => console.log("before render")}
    afterRender={(e) => console.log("after render")}
>
    Navigate
</Link>
```

**Programmatic navigation:**

```typescript
import { makeTypedNavigate } from "vorma/client";
import { vormaAppConfig } from "./vorma.gen/index.ts";

export const navigate = makeTypedNavigate(vormaAppConfig);

// Usage:
await navigate({ pattern: "/" });
await navigate({
	pattern: "/users/:id",
	params: { id: "123" },
	replace: true,
	scrollToTop: false,
	search: "?tab=profile",
	hash: "#section",
	state: { from: "home" },
});
```

**Untyped navigation (for dynamic patterns):**

```typescript
import { vormaNavigate } from "vorma/client";

await vormaNavigate("/dynamic/path", {
	replace: true,
	scrollToTop: false,
	search: "?key=value",
	hash: "#section",
	state: { custom: "data" },
});
```

**Location access:**

```typescript
import { getLocation, useLocation, location } from "vorma/client";

// React:
import { useLocation } from "vorma/react";
function MyComponent() {
    const loc = useLocation();
    return <div>{loc.pathname}</div>;
}

// Preact:
import { location } from "vorma/preact";
function MyComponent() {
    return <div>{location.value.pathname}</div>;
}

// Solid:
import { location } from "vorma/solid";
function MyComponent() {
    return <div>{location().pathname}</div>;
}

// Anywhere (not reactive):
const loc = getLocation();
// { pathname: string, search: string, hash: string, state: unknown }
```

### 4.6 API Client (Actions/Queries)

**Setup** (frontend/src/vorma.api.ts):

```typescript
import {
	buildMutationURL,
	buildQueryURL,
	resolveBody,
	submit,
} from "vorma/client";
import {
	vormaAppConfig,
	type MutationOutput,
	type MutationPattern,
	type MutationProps,
	type QueryOutput,
	type QueryPattern,
	type QueryProps,
} from "./vorma.gen/index.ts";

export const api = { query, mutate };

async function query<P extends QueryPattern>(props: QueryProps<P>) {
	return await submit<QueryOutput<P>>(
		buildQueryURL(vormaAppConfig, props),
		{ method: "GET", ...props.requestInit },
		props.options,
	);
}

async function mutate<P extends MutationPattern>(props: MutationProps<P>) {
	return await submit<MutationOutput<P>>(
		buildMutationURL(vormaAppConfig, props),
		{
			method: "POST",
			...props.requestInit,
			body: resolveBody(props),
		},
		props.options,
	);
}
```

**Usage:**

```typescript
import { api } from "./vorma.api.ts";

// Query (GET)
const result = await api.query({
	pattern: "/search",
	input: { q: "hello", limit: 10 },
});
if (result.success) {
	console.log(result.data);
} else {
	console.error(result.error);
}

// Mutation (POST)
const result = await api.mutate({
	pattern: "/users",
	input: { name: "John", email: "john@example.com" },
});

// With params
const result = await api.mutate({
	pattern: "/users/:id",
	params: { id: "123" },
	input: { name: "Jane" },
});

// Custom method
const result = await api.mutate({
	pattern: "/users/:id",
	params: { id: "123" },
	input: { name: "Jane" },
	requestInit: { method: "PUT" },
});

// FormData
const formData = new FormData();
formData.append("file", file);

const result = await api.mutate({
	pattern: "/upload",
	input: formData,
});

// Options
const result = await api.mutate({
	pattern: "/action",
	input: {},
	options: {
		revalidate: false, // Don't auto-revalidate after mutation
		dedupeKey: "unique-key", // Deduplicate concurrent requests
		skipGlobalLoadingIndicator: true,
	},
});
```

**Direct submit (untyped):**

```typescript
import { submit } from "vorma/client";

const result = await submit<ResponseType>(
	"/api/endpoint",
	{
		method: "POST",
		body: JSON.stringify({ key: "value" }),
		headers: { "Content-Type": "application/json" },
	},
	{
		revalidate: false,
		dedupeKey: "key",
	},
);
```

### 4.7 Client Loaders

Client loaders run on the client before rendering, allowing you to fetch
additional data, add delays, or transform server data.

**React:**

```typescript
import { addClientLoader } from "./vorma.utils.tsx";

const useMyClientData = addClientLoader({
    pattern: "/my-pattern",
    clientLoader: async ({ params, splatValues, serverDataPromise, signal }) => {
        // Wait for server data
        const serverData = await serverDataPromise;

        // Fetch additional client-side data
        const response = await fetch(`/api/extra/${params.id}`, { signal });
        const extraData = await response.json();

        // Return combined data
        return {
            ...serverData.loaderData,
            extra: extraData,
        };
    },
    reRunOnModuleChange: import.meta,  // Dev only: re-run on HMR
});

// Usage in component:
function MyComponent(props: RouteProps<"/my-pattern">) {
    const clientData = useMyClientData(props);
    // OR: const clientData = useMyClientData(); // if called from within the route

    return <div>{clientData.extra}</div>;
}
```

**Preact:**

```typescript
const useMyClientData = addClientLoader({
    pattern: "/my-pattern",
    clientLoader: async ({ params, serverDataPromise, signal }) => {
        const serverData = await serverDataPromise;
        return { modified: true, ...serverData.loaderData };
    },
});

// Returns signal:
const clientData = useMyClientData(props);
return <div>{clientData.value.modified}</div>;
```

**Solid:**

```typescript
const useMyClientData = addClientLoader({
    pattern: "/my-pattern",
    clientLoader: async ({ params, serverDataPromise, signal }) => {
        const serverData = await serverDataPromise;
        return { modified: true, ...serverData.loaderData };
    },
});

// Returns accessor:
const clientData = useMyClientData(props);
return <div>{clientData().modified}</div>;
```

**Client loader arguments:**

```typescript
type ClientLoaderArgs = {
	params: Record<string, string>;
	splatValues: string[];
	serverDataPromise: Promise<{
		matchedPatterns: string[];
		loaderData: LoaderOutput | undefined;
		rootData: RootData | null;
		buildID: string;
	}>;
	signal: AbortSignal; // Aborted if navigation is cancelled
};
```

### 4.8 Revalidation

```typescript
import { revalidate } from "vorma/client";

// Manually revalidate current route
await revalidate();

// Auto-revalidate on window focus
import { revalidateOnWindowFocus } from "vorma/client";

const cleanup = revalidateOnWindowFocus({
	staleTimeMS: 5000, // Only revalidate if >5s since last revalidation
});

// Later: cleanup();
```

### 4.9 Loading States

**Global loading indicator:**

```typescript
import { setupGlobalLoadingIndicator } from "vorma/client";

setupGlobalLoadingIndicator({
	start: () => {
		document.getElementById("loader").style.display = "block";
	},
	stop: () => {
		document.getElementById("loader").style.display = "none";
	},
	isRunning: () => {
		return document.getElementById("loader").style.display === "block";
	},
	include: "all", // or ["navigations", "submissions", "revalidations"]
	startDelayMS: 100,
	stopDelayMS: 100,
});
```

**Status events:**

```typescript
import { addStatusListener, getStatus } from "vorma/client";

const cleanup = addStatusListener((event) => {
	const { isNavigating, isSubmitting, isRevalidating } = event.detail;
	console.log("Status:", { isNavigating, isSubmitting, isRevalidating });
});

const status = getStatus();
if (status.isNavigating) {
	// Show loading state
}
```

### 4.10 Events

```typescript
import {
	addRouteChangeListener,
	addLocationListener,
	addBuildIDListener,
	addStatusListener,
} from "vorma/client";

// Route change (after navigation completes)
const cleanup1 = addRouteChangeListener((event) => {
	console.log("Route changed");
});

// Location change (URL changed)
const cleanup2 = addLocationListener((event) => {
	console.log("Location changed");
});

// Build ID change (new deployment detected)
const cleanup3 = addBuildIDListener((event) => {
	const { oldID, newID } = event.detail;
	console.log("Build ID changed:", oldID, "->", newID);
});

// Status change (navigation/submission state)
const cleanup4 = addStatusListener((event) => {
	const { isNavigating, isSubmitting, isRevalidating } = event.detail;
});

// Cleanup when done:
cleanup1();
cleanup2();
cleanup3();
cleanup4();
```

### 4.11 History API

```typescript
import { getHistoryInstance } from "vorma/client";

const history = getHistoryInstance();

history.push("/path", { state: "value" });
history.replace("/path", { state: "value" });

history.go(-1); // Back
history.go(1); // Forward
history.back();
history.forward();

const location = history.location;
// { pathname, search, hash, state, key }

const unlisten = history.listen(({ action, location }) => {
	console.log("Navigation:", action, location);
});
```

### 4.12 Build ID

```typescript
import { getBuildID } from "vorma/client";

const buildID = getBuildID();
console.log("Current build:", buildID);
```

### 4.13 Utilities

```typescript
import {
	getRootEl,
	getLocation,
	getBuildID,
	getStatus,
	getHistoryInstance,
} from "vorma/client";

const root = getRootEl(); // document.getElementById("vorma-root")
const location = getLocation(); // { pathname, search, hash, state }
const buildID = getBuildID();
const status = getStatus(); // { isNavigating, isSubmitting, isRevalidating }
const history = getHistoryInstance();
```

---

## 5. Advanced Patterns

### 5.1 Parallel Data Loading

All loaders in a route hierarchy run in parallel:

```go
// All three run simultaneously for /blog/my-post
var _ = NewLoader("/", func(c *LoaderCtx) (*RootData, error) {
    // Root layout loader
})

var _ = NewLoader("/blog", func(c *LoaderCtx) (*BlogData, error) {
    // Blog layout loader
})

var _ = NewLoader("/blog/:slug", func(c *LoaderCtx) (*Post, error) {
    // Post loader
})
```

### 5.2 Authentication Patterns

**Server-side auth check:**

```go
func requireAuth(c *LoaderCtx) (*User, error) {
    user := getUserFromSession(c.Request())
    if user == nil {
        c.Redirect("/login")
        return nil, nil
    }
    return user, nil
}

var _ = NewLoader("/", requireAuth)
```

**Client-side auth state:**

```typescript
export function useAuth() {
    const router = useRouterData();
    return router.rootData;  // User from root loader
}

const user = useAuth();
if (!user) {
    return <Navigate to="/login" />;
}
```

### 5.3 Form Handling

**Server action:**

```go
type LoginInput struct {
    Email    string `json:"email" validate:"required,email"`
    Password string `json:"password" validate:"required,min=8"`
}

type LoginOutput struct {
    Success bool   `json:"success"`
    Token   string `json:"token,omitempty"`
}

var _ = NewAction("POST", "/login", func(c *ActionCtx[LoginInput]) (*LoginOutput, error) {
    input := c.Input()

    user, err := db.AuthenticateUser(input.Email, input.Password)
    if err != nil {
        return &LoginOutput{Success: false}, nil
    }

    token := generateToken(user)
    c.SetResponseCookie(&http.Cookie{
        Name:     "session",
        Value:    token,
        HttpOnly: true,
        Secure:   true,
        SameSite: http.SameSiteStrictMode,
    })

    return &LoginOutput{Success: true, Token: token}, nil
})
```

**Client form:**

```typescript
import { api } from "./vorma.api.ts";

function LoginForm() {
    const [loading, setLoading] = useState(false);

    const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        setLoading(true);

        const formData = new FormData(e.currentTarget);
        const result = await api.mutate({
            pattern: "/login",
            input: {
                email: formData.get("email") as string,
                password: formData.get("password") as string,
            },
        });

        setLoading(false);

        if (result.success && result.data.success) {
            navigate({ pattern: "/" });
        } else {
            alert("Login failed");
        }
    };

    return (
        <form onSubmit={handleSubmit}>
            <input name="email" type="email" required />
            <input name="password" type="password" required />
            <button disabled={loading}>Login</button>
        </form>
    );
}
```

### 5.4 File Uploads

**Server action:**

```go
var _ = NewAction("POST", "/upload", func(c *ActionCtx[vorma.FormData]) (string, error) {
    r := c.Request()

    file, header, err := r.FormFile("file")
    if err != nil {
        return "", err
    }
    defer file.Close()

    if header.Size > 10*1024*1024 {  // 10MB
        return "", fmt.Errorf("file too large")
    }

    data, err := io.ReadAll(file)
    if err != nil {
        return "", err
    }

    url, err := saveToStorage(header.Filename, data)
    if err != nil {
        return "", err
    }

    return url, nil
})
```

**Client upload:**

```typescript
function FileUpload() {
    const handleSubmit = async (e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();

        const formData = new FormData(e.currentTarget);

        const result = await api.mutate({
            pattern: "/upload",
            input: formData,
        });

        if (result.success) {
            console.log("File URL:", result.data);
        }
    };

    return (
        <form onSubmit={handleSubmit}>
            <input name="file" type="file" />
            <button>Upload</button>
        </form>
    );
}
```

### 5.5 Optimistic Updates

```typescript
import { revalidate } from "vorma/client";

function TodoItem({ todo }) {
    const [optimisticComplete, setOptimisticComplete] = useState(todo.completed);

    const handleToggle = async () => {
        setOptimisticComplete(!optimisticComplete);

        const result = await api.mutate({
            pattern: "/todos/:id",
            params: { id: todo.id },
            input: { completed: !optimisticComplete },
            options: { revalidate: false },
        });

        if (!result.success) {
            setOptimisticComplete(optimisticComplete);
        } else {
            await revalidate();
        }
    };

    return (
        <div>
            <input
                type="checkbox"
                checked={optimisticComplete}
                onChange={handleToggle}
            />
            {todo.title}
        </div>
    );
}
```

### 5.6 Pagination

**Server loader:**

```go
type PaginatedPosts struct {
    Posts      []*Post `json:"posts"`
    Page       int     `json:"page"`
    TotalPages int     `json:"totalPages"`
}

var _ = NewLoader("/posts", func(c *LoaderCtx) (*PaginatedPosts, error) {
    page := 1
    if p := c.Request().URL.Query().Get("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil {
            page = parsed
        }
    }

    posts, total := db.GetPosts(page, 20)

    return &PaginatedPosts{
        Posts:      posts,
        Page:       page,
        TotalPages: (total + 19) / 20,
    }, nil
})
```

**Client component:**

```typescript
function Posts(props: RouteProps<"/posts">) {
    const data = useLoaderData(props);

    return (
        <div>
            {data.posts.map(post => (
                <div key={post.id}>{post.title}</div>
            ))}

            <div>
                {data.page > 1 && (
                    <Link pattern="/posts" search={`?page=${data.page - 1}`}>
                        Previous
                    </Link>
                )}

                {data.page < data.totalPages && (
                    <Link pattern="/posts" search={`?page=${data.page + 1}`}>
                        Next
                    </Link>
                )}
            </div>
        </div>
    );
}
```

---

## 6. TypeScript Generated Types

After running build, Vorma generates complete TypeScript types in
`frontend/src/vorma.gen/index.ts`:

```typescript
// Generated types include:
export const routes = [...] as const;

export type VormaApp = {
    routes: typeof routes;
    appConfig: typeof vormaAppConfig;
    rootData: RootData;
};

export const vormaAppConfig = {
    actionsRouterMountRoot: "/api/",
    actionsDynamicRune: ":",
    actionsSplatRune: "*",
    loadersDynamicRune: ":",
    loadersSplatRune: "*",
    loadersExplicitIndexSegment: "_index",
    __phantom: null as unknown as VormaApp,
} as const;

// Helper types
export type QueryPattern = VormaQueryPattern<VormaApp>;
export type QueryProps<P extends QueryPattern> = VormaQueryProps<VormaApp, P>;
export type QueryInput<P extends QueryPattern> = VormaQueryInput<VormaApp, P>;
export type QueryOutput<P extends QueryPattern> = VormaQueryOutput<VormaApp, P>;

export type MutationPattern = VormaMutationPattern<VormaApp>;
export type MutationProps<P extends MutationPattern> = VormaMutationProps<VormaApp, P>;
export type MutationInput<P extends MutationPattern> = VormaMutationInput<VormaApp, P>;
export type MutationOutput<P extends MutationPattern> = VormaMutationOutput<VormaApp, P>;

export type RouteProps<P extends VormaLoaderPattern<VormaApp>> = VormaRouteProps<VormaApp, P>;

// Vite config for build
export const vormaViteConfig = {...} as const;
```

---

## 7. Configuration Reference

### 7.1 wave.config.json Complete Schema

```json
{
	"$schema": "dist/static/internal/schema.json",
	"Core": {
		"ConfigLocation": "string, required, path to this config file",
		"DevBuildHook": "string, required, command for dev builds",
		"ProdBuildHook": "string, required, command for prod builds",
		"MainAppEntry": "string, required, path to main.go",
		"DistDir": "string, required, output directory for binary",
		"StaticAssetDirs": {
			"Private": "string, required, private static files (templates)",
			"Public": "string, required, public static files (served)"
		},
		"CSSEntryFiles": {
			"Critical": "string, optional, critical CSS (inlined)",
			"NonCritical": "string, optional, non-critical CSS (linked)"
		},
		"PublicPathPrefix": "string, required, URL prefix for assets"
	},
	"Vorma": {
		"IncludeDefaults": "bool, optional, default true",
		"UIVariant": "string, required, 'react'|'preact'|'solid'",
		"HTMLTemplateLocation": "string, required, path within Private dir",
		"ClientEntry": "string, required, TypeScript entry file",
		"ClientRouteDefsFile": "string, required, route definitions file",
		"TSGenOutDir": "string, required, generated TypeScript output file",
		"BuildtimePublicURLFuncName": "string, optional, default 'waveBuildtimeURL'"
	},
	"Vite": {
		"JSPackageManagerBaseCmd": "string, required, 'npx'|'pnpm'|'yarn'|'bunx'"
	},
	"Watch": {
		"HealthcheckEndpoint": "string, required, health check path",
		"Include": [
			{
				"Pattern": "string, required, glob pattern",
				"OnChangeHooks": [
					{
						"Cmd": "string, optional, 'DevBuildHook' or custom",
						"Timing": "string, optional, 'concurrent'|'sequential'",
						"Strategy": {
							"HttpEndpoint": "string, optional, fast rebuild endpoint",
							"SkipDevHook": "bool, optional",
							"SkipGoCompile": "bool, optional",
							"WaitForApp": "bool, optional",
							"WaitForVite": "bool, optional",
							"ReloadBrowser": "bool, optional",
							"FallbackAction": "string, optional, 'RestartNoGo'|'Restart'"
						}
					}
				],
				"SkipRebuildingNotification": "bool, optional"
			}
		]
	}
}
```

### package.json Scripts

```json
{
	"scripts": {
		"dev": "go run ./backend/cmd/build --dev",
		"build": "go run ./backend/cmd/build"
	}
}
```

### 7.2 tsconfig.json

**React:**

```json
{
	"compilerOptions": {
		"target": "ES2022",
		"module": "ESNext",
		"moduleResolution": "Bundler",
		"jsx": "react-jsx",
		"jsxImportSource": "react",
		"strict": true,
		"skipLibCheck": true,
		"noEmit": true,
		"esModuleInterop": true,
		"allowImportingTsExtensions": true,
		"verbatimModuleSyntax": true,
		"noUncheckedIndexedAccess": true
	},
	"exclude": ["node_modules"]
}
```

**Preact:**

```json
{
	"compilerOptions": {
		"jsx": "react-jsx",
		"jsxImportSource": "preact"
	}
}
```

**Solid:**

```json
{
	"compilerOptions": {
		"jsx": "preserve",
		"jsxImportSource": "solid-js"
	}
}
```

### 7.3 Vite Plugins by UI Variant

- **React**: `@vitejs/plugin-react-swc`
- **Preact**: `@preact/preset-vite`
- **Solid**: `vite-plugin-solid`

---

## 8. Deployment

### 8.1 Production Build

```bash
npm run build
```

Output:

- `backend/dist/main`: Compiled binary
- `backend/dist/static/`: Static assets (hashed, immutable)

### 8.2 Running in Production

```bash
./backend/dist/main
# Or with custom port:
PORT=3000 ./backend/dist/main
```

### 8.3 Docker Deployment

```dockerfile
FROM golang:1.24 AS builder
WORKDIR /app
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
RUN apt-get install -y nodejs
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN npm ci
RUN go run ./backend/cmd/build --no-binary
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly -o ./backend/dist/main ./backend/cmd/serve

FROM alpine
WORKDIR /app
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/backend/dist/main /main
RUN adduser -D serveruser
USER serveruser
ENTRYPOINT ["/main"]
```

---

## 9. Common Gotchas

### 9.1 Don't Return nil from Loaders

```go
// Bad:
var _ = NewLoader("/data", func(c *LoaderCtx) (*Data, error) {
    return nil, nil
})

// Good:
var _ = NewLoader("/data", func(c *LoaderCtx) (*Data, error) {
    return &Data{}, nil
})

// Or use None:
var _ = NewLoader("/page", func(c *LoaderCtx) (vorma.None, error) {
    return vorma.None{}, nil
})
```

### 9.2 Root Layout Pattern is "/"

The root layout loader uses pattern `"/"`, not `""`:

```go
var _ = NewLoader("/", func(c *LoaderCtx) (*RootData, error) {
    return &RootData{}, nil
})
```

### 9.3 Index Routes

- `"/_index"` matches `/` exactly (home page)
- `"/blog/_index"` matches `/blog` exactly
- `"/blog"` is a layout that wraps `/blog/*` routes

### 9.4 Form Revalidation

Actions auto-revalidate unless you opt out:

```typescript
const result = await api.mutate({
	pattern: "/action",
	input: {},
	options: { revalidate: false },
});
```

### 9.5 Accessing Loader Data by Framework

- **React**: `useLoaderData(props)` returns data directly
- **Preact**: `useLoaderData(props).value` (signals)
- **Solid**: `useLoaderData(props)()` (accessors)

---

## 10. Summary

Vorma provides:

**Server (Go):**

- `NewLoader[O](pattern, func)` - Fetch data for routes
- `NewAction[I,O](method, pattern, func)` - Handle mutations
- Automatic TypeScript type generation
- Head element management
- Response manipulation

**Client (TypeScript):**

- `<Link>` - Type-safe navigation with prefetching
- `navigate()` - Programmatic navigation
- `useLoaderData()` - Access server data
- `api.query()` / `api.mutate()` - Call server actions
- `addClientLoader()` - Client-side data fetching
- React/Preact/Solid support

**Key concepts:**

- Routes defined in `vorma.routes.ts`
- Loaders run in parallel on server
- Nested layouts via route hierarchy
- End-to-end type safety
- Content-hashed assets
