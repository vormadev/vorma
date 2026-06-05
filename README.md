# Vorma

Vorma is a Rust-first full-stack framework with a build/runtime crate pair, procedural
macros, a route-pattern matcher, and a task runtime used by the framework.

The published Rust crates are:

- `vorma`: application declaration and runtime serving API.
- `vorma-build`: build entry, dev server, Vite integration, and manifest generation.
- `vorma-macros`: procedural macros used by `vorma`.
- `vorma-matcher`: standalone path-pattern parser and matcher.
- `vorma-tasks`: standalone async task runtime with execution-context memoization.

The TypeScript package and UI adapters live under `packages/vorma`.
