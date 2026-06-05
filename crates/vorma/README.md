# vorma

Rust full-stack framework runtime and app declaration API.

Application code declares views, resources, middleware, document defaults, and
build/runtime config through this crate. `vorma-build` consumes the same declarations to
generate manifests and TypeScript contracts, while `RuntimeHost` serves the runtime
handler that users mount into their HTTP stack.
