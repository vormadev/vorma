# Vorma Tests

Tests in this repo are organized by ownership and by whether they cover the
Vorma framework surface or everything else. Framework tests belong in
`internal/framework_tests`, except for the framework Vitest suites under
`internal/pkg/npm/vorma/*`; other Go packages keep their package tests local,
and other TypeScript package tests live under `internal/pkg/npm`.

The maintenance commands treat `framework` and `other` as a partition. Broad
commands run both sides. `other` is a complement where the tool supports it:
root Go tests use `./...`, docs and framework are separate Go modules, and
non-framework Vitest runs everything outside the framework package tree. The
gate fails when a new Go module, package manifest, or TypeScript project appears
without being classified. The classified root lists live in
`internal/cmd/maint/coherence.go`.

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
