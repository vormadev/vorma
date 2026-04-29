# Vorma Tests

Tests in this repo are organized by ownership. Kit and lab packages keep their
tests local to the package. Vorma TypeScript packages keep their direct package
tests in the existing Vitest setup under `internal/pkg/npm`; kit package tests
and framework package tests are separate buckets there. Framework behavior
belongs in `internal/framework_tests` when it is about how an app uses the
framework: `vormarun`, `vormabuild`, generated artifacts, Vite/plugin behavior,
adapters, dev/prod servers, and browser-observable behavior.

The maintenance commands treat `framework` and `other` as a partition. Broad
commands run both sides. `other` is a complement where the tool supports it, so
new root-module Go packages and non-framework Vitest files are covered without
being listed manually. The gate fails when a new Go module, package manifest, or
TypeScript project appears without being classified. The classified root lists
live in `internal/cmd/maint/coherence.go`.

Do not recreate dependency test suites in the framework tests (for example,
route matching details belong in `kit/matcher`, schema behavior belongs in
`kit/schema`, and direct React/Preact/Solid/Vite behavior belongs to those
projects). The framework suite can use tools like Bombadil, Go tests,
Hegel/property tests, TypeScript checks, CLI tests, or artifact inspectors, but
the boundary is the public Vorma app/framework surface.

Run the normal repo gate with:

```bash
make gate
```

Run the repo stress workflow with:

```bash
make stress intensity=10
```
