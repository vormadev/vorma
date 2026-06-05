# Framework Tests

This directory is the shared test area for framework behavior, not lower-level kit/lab
primitives. It is not an application product. It contains a realistic Vorma app plus
whatever runners and checks are needed for public framework behavior through observable
surfaces.

Observable surfaces include subprocesses, generated artifacts, filesystem output, HTTP
traffic, and browser behavior.

Bombadil is one runner here.

See [../../TEST_README.md](../../TEST_README.md) for the repo-level testing map.

## Suite Ownership

- Bombadil owns broad runtime behavior: browser navigation, view handlers, resources,
  revalidation, prefetch, history, build outputs, dev-server behavior, and production
  deployment switching.
- Framework-level Rust and TypeScript tests can live here when the behavior is covered
  through the shared app setup.
- Vorma TypeScript package tests stay in the existing local Vitest suites when they cover
  direct package behavior rather than app-boundary behavior.
- Primitive `kit/*` and lab behavior belongs to the owning package, not here.
- Dependency behavior belongs to the dependency suite, not here.

## Layout

- `src/scenario.rs` owns the Rust view/resource topology and shared Vorma app config
  factory.
- `components/routes/` owns the client view fixtures used by every adapter.
- `runtime/` owns the React, Preact, and Solid adapter shims.
- `shared/` owns browser instrumentation and shared CSS.
- `specs/vorma.property.ts` contains the Bombadil properties and fixture-specific action
  generator.
- `vite.*.config.ts` files select adapter-specific Vite configs while sharing the common
  config logic in `vite.shared.config.ts`.
- `vorma.*.gen.ts` files are ignored generated type files.
- `.dist.{react,preact,solid}.a/` and `.dist.{react,preact,solid}.b/` are ignored
  production fixture deployment outputs.
- `.dist.{react,preact,solid}.dev.a/` is ignored dev-server fixture output.
- `.bombadil/` is ignored test output. Failed runs keep inspectable artifacts there.

## Artifacts

Bombadil writes run artifacts under `.bombadil/`:

- `.bombadil/<variant>/` for prod root-view runs.
- `.bombadil/<variant>-nested/` for prod nested-view runs.
- `.bombadil/<variant>-counter/` for prod counter-view runs.
- `.bombadil/dev-<variant>/` for dev root-view runs.
- `.bombadil/dev-<variant>-nested/` for dev nested-view runs.
- `.bombadil/dev-<variant>-counter/` for dev counter-view runs.
- `.bombadil/logs/build-prod-<variant>-a.log` for production deployment A build output.
- `.bombadil/logs/build-prod-<variant>-b.log` for production deployment B build output.
- `.bombadil/logs/build-prod-<variant>-server.log` for production server build output.
- `.bombadil/logs/build-dev-<variant>.log` for dev fixture build output.
- `.bombadil/logs/prod-<variant>.log` for prod fixture server logs.
- `.bombadil/logs/dev-<variant>.log` for dev fixture server logs.
- `.bombadil/logs/test-<run-name>.log` for Bombadil browser run output.
- `.bombadil/server-bin/<variant>/` for harness-owned production fixture server binaries
  while a production run is active.
- `.bombadil/cargo/prod/<variant>/` for harness-owned production fixture server Cargo
  build output while a production run is active.
- `.bombadil/vite-cache/<mode>-<variant>/` for per-adapter Vite caches while a run is
  active.

The harness roots these paths at this directory via its crate manifest directory, not the
shell cwd. It must not create or use `.vorma` as a scratch namespace; `.vorma` is owned by
the Vorma framework output pipeline.

Inspect artifacts from this directory:

```bash
cargo run -p vorma-framework-tests --bin framework-bombadil -- inspect .bombadil/dev-react
```

Each Bombadil test clears its own artifact directory before writing. A successful Bombadil
test removes its browser artifact directory after the run. Successful dev and prod runs
also remove their heavy framework `.dist.*`, Cargo, server-binary, and Vite cache outputs.
Failed runs keep their artifacts for inspection, and failure output points at the path to
inspect. Server logs are truncated on each run and remain under `.bombadil/logs/`.

Remove all Bombadil/framework generated artifacts from the repo root with:

```bash
make clean-bombadil
```

## Install

From this directory:

```bash
pnpm install
```

The fixture expects the local `vorma` npm package to have been built:

```bash
cd ../..
make ts-build
```

## Commands

Use Bombadil directly for selected variants, custom intensity values, manual dev servers,
or artifact inspection:

```bash
cargo run -p vorma-framework-tests --bin framework-bombadil -- test-prod -variant react -intensity 1
cargo run -p vorma-framework-tests --bin framework-bombadil -- test-dev -variant react -intensity 1
cargo run -p vorma-framework-tests --bin framework-bombadil -- serve-dev react
cargo run -p vorma-framework-tests --bin framework-bombadil -- inspect .bombadil/dev-react
```

Supported variants are `react`, `preact`, and `solid`.

When `test-prod` runs without `-variant`, the harness builds production variants in
parallel, then runs browser property suites serially. Parallel browser instrumentation can
starve Bombadil's temporal checks and make the gate flaky for reasons unrelated to Vorma.

## Dev Scope

The dev-server suite intentionally does not use the A/B deployment switching fixture. That
is a scope decision about this fixture: Vite dev mode, HMR, and the dev manifest make
deployment switching a different problem than the production build-output switchboard. Dev
writes to separate `.dist.*.dev.*` directories so it can run alongside prod.

This does not mean build skew is impossible or irrelevant in dev. Dev pages still boot
with a client build ID, view-data requests still submit that ID, and the server can still
report a stale client build with `X-Vorma-Build-Skew`. Production Bombadil currently owns
the A/B cases needed to exercise build-skew hard reloads. That skew case is one behavior
inside the broader production suite, not the point of the suite.
