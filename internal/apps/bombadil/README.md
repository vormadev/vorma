# Vorma Bombadil Fixture

This fixture is intentionally separate from the docs app. It gives Bombadil a
small, deterministic Vorma application matrix with the same route/action
scenario rendered through each client adapter.

## Layout

- `scenario/` owns the Go route topology, loaders, actions, and shared Vorma app
  config factory.
- `components/routes/` owns the single client route tree used by every adapter.
- `runtime/` owns the small React, Preact, and Solid adapter shims.
- `shared/` owns browser instrumentation and shared CSS.
- `vorma.{react,preact,solid}.gen.ts` are the tracked generated type files.
- `.dist.{react,preact,solid}/` hold ignored build output.
- `specs/vorma.spec.ts` contains the Bombadil properties and fixture-specific
  action generator.

## Install

Install the Bombadil CLI from the upstream instructions, then install this
fixture's package dependencies:

```bash
cd internal/apps/bombadil
pnpm install
```

The fixture also expects the local `vorma` npm package to have been built:

```bash
cd ../../pkg/npm
pnpm tsdown
```

## Run

Start one adapter variant:

```bash
cd internal/apps/bombadil
pnpm dev:react
```

Then, in another terminal:

```bash
pnpm bombadil:react
```

Swap `react` for `preact` or `solid` to run the same scenario against another
adapter.
