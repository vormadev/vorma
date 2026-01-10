---
title: Standard Project Structure
description: A review of the standard Vorma project structure
order: 3
---

Below is the standard Vorma project structure as instantiated by the
[Vorma bootstrapper CLI](/docs/new-project-from-cli).

The locations and names of everything is configurable to fit your personal or
team preferences. This is done via your `wave.config.json` file (which itself
can be named anything and located anywhere you want).

```txt
your-vorma-app/
├── backend/
│   ├── assets/
│   │   └── entry.go.html
│   ├── cmd/
│   │   ├── build/
│   │   │   └── main.go
│   │   └── serve/
│   │       └── main.go
│   ├── dist/
│   │   ├── static/
│   │   │   └── .keep
│   │   └── main(.exe) (compiled Go binary)
│   ├── src/
│   │   └── router/
│   │       └── router.go
│   ├── wave.config.json
│   ├── wave.dev.go
│   └── wave.prod.go
├── frontend/
│   ├── assets/
│   ├── src/
│   │   ├── components/
│   │   ├── styles/
│   │   │   ├── main.css
│   │   │   └── main.critical.css
│   │   ├── vorma.api.ts
│   │   ├── vorma.entry.tsx
│   │   ├── vorma.gen/
│   │   ├── vorma.routes.ts
│   │   └── vorma.utils.tsx
├── .gitignore
├── go.mod
├── package.json
├── tsconfig.json
└── vite.config.ts
```

## Backend

### Assets

- `backend/assets/` -- Where you put _private_ static assets, such as templates
  or config files you want to read
- `backend/assets/entry.go.html` -- Your HTML entry template that gets
  server-rendered to users when they first visit your site

### Cmd

- `backend/cmd/build/main.go` -- Go program that builds your application
- `backend/cmd/server/main.go` -- Go program that serves your application

### Dist

- `backend/dist/` -- Where Vorma outputs its build artifacts
- `backend/dist/static/.keep` -- A no-op text file commited to git to satisfy
  compiler relating to `go:embed` directives
- `backend/dist/static/main` -- Your actual Go application binary (_i.e._, the
  compiled result of `backend/cmd/server/main.go`).

### Src

- `backend/src/router/router.go` -- Where your core router lives. You can do
  anything you want here.

### Wave

Wave is the lower-level build tool used by Vorma. It is configured via a JSON
config file that is itself read into the Go Wave instance. This allows Wave to
be a completely standalone system in your project repository, offering hot
reloading without installing any tools on your machine.

- `backend/wave.config.json` -- If you want to edit the names or locations of
  any items mentioned here, this is where you'll do it. You can also put this
  file anywhere and name it whatever you want (you just have to point to it when
  you instantiate your Wave instance in Go).
- `backend/wave.dev.go` -- Dev-time Wave instantiation (with a `!prod` Go build
  tag)
- `backend/wave.prod.go` -- Prod-time Wave instantiation (with a `prod` Go build
  tag). This version usually will embed your static assets into the actual
  compiled Go binary (not done during dev for performance reasons).

## Frontend

### Assets

- `frontend/assets` -- Where you put _public_ static assets that you want to
  serve to your frontend application, such as images, fonts, etc.

### Styles

- `frontend/src/styles/main.css` -- Your global CSS file that gets loaded into
  your document head as a traditional stylesheet.
- `frontend/src/styles/main.critical.css` -- A global CSS file that gets
  _inlined_ into your document head (for preventing flash of unstyled content).
  Usually, at minimum, you'll want to set the html background color, body
  margins, and font-family in here.

### Core Vorma Parts

- `frontend/src/vorma.api.ts` -- Your type-safe API client
- `frontend/src/vorma.entry.tsx` -- Your frontend application entry point that
  instantiates the Vorma client and renders your frontend application.
- `frontend/src/vorma.gen/` -- Holds your generated TypeScript types, downstream
  of your Go loaders and actions. Do not edit these files directly.
- `frontend/src/vorma.routes.ts` -- Centralized, build-time TypeScript module
  for registering frontend routes.
- `frontend/src/vorma.utils.tsx` -- Exports various type-safe helpers based on
  your specific application types, such as a type-safe `Link` component,
  type-safe `useLoaderData` hook, etc.

## Other Root-Level Files

- `.gitignore`
- `go.mod`
- `package.json`
- `tsconfig.json`
- `vite.config.ts` -- Uses Vorma's Vite plugin, instantiated with an exported
  config from `vorma.gen/index.ts`
