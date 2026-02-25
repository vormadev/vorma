# Build a Hacker News Clone with Vorma + React

In this tutorial, you'll build a small Hacker News-style app from scratch using
Vorma. By the end, you'll have a working front page with stories, a story detail
page with comments, and forms to submit stories and add comments -- all with
end-to-end type safety between your Go backend and your React frontend.

**Prerequisites:** Go 1.24+, Node.js 22.11+, and a package manager (npm, pnpm,
yarn, or bun).

**What you'll learn:**

- How Vorma's project structure works
- Defining loaders (server-side data fetching) and actions (mutations)
- Wiring frontend routes to backend loaders
- Using typed `Link`, `navigate`, `useLoaderData`, `usePatternLoaderData`, and
  `api.mutate`
- How `vorma.gen` gives you end-to-end type safety for free
- Nested routing with parallel loader execution and `_index` conventions
- Using `tasks` for request-scoped caching, thundering-herd protection, and
  parallel execution with shared dependencies
- Validating action inputs with the fluent `validate` API
- Controlling HTTP responses: redirects, headers, cookies
- Client-side APIs: loading indicators, prefetching, error boundaries,
  revalidation
- Customizing TypeScript generation with custom types and overrides
- Building and deploying a single-binary Go server with all assets embedded

**What this tutorial is _not_:**

- A database tutorial. We use SQLite because setup is trivial. The DB code is
  minimal and not the point.
- An auth tutorial. We use cookie-based usernames because setup is trivial.
  Don't ship this auth to production.
- A CSS tutorial. We'll keep styles basic so you can focus on the Vorma
  patterns.

---

## 1. Create the Project

Vorma has an official scaffolder. Run it:

```sh
npm create vorma@latest
```

When prompted:

- **Project name:** `hn-vorma`
- **UI variant:** `react`
- **JS package manager:** pick your preferred one (`npm`, `pnpm`, `yarn`, or
  `bun`)
- **Deployment target:** `none` (fine for this tutorial)

The scaffolder creates everything -- `go.mod`, `package.json`, `vite.config.ts`,
backend and frontend directories, example routes, and an initial build. `cd`
into your new project and start the dev server to make sure it works:

```sh
cd hn-vorma
# Use the package manager you selected:
npm run dev
# or: pnpm dev / yarn dev / bun dev
```

You should see the example app running. Stop the dev server (`Ctrl+C`) and let's
build our HN clone.

---

## 2. Understand the Project Structure

Before changing anything, let's understand what the scaffolder gave us:

```
hn-vorma/
├── backend/
│   ├── cmd/
│   │   ├── serve/main.go      # HTTP server entry point
│   │   └── build/main.go      # Build pipeline entry point
│   ├── src/router/
│   │   ├── app.go             # VormaApp configuration
│   │   ├── context.go         # Custom context types + DefineLoader/DefineAction helpers
│   │   ├── init.go            # Router initialization + middleware
│   │   └── example_routes.go  # Example loaders and actions (we'll replace this)
│   ├── assets/                # Private assets (HTML template, etc.)
│   ├── dist/                  # Build output (gitignored)
│   ├── wave.config.json       # Build pipeline configuration
│   ├── wave.dev.go            # Dev: reads config from filesystem
│   └── wave.prod.go           # Prod: embeds config into binary
├── frontend/
│   ├── src/
│   │   ├── vorma.entry.tsx    # Client entry point
│   │   ├── vorma.bindings.ts  # Typed helpers (Link, navigate, useLoaderData, api)
│   │   ├── vorma.gen/         # Generated types (DON'T EDIT)
│   │   ├── routes/            # Route declarations
│   │   ├── components/        # React components
│   │   └── styles/            # CSS
│   └── assets/                # Public assets (favicon, images)
├── package.json
├── vite.config.ts
└── go.mod
```

**Key concept:** Vorma has a clear split. The `backend/` directory is pure Go.
The `frontend/` directory is TypeScript + React. They communicate through
**loaders** (data fetching) and **actions** (mutations), and Vorma generates
TypeScript types from your Go types automatically.

Let's look at the important files.

### `backend/src/router/app.go` -- the VormaApp

This is where your Vorma app is configured:

```go
package router

import (
	"hn-vorma/backend"
	"net/http"

	"github.com/vormadev/vorma"
)

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: backend.Wave,
	HeadDedupeKeysFunc: func(h *vorma.HeadEls) {
		h.Meta(h.Property("og:title"))
		h.Meta(h.Property("og:description"))
	},
	DefaultHeadElsFunc: func(r *http.Request, app *vorma.Vorma, h *vorma.HeadEls) error {
		h.Title("HN Clone")
		h.Description("A Hacker News clone built with Vorma.")
		h.Link(
			h.Rel("icon"),
			h.Href(app.PublicURL("favicon.svg")),
			h.Type("image/svg+xml"),
		)
		return nil
	},
	RootTemplateDataFunc: func(r *http.Request) (map[string]any, error) {
		return map[string]any{}, nil
	},
})
```

`DefaultHeadElsFunc` sets the default `<title>` and `<meta>` tags for every
page. Individual loaders can override these per-route (we'll do this later).

### `backend/src/router/context.go` -- DefineLoader and DefineAction

This file sets up the helper functions you'll use to register routes:

```go
package router

import "github.com/vormadev/vorma"

type LoaderCtx struct{ *vorma.LoaderReqData }
type ActionCtx[I any] struct{ *vorma.ActionReqData[I] }

func decorateLoaderCtx(reqData *vorma.LoaderReqData) *LoaderCtx {
	return &LoaderCtx{LoaderReqData: reqData}
}

func decorateActionCtx[I any](reqData *vorma.ActionReqData[I]) *ActionCtx[I] {
	return &ActionCtx[I]{ActionReqData: reqData}
}

func DefineLoader[O any](
	pattern string,
	loader vorma.LoaderFunc[LoaderCtx, O],
) *vorma.Loader[O] {
	return vorma.DefineLoaderForRegistration(App, pattern, loader, decorateLoaderCtx)
}

func DefineAction[I any, O any](
	method string,
	pattern string,
	action vorma.ActionFunc[ActionCtx[I], O],
) *vorma.Action[I, O] {
	return vorma.DefineActionForRegistration(App, method, pattern, action, decorateActionCtx)
}
```

`LoaderCtx` and `ActionCtx` are thin wrappers around Vorma's request data. You
can add your own methods to them later (e.g.,
`func (c *LoaderCtx) CurrentUser() *User`). The `DefineLoader` and
`DefineAction` functions are what you'll call to register routes -- they wire
everything up including type generation.

The `decorateLoaderCtx` / `decorateActionCtx` step is the extension point that
makes this possible. It lets you keep handlers clean by moving app-specific
access patterns into context methods.

Common examples:

- Shared dependencies: `Store()`, `Mailer()`, `Logger()`
- Request-scoped helpers: `CurrentUser()`, `SessionID()`, `RequireAdmin()`
- Cross-cutting policy helpers: `CanEditStory(id)`, `RequireFeature("x")`

In other words, instead of repeating lookup/plumbing logic in every loader or
action, you centralize it in your custom context methods.

### `backend/src/router/init.go` -- router startup

```go
package router

import (
	"net/http"

	"github.com/vormadev/vorma/kit/middleware/healthcheck"
)

func Init() (addr string, handler http.Handler) {
	r := App.MustInitWithDefaultRouter()

	r.AddGlobalHTTPMiddleware(App.MustStaticMiddleware())
	r.AddGlobalHTTPMiddleware(healthcheck.Healthz)

	return App.ServerAddr(), r
}
```

`MustInitWithDefaultRouter()` initializes the Vorma runtime and mounts your
loader/action HTTP handlers. `MustStaticMiddleware()` serves your hashed
frontend assets. You can add your own middleware here later (auth, logging,
CORS, etc.).

### Frontend: `vorma.entry.tsx`, `vorma.bindings.ts`, and route files

The entry point (`vorma.entry.tsx`) initializes the Vorma client and mounts
React:

```tsx
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

You'll rarely touch this file. `VormaRootOutlet` is the root component that
renders the matched route tree.

The bindings file (`vorma.bindings.ts`) exports all the typed tools you'll use
in components:

```tsx
// These are all type-safe -- patterns, params, and data types
// are inferred from your Go backend definitions.
export const Link = makeTypedLink(vormaAppConfig, { prefetch: "intent" });
export const navigate = makeTypedNavigate(vormaAppConfig);
export const useRouterData = makeTypedUseRouterData(vormaAppConfig);
export const useLoaderData = makeTypedUseLoaderData(vormaAppConfig);
export const usePatternLoaderData =
	makeTypedUsePatternLoaderData(vormaAppConfig);
export const addClientLoader = makeTypedAddClientLoader(vormaAppConfig);
export const api = makeTypedAPIClient(vormaAppConfig, (ctx) => {
	if (ctx.type === "mutation") {
		return undefined;
	}
	return undefined;
});
```

The generated `api` surface is intentionally small (`api.query()` and
`api.mutate()`). If you need shared request decoration (headers, credentials,
etc.), edit that `makeTypedAPIClient` callback.

How to use this in practice:

- Keep `vorma.bindings.ts` as your single app-level integration file for Vorma
  client helpers.
- In components, import helpers from this file only.
- Use `Link` and `navigate` for typed routing.
- Use `useLoaderData` / `useRouterData` / `usePatternLoaderData` for typed data.
- Use `api.query` / `api.mutate` for typed action calls.
- Treat the `makeTypedAPIClient(..., (ctx) => ...)` callback as optional global
  request policy. Most apps can start with `return undefined`.

Basic usage looks like:

```tsx
const searchResult = await api.query({
	pattern: "/search",
	input: { q: "vorma" },
});

const createResult = await api.mutate({
	pattern: "/stories",
	input: { title: "Hello", by: "alice" },
});
```

And the route declarations (`routes/core.vorma.routes.ts`) connect URL patterns
to components:

```ts
import { route } from "vorma/buildtime";

route("/", import("../components/root.tsx"), "Root");
route("/_index", import("../components/home.tsx"), "Home");
```

`route()` takes a pattern, a module reference, and the name of the exported
component in that module. In day-to-day code, the recommended module form is
`import("./your-component.tsx")` for code-splitting.

Build-time parsing can also resolve a static string module path (or a top-level
variable assigned to one), but dynamic expressions cannot be statically
resolved.

`route()` is a build-time marker. Vorma parses these calls during build to
generate route artifacts and types; at runtime, the function itself is a no-op.

---

## 3. Plan the Routes

Here's what we'll build:

| URL         | What it shows                          | Loader data      | Actions                                    |
| ----------- | -------------------------------------- | ---------------- | ------------------------------------------ |
| `/`         | Root layout (nav bar, wraps all pages) | Site name        | --                                         |
| `/_index`   | Front page story list                  | List of stories  | --                                         |
| `/item/:id` | Single story + comments                | Story + comments | --                                         |
| `/submit`   | Submit a new story                     | (none)           | --                                         |
| --          | --                                     | --               | `POST /stories` (create story)             |
| --          | --                                     | --               | `POST /stories/:id/upvote` (upvote)        |
| --          | --                                     | --               | `POST /stories/:id/comments` (add comment) |

**Loaders** serve data when a page loads (think: GET requests that return JSON).
**Actions** handle mutations (think: POST, PUT, DELETE, or any HTTP method).
Both are defined in Go, and Vorma generates TypeScript types for their inputs
and outputs automatically.

---

## 4. Create the SQLite Store

Create `backend/src/store/store.go`. This is plain Go -- nothing Vorma-specific.

```go
package store

import (
	"context"
	"database/sql"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db *sql.DB
}

// --- Data types ---

type Story struct {
	ID            int64  `json:"id"`
	Title         string `json:"title"`
	URL           string `json:"url,omitempty"`
	Text          string `json:"text,omitempty"`
	By            string `json:"by"`
	Score         int    `json:"score"`
	CreatedAtUnix int64  `json:"createdAtUnix"`
	CommentCount  int    `json:"commentCount"`
}

type Comment struct {
	ID            int64  `json:"id"`
	StoryID       int64  `json:"storyId"`
	ParentID      *int64 `json:"parentId,omitempty"`
	By            string `json:"by"`
	Text          string `json:"text"`
	CreatedAtUnix int64  `json:"createdAtUnix"`
}

type CreateStoryInput struct {
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
	Text  string `json:"text,omitempty"`
	By    string `json:"by"`
}

type CreateCommentInput struct {
	By       string `json:"by"`
	Text     string `json:"text"`
	ParentID *int64 `json:"parentId,omitempty"`
}

// --- Constructor ---

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	if _, err := db.Exec(`PRAGMA journal_mode = WAL`); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS stories (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			title TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			text_content TEXT NOT NULL DEFAULT '',
			by_user TEXT NOT NULL,
			score INTEGER NOT NULL DEFAULT 1,
			comment_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS comments (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			story_id INTEGER NOT NULL REFERENCES stories(id),
			parent_id INTEGER,
			by_user TEXT NOT NULL,
			text_content TEXT NOT NULL,
			created_at INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_comments_story ON comments(story_id);

		CREATE TABLE IF NOT EXISTS votes (
			story_id INTEGER NOT NULL REFERENCES stories(id),
			voter TEXT NOT NULL,
			UNIQUE(story_id, voter)
		);
	`)
	return err
}

// --- Queries ---

func (s *Store) ListStories(ctx context.Context, limit int) ([]Story, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, title, url, text_content, by_user, score, comment_count, created_at
		FROM stories
		ORDER BY score DESC, created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stories []Story
	for rows.Next() {
		var st Story
		if err := rows.Scan(
			&st.ID, &st.Title, &st.URL, &st.Text,
			&st.By, &st.Score, &st.CommentCount, &st.CreatedAtUnix,
		); err != nil {
			return nil, err
		}
		stories = append(stories, st)
	}
	return stories, nil
}

func (s *Store) GetStory(ctx context.Context, id int64) (*Story, error) {
	var st Story
	err := s.db.QueryRowContext(ctx, `
		SELECT id, title, url, text_content, by_user, score, comment_count, created_at
		FROM stories WHERE id = ?
	`, id).Scan(
		&st.ID, &st.Title, &st.URL, &st.Text,
		&st.By, &st.Score, &st.CommentCount, &st.CreatedAtUnix,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &st, nil
}

func (s *Store) GetComments(ctx context.Context, storyID int64) ([]Comment, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, story_id, parent_id, by_user, text_content, created_at
		FROM comments
		WHERE story_id = ?
		ORDER BY created_at ASC
	`, storyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var comments []Comment
	for rows.Next() {
		var c Comment
		if err := rows.Scan(
			&c.ID, &c.StoryID, &c.ParentID,
			&c.By, &c.Text, &c.CreatedAtUnix,
		); err != nil {
			return nil, err
		}
		comments = append(comments, c)
	}
	return comments, nil
}

// --- Mutations ---

func (s *Store) CreateStory(ctx context.Context, input CreateStoryInput) (int64, error) {
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO stories (title, url, text_content, by_user, score, created_at)
		VALUES (?, ?, ?, ?, 1, ?)
	`, input.Title, input.URL, input.Text, input.By, time.Now().Unix())
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

func (s *Store) AddComment(ctx context.Context, storyID int64, input CreateCommentInput) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `
		INSERT INTO comments (story_id, parent_id, by_user, text_content, created_at)
		VALUES (?, ?, ?, ?, ?)
	`, storyID, input.ParentID, input.By, input.Text, time.Now().Unix())
	if err != nil {
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE stories SET comment_count = comment_count + 1 WHERE id = ?
	`, storyID); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (s *Store) Upvote(ctx context.Context, storyID int64, voter string) (int, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	// INSERT OR IGNORE: if the voter already voted, this is a no-op
	res, err := tx.ExecContext(ctx, `
		INSERT OR IGNORE INTO votes (story_id, voter) VALUES (?, ?)
	`, storyID, voter)
	if err != nil {
		return 0, err
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected > 0 {
		// New vote -- increment score
		if _, err := tx.ExecContext(ctx, `
			UPDATE stories SET score = score + 1 WHERE id = ?
		`, storyID); err != nil {
			return 0, err
		}
	}

	var score int
	if err := tx.QueryRowContext(ctx, `
		SELECT score FROM stories WHERE id = ?
	`, storyID).Scan(&score); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return score, nil
}
```

Now install the SQLite driver:

```sh
go get modernc.org/sqlite
go mod tidy
```

---

## 5. Wire the Store into the Router

Open `backend/src/router/app.go` and add the store initialization. Replace the
existing contents with:

```go
package router

import (
	"hn-vorma/backend"
	"hn-vorma/backend/src/store"
	"net/http"

	"github.com/vormadev/vorma"
)

var DB = mustOpenStore()

func mustOpenStore() *store.Store {
	s, err := store.Open("backend/dev.db")
	if err != nil {
		panic(err)
	}
	return s
}

var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: backend.Wave,
	HeadDedupeKeysFunc: func(h *vorma.HeadEls) {
		h.Meta(h.Property("og:title"))
		h.Meta(h.Property("og:description"))
	},
	DefaultHeadElsFunc: func(r *http.Request, app *vorma.Vorma, h *vorma.HeadEls) error {
		h.Title("HN Clone | Vorma")
		h.Description("A Hacker News clone built with Vorma.")
		h.Link(
			h.Rel("icon"),
			h.Href(app.PublicURL("favicon.svg")),
			h.Type("image/svg+xml"),
		)
		return nil
	},
	RootTemplateDataFunc: func(r *http.Request) (map[string]any, error) {
		return map[string]any{}, nil
	},
})
```

The `DB` variable is available to all route handlers in this package. The
`dev.db` file will be auto-created by SQLite on first run.

> **Tip:** Add `*.db` to your `.gitignore`.

---

## 6. Define the Backend Routes

Now the interesting part. Delete `backend/src/router/example_routes.go` and
create a new file `backend/src/router/routes.go`:

```sh
rm backend/src/router/example_routes.go
```

```go
package router

import (
	"fmt"
	"strconv"

	"hn-vorma/backend/src/store"

	"github.com/vormadev/vorma"
)

// --- Loader output types ---

type RootData struct {
	SiteName string `json:"siteName"`
}

type FrontPageData struct {
	Stories []store.Story `json:"stories"`
}

type StoryPageData struct {
	Story    store.Story     `json:"story"`
	Comments []store.Comment `json:"comments"`
}

// --- Action output types ---

type CreateStoryResult struct {
	StoryID int64 `json:"storyId"`
}

type UpvoteResult struct {
	Score int `json:"score"`
}

type CreateCommentResult struct {
	CommentID int64 `json:"commentId"`
}

// --- Loaders ---

// Root layout loader: provides data available to every page.
var _ = DefineLoader("/", func(c *LoaderCtx) (*RootData, error) {
	return &RootData{SiteName: "HN Clone"}, nil
})

// Front page: list of top stories.
var _ = DefineLoader("/_index", func(c *LoaderCtx) (*FrontPageData, error) {
	stories, err := DB.ListStories(c.Request().Context(), 30)
	if err != nil {
		return nil, err
	}
	return &FrontPageData{Stories: stories}, nil
})

// Story detail page: single story + its comments.
var _ = DefineLoader("/item/:id", func(c *LoaderCtx) (*StoryPageData, error) {
	storyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return nil, err
	}

	story, err := DB.GetStory(c.Request().Context(), storyID)
	if err != nil {
		return nil, err
	}
	if story == nil {
		return nil, &vorma.LoaderError{
			Client: "Story not found",
			Server: fmt.Errorf("story %d not found", storyID),
		}
	}

	comments, err := DB.GetComments(c.Request().Context(), storyID)
	if err != nil {
		return nil, err
	}

	// Override the page title for this specific story
	h := c.HeadEls()
	h.Title(fmt.Sprintf("%s | HN Clone", story.Title))

	return &StoryPageData{Story: *story, Comments: comments}, nil
})

