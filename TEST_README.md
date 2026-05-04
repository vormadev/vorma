# Vorma Tests

Tests in this repo are organized by enforcer scope. Framework tests belong in
`internal/framework_tests`, except for direct framework package tests under
`internal/pkg/npm/vorma/*`.

Matcher conformance tests and benchmarks live in `internal/matcher_tests`. That
suite covers the Go implementation in `kit/matcher` and the TypeScript mirror in
`internal/pkg/npm/kit/matcher`. The conformance tests are Go tests that run
every case, including the Hegel/property tests, against both implementations
through small JSON CLI adapters. The matcher enforcer test path builds the
TypeScript package first because the TypeScript adapter imports built package
exports. `internal/matcher_tests` is its own test package; its TypeScript files
import `vorma/kit/matcher` through the local package link instead of reaching
into implementation source files.

Other Go package tests stay local to their packages, and other TypeScript
package tests live under `internal/pkg/npm`.

Do not recreate dependency test suites in the framework tests (for example,
matcher conformance belongs in `internal/matcher_tests`, schema behavior belongs
in `kit/schema`, and direct React/Preact/Solid/Vite behavior belongs to those
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
