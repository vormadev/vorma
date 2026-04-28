# Vorma Integration Tests

This directory is the shared black-box integration fixture for Vorma. It is not
an application product. It gives the test suite one realistic Vorma app matrix
with routes, loaders, actions, generated TypeScript, dev mode, production
builds, and browser-level Bombadil properties.

Bombadil is one runner in this harness. The directory can also host normal Go
integration tests and Hegel/type-surface tests when they need the same fixture.

## Suite Ownership

- Bombadil owns broad runtime behavior: browser navigation, loaders, actions,
  revalidation, prefetch, history, build outputs, dev-server behavior, and
  production deployment switching.
- Go tests in product packages should stay narrow and public. They are for cheap
  exported API contracts that do not need a real app fixture.
- Hegel and TypeScript checks own generated/public TypeScript surface contracts.
- Dependency behavior belongs to the dependency suite, not here.

## Layout

- `scenario/` owns the Go route topology, loaders, actions, and shared Vorma app
  config factory.
- `components/routes/` owns the single client route tree used by every adapter.
- `runtime/` owns the React, Preact, and Solid adapter shims.
- `shared/` owns browser instrumentation and shared CSS.
- `specs/vorma.spec.ts` contains the Bombadil properties and fixture-specific
  action generator.
- `vite.*.config.ts` files select adapter-specific Vite configs while sharing
  the common config logic in `vite.shared.config.ts`.
- `vorma.*.gen.ts` files are ignored generated type files.
- `.dist.{react,preact,solid}.a/` and `.dist.{react,preact,solid}.b/` are
  ignored production fixture deployment outputs.
- `.dist.{react,preact,solid}.dev.a/` is ignored dev-server fixture output.
- `.bombadil/` is ignored test output. Keep it when inspecting failures.

## Artifacts

Bombadil writes run artifacts under `.bombadil/`:

- `.bombadil/<variant>/` for prod root-route runs.
- `.bombadil/<variant>-nested/` for prod nested-route runs.
- `.bombadil/<variant>-counter/` for prod counter-route runs.
- `.bombadil/dev-<variant>/` for dev root-route runs.
- `.bombadil/dev-<variant>-nested/` for dev nested-route runs.
- `.bombadil/dev-<variant>-counter/` for dev counter-route runs.
- `.bombadil/logs/prod-<variant>.log` for prod fixture server logs.
- `.bombadil/logs/dev-<variant>.log` for dev fixture server logs.
- `.bombadil/vite-cache/<mode>-<variant>/` for per-adapter Vite caches.

Inspect artifacts from this directory:

```bash
make inspect
make inspect artifact=.bombadil/dev-react
```

Each Bombadil test clears its own artifact directory before writing, so repeated
runs do not accumulate stale screenshots and traces for the same run name.
Server logs are truncated on each run. Remove all Bombadil artifacts with:

```bash
make clean-artifacts
```

The console output includes the exact `make inspect artifact=...` command for a
failing Bombadil run, and server logs stay under `.bombadil/logs/`.

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

## Targets

From the repo root:

```bash
make bombadil
make bombadil-prod
make bombadil-dev
make bombadil-build
```

From this directory:

```bash
make matrix
make parallel
make check
make prod
make dev
make build
make inspect
make clean-artifacts
```

`check` installs dependencies, runs the integration module Go tests, runs the
TypeScript check, and then runs the full prod/dev Bombadil matrix.

`matrix` runs the prod suite and then the dev suite. Within each suite, adapter
variants run at the same time. `parallel` is an alias for `matrix`.

`prod` and `dev` run every adapter by default. To select one adapter:

```bash
make prod variant=react
make dev variant=react
```

Convenience aliases are also available:

```bash
make prod-react
make prod-preact
make prod-solid
make dev-react
make dev-preact
make dev-solid
```

Use `multiplier` to scale Bombadil time limits:

```bash
make matrix multiplier=2
make dev variant=react multiplier=1
```

## Manual Dev Fixture

Start one adapter variant by hand:

```bash
go run ./cmd/bombadil dev react
```

Swap `react` for `preact` or `solid`.

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