// Submit page: no server data needed, but we need a loader
// to register the pattern. Returning an empty struct is fine.
var _ = DefineLoader("/submit", func(c *LoaderCtx) (vorma.None, error) {
	return vorma.None{}, nil
})

// --- Actions ---

// Create a new story.
var _ = DefineAction("POST", "/stories",
	func(c *ActionCtx[store.CreateStoryInput]) (*CreateStoryResult, error) {
		storyID, err := DB.CreateStory(c.Request().Context(), c.Input())
		if err != nil {
			return nil, err
		}
		return &CreateStoryResult{StoryID: storyID}, nil
	},
)

// Upvote a story. No input body needed -- the story ID is in the URL.
var _ = DefineAction("POST", "/stories/:id/upvote",
	func(c *ActionCtx[vorma.None]) (*UpvoteResult, error) {
		storyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			return nil, err
		}
		score, err := DB.Upvote(c.Request().Context(), storyID, "anon")
		if err != nil {
			return nil, err
		}
		return &UpvoteResult{Score: score}, nil
	},
)

// Add a comment to a story.
var _ = DefineAction("POST", "/stories/:id/comments",
	func(c *ActionCtx[store.CreateCommentInput]) (*CreateCommentResult, error) {
		storyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			return nil, err
		}
		commentID, err := DB.AddComment(c.Request().Context(), storyID, c.Input())
		if err != nil {
			return nil, err
		}
		return &CreateCommentResult{CommentID: commentID}, nil
	},
)
```

Let's break down what just happened:

**Loaders** are registered with `DefineLoader(pattern, handler)`. The handler
receives a `*LoaderCtx` and returns a typed value. That return type becomes the
loader data available on the frontend -- Vorma generates TypeScript types for it
automatically.

- `"/"` is the **root layout** loader. Its data is available on every page via
  `useRouterData()`.
- `"/_index"` is the **index** loader -- it runs when the user is at exactly
  `/`.
- `"/item/:id"` is a **dynamic** route. `:id` is a URL parameter accessed via
  `c.Param("id")`.
- `"/submit"` returns `vorma.None` (an empty struct) -- useful when a page needs
  a loader to exist but doesn't need server data.

**Actions** are registered with `DefineAction(method, pattern, handler)`. The
first type parameter is the **input** type (the JSON body the frontend sends),
and the output is the response.

- `vorma.None` as input means the action doesn't expect a request body.
- `store.CreateStoryInput` as input means the frontend must send JSON matching
  that struct.

**`vorma.LoaderError`** lets you return different messages to the client vs. the
server log. The `Client` field is what the user sees; `Server` is what gets
logged.

**`c.HeadEls()`** lets you override `<head>` elements per-route. Here we set a
custom page title for each story.

Most day-to-day handler logic uses these `LoaderCtx` / `ActionCtx` methods:

| Method                                                                          | Where             | What it gives you                                     |
| ------------------------------------------------------------------------------- | ----------------- | ----------------------------------------------------- |
| `c.Input()`                                                                     | Actions           | Typed request body (after parsing/validation)         |
| `c.Param("id")`, `c.Params()`                                                   | Loaders + actions | Dynamic URL params                                    |
| `c.SplatValues()`                                                               | Loaders + actions | Captured `*` segments                                 |
| `c.Request()`                                                                   | Loaders + actions | Raw `*http.Request` (context, headers, cookies, etc.) |
| `c.TasksCtx()`                                                                  | Loaders + actions | Request-scoped tasks context for dedupe/parallel work |
| `c.HeadEls()`                                                                   | Loaders + actions | Route-specific `<head>` tags                          |
| `c.Redirect(...)`, `c.SetResponseStatus(...)`, `c.SetResponseHeader(...)`, etc. | Loaders + actions | Recommended response controls                         |
| `c.ResponseProxy()`                                                             | Loaders + actions | Low-level response proxy access when needed           |

---

## 7. Declare the Frontend Routes

Now we connect URL patterns to React components. Open
`frontend/src/routes/core.vorma.routes.ts` and replace it:

```ts
import { route } from "vorma/buildtime";

route("/", import("../components/layout.tsx"), "Layout");
route("/_index", import("../components/front-page.tsx"), "FrontPage");
route("/item/:id", import("../components/item-page.tsx"), "ItemPage");
route("/submit", import("../components/submit-page.tsx"), "SubmitPage");
```

Each `route()` call connects:

1. A **pattern** (must match a backend `DefineLoader` pattern)
2. A **module reference** (typically a dynamic `import(...)`)
3. The **exported component name** from that module

Important build-time rules:

- `route(...)` declarations are parsed at build time; they are metadata, not
  runtime registration side effects.
- Module expressions must be statically resolvable. Dynamic expressions (for
  example, `route("/x", getPath(), "X")`) are unresolved.
- Unresolved route module expressions are treated as errors unless you set
  `Vorma.UnresolvedRoutePolicy` to `"warn"` (log + skip).
- Duplicate route patterns across route definition files fail the build.

The route `"/"` is special -- it's the **root layout**. Its component receives
an `Outlet` prop that renders the matched child route. Think of it like a
wrapper: the layout is always rendered, and the content inside changes based on
the URL.

You can also delete the existing links route file if it exists:

```sh
rm frontend/src/routes/links.vorma.routes.ts
```

---

## 8. Build the React Components

### 8.1 The Root Layout

Create `frontend/src/components/layout.tsx`:

```tsx
import type { RouteProps } from "../vorma.gen/index.ts";
import { Link, useRouterData } from "../vorma.bindings.ts";

