# Vorma Tests

Tests in this repo are organized by ownership. Framework tests belong in
`internal/framework_tests`, except for direct framework package tests under
`internal/pkg/npm/vorma/*`. Other Go packages keep their tests local, and other
TypeScript package tests live under `internal/pkg/npm`.

Do not recreate dependency test suites in the framework tests (for example,
route matching details belong in `kit/matcher`, schema behavior belongs in
`kit/schema`, and direct React/Preact/Solid/Vite behavior belongs to those
projects). The framework suite can use tools like Bombadil, Go tests,
Hegel/property tests, TypeScript checks, CLI tests, or artifact inspectors, but
it should cover app/framework behavior rather than lower-level package behavior.

Run the normal repo gate with:

```bash
make gate
```

Run the repo stress workflow with:

```bash
make stress intensity=10
```

Stress is not a replacement for the normal gate.
