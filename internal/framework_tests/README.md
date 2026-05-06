# Framework Tests

This directory is the shared test area for framework behavior, not lower-level
kit/lab primitives. It is not an application product. It contains a realistic
Vorma app plus whatever runners and checks are needed for public framework
behavior through observable surfaces.

Observable surfaces include subprocesses, generated artifacts, filesystem
output, HTTP traffic, and browser behavior.

Bombadil is one runner here.

See [../../TEST_README.md](../../TEST_README.md) for the repo-level testing map.

## Suite Ownership

- Bombadil owns broad runtime behavior: browser navigation, loaders, actions,
  revalidation, prefetch, history, build outputs, dev-server behavior, and
  production deployment switching.
- Framework-level Go and TypeScript tests can live here when the behavior is
  covered through the shared app setup.
- Vorma TypeScript package tests stay in the existing local Vitest suites when
  they cover direct package behavior rather than app-boundary behavior.
- Primitive `kit/*` and lab behavior belongs to the owning package, not here.
- Dependency behavior belongs to the dependency suite, not here.

## Layout

- `scenario/` owns the Go route topology, loaders, actions, and shared Vorma app
  config factory.
- `components/routes/` owns the single client route tree used by every adapter.
- `runtime/` owns the React, Preact, Remix, and Solid adapter shims.
- `shared/` owns browser instrumentation and shared CSS.
- `specs/vorma.spec.ts` contains the Bombadil properties and fixture-specific
  action generator.
- `vite.*.config.ts` files select adapter-specific Vite configs while sharing
  the common config logic in `vite.shared.config.ts`.
- `vorma.*.gen.ts` files are ignored generated type files.
- `.dist.{react,preact,remix,solid}.a/` and
  `.dist.{react,preact,remix,solid}.b/` are ignored production fixture
  deployment outputs.
- `.dist.{react,preact,remix,solid}.dev.a/` is ignored dev-server fixture
  output.
- `.bombadil/` is ignored test output. Keep it when inspecting failures.

## Artifacts

Bombadil writes run artifacts under `.bombadil/`:

- `.bombadil/<variant>/` for prod root-route runs.
- `.bombadil/<variant>-nested/` for prod nested-route runs.
- `.bombadil/<variant>-counter/` for prod counter-route runs.
- `.bombadil/dev-<variant>/` for dev root-route runs.
- `.bombadil/dev-<variant>-nested/` for dev nested-route runs.
- `.bombadil/dev-<variant>-counter/` for dev counter-route runs.
- `.bombadil/logs/build-prod-<variant>-a.log` for production deployment A build
  output.
- `.bombadil/logs/build-prod-<variant>-b.log` for production deployment B build
  output.
- `.bombadil/logs/build-prod-<variant>-server.log` for production server build
  output.
- `.bombadil/logs/build-dev-<variant>.log` for dev fixture build output.
- `.bombadil/logs/prod-<variant>.log` for prod fixture server logs.
- `.bombadil/logs/dev-<variant>.log` for dev fixture server logs.
- `.bombadil/logs/test-<run-name>.log` for Bombadil browser run output.
- `.bombadil/vite-cache/<mode>-<variant>/` for per-adapter Vite caches.

Inspect artifacts from this directory:

```bash
go run ./cmd/bombadil inspect .bombadil/dev-react
```

Each Bombadil test clears its own artifact directory before writing, so repeated
runs do not accumulate stale screenshots and traces for the same run name.
Server logs are truncated on each run. Remove all Bombadil artifacts with:

```bash
rm -rf .bombadil
```

Bombadil failure output includes the artifact path to inspect, and server logs
stay under `.bombadil/logs/`.

## Install

From this directory:

```bash
pnpm install
```

The fixture expects the local `vorma` npm package to have been built:

```bash
cd ../pkg/npm
pnpm tsdown
```

## Commands

Use Bombadil directly for selected variants, custom intensity values, manual dev
servers, or artifact inspection:

```bash
go run ./cmd/bombadil test-prod -variant react -intensity 1
go run ./cmd/bombadil test-dev -variant react -intensity 1
go run ./cmd/bombadil serve-dev react
go run ./cmd/bombadil inspect .bombadil/dev-react
```

Supported variants are `react`, `preact`, `remix`, and `solid`.

## Dev Scope

The dev-server suite intentionally does not use the A/B deployment switching
fixture. That is a scope decision about this fixture: Vite dev mode, HMR, and
the dev manifest make deployment switching a different problem than the
production build-output switchboard. Dev writes to separate `.dist.*.dev.*`
directories so it can run alongside prod.

This does not mean build skew is impossible or irrelevant in dev. Dev pages
still boot with a client build ID, route-data requests still submit that ID, and
the server can still report a stale client build with `X-Vorma-Build-Skew`.
Production Bombadil currently owns the A/B cases needed to exercise build-skew
hard reloads. That skew case is one behavior inside the broader production
suite, not the point of the suite.