export function Layout(props: RouteProps<"/">) {
	const routerData = useRouterData(props);

	return (
		<div style={{ maxWidth: 800, margin: "0 auto", padding: "0 16px" }}>
			<header
				style={{
					background: "#ff6600",
					padding: "4px 8px",
					display: "flex",
					alignItems: "center",
					gap: 12,
				}}
			>
				<Link
					pattern="/"
					style={{
						fontWeight: "bold",
						color: "#000",
						textDecoration: "none",
					}}
				>
					{routerData.rootData.siteName}
				</Link>
				<Link
					pattern="/_index"
					style={{ color: "#000", textDecoration: "none" }}
				>
					top
				</Link>
				<Link
					pattern="/submit"
					style={{ color: "#000", textDecoration: "none" }}
				>
					submit
				</Link>
			</header>

			<main style={{ padding: "16px 0" }}>
				<props.Outlet />
			</main>
		</div>
	);
}
```

Key patterns:

- **`RouteProps<"/">`**: Typed props for the `"/"` route. This includes the
  `Outlet` component for rendering child routes.
- **`useRouterData(props)`**: Returns the root loader data (our `RootData`
  struct with `siteName`). This works on _any_ page because the root loader runs
  on every navigation.
- **`Link`**: A type-safe link component. The `pattern` prop must be a valid
  loader pattern -- TypeScript will error if you typo it. Vorma automatically
  resolves it to the correct URL, including filling in params for dynamic
  routes.
- **`props.Outlet`**: Renders whichever child route is matched. On `/`, it
  renders `FrontPage`. On `/item/42`, it renders `ItemPage`. The layout stays
  the same.

### 8.2 The Front Page

Create `frontend/src/components/front-page.tsx`:

```tsx
import type { RouteProps } from "../vorma.gen/index.ts";
import { api, Link, useLoaderData } from "../vorma.bindings.ts";

export function FrontPage(props: RouteProps<"/_index">) {
	const data = useLoaderData(props);

	async function handleUpvote(storyId: number) {
		await api.mutate({
			pattern: "/stories/:id/upvote",
			params: { id: String(storyId) },
		});
	}

	return (
		<div>
			{(!data.stories || data.stories.length === 0) && (
				<p style={{ color: "#666" }}>
					No stories yet. <Link pattern="/submit">Submit one!</Link>
				</p>
			)}

			<ol style={{ paddingLeft: 0, listStyle: "none" }}>
				{data.stories?.map((story, i) => (
					<li key={story.id} style={{ marginBottom: 8 }}>
						<div style={{ display: "flex", gap: 8 }}>
							<span style={{ color: "#666", minWidth: 24 }}>
								{i + 1}.
							</span>
							<div>
								<div>
									<button
										onClick={() => handleUpvote(story.id)}
										style={{
											cursor: "pointer",
											background: "none",
											border: "none",
											color: "#666",
											padding: 0,
											marginRight: 4,
										}}
									>
										▲
									</button>
									{story.url ? (
										<a
											href={story.url}
											target="_blank"
											rel="noreferrer"
										>
											{story.title}
										</a>
									) : (
										<Link
											pattern="/item/:id"
											params={{ id: String(story.id) }}
										>
											{story.title}
										</Link>
									)}
									{story.url && (
										<span
											style={{
												color: "#666",
												fontSize: "0.85em",
											}}
										>
											{" "}
											({new URL(story.url).hostname})
										</span>
									)}
								</div>
								<div
									style={{
										fontSize: "0.85em",
										color: "#666",
									}}
								>
									{story.score} points by {story.by} |{" "}
									<Link
										pattern="/item/:id"
										params={{ id: String(story.id) }}
									>
										{story.commentCount} comments
									</Link>
								</div>
							</div>
						</div>
					</li>
				))}
			</ol>
		</div>
	);
}
```

Key patterns:

- **`useLoaderData(props)`**: Returns the typed loader data for this specific
  route. For `"/_index"`, that's `FrontPageData` with a `stories` array.
  TypeScript knows the exact shape.
- **`api.mutate()`**: Calls a backend action. The `pattern` must match a
  `DefineAction` pattern. `params` fills in dynamic segments (`:id`). If the
  action expects a JSON body, you'd pass `input: { ... }` too. The types are
  fully inferred -- TypeScript knows `/stories/:id/upvote` expects no input and
  returns `UpvoteResult`. For non-GET requests, Vorma auto-revalidates the
  current page by default, so the score updates without a manual `navigate()`.
- **`navigate()`**: Client-side navigation when you intentionally want to move
  to another route (we use this on the submit page after story creation).
- **`Link` with `params`**: For dynamic routes like `/item/:id`, you pass
  `params={{ id: "42" }}` and Vorma resolves it to `/item/42`. The `params` type
  is inferred from the pattern -- TypeScript enforces that you pass `id`.

### 8.3 The Item Page

Create `frontend/src/components/item-page.tsx`:

```tsx
import { useState } from "react";
import type { RouteProps } from "../vorma.gen/index.ts";
import { api, Link, useLoaderData } from "../vorma.bindings.ts";

export function ItemPage(props: RouteProps<"/item/:id">) {
	const data = useLoaderData(props);
	const [commentBy, setCommentBy] = useState("");
	const [commentText, setCommentText] = useState("");

	async function handleComment(e: React.FormEvent) {
		e.preventDefault();
		if (!commentBy.trim() || !commentText.trim()) return;

		await api.mutate({
			pattern: "/stories/:id/comments",
			params: { id: String(data.story.id) },
			input: {
				by: commentBy,
				text: commentText,
				parentId: null,
			},
		});

		setCommentBy("");
		setCommentText("");
	}

	return (
		<div>
			<div style={{ marginBottom: 16 }}>
				<h2 style={{ margin: 0 }}>
					{data.story.url ? (
						<a
							href={data.story.url}
							target="_blank"
							rel="noreferrer"
						>
							{data.story.title}
						</a>
					) : (
						data.story.title
					)}
				</h2>
				<div style={{ fontSize: "0.85em", color: "#666" }}>
					{data.story.score} points by {data.story.by}
				</div>
				{data.story.text && (
					<p style={{ marginTop: 8 }}>{data.story.text}</p>
				)}
			</div>

			<hr />

			<form
				onSubmit={handleComment}
				style={{ marginTop: 16, marginBottom: 24 }}
			>
				<h3>Add a comment</h3>
				<div style={{ marginBottom: 8 }}>
					<input
						type="text"
						placeholder="Your name"
						value={commentBy}
						onChange={(e) => setCommentBy(e.target.value)}
						style={{
							padding: "4px 8px",
							marginRight: 8,
							width: 200,
						}}
					/>
				</div>
				<div style={{ marginBottom: 8 }}>
					<textarea
						placeholder="Write a comment..."
						value={commentText}
						onChange={(e) => setCommentText(e.target.value)}
						rows={4}
						style={{
							padding: "4px 8px",
							width: "100%",
							maxWidth: 500,
						}}
					/>
				</div>
				<button
					type="submit"
					style={{ padding: "4px 12px", cursor: "pointer" }}
				>
					Add Comment
				</button>
			</form>

			{data.comments.length > 0 && (
				<div>
					<h3>
						{data.comments.length} comment
						{data.comments.length > 1 ? "s" : ""}
					</h3>
					{data.comments.map((comment) => (
						<div
							key={comment.id}
							style={{
								borderLeft: "2px solid #ddd",
								paddingLeft: 12,
								marginBottom: 12,
							}}
						>
							<div style={{ fontSize: "0.85em", color: "#666" }}>
								{comment.by}
							</div>
							<p>{comment.text}</p>
						</div>
					))}
				</div>
			)}

			<div style={{ marginTop: 16 }}>
				<Link pattern="/_index">Back to front page</Link>
			</div>
		</div>
	);
}
```

Notice how `api.mutate` for comments sends an `input` object. The type of
`input` is inferred from the action definition: since we used
`ActionCtx[store.CreateCommentInput]` in Go, TypeScript knows the frontend must
send `{ by: string, text: string, parentId: number | null }`.

### 8.4 The Submit Page

Create `frontend/src/components/submit-page.tsx`:

```tsx
import { useState } from "react";
import type { RouteProps } from "../vorma.gen/index.ts";
import { api, navigate } from "../vorma.bindings.ts";

export function SubmitPage(_props: RouteProps<"/submit">) {
	const [title, setTitle] = useState("");
	const [url, setUrl] = useState("");
	const [text, setText] = useState("");
	const [by, setBy] = useState("");

	async function handleSubmit(e: React.FormEvent) {
		e.preventDefault();
		if (!title.trim() || !by.trim()) return;

		const result = await api.mutate({
			pattern: "/stories",
			input: { title, url, text, by },
		});

		if (result.success) {
			// Navigate to the newly created story
			await navigate({
				pattern: "/item/:id",
				params: { id: String(result.data.storyId) },
			});
		}
	}

	return (
		<div style={{ maxWidth: 500 }}>
			<h2>Submit a Story</h2>

			<form onSubmit={handleSubmit}>
				<div style={{ marginBottom: 12 }}>
					<label style={{ display: "block", marginBottom: 4 }}>
						Title *
					</label>
					<input
						type="text"
						value={title}
						onChange={(e) => setTitle(e.target.value)}
						style={{ padding: "4px 8px", width: "100%" }}
					/>
				</div>

				<div style={{ marginBottom: 12 }}>
					<label style={{ display: "block", marginBottom: 4 }}>
						URL
					</label>
					<input
						type="url"
						value={url}
						onChange={(e) => setUrl(e.target.value)}
						placeholder="https://example.com"
						style={{ padding: "4px 8px", width: "100%" }}
					/>
				</div>

				<div style={{ marginBottom: 12 }}>
					<label style={{ display: "block", marginBottom: 4 }}>
						Text <span style={{ color: "#666" }}>(if no URL)</span>
					</label>
					<textarea
						value={text}
						onChange={(e) => setText(e.target.value)}
						rows={4}
						style={{ padding: "4px 8px", width: "100%" }}
					/>
				</div>

				<div style={{ marginBottom: 12 }}>
					<label style={{ display: "block", marginBottom: 4 }}>
						Your name *
					</label>
					<input
						type="text"
						value={by}
						onChange={(e) => setBy(e.target.value)}
						style={{ padding: "4px 8px", width: 200 }}
					/>
				</div>

				<button
					type="submit"
					style={{ padding: "6px 16px", cursor: "pointer" }}
				>
					Submit
				</button>
			</form>
		</div>
	);
}
```

`api.mutate` returns
`{ success: true, data: T } | { success: false, error: string }`. The `data`
field is typed to the action's output -- in this case, `CreateStoryResult` with
a `storyId` field. After successful submission, we navigate to the new story's
page.

### 8.5 Clean Up Old Components

Delete the scaffolded example components:

```sh
rm frontend/src/components/root.tsx
rm frontend/src/components/home.tsx
rm frontend/src/components/links.tsx
```

---

## 9. Run It

```sh
# Use the package manager you selected:
npm run dev
# or: pnpm dev / yarn dev / bun dev
```

Vorma will:

1. Discover your `DefineLoader` and `DefineAction` calls via AST analysis
2. Generate TypeScript types in `frontend/src/vorma.gen/`
3. Start Vite for hot module reloading
4. Start the Go server

Open the URL shown in the terminal. Try:

1. Go to `/submit` and create a story
2. See it appear on the front page
3. Click the upvote arrow
4. Open the story detail page and add a comment

---

## 10. How the Pieces Fit Together

Here's the full data flow for a page load:

```
Browser navigates to /item/42
        |
        v
Vorma client fetches loader data (JSON)
        |
        v
Go server matches pattern /item/:id
        |
        v
Loader runs: DB.GetStory(42) + DB.GetComments(42)
        |
        v
Returns StoryPageData as JSON
        |
        v
React component receives typed data via useLoaderData()
        |
        v
ItemPage renders with story + comments
```

And for a mutation:

```
User clicks the upvote button
        |
        v
api.mutate({ pattern: "/stories/:id/upvote", params: { id: "42" } })
        |
        v
POST request to /api/stories/42/upvote
        |
        v
Go action handler runs: DB.Upvote(42, "anon")
        |
        v
Returns UpvoteResult { score: 5 } as JSON
        |
        v
Frontend receives { success: true, data: { score: 5 } }
        |
        v
