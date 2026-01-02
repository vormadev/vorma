---
title: Bob
description: Documentation for the Vorma framework
order: 2
---

## Bootstrapping a New Vorma Project

To start a new Vorma project, run the following command in your terminal:

`npm create vorma@latest`

After ensuring you have the required minimum versions of `Go` and `Node`
installed, the bootstrapper CLI will ask you a series of questions about how you
want to configure your Vorma project.

## Choosing a Frontend UI Library

During the bootstrapping process, you will be asked to choose from `React`,
`Preact`, and `Solid` for your frontend UI library.

With one small exception (explained [below](#solid-only-location-accessor)),
Vorma's APIs are **_identical_** across all three options.

### Solid-only Location Accessor

To provide reactive location data in a React or Preact project, Vorma provides a
`useLocation()` hook. In a Solid project, however, such a zero-arguments hook
would be pointless. So instead of providing a `useLocation()` hook,
`vorma/solid` provides a direct `location` accessor (which represents what is in
effect what is returned by `useLocation()` in React and Preact projects). Here's
what I mean:

In React (or Preact), we need to call `useLocation()`:

```tsx
import { useLocation } from "vorma/react";

function Component() {
	const location = useLocation();
	return <div>Current path: {location.pathname}</div>;
}
```

In Solid, we can just use the `location` accessor directly:

```tsx
import { location } from "vorma/solid";

function Component() {
	return <div>Current path: {location().pathname}</div>;
}
```

## Vorma Project Structure

Vorma apps are highly flexible in terms of their project structure. As long as
you follow the rules of `Go` (and, if applicable to your deployment strategy,
`go:embed`), there are **_no required file conventions_** in Vorma. In other
words, all Vorma file conventions are configurable to suit the needs of your
team.

In the cases where Vorma does care where your files live and what they're named,
it is configurable via your Wave config file (which typically lives at
`backend/wave.config.json`). What is Wave? It's Vorma's lower-level build tool,
but more on that later.

Throughout this documentation, we'll follow the typical Vorma project structure
as instantiated by the bootstrapper when you run `npm create vorma@latest`.

### Default Bootstrapped Project Structure

```
your-app/
├── frontend/
│   ├── assets/                  # Client-exposed assets
│   └── src/
│       ├── components/
│       ├── styles/
│       ├── vorma.api.ts         # Type-safe API client wrapper
│       ├── vorma.entry.tsx      # Client entry point
│       ├── vorma.gen.ts         # Vorma-generated TypeScript
│       ├── vorma.routes.ts      # Route components registry
│       └── vorma.utils.tsx      # Type-safe hooks & utilities
│
├── backend/
│   ├── assets/                  # Server-only assets (e.g., templates)
│   │   └── entry.go.html        # Core HTML template
│   ├── cmd/
│   │   ├── build/
│   │   │   └── main.go          # Build script
│   │   └── serve/
│   │       └── main.go          # Actual HTTP server
│   ├── dist/                    # Build output
│   ├── src/
│   │   └── router/
│   │       ├── actions.go       # API queries and mutations
│   │       ├── loaders.go       # Nested UI route data loaders
│   │       └── core.go          # Core HTTP router setup
│   ├── vorma.config.go          # Vorma/Wave dynamic config
│   └── wave.config.json         # Wave static config
│
├── .gitignore
├── go.mod
├── go.sum
├── package.json
├── tsconfig.json
└── vite.config.ts
```

### Wave Config File

Vorma's lower-level build tool is called Wave. Wave handles things like watching
for file changes during dev, running build hooks, and integrating with the Vite
CLI.

You can configure Wave via a JSON file, which typically lives at
`./backend/wave.config.json`.

To learn more, check out the
[docs on configuring Wave](/docs/advanced/configuring-wave).

### Generated TypeScript

The foundation of Vorma's backend-frontend type safety is its generated
TypeScript file. Let's call this the "**generated TS file**".

By convention, the bootstrapper will create your generated TS file at
`./frontend/src/vorma.gen.ts`. You can configure its name/location via the
`Vorma.TSGenOutPath` field in your Wave config file.

Your generated TS file will contain several things:

- All your loader and action route definitions, with input/output types,
  patterns, and param keys
- All your ad hoc types that are used by your routes and/or otherwise generated
  by your backend (see [Sharing Ad Hoc Types](#sharing-ad-hoc-types) below).
- Any extra TypeScript code that you generated from the Go-based builder (for
  example, any constants or enums you passed down from Go to TypeScript)
- A variety of app-specific type helpers to help make your entire application
  (even links) 100% type-safe.
- An app-specific `vormaViteConfig` object to pass into Vorma's Vite plugin in
  your `vite.config.ts` file

## Sharing Ad Hoc Types

If you want to share additional ad hoc types to your frontend from your Go
backend (that aren't already being used by any Vorma routes), you can do so by
passing an instance of them to the `vorma.BuildOptions.AdHocTypes` slice in your
`./backend/cmd/build/main.go` file (let's call it the "**Vorma build script**").

## Vorma Build Script

By convention, your Vorma build script lives at `./backend/cmd/build/main.go`,
but you can put it wherever you want by editing your Wave config file.

---

TO BE CONTINUED...
