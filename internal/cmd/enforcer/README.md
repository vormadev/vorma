# Enforcer

Enforcer is the repo command for code-quality enforcement: prerequisites,
formatting, linting, safe fixes, typechecks, tests, builds, stress runs, and
gates.

The root `Makefile` is the user-facing command menu. Enforcer is the command it
calls when a repo workflow needs the shared task graph for ordering, deduping,
and parallel execution.

## Public Interface

The command shape is:

```bash
go run ./internal/cmd/enforcer <action...> [--lang all|go|ts] [--scope all|fw|matcher|other]
```

Actions:

- `install`
- `fmt`
- `lint`
- `fix`
- `typecheck`
- `test`
- `build`
- `stress`
- `gate`

Selectors:

- `--lang all|go|ts`
- `--scope all|fw|matcher|other`

Multiple actions in one invocation share one task context, so dependencies are
ordered, duplicate work is deduped, and independent work can run in parallel.

The root `Makefile` exposes dash-style aliases for common cells and rollups, but
those aliases should call this grammar rather than inventing separate workflows.

Do not add public enforcer commands like `test-docs`, `test-kit`,
`typecheck-docs`, or adapter-specific framework variants. Those are lower-level
tool concerns, not repo enforcement actions.

## Partitions

`fw` is the public Vorma framework surface. It includes the framework fixture in
`internal/framework_tests` and the framework TypeScript packages under
`internal/pkg/npm/vorma/*`.

`matcher` is the cross-language matcher conformance suite in
`internal/matcher_tests`. It drives the Go implementation and the TypeScript
mirror through CLI adapters; its test action builds the TypeScript package first
because the TypeScript adapter imports built package exports. The matcher tests
are their own package, and their TypeScript files import `vorma/kit/matcher`
through the local package link instead of reaching into implementation source
files.

`other` is the complement where the command supports one. It includes repo Go
tests outside the framework fixture, docs checks, and non-framework TypeScript
checks under `internal/pkg/npm`.

The classified root lists live in `coherence.go`. The shape check should fail
when a new Go module, package manifest, or TypeScript project appears without
being classified.

## Internals

The private task graph is the point of the tool. Public actions should compose
shared action/language/scope cells, so one invocation can dedupe repeated setup
work and run independent cells in parallel.

Avoid adding granular internal leaves unless they are needed to express the
public workflows. Prefer one private DAG over scattered one-off command helpers.