Vorma auto-revalidates current route data, UI updates
```

The type safety chain goes: **Go struct** -> Vorma code generation ->
**TypeScript type** -> React component. If you rename a field in your Go struct,
TypeScript will immediately show errors everywhere the old field name was used.

---

## 11. Routing In Depth

Our app already uses routing, but let's step back and understand the full system
-- it's one of Vorma's most important concepts.

### 11.1 Pattern Types

Vorma supports four kinds of URL pattern segments:

| Segment     | Example     | Matches                                                        |
| ----------- | ----------- | -------------------------------------------------------------- |
| **Static**  | `/about`    | Exactly `/about`                                               |
| **Dynamic** | `/item/:id` | `/item/42`, `/item/abc` -- any non-empty segment               |
| **Index**   | `/_index`   | The "index page" at the parent level (explained below)         |
| **Splat**   | `/files/*`  | `/files/a`, `/files/a/b/c` -- any number of remaining segments |

Dynamic parameters are extracted by name. In a loader, you access them with
`c.Param("id")`. Static segments always win over dynamic ones -- if both
`/item/new` and `/item/:id` are registered, a request for `/item/new` matches
the static pattern.

### 11.2 How Nested Matching Works

This is where Vorma differs from most routers. A typical router matches one
route per request. Vorma matches **all ancestor patterns** in the route tree and
runs their loaders in parallel.

Let's say you have these loaders registered:

```
/                              → RootData (site name, current user)
/_index                        → FrontPageData (story list)
/item/:id                      → StoryPageData (story + comments)
/submit                        → None
```

When a user navigates to `/item/42`, Vorma finds **two** matches:

```
/           → root layout (always matches as the parent)
/item/:id   → the page content
```

Both loaders execute **in parallel** -- the root loader and the item loader
start at the same time. The root component renders first, and the item component
renders inside its `Outlet`.

When the user navigates to `/`, Vorma matches:

```
/           → root layout
/_index     → the index page
```

Again, both loaders run in parallel.

**Why this matters:** The root loader fetches data that every page needs (site
name, current user, nav state). Child loaders fetch page-specific data. Because
they run in parallel, you're never waiting for the layout data before the page
data starts loading -- they're concurrent.

### 11.3 The `_index` Convention

The `_index` suffix is how Vorma distinguishes "this pattern is a layout" from
"this pattern is the index page at that level."

Without `_index`, there's an ambiguity: does `/dashboard` mean "the dashboard
layout that wraps nested pages" or "the dashboard index page"? With `_index`,
it's clear:

```
/dashboard              → layout (wraps all dashboard pages)
/dashboard/_index       → the index page shown at /dashboard
/dashboard/settings     → a nested page under the dashboard layout
/dashboard/users/:id    → another nested page
```

When someone visits `/dashboard`, Vorma matches both `/dashboard` and
`/dashboard/_index`. The dashboard layout renders, and the index page appears in
its `Outlet`.

When someone visits `/dashboard/settings`, Vorma matches `/dashboard` and
`/dashboard/settings`. The same dashboard layout renders, but now `Outlet` shows
the settings page. The layout never unmounts -- transitions between dashboard
pages are instant because only the `Outlet` content changes.

In our HN clone, `/` is the root layout and `/_index` is the front page. Same
idea at the root level.

### 11.4 Deeper Nesting

You can nest as deep as you want. Consider a full admin panel:

```
/                                          → app shell
/dashboard                                 → dashboard layout (sidebar, breadcrumbs)
/dashboard/_index                          → dashboard home
/dashboard/customers                       → customers layout (tabs, search)
/dashboard/customers/_index                → customer list
/dashboard/customers/:customer_id          → single customer layout
/dashboard/customers/:customer_id/_index   → customer overview
/dashboard/customers/:customer_id/orders   → customer orders layout
/dashboard/customers/:customer_id/orders/_index → orders list
/dashboard/customers/:customer_id/orders/:order_id → single order
```

A request for `/dashboard/customers/123/orders/456` matches **six** patterns:

```
/
/dashboard
/dashboard/customers
/dashboard/customers/:customer_id
/dashboard/customers/:customer_id/orders
/dashboard/customers/:customer_id/orders/:order_id
```

All six loaders run in parallel. Each layout wraps its children via `Outlet`.
The result is a deeply nested UI (app shell → dashboard sidebar → customers tabs
→ customer detail → orders list → order detail) where every layer loaded its own
data concurrently.

Parameters are shared across all matched patterns -- every loader in this chain
can call `c.Param("customer_id")` and `c.Param("order_id")`.

### 11.5 Gaps Are Fine

You don't have to register every intermediate pattern. If you only register
`/dashboard` and `/dashboard/customers/:customer_id/orders/:order_id`, a request
for that deep path still matches both -- the missing intermediates are simply
skipped. This is useful when you don't need a layout at every level.

### 11.6 Splat Routes (Catch-All)

A **splat** pattern ends with `/*` and matches any number of remaining path
segments. The captured segments are available as an array.

```
/files/*    matches /files/readme.txt        → splatValues: ["readme.txt"]
/files/*    matches /files/docs/report.pdf   → splatValues: ["docs", "report.pdf"]
```

In nested matching, splats interact with other patterns through **precedence**:

- If both `/about/:id` and `/about/*` are registered, a request for `/about/123`
  matches `/about/:id` — the dynamic param is more specific than the splat for a
  single segment.
- A request for `/about/something/else` matches `/about/*` — no other pattern
  handles two segments under `/about`, so the splat catches them.
- At the root level, if `/`, `/*`, and `/_index` are all registered, a request
  for `/` matches `/` and `/_index` but **not** `/*` — the splat is pruned when
  better matches exist.
- A request for `/docs` (with those same three patterns) matches `/` and `/*` —
  the root layout plus the splat, since no specific `/docs` pattern exists.

This makes splats useful for fallback pages (like a custom 404), catch-all file
serving, or rendering dynamic content from a CMS:

```go
// Backend
var _ = DefineLoader("/*", func(c *LoaderCtx) (*CatchAllData, error) {
    // c.SplatValues() returns the captured segments
    slug := strings.Join(c.SplatValues(), "/")
    page, err := DB.GetPageBySlug(c.Request().Context(), slug)
    if err != nil {
        return nil, err
    }
    return &CatchAllData{Page: page}, nil
})
```

```ts
// Frontend
route("/*", import("../components/catch-all.tsx"), "CatchAll");
```

### 11.7 What Doesn't Match

Vorma's routing is strict. A few things that might surprise you coming from
other routers:

**The root layout doesn't catch everything.** If you only register `/`, a
request for `/some/random/path` returns a 404. You'd need `/*` (a splat) to
catch arbitrary paths.

**Dynamic params require a non-empty segment.** `/users/:id` matches
`/users/123` but does **not** match `/users/` (trailing slash with no value).

**Unregistered paths are 404s.** If you register `/users` and `/users/:id`, a
request for `/users/123/settings` is a 404 -- there's no pattern that matches
three segments under `/users`.

### 11.8 Parallel Execution and Error Propagation

When Vorma matches multiple nested patterns, their loaders run as tasks in the
same task context (which is why shared dependencies are deduplicated -- we'll
cover this in the tasks section).

Error propagation follows the nesting hierarchy:

- If a **parent** loader fails, its **child** loaders are automatically
  cancelled. There's no point rendering a customer detail page if the dashboard
  layout failed to load.
- If a **child** loader fails, the **parent** is unaffected. The dashboard
  layout still renders, and the child can show an error state.
- **Sibling** loaders don't affect each other. If you had two independent
  loaders at the same level, one failing wouldn't cancel the other.

### 11.9 Frontend Route Declarations

On the frontend, route declarations mirror the backend patterns:

```ts
import { route } from "vorma/buildtime";

route("/", import("../components/layout.tsx"), "Layout");
route("/_index", import("../components/front-page.tsx"), "FrontPage");
route("/item/:id", import("../components/item-page.tsx"), "ItemPage");
route("/submit", import("../components/submit-page.tsx"), "SubmitPage");
```

The first argument must match a backend `DefineLoader` pattern exactly. The
second is usually a dynamic import (Vite code-splits each component). The third
is the exported component name from that module.

Layout components receive an `Outlet` prop via their `RouteProps`:

```tsx
export function Layout(props: RouteProps<"/">) {
	return (
		<div>
			<nav>...</nav>
			<props.Outlet /> {/* child route renders here */}
		</div>
	);
}
```

You can also pass a fourth argument -- an error boundary component -- which
we'll cover later.

### 11.10 Navigation and Data Refetching

When a user navigates between pages, Vorma fetches route data for the target URL
and executes the loaders matched by that URL.

- If you go from `/_index` to `/item/42`, the matched loaders are `/` and
  `/item/:id` (both run).
- If you go from `/item/42` to `/item/99`, the matched loaders are still `/` and
  `/item/:id`, and the item loader gets the new `id`.
- If you go from `/item/42` back to `/_index`, the matched loaders are `/` and
  `/_index`.

The layout component stays mounted throughout -- it never unmounts and remounts.
This is why your navigation bar never flickers and transitions feel instant.

You can force a refetch with `revalidate()` if you need to refresh the current
page data (for example, after a mutation that affects the parent layout).

### 11.11 Build-Time Route Parsing Rules

Vorma's route declarations are compile-time metadata, so static analysis rules
matter:

- **Statically resolvable modules only:** `route("/x", import("./x.tsx"), "X")`
  is fine. Static string module paths are also parseable. Dynamic expressions
  are unresolved.
- **Unresolved module policy:** unresolved route calls are treated as errors
  unless you set `Vorma.UnresolvedRoutePolicy: "warn"` (log + skip).
- **Unique patterns required:** duplicate `pattern` values (even across
  different `*.vorma.routes.ts` files) fail parsing/build.
- **Runtime behavior:** `route()` itself is a no-op function at runtime; all
  meaningful effects happen during build/codegen.

---

## 12. What's in `vorma.gen`?

The generated `frontend/src/vorma.gen/index.ts` file is the bridge between your
Go backend and TypeScript frontend. You never edit it -- Vorma regenerates it on
every build.

It contains:

- **TypeScript types** for every Go struct used as a loader output or action
  input/output (`FrontPageData`, `StoryPageData`, `CreateStoryInput`, etc.)
- **A `routes` constant** listing every loader and action with their patterns
  and phantom types
- **A `VormaApp` type** that encodes your entire route tree
- **A `vormaAppConfig`** constant used by `Link`, `navigate`, and `api` to
  resolve URLs
- **Type aliases** like `RouteProps<P>`, `QueryPattern`, `MutationPattern` that
  make your components type-safe

This is why `Link pattern="/item/:id"` knows it needs a
`params={{ id: string }}` prop -- the pattern metadata is all encoded in the
generated types.

---

## 13. Tasks: The Engine Under the Hood

Now that you have a working app, let's talk about the most powerful primitive in
Vorma's toolkit: **tasks**.

You've actually been using tasks this whole time without knowing it. Every
`DefineLoader` and `DefineAction` call creates a `tasks.Task` internally. When
Vorma handles a request, it runs matched loaders as tasks inside a shared task
context. This is why nested loaders execute in parallel -- they're just tasks
running concurrently within the same context.

But tasks aren't just an implementation detail. They're a general-purpose tool
you can use directly, and they solve three problems that come up in virtually
every web app:

1. **Request-scoped caching** -- run an expensive operation once per request,
   reuse the result everywhere
2. **Thundering-herd protection** -- collapse many concurrent callers onto a
   single execution
3. **Parallel execution with shared dependencies** -- run things concurrently
   while automatically deduplicating the work they share

Let's add all three to our HN clone.

### 13.1 The Basics: What Is a Task?

A task is a function that takes an input and returns an output. The key
property: **same task + same input + same context = runs once**. Call it ten
times, it executes once and returns the cached result to all callers.

```go
import "github.com/vormadev/vorma/kit/tasks"

var FetchUserTask = tasks.NewTask(func(ctx *tasks.Ctx, userID string) (*User, error) {
	return db.GetUser(ctx.NativeContext(), userID)
})
```

You always define tasks as **package-level variables**. This makes dependency
graphs explicit, and Go's initialization-cycle checks will catch accidental
circular dependencies at compile time.

To run a task, you need a `tasks.Ctx` -- a context that tracks which tasks have
already been executed:

```go
ctx := tasks.NewCtx(r.Context())

user, err := FetchUserTask.Run(ctx, "alice")   // executes the function
user2, err := FetchUserTask.Run(ctx, "alice")  // returns cached result instantly
user3, err := FetchUserTask.Run(ctx, "bob")    // different input, executes again
```

In Vorma, you don't create the context yourself -- every incoming HTTP request
already has one. You access it from a loader or action via `c.TasksCtx()`.

### 13.2 Request-Scoped Caching: Load the Current User Once

Our HN clone currently hardcodes `"anon"` as the voter name. Let's add basic
auth so each user has their own identity. We'll keep it intentionally minimal --
just a cookie with a username, no passwords, not how you'd do it in production.
But it's enough to show why request-scoped task caching matters.

First, add a `users` table to the store. Update the `migrate` function in
`backend/src/store/store.go`:

```go
func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			name TEXT PRIMARY KEY,
			karma INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		);

		CREATE TABLE IF NOT EXISTS stories (
			-- ... (same as before)
		);

		-- ... (rest of tables same as before)
	`)
	return err
}
```

Add a user lookup method:

```go
type User struct {
	Name      string `json:"name"`
	Karma     int    `json:"karma"`
}

func (s *Store) GetOrCreateUser(ctx context.Context, name string) (*User, error) {
	_, err := s.db.ExecContext(ctx, `
		INSERT OR IGNORE INTO users (name, karma, created_at) VALUES (?, 0, ?)
	`, name, time.Now().Unix())
	if err != nil {
		return nil, err
	}
	var u User
	err = s.db.QueryRowContext(ctx, `SELECT name, karma FROM users WHERE name = ?`, name).
		Scan(&u.Name, &u.Karma)
	if err != nil {
		return nil, err
	}
	return &u, nil
}
```

Now for the task. Create `backend/src/router/user_tasks.go`:

```go
package router

import (
	"net/http"

	"hn-vorma/backend/src/store"

	"github.com/vormadev/vorma/kit/tasks"
)

// This task runs at most once per (context, username) pair.
// Multiple loaders calling it with the same username in the same request
// will only hit the database once.
var FetchCurrentUserTask = tasks.NewTask(func(ctx *tasks.Ctx, username string) (*store.User, error) {
	if username == "" {
		return nil, nil
	}
	return DB.GetOrCreateUser(ctx.NativeContext(), username)
})

// Helper that extracts the username from the request cookie and runs the task.
func GetCurrentUser(c interface {
	TasksCtx() *tasks.Ctx
	Request() *http.Request
}) (*store.User, error) {
	cookie, err := c.Request().Cookie("hn_user")
	if err != nil || cookie.Value == "" {
		return nil, nil // not logged in
	}
	return FetchCurrentUserTask.Run(c.TasksCtx(), cookie.Value)
}
```

Notice that the task input is `string` (the username), not `*http.Request`. Task
inputs must be `comparable` (they're used as map keys internally), and more
importantly, the input is part of the **cache key** -- it should be the value
that determines the result. The same username always produces the same user, so
`string` is the right key. We extract the cookie in a thin helper function and
pass just the username to the task.

Now use it in your loaders. Update the root data type and loader in `routes.go`:

```go
type RootData struct {
	SiteName    string     `json:"siteName"`
	CurrentUser *store.User `json:"currentUser"`
}

var _ = DefineLoader("/", func(c *LoaderCtx) (*RootData, error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return nil, err
	}
	return &RootData{SiteName: "HN Clone", CurrentUser: user}, nil
})
```

And use it in the upvote action too, so we know _who_ is voting:

```go
var _ = DefineAction("POST", "/stories/:id/upvote",
	func(c *ActionCtx[vorma.None]) (*UpvoteResult, error) {
		user, err := GetCurrentUser(c)
		if err != nil {
			return nil, err
		}
		if user == nil {
			return nil, &vorma.LoaderError{Client: "You must be logged in to vote"}
		}

		storyID, err := strconv.ParseInt(c.Param("id"), 10, 64)
		if err != nil {
			return nil, err
		}

		score, err := DB.Upvote(c.Request().Context(), storyID, user.Name)
		if err != nil {
			return nil, err
		}
		return &UpvoteResult{Score: score}, nil
	},
)
```

You'd also add a simple "login" action that sets the cookie:

```go
type LoginInput struct {
	Name string `json:"name"`
}

type LoginResult struct {
	Name string `json:"name"`
}

var _ = DefineAction("POST", "/login",
	func(c *ActionCtx[LoginInput]) (*LoginResult, error) {
		name := c.Input().Name
		if name == "" {
			return nil, &vorma.LoaderError{Client: "Name is required"}
		}
		c.SetResponseCookie(&http.Cookie{
			Name:     "hn_user",
			Value:    name,
			Path:     "/",
			HttpOnly: true,
			MaxAge:   86400 * 30,
		})
		return &LoginResult{Name: name}, nil
	},
)
```

Here's what happens when a logged-in user navigates to `/item/42`:

1. Vorma matches the `"/"` and `"/item/:id"` loaders
2. Both run **in parallel** (they're tasks in the same context)
3. The root loader calls `GetCurrentUser(c)` →
   `FetchCurrentUserTask.Run(ctx, "alice")`
4. The item loader also calls `GetCurrentUser(c)` →
   `FetchCurrentUserTask.Run(ctx, "alice")`
5. Same task, same input (`"alice"`), same context → the database query runs
   **once**

Internally, deduplication keys by `(task pointer, input value)` within a single
`tasks.Ctx`. Since each request has its own `tasks.Ctx`, the practical behavior
is still: same task + same input + same request context = one execution.

### 13.3 Task Composition and Parallel Execution

Tasks can call other tasks. When they do, shared dependencies are automatically
deduplicated across the full graph. Let's make our HN clone a bit more realistic
by adding a spam check and a karma check that both need the current user:

```go
// Checks if the user is banned from posting.
// Internally calls FetchCurrentUserTask -- if it already ran this request, the
// cached result is reused.
var CheckUserBanStatusTask = tasks.NewTask(func(ctx *tasks.Ctx, username string) (bool, error) {
	user, err := FetchCurrentUserTask.Run(ctx, username)  // <-- reuses cached user
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}
	// In a real app, you'd check a ban list in the database
	return user.Name != "banned_user", nil
})

// Checks if the user has enough karma for an action.
// Also calls FetchCurrentUserTask -- same cache hit.
var CheckKarmaThresholdTask = tasks.NewTask(func(ctx *tasks.Ctx, username string) (bool, error) {
	user, err := FetchCurrentUserTask.Run(ctx, username)  // <-- reuses cached user
	if err != nil {
		return false, err
	}
	if user == nil {
		return false, nil
	}
	return user.Karma >= 10, nil
})
```

Both tasks depend on `FetchCurrentUserTask`. If you run them in parallel with
the same username, the user is still only fetched once:

```go
username := "" // extract from cookie
if cookie, err := c.Request().Cookie("hn_user"); err == nil {
	username = cookie.Value
}

var canPost bool
var hasKarma bool

err := c.TasksCtx().RunParallel(
	CheckUserBanStatusTask.Bind(username, &canPost),
	CheckKarmaThresholdTask.Bind(username, &hasKarma),
)
if err != nil {
	return nil, err
}
```

`RunParallel` executes the bound tasks concurrently. `Bind` pairs a task's input
with a destination variable for the output. If any task returns an error, the
others are cancelled via context cancellation.

Here's the dependency graph for this execution:

```
RunParallel
 ├── CheckUserBanStatusTask("alice") ──┐
 │                                     ├── FetchCurrentUserTask("alice") ← runs ONCE
 └── CheckKarmaThresholdTask("alice") ─┘
```

Three tasks, two run in parallel, one shared dependency executes exactly once.
You get this for free -- no manual caching, no mutexes, no coordination code.

You could use this in the create story action to gate submissions:

```go
var _ = DefineAction("POST", "/stories",
	func(c *ActionCtx[store.CreateStoryInput]) (*CreateStoryResult, error) {
		cookie, err := c.Request().Cookie("hn_user")
		if err != nil || cookie.Value == "" {
			return nil, &vorma.LoaderError{Client: "You must be logged in to post"}
		}
		username := cookie.Value

		var canPost bool
		var hasKarma bool

		err = c.TasksCtx().RunParallel(
			CheckUserBanStatusTask.Bind(username, &canPost),
			CheckKarmaThresholdTask.Bind(username, &hasKarma),
		)
		if err != nil {
			return nil, err
		}
		if !canPost {
			return nil, &vorma.LoaderError{Client: "Your account is suspended"}
		}
		if !hasKarma {
			return nil, &vorma.LoaderError{Client: "You need at least 10 karma to post"}
		}

		storyID, err := DB.CreateStory(c.Request().Context(), c.Input())
		if err != nil {
			return nil, err
		}
		return &CreateStoryResult{StoryID: storyID}, nil
	},
)
```

The ban check and karma check run concurrently, but the underlying user fetch
(which both depend on) only hits the database once.

### 13.4 Thundering-Herd Protection: Global Deduplication with TTL

Request-scoped tasks use a `Ctx` that lives for one request. But you can also
create long-lived contexts with a TTL (time-to-live) for caching across
requests. This is how you get thundering-herd protection.

Imagine our HN clone fetches metadata for submitted URLs -- the page title, a
description, maybe an OpenGraph image. This is an external HTTP call, it's slow,
and if a story is trending, hundreds of requests might try to unfurl the same
URL simultaneously.

Create `backend/src/router/url_metadata.go`:

```go
package router

import (
	"context"
	"time"

	"github.com/vormadev/vorma/kit/tasks"
)

type URLMetadata struct {
	Domain      string `json:"domain"`
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
}

// This context is shared across all requests. Results are cached for 5 minutes.
// After 5 minutes, the next caller triggers a fresh execution, and concurrent
// callers during that execution share the same in-flight result.
var urlMetadataCtx = tasks.NewCtxWithTTL(context.Background(), 5*time.Minute)

var FetchURLMetadataTask = tasks.NewTask(func(ctx *tasks.Ctx, rawURL string) (*URLMetadata, error) {
	// In a real app, this would make an HTTP request to the URL,
	// parse the HTML, and extract OpenGraph tags.
	// For now, we'll just extract the domain.
	return &URLMetadata{Domain: rawURL, Title: "Example Page"}, nil
})

// FetchURLMetadata is the function you call from loaders/actions.
// It bridges the global cache with the per-request cancellation context.
func FetchURLMetadata(r context.Context, rawURL string) (*URLMetadata, error) {
	// WithNativeContext creates a child context that shares the global cache
	// but respects this specific request's cancellation/deadline.
	requestScopedCtx := urlMetadataCtx.WithNativeContext(r)
	return FetchURLMetadataTask.Run(requestScopedCtx, rawURL)
}
```

Let's break down what's happening:

**`tasks.NewCtxWithTTL(context.Background(), 5*time.Minute)`** creates a
long-lived context. Unlike request-scoped contexts that die when the request
ends, this one persists across all requests. Cached results expire after 5
minutes.

**`urlMetadataCtx.WithNativeContext(r)`** is the key bridge. It creates a child
context that:

- **Shares the same cache** as the parent (so results are shared across
  requests)
- **Uses the request's context** for cancellation (so if this specific request
  is cancelled, it doesn't kill the task for everyone else)

This means:

- Request A asks for metadata for `https://example.com` -- the task runs
- Requests B, C, D ask for the same URL while A is still running -- they wait
  for A's result (no duplicate work)
- Request E asks 3 minutes later -- gets the cached result (within TTL)
- Request F asks 6 minutes later -- the cache has expired, a fresh execution
  starts
- If Request B gets cancelled (client disconnected), it doesn't cancel the
  in-flight work that A, C, and D are also waiting on

Use it in the item page loader to show URL metadata:

```go
var _ = DefineLoader("/item/:id", func(c *LoaderCtx) (*StoryPageData, error) {
	// ... existing story + comments fetching ...

	// If the story has a URL, fetch its metadata (cached globally)
	if story.URL != "" {
		metadata, err := FetchURLMetadata(c.Request().Context(), story.URL)
		if err == nil {
			// Use metadata for OpenGraph tags, preview cards, etc.
			h := c.HeadEls()
			if metadata.Title != "" {
				h.MetaPropertyContent("og:title", metadata.Title)
			}
		}
	}

	return &StoryPageData{Story: *story, Comments: comments}, nil
})
```

### 13.5 When to Use Which Pattern

Here's a simple decision guide:

**Request-scoped tasks** (no TTL, one `Ctx` per request):

- "Load the current user" -- needed by multiple loaders/middleware in the same
  request
- "Check permissions" -- called by several route handlers
- Any data that's specific to this request and shouldn't leak to other requests

**Global tasks with TTL** (long-lived `Ctx`, shared across requests):

- "Fetch URL metadata" -- expensive external HTTP call, same URL returns the
  same result
- "Check anti-spam service" -- external API with rate limits
- "Fetch feature flags" -- same for all users, refreshed periodically
- Any call where slightly stale data is acceptable and upstream load matters

**The mental model**: request-scoped tasks prevent duplicate work _within_ a
request. Global tasks with TTL prevent duplicate work _across_ requests.

### 13.6 How Vorma Uses Tasks Internally

You now know enough to understand what Vorma is actually doing under the hood:

1. When a request arrives, Vorma creates a `tasks.NewCtx(r.Context())` -- a
   fresh, request-scoped task context.

2. It matches the URL against nested routes. For `/item/42`, that matches `"/"`
   and `"/item/:id"`.

3. Both loaders are **tasks**. Vorma runs them in parallel using `RunParallel`:

    ```
    RunParallel
     ├── Loader "/"          → RootData
     └── Loader "/item/:id"  → StoryPageData
    ```

4. If both loaders call `FetchCurrentUserTask.Run(ctx, "alice")`, the user is
   fetched once.

5. Task middleware (if you add any) also runs in this same context, in parallel,
   before the loaders. Shared dependencies between middleware and loaders are
   deduplicated automatically.

6. The results are collected, serialized to JSON, and sent to the frontend.

This is why Vorma can run your loaders and middleware in parallel without you
having to think about coordination -- the task context handles deduplication,
and `RunParallel` handles concurrency. You just write functions that call other
functions, and the framework makes it efficient.

---

## 14. Validation

Our HN clone accepts user input for creating stories and comments, but right now
there's no validation -- someone could submit an empty title or a garbage URL.
Let's fix that with Vorma's `validate` package.

### 14.1 The Validator Interface

Vorma uses a simple interface for validation:

```go
type Validator interface {
    Validate() error
}
```

If your action input struct implements `Validator`, Vorma calls `Validate()`
automatically after parsing the JSON body. If it returns an error, Vorma sends a
400 response with the error message -- your action handler never runs.

This gives you something similar to tRPC: validation runs on the Go side, type
safety lives on the TypeScript side, and the boundary between them is automatic.

### 14.2 Adding Validation to the HN Clone

Let's validate story submissions. Update `CreateStoryInput` in
`backend/src/store/store.go` to implement `Validator`:

```go
import "github.com/vormadev/vorma/kit/validate"

func (in CreateStoryInput) Validate() error {
    return validate.Object(in).
        Required("Title").Min(3).Max(200).
        Required("By").Min(1).Max(50).
        Optional("URL").URL().
        Optional("Text").Max(10000).
        MutuallyExclusive("content", "URL", "Text").
        Error()
}
```

That's it. Now if someone submits a story with an empty title, a URL that isn't
a valid URL, or both a URL _and_ text (which HN doesn't allow), they'll get a
400 error with a descriptive message before your handler code even runs.

Let's add comment validation too:

```go
func (in CreateCommentInput) Validate() error {
    return validate.Object(in).
        Required("By").Min(1).Max(50).
        Required("Text").Min(1).Max(5000).
        Error()
}
```

### 14.3 The Fluent API

`validate.Object(input)` starts a fluent validation chain for a struct. You
check fields with `Required("FieldName")` or `Optional("FieldName")`, then chain
validators onto each field:

| Validator                                | What it checks                                              |
| ---------------------------------------- | ----------------------------------------------------------- |
| `.Required("Field")`                     | Field must be non-zero (non-empty string, non-nil, etc.)    |
| `.Optional("Field")`                     | If zero, skip remaining checks; if non-zero, continue       |
| `.Email()`                               | Valid email address                                         |
| `.URL()`                                 | Valid URL                                                   |
| `.Min(n)`                                | Minimum value (numbers) or minimum length (strings, slices) |
| `.Max(n)`                                | Maximum value or length                                     |
| `.RangeInclusive(min, max)`              | Value between min and max (both inclusive)                  |
| `.In([]string{"a", "b", "c"})`           | Value must be one of the allowed values                     |
| `.Regex(re)`                             | String must match the regex                                 |
| `.MutuallyExclusive("label", fields...)` | At most one of these fields can be set                      |
| `.MutuallyRequired("label", fields...)`  | Either all or none of these fields must be set              |

The chain ends with `.Error()`, which returns a `*validate.ValidationError`
containing all accumulated errors (joined together), or `nil` if everything
passed.

The key ergonomic detail: **`Required` short-circuits.** If a required field is
missing, subsequent validators on that field don't run. You won't get both
"Title is required" and "minimum length for Title is 3" -- just the first.

### 14.4 Standalone Value Validation

You can also validate individual values outside of a struct with
`validate.Any()`:

```go
err := validate.Any("age", userAge).Required().RangeInclusive(13, 150).Error()
```

This is useful in action handlers where you need to validate a single value
extracted from the URL or computed from logic, rather than a whole struct.

### 14.5 How Validation Errors Reach the Frontend

When `Validate()` returns an error, Vorma wraps it as a
`*validate.ValidationError` and sends a `400 Bad Request` response. On the
frontend, `api.mutate()` returns `{ success: false, error: string }` for failed
responses. In the current client runtime, non-2xx responses are surfaced as the
HTTP status code string (for example, `"400"`).

You can use this in your submit page to show errors:

```tsx
const result = await api.mutate({
	pattern: "/stories",
	input: { title, url, text, by },
});

if (!result.success) {
	if (result.error === "400") {
		setError("Please check the form fields and try again.");
	} else {
		setError("Request failed. Please try again.");
	}
	return;
}
```

---

## 15. Response Control

You've already seen `c.SetResponseCookie()` in the login action. The response
proxy is Vorma's way of letting loaders and actions modify the HTTP response --
status codes, headers, cookies, and redirects -- without writing directly to the
response writer.

In day-to-day code, you'll usually use the convenience methods directly on
`LoaderCtx` / `ActionCtx` (`c.Redirect`, `c.SetResponseStatus`, etc.), which
write to that same underlying proxy.

### 15.1 The Response Proxy

Every loader and action has access to a response proxy via `c.ResponseProxy()`.
The proxy accumulates response modifications without applying them immediately.
This is important because loaders run in parallel -- Vorma needs to collect all
their modifications and merge them before sending the response.

```go
proxy := c.ResponseProxy()
```

### 15.2 Convenience Helpers on `c`

Vorma exposes response helpers directly on request data:

```go
c.SetResponseStatus(http.StatusNotFound)
c.SetResponseHeader("X-Custom-Header", "my-value")
c.AddResponseHeader("X-Trace", "route-a")
c.SetResponseCookie(&http.Cookie{Name: "session", Value: "abc"})
c.Redirect("/login", http.StatusSeeOther)
```

These are thin wrappers around `c.ResponseProxy()`, but they keep handler code
clean.

### 15.3 Redirects

Use `c.Redirect()` (or `c.ResponseProxy().Redirect(...)`) to redirect from a
loader or action:

```go
var _ = DefineLoader("/dashboard", func(c *LoaderCtx) (*DashboardData, error) {
	user, err := GetCurrentUser(c)
	if err != nil {
		return nil, err
	}
	if user == nil {
		// Not logged in -- redirect to home page
		c.Redirect("/", http.StatusSeeOther)
		return nil, nil
	}
	return &DashboardData{User: user}, nil
})
```

The default status code is `303 See Other` if you omit it. You can pass any 3xx
status code.

**Client-side vs server-side redirects:** When the Vorma client fetches loader
data, it sends an `X-Accepts-Client-Redirect` header. If the server sees this
header, it returns a `200` response with an `X-Client-Redirect` header instead
of a traditional 3xx redirect. The client then navigates to the redirect URL via
JavaScript. This is how Vorma handles redirects gracefully during client-side
navigation -- the page doesn't flash or lose state.

For direct browser requests (initial page load, non-JS clients), Vorma falls
back to a standard HTTP redirect.

### 15.4 Status Codes and Headers

```go
// Set the HTTP status code
c.SetResponseStatus(http.StatusNotFound)

// Set a response header (replaces any previous value for this key)
c.SetResponseHeader("X-Custom-Header", "my-value")

// Add a response header (appends to existing values)
c.AddResponseHeader("X-Processed-By", "auth-service")
```

### 15.5 Cookies

You've already seen this in the login action:

```go
c.SetResponseCookie(&http.Cookie{
    Name:     "hn_user",
    Value:    name,
    Path:     "/",
    HttpOnly: true,
    MaxAge:   86400 * 30,
})
```

Cookies set in loaders and actions are accumulated in the proxy and applied to
the response after all handlers complete.

### 15.6 Proxy Merging

Because loaders run in parallel, each one gets its own response proxy. After all
matched loaders complete, Vorma merges the proxies with these rules:

- **Status:** The first error status (4xx/5xx) wins. Otherwise the last success
  status wins.
- **Headers:** Merged in order. `SetHeader` clears previous values for that key;
  `AddHeader` appends.
- **Cookies:** Later cookies with the same name override earlier ones.
- **Redirects:** The first redirect wins (unless an error status is also set, in
  which case the error takes precedence).

In practice, you rarely need to think about merging. The most common pattern is
a single loader or action setting a redirect or a cookie. But it's good to know
the rules if you have multiple nested loaders setting headers.

### 15.7 Inspecting Proxy State

`ResponseProxy` also has read helpers that are useful in shared helpers and
tests:

```go
proxy := c.ResponseProxy()

statusCode, statusText := proxy.Status()
location := proxy.Location()
allTraceValues := proxy.Headers("X-Trace")
firstTraceValue := proxy.Header("X-Trace")
cookies := proxy.Cookies()

if proxy.IsError() {
	// ...
}
if proxy.IsRedirect() {
	// ...
}
```

Write helpers (`SetHeader`, `SetStatus`, `Redirect`, etc.) are still the common
case, but these read APIs are available when you need to inspect accumulated
state.

---

## 16. Client-Side APIs

Let's enhance the HN clone's user experience with Vorma's client-side APIs.

### 16.1 Global Loading Indicator

Vorma does not render a global loading bar by default. The loading indicator API
is opt-in so you can wire it to your own UI (NProgress, a top bar, a spinner,
etc.).

When a user navigates between pages, there's a brief delay while loader data is
fetched. Let's show a loading bar. In `frontend/src/vorma.entry.tsx`, add:

```tsx
import { createRoot } from "react-dom/client";
import {
	getRootEl,
	initClient,
	setupGlobalLoadingIndicator,
} from "vorma/client";
import { VormaRootOutlet } from "vorma/react";
import { vormaAppConfig } from "./vorma.gen/index.ts";

await initClient({
	vormaAppConfig,
	renderFn: () => {
		createRoot(getRootEl()).render(<VormaRootOutlet />);
	},
});

// Show a simple top bar during navigations
const cleanupGlobalLoadingIndicator = setupGlobalLoadingIndicator({
	start: () => {
		document.body.classList.add("loading");
	},
	stop: () => {
		document.body.classList.remove("loading");
	},
	isRunning: () => {
		return document.body.classList.contains("loading");
	},
});
```

The config object has three required callbacks: `start()`, `stop()`, and
`isRunning()`. You can wire these up to NProgress, a custom progress bar, or
anything else. `setupGlobalLoadingIndicator(...)` returns a cleanup function;
call it if you need to unregister listeners (for example in HMR teardown or a
custom mount/unmount integration).

Optional settings:

- **`include`**: Filter which operations trigger the indicator. Defaults to
  `"all"`. You can also pass an explicit list like
  `["navigations", "submissions", "revalidations"]` to be selective.
- **`startDelayMS`**: Milliseconds to wait before calling `start()` (default:
  12ms). Prevents flash for fast navigations.
- **`stopDelayMS`**: Milliseconds to wait before calling `stop()` (default:
  12ms).

### 16.2 Revalidate on Window Focus

Add this after `setupGlobalLoadingIndicator`:

```tsx
import { revalidateOnWindowFocus } from "vorma/client";

const cleanupFocusRevalidation = revalidateOnWindowFocus({
	staleTimeMS: 5000,
});
```

Now when a user switches away from the browser tab and comes back, Vorma
automatically refetches the current page's data if it's been more than 5 seconds
(the default). This keeps your HN front page fresh without requiring a manual
refresh. Like the loading indicator setup, this also returns a cleanup function.

### 16.3 Manual Revalidation

You can also trigger a revalidation programmatically:

```tsx
import { revalidate } from "vorma/client";

// After some side effect that might have changed data
await revalidate();
```

This refetches all matched loaders for the current URL without changing the page
or history. Useful after a mutation that might affect the current page's data --
for example, if you delete a comment and want the comment count to update.

### 16.4 Checking Navigation Status

`getStatus()` tells you what the client is currently doing:

```tsx
import { getStatus } from "vorma/client";

const status = getStatus();
// status.isNavigating   -- true while route data is being fetched
// status.isSubmitting   -- true while an action is in-flight
// status.isRevalidating -- true during a background revalidation
```

This is what `setupGlobalLoadingIndicator` uses internally. You can use it
directly if you need more fine-grained control.

### 16.5 Error Boundaries

The `route()` function accepts an optional fourth argument: an error boundary
component.

```ts
import { route } from "vorma/buildtime";

route(
	"/item/:id",
	import("../components/item-page.tsx"),
	"ItemPage",
	"ItemPageError", // 4th argument: error boundary export name
);
```

The error boundary component receives an `error` prop with the error message:

```tsx
export function ItemPageError(props: { error: string }) {
	return (
		<div style={{ padding: 16, color: "#c00" }}>
			<h2>Something went wrong</h2>
			<p>{props.error}</p>
			<Link pattern="/_index">Back to front page</Link>
		</div>
	);
}
```

If the `/item/:id` loader fails (e.g., story not found), the error boundary
renders instead of the `ItemPage` component. The parent layout still renders
normally -- only the erroring route gets replaced.

You can also set a default error boundary for all routes in `vorma.entry.tsx`:

```tsx
await initClient({
	vormaAppConfig,
	renderFn: () => {
		createRoot(getRootEl()).render(<VormaRootOutlet />);
	},
	defaultErrorBoundary: (props: { error: string }) => {
		return <div>Error: {props.error}</div>;
	},
});
```

### 16.6 Intent-Based Prefetching

You've already been using prefetching without knowing it. The `Link` component
in `vorma.bindings.ts` is configured with `prefetch: "intent"` by default:

```tsx
export const Link = makeTypedLink(vormaAppConfig, { prefetch: "intent" });
```

When a user hovers over (or touches) a link, Vorma starts prefetching the loader
data for that route. By the time they click, the data is often already loaded
and the navigation feels instant.

You can customize the prefetch delay per-link:

```tsx
<Link pattern="/item/:id" params={{ id: "42" }} prefetchDelayMs={200}>
	Story title
</Link>
```

The default delay is 100ms -- long enough to avoid prefetching on accidental
mouse-overs, short enough to feel responsive for intentional hovers.

### 16.7 Link Lifecycle Hooks

For advanced control over what happens during a link click, you can use
lifecycle hooks:

```tsx
<Link
	pattern="/item/:id"
	params={{ id: "42" }}
	beforeBegin={(event) => {
		// Fires before navigation starts. Good for closing modals,
		// saving form state, or analytics.
	}}
	beforeRender={(event) => {
		// Fires after data is fetched, before the new component renders.
		// Good for animation setup.
	}}
	afterRender={(event) => {
		// Fires after the new component has rendered.
		// Good for focus management or post-render effects.
	}}
>
	Story title
</Link>
```

All three are optional and can be async.

Both `Link` and `navigate()` accept additional options beyond `pattern` and
`params`:

- **`replace`**: Use `replace` history instead of pushing a new entry. The user
  can't press Back to return to the current page.
- **`scrollToTop`**: Set to `false` to prevent scrolling to the top of the page
  after navigation. Defaults to `true`.
- **`state`**: Pass arbitrary data via the History API's state object.
  Accessible on the next page via `window.history.state`.
- **`search` / `hash`**: Add query strings and hash fragments.

For `Link`, these are props:

```tsx
<Link
	pattern="/_index"
	replace
	scrollToTop={false}
	search="?tab=top"
	hash="#stories"
	state={{ fromUpvote: true }}
>
	top
</Link>
```

For `navigate()`, they're options:

```tsx
await navigate({
	pattern: "/item/:id",
	params: { id: "42" },
	replace: true,
	scrollToTop: false,
	search: "?tab=comments",
	hash: "#latest",
	state: { fromUpvote: true },
});
```

For explicit index routes, typed navigation is permissive on purpose:

```tsx
// If your real loader pattern is "/dashboard/_index",
// you can navigate/link to either:
await navigate({ pattern: "/dashboard/_index" });
await navigate({ pattern: "/dashboard" });
```

All typed URL resolution is fail-fast at runtime:

- Missing required params throw.
- Unexpected extra params throw.
- Missing required splat values throw.
- Supplying splat values to a non-splat route throws.

### 16.8 `api.query()` and `api.mutate()` — Actions Beyond POST

So far we've used `api.mutate()` for POST actions. But actions aren't limited to
POST -- you can define actions with any HTTP method:

```go
// A GET action -- useful for data fetching that doesn't fit the loader model
var _ = DefineAction("GET", "/search",
    func(c *ActionCtx[SearchInput]) (*SearchResults, error) {
        // ...
    },
)

// A DELETE action
var _ = DefineAction("DELETE", "/stories/:id",
    func(c *ActionCtx[vorma.None]) (vorma.None, error) {
        // ...
    },
)

// A PUT action
var _ = DefineAction("PUT", "/stories/:id",
    func(c *ActionCtx[UpdateStoryInput]) (*UpdateStoryResult, error) {
        // ...
    },
)
```

On the frontend, GET actions are called with `api.query()`, and everything else
uses `api.mutate()`:

```tsx
// GET action -- sends input as URL search parameters
const result = await api.query({
	pattern: "/search",
	input: { q: "golang", page: 1 },
});

// DELETE action -- you must specify the method in requestInit
const result = await api.mutate({
	pattern: "/stories/:id",
	params: { id: "42" },
	requestInit: { method: "DELETE" },
});

// PUT action
const result = await api.mutate({
	pattern: "/stories/:id",
	params: { id: "42" },
	input: { title: "Updated Title" },
	requestInit: { method: "PUT" },
});
```

The key difference: `api.query()` sends input as URL search params (appropriate
for GET requests). `api.mutate()` sends input as a JSON body (appropriate for
POST, PUT, DELETE, PATCH, etc.). For POST actions, the method defaults to POST
so you don't need `requestInit`. For other methods, you specify it explicitly.

Both return `{ success: true, data: T } | { success: false, error: string }`
with fully typed `data`.

One important contract for `api.query()`: query input root must be an object,
`null`, or `undefined`. Primitive roots (string/number/boolean) and arrays are
rejected.

### 16.9 `submit()` — Low-Level Action Submission

For fine-grained control over submissions, use the `submit()` function directly:

```tsx
import { submit } from "vorma/client";

const result = await submit<UpvoteResult>(
	"/api/stories/42/upvote",
	{
		method: "POST",
	},
	{
		dedupeKey: "upvote-42",
		skipGlobalLoadingIndicator: true,
		revalidate: false,
	},
);
```

Options:

- **`dedupeKey`**: If two submissions share the same dedupe key, the earlier one
  is cancelled. Useful for rapid-fire actions like clicking an upvote button
  repeatedly -- only the latest submission goes through.
- **`skipGlobalLoadingIndicator`**: Don't trigger the global loading bar for
  this submission. Useful for background syncs or auto-saves.
- **`revalidate`**: Whether to refetch the current page's data after the
  submission completes. Defaults to `true`. Set to `false` for actions that
  don't affect the current page (like analytics events).

`submit()` only supports same-origin targets. Passing an external URL throws.

### 16.10 Client Loaders

For cases where you need to fetch or transform data on the client side,
`addClientLoader()` lets you add a client-side data-fetching layer on top of the
server loader:

```tsx
import { addClientLoader } from "../vorma.bindings.ts";

export const useItemClientData = addClientLoader({
	pattern: "/item/:id",
	clientLoader: async ({ params, serverDataPromise, signal }) => {
		// Wait for the server data
		const serverData = await serverDataPromise;
		if (signal.aborted) {
			return serverData.loaderData;
		}

		// Augment with client-side data
		const localBookmarks = localStorage.getItem("bookmarks");
		const isBookmarked = localBookmarks?.includes(params.id);

		return {
			...serverData.loaderData,
			isBookmarked,
		};
	},
});
```

The client loader receives the server's data via `serverDataPromise` and can
augment or transform it. `useLoaderData()` still returns the server loader data;
the value returned by `addClientLoader(...)` is accessed through the hook it
returns:

```tsx
const serverData = useLoaderData(props);
const clientData = useItemClientData(props);
const isBookmarked = clientData?.isBookmarked ?? false;
```

The returned hook has two usage patterns:

- **`useItemClientData(props)`**: Use this in the matching route component when
  you have `RouteProps<"...">`. This is the strict route-bound form.
- **`useItemClientData()`**: Use this in shared components that may render in
  multiple routes. This returns `... | undefined`, so handle the `undefined`
  case.

If you pass `route props` to a client-loader hook, they must correspond to that
same route pattern.

Client-loader execution model details:

- Client loaders run speculatively during prefetch/navigation.
- If navigation ownership changes (newer navigation wins), in-flight work can be
  discarded.
- `signal` is your cancellation channel. Forward it to any async work you do
  (`fetch(..., { signal })`, etc.).
- `serverDataPromise` may reject when server loader data becomes unavailable
  (abandoned/failed navigation).
- Registering the same `pattern` again overwrites the previous client loader for
  that pattern (intentional behavior for HMR/module replacement).

Client loaders also run during prefetch, so successful augmented data is often
ready before the user clicks.

### 16.11 Hook Usage Matrix (What to Call Where)

For React projects, this is the practical guide:

| What you need                                              | Hook call                          | Return shape                 | Type-safety notes                                                                    |
| ---------------------------------------------------------- | ---------------------------------- | ---------------------------- | ------------------------------------------------------------------------------------ |
| Current route's loader output                              | `useLoaderData(props)`             | Exact loader output type     | Strongest route-local typing. Requires matching `RouteProps<"...">`.                 |
| Root data + router metadata from a route component         | `useRouterData(props)`             | `{ rootData, params, ... }`  | `params` are typed for that route pattern.                                           |
| Root data + typed params without route props               | `useRouterData<"/item/:id">()`     | `{ rootData, params, ... }`  | Useful in shared components where you still want param narrowing for one pattern.    |
| Root data + router metadata from generic shared components | `useRouterData()`                  | `{ rootData, params, ... }`  | `params` are a generic string map.                                                   |
| Read a specific matched loader by pattern (no route props) | `usePatternLoaderData("/pattern")` | Loader output or `undefined` | Pattern string is type-checked; returns `undefined` when that pattern isn't matched. |
| Read client-loader output in matching route                | `useYourClientLoader(props)`       | Typed value                  | Route-props contract is enforced; mismatched props throw.                            |
| Read client-loader output from shared components           | `useYourClientLoader()`            | Typed value or `undefined`   | Optional lookup style for components reused across routes.                           |

Examples:

```tsx
const routeData = useLoaderData(props);
const routerDataForRoute = useRouterData(props);
const routerDataTypedNoProps = useRouterData<"/item/:id">();
const routerDataAnywhere = useRouterData();
const maybeRootData = usePatternLoaderData("/");

const strictClientData = useItemClientData(props);
const maybeClientData = useItemClientData();
```

All of these are still React hooks, so normal Rules of Hooks apply (top-level
only, no conditionals or loops).

### 16.12 Optional: Global Request Decoration

If your app needs shared request decoration (for example auth headers or
credentials), configure it in `makeTypedAPIClient` inside
`frontend/src/vorma.bindings.ts`:

```tsx
export const api = makeTypedAPIClient(vormaAppConfig, (ctx) => {
	if (ctx.type === "mutation" && ctx.pattern.startsWith("/stories")) {
		return {
			credentials: "include",
		};
	}
	return undefined;
});
```

This decorator applies to both `api.query()` and `api.mutate()`. Per-call
`requestInit` still overrides decorator values when needed.

The `ctx` object tells you what request is being built:

- `ctx.type`: `"query"` or `"mutation"`.
- `ctx.pattern`: the typed route/action pattern.
- `ctx.requestInit`: callsite overrides passed by the current `api.query` /
  `api.mutate` call.
- `ctx.input`: the typed action/query input value for this call.

Use these fields to keep policy logic in one place. Example patterns:

```tsx
export const api = makeTypedAPIClient(vormaAppConfig, (ctx) => {
	if (ctx.type === "mutation") {
		return {
			credentials: "include",
			headers: { "X-CSRF-Token": readCSRFToken() },
		};
	}

	if (ctx.pattern === "/search" && ctx.input !== undefined) {
		return {
			headers: { "X-Request-Source": "search-ui" },
		};
	}

	return undefined;
});
```

Important detail: the decorator returns `RequestInit` overrides for transport
policy (headers, credentials, cache, mode, etc.). Per-call `requestInit` is
still applied after decorator values, so callsite-specific overrides win.

The generated API client intentionally stays on `api.query()` / `api.mutate()`
for day-to-day app code.

### 16.13 Runtime Event Listeners

For advanced instrumentation and integrations, Vorma exposes low-level runtime
listeners:

```tsx
import {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
} from "vorma/client";

const cleanupStatus = addStatusListener((event) => {
	console.log(event.detail.isNavigating);
});

const cleanupRouteChange = addRouteChangeListener(() => {
	console.log("route committed");
});

const cleanupBuildID = addBuildIDListener((event) => {
	console.log("build id changed", event.detail.oldID, event.detail.newID);
});

const cleanupLocation = addLocationListener(() => {
	console.log("location changed");
});
```

Each returns an unsubscribe function. These are useful for analytics, telemetry,
debugging overlays, or app-specific runtime coordination.

---

## 17. TypeScript Generation Customization

Vorma automatically generates TypeScript types from your Go structs. But
sometimes you need more control -- custom type names, browser-native types, or
extra TypeScript code in the generated file.

### 17.1 `AdHocType` — Register Custom Types

If you have Go types that aren't used as loader outputs or action inputs but
still need TypeScript definitions, register them as ad hoc types:

```go
var App = vorma.NewVormaApp(vorma.VormaAppConfig{
    Wave: backend.Wave,
    AdHocTypes: []*vorma.AdHocType{
        {
            TypeInstance: PagedResult[store.Story]{},
            TSTypeName:   "PagedStoryResult",
        },
    },
    // ...
})
```

Vorma will generate a TypeScript type for `PagedStoryResult` in `vorma.gen`,
even though no loader or action directly returns it. This is useful for shared
types used in utility functions or client-side logic.

### 17.2 `ts_type` Struct Tag — Override a Field's Type

Add a `ts_type` tag to override the generated TypeScript type for a specific
field:

```go
type StoryPageData struct {
    Story    store.Story     `json:"story"`
    Comments []store.Comment `json:"comments"`
    EditedAt string          `json:"editedAt" ts_type:"ISO8601DateTime"`
}
```

The `editedAt` field will be typed as `ISO8601DateTime` in TypeScript instead of
`string`. This is useful when the Go type doesn't capture the full semantics of
the value.

### 17.3 `TSTyper` Interface — Override Multiple Fields

For more complex overrides, implement the `TSTyper` interface on your struct:

```go
type TSTyper interface {
    TSType() map[string]string
}
```

```go
type UserProfile struct {
    ID     string
    Email  string
    Status string
}

func (u UserProfile) TSType() map[string]string {
    return map[string]string{
        "ID":     "UUID",
        "Email":  "EmailAddress",
        "Status": "'active' | 'inactive' | 'pending'",
    }
}
```

The returned map overrides field types by name. Fields not in the map use
standard reflection. `TSType()` takes precedence over `ts_type` struct tags.

### 17.4 `TSTyperRaw` — Raw TypeScript Type

For types that should map to a browser-native or entirely custom TypeScript
type, implement `TSTyperRaw`:

```go
type TSTyperRaw interface {
    TSTypeRaw() string
}
```

Vorma uses this internally for `vorma.FormData`:

```go
type FormData struct{}

func (m FormData) TSTypeRaw() string { return "FormData" }
```

When you use `vorma.FormData` as an action input, the generated TypeScript type
is the browser's native `FormData` -- not an empty object. This bypasses
reflection entirely and uses the raw string as the type.

### 17.5 `vorma.FormData` — File Uploads

To accept file uploads in an action, use `vorma.FormData` as the input type:

```go
type UploadResult struct {
    FileName string `json:"fileName"`
    Size     int64  `json:"size"`
}

var _ = DefineAction("POST", "/upload",
    func(c *ActionCtx[vorma.FormData]) (*UploadResult, error) {
        file, header, err := c.Request().FormFile("file")
        if err != nil {
            return nil, err
        }
        defer file.Close()

        // Process the file...
        return &UploadResult{
            FileName: header.Filename,
            Size:     header.Size,
        }, nil
    },
)
```

On the frontend, TypeScript knows the input type is `FormData`:

```tsx
const formData = new FormData();
formData.append("file", fileInput.files[0]);

const result = await api.mutate({
	pattern: "/upload",
	input: formData,
});
```

### 17.6 `ExtraTSCode` — Inject Raw TypeScript

For custom type aliases, utility types, or anything else that should live in the
generated file:

```go
var App = vorma.NewVormaApp(vorma.VormaAppConfig{
    Wave: backend.Wave,
    ExtraTSCode: `
export type ISO8601DateTime = string & { readonly __iso: true };
export type UUID = string & { readonly __uuid: true };
export type EmailAddress = string & { readonly __email: true };
`,
    // ...
})
```

This raw TypeScript is appended to the end of `vorma.gen/index.ts`. Use it for
branded types, type guards, or anything your `ts_type` and `TSTyper` overrides
reference.

---

## 18. Middleware

### Adding Middleware in `init.go`

The `init.go` file is where you add HTTP middleware to the global request
pipeline.

```go
func Init() (addr string, handler http.Handler) {
    r := App.MustInitWithDefaultRouter()

    // Order matters: first registered = outermost (runs first on request,
    // last on response)
    r.AddGlobalHTTPMiddleware(App.MustStaticMiddleware())
    r.AddGlobalHTTPMiddleware(healthcheck.Healthz)

    // Add your own middleware here
    r.AddGlobalHTTPMiddleware(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Before handler
            start := time.Now()
            next.ServeHTTP(w, r)
            // After handler
            log.Printf("%s %s %s", r.Method, r.URL.Path, time.Since(start))
        })
    })

    return App.ServerAddr(), r
}
```

Order details that matter in practice:

- First registered global middleware is outermost (runs first on request, last
  on response).
- `App.MustStaticMiddleware()` short-circuits matching asset requests and does
  not call `next` for those requests.
- So if static middleware is first (as in the scaffold), later middleware may
  not run for static assets. Dynamic loader/action requests still flow through
  your full middleware chain.

Middleware uses the standard `func(http.Handler) http.Handler` signature, so any
Go middleware library (Chi, gorilla/mux, etc.) works out of the box:

```go
import chimw "github.com/go-chi/chi/v5/middleware"

r.AddGlobalHTTPMiddleware(chimw.Logger)
r.AddGlobalHTTPMiddleware(chimw.Recoverer)
r.AddGlobalHTTPMiddleware(chimw.Compress(5))
```

### 18.1 Third-Party Router Integration

If you run Vorma inside another router stack, wrap the outer handler with
`vorma.EnableThirdPartyRouter(...)` so request task context is injected:

```go
import "github.com/go-chi/chi/v5"

outer := chi.NewRouter()
outer.Mount("/", App.MustInitWithDefaultRouter())

finalHandler := vorma.EnableThirdPartyRouter(outer)
```

This keeps Vorma task-aware behavior working correctly in non-default router
setups, including request-scoped task context features (`c.TasksCtx()`, task
middleware, task deduplication).

---

## 19. Production and Deployment

### 19.1 Building for Production

For production builds, use the scaffolded `build` script. It runs codegen,
builds frontend assets with Vite, and compiles the Go binary.

```sh
# Use the package manager you selected:
npm run build
# or: pnpm build / yarn build / bun build

# Equivalent direct command:
go run ./backend/cmd/build
```

By default, the output binary is `backend/dist/main`.

### 19.2 How Embedding Works: `wave.dev.go` vs `wave.prod.go`

The scaffolder generates two files that control how assets are loaded:

**`backend/wave.dev.go`** (active by default):

```go
//go:build !prod

package backend

import (
    "os"
    "github.com/vormadev/vorma/kit/fsutil"
    "github.com/vormadev/vorma/wave"
)

var Wave = wave.New(wave.Config{
    WaveConfigJSON: fsutil.MustReadFile(os.DirFS("backend"), "wave.config.json"),
    DistStaticFS:   fsutil.MustSub(os.DirFS("backend"), "dist", "static"),
})
```

In dev mode, assets are read from the filesystem on each request. This supports
hot reloading -- change a file, and the server picks it up immediately.

**`backend/wave.prod.go`** (active with `-tags prod`):

```go
//go:build prod

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

The `//go:embed` directive bakes the entire `dist/static/` directory and
`wave.config.json` into the binary at compile time. The `embed.FS` implements
the same `fs.FS` interface, so the rest of the code doesn't know or care whether
it's reading from disk or from the binary.

The build tag (`//go:build prod`) ensures only one file is active at a time.
`go build` (without flags) uses `wave.dev.go`. `go build -tags prod` uses
`wave.prod.go`.

### 19.3 Dockerfile

Here's a multi-stage Dockerfile for deploying the HN clone. This example uses
`npm` for dependency install; if you chose another package manager, swap that
one line.

```dockerfile
# Stage 1: Build
FROM golang:1.24 AS builder
WORKDIR /app

# Install Node.js
RUN curl -fsSL https://deb.nodesource.com/setup_22.x | bash -
RUN apt-get install -y nodejs

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy everything and build
COPY . .

# Install JS dependencies
RUN npm ci

# Build assets/codegen (no binary), then compile binary
RUN go run ./backend/cmd/build --no-binary
RUN CGO_ENABLED=0 GOOS=linux go build -mod=readonly \
    -o ./backend/dist/main ./backend/cmd/serve

# Stage 2: Run
FROM alpine
RUN apk --no-cache add ca-certificates
RUN adduser -D appuser
USER appuser
COPY --from=builder /app/backend/dist/main /main
ENTRYPOINT ["/main"]
```

The builder stage has Go, Node.js, and all your source code (~1GB+). The runtime
stage is Alpine with a single binary (~50-100MB). Everything the server needs is
inside the binary.

### 19.4 Vercel Deployment

Vorma also supports deployment to Vercel. The scaffolder generates a
`vercel.json` if you select Vercel as your deployment target. The setup uses a
serverless function to proxy requests to your Go binary, with static assets
served directly from Vercel's CDN.

Hashed assets (Vite output) get immutable cache headers, so returning visitors
load them from the browser cache.

---

## 20. Configuration and Customization

Let's revisit the configuration options in `app.go` that you've been using but
might not have fully explored.

### 20.1 Custom Context Methods

You've already seen `LoaderCtx` and `ActionCtx` in `context.go`. Since they're
your own types, you can add any methods you want:

```go
type LoaderCtx struct{ *vorma.LoaderReqData }

func (c *LoaderCtx) CurrentUser() (*store.User, error) {
    return GetCurrentUser(c)
}

func (c *LoaderCtx) RequireUser() (*store.User, error) {
    user, err := c.CurrentUser()
    if err != nil {
        return nil, err
    }
    if user == nil {
        c.Redirect("/", http.StatusSeeOther)
        return nil, nil
    }
    return user, nil
}
```

Now every loader can call `c.CurrentUser()` or `c.RequireUser()` instead of
manually extracting cookies and running tasks. The task deduplication still
works -- if multiple loaders call `c.CurrentUser()` in the same request, the
user is fetched once.

### 20.2 `HeadDedupeKeysFunc`

This function tells Vorma how to deduplicate `<head>` elements when multiple
loaders set them. You've already seen it in `app.go`:

```go
HeadDedupeKeysFunc: func(h *vorma.HeadEls) {
    h.Meta(h.Property("og:title"))
    h.Meta(h.Property("og:description"))
},
```

This tells Vorma: "if two loaders both set `og:title`, keep the one from the
deepest (most specific) route." Without deduplication, you'd get duplicate meta
tags in the HTML.

### 20.3 `DefaultHeadElsFunc`

This function sets default `<title>` and `<meta>` tags for every page.
Individual loaders can override them with `c.HeadEls()`:

```go
DefaultHeadElsFunc: func(r *http.Request, app *vorma.Vorma, h *vorma.HeadEls) error {
    h.Title("HN Vorma")
    h.Meta(h.Name("description"), h.Content("A Hacker News clone built with Vorma"))
    return nil
},
```

If a loader sets its own `<title>`, the loader's title wins (deepest route takes
priority). `DefaultHeadElsFunc` provides the fallback for routes that don't set
their own head elements.

### 20.4 `RootTemplateDataFunc`

This function provides extra data to the HTML template on every request:

```go
RootTemplateDataFunc: func(r *http.Request) (map[string]any, error) {
    return map[string]any{
        "nonce": generateCSPNonce(),
    }, nil
},
```

The values are available in the Go HTML template (the `entry.go.html` file) as
template variables. This is useful for Content-Security-Policy nonces,
per-request feature flags, or any data that needs to be in the raw HTML before
JavaScript runs.

### 20.5 `useViewTransitions`

Enable the View Transitions API for smooth visual transitions during navigation:

```tsx
await initClient({
	vormaAppConfig,
	useViewTransitions: true,
	renderFn: () => {
		createRoot(getRootEl()).render(<VormaRootOutlet />);
	},
});
```

When enabled, Vorma wraps component re-renders in
`document.startViewTransition()`. This lets you use CSS `::view-transition-*`
pseudo-elements to animate between pages. View transitions only apply to user
navigations -- prefetch and revalidation skip them.

### 20.6 Loader and Action Router Options

If you need custom route syntax or action mount behavior, configure router
options in `VormaAppConfig`:

```go
var App = vorma.NewVormaApp(vorma.VormaAppConfig{
	Wave: Wave,
	LoadersRouterOptions: vorma.LoadersRouterOptions{
		DynamicParamPrefix:             ':',
		SplatSegmentIdentifier:         '*',
		ExplicitIndexSegmentIdentifier: "_index",
	},
	ActionsRouterOptions: vorma.ActionsRouterOptions{
		DynamicParamPrefix:     ':',
		SplatSegmentIdentifier: '*',
		MountRoot:              "/api",
		SupportedMethods:       []string{"GET", "POST", "PUT", "DELETE", "PATCH"},
	},
})
```

Most apps keep these at defaults. Reach for them when integrating Vorma into an
existing URL/method convention.

Why this matters:

- You can align Vorma's matching syntax with existing app conventions.
- `MountRoot` controls where actions live (default is `/api`).
- Restricting `SupportedMethods` can enforce tighter API policy.
- Frontend typed helpers consume generated config, so URL generation stays in
  sync with backend settings.

### 20.7 `wave.config.json`

The `wave.config.json` file controls the build pipeline. The scaffolder
generates it with sensible defaults, but here are the key options:

```jsonc
{
	"Core": {
		"MainAppEntry": "backend/cmd/serve",
		"DistDir": "backend/dist",
		"StaticAssetDirs": {
			"Private": "backend/assets", // Server-only assets (HTML template)
			"Public": "frontend/assets", // Browser-accessible (favicon, images)
		},
		"CSSEntryFiles": {
			"Critical": "frontend/src/styles/main.critical.css",
			"NonCritical": "frontend/src/styles/main.css",
		},
	},
	"Vorma": {
		"UIVariant": "react", // "react" | "solid" | "preact"
		"ClientEntry": "frontend/src/vorma.entry.tsx",
		"ClientRouteDefinitionPatterns": ["frontend/src/**/*vorma.routes.ts"],
		"ServerRouteDefinitionPatterns": ["backend/src/router/**/*.go"],
		"TSGenOutDir": "frontend/src/vorma.gen",
		"UnresolvedRoutePolicy": "error", // optional: "warn" or "error"
	},
	"Vite": {
		"JSPackageManagerBaseCmd": "pnpm", // set by scaffolder: npm | pnpm | yarn | bun
	},
}
```

Most of the time you won't need to change this. The `CSSEntryFiles.Critical`
field is worth knowing about -- CSS in this file is inlined directly into the
HTML for fast first-paint, while `NonCritical` CSS is emitted as a normal
stylesheet link.

For route definitions, `UnresolvedRoutePolicy` is especially useful:

- `"error"`: fail build/runtime route parsing on unresolved module expressions.
- `"warn"`: log unresolved route calls and skip them.

---

## 21. Wrapping Up

This tutorial covered Vorma from project creation through production deployment.
You now know how to:

- Define loaders and actions with end-to-end type safety
- Use nested routing with parallel loader execution
- Build task graphs for request-scoped caching and thundering-herd protection
- Validate action inputs with the fluent `validate` API
- Control HTTP responses (redirects, headers, cookies) from loaders and actions
- Add loading indicators, error boundaries, prefetching, and revalidation
- Customize TypeScript generation with custom types and overrides
- Build and deploy a single-binary Go server with all assets embedded

The HN clone you've built is intentionally simple, but the patterns scale.
Vorma's task system and nested routing are designed for applications with deep
route trees, complex data dependencies, and high concurrency. The same
primitives that power this two-page app will power a production dashboard with
dozens of nested layouts and hundreds of loaders.
