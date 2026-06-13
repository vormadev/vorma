//! Vorma build/runtime boundary contracts.
//!
//! Everything here is shared truth between the runtime crate (`vorma`) and the
//! build crate (`vorma-build`): the canonical framework graph, the compiled
//! execution plan, the committed runtime manifest, document/type declaration
//! contracts, shared constants, and the TypeScript type model that code
//! generation renders from.

#![deny(missing_docs)]
#![forbid(unsafe_code)]

/*
The TsGen derive emits `::vorma::tsgen::*` paths — that path is the macro ABI.
This crate owns the `tsgen` module the runtime crate re-exports, so aliasing
itself as `vorma` satisfies those paths locally without depending on the
runtime crate.
*/
extern crate self as vorma;

/// Shared public and internal framework constants.
pub mod constants;
/// Canonical declaration and TypeScript type contracts.
pub mod contracts;
/// Trusted document-contract HTML rendering.
pub mod document_renderer;
/// Compiled per-request execution plan projected from the framework graph.
pub mod execution_plan;
/// Canonical serializable framework graph.
pub mod framework_graph;
/// Route-pattern semantics and matcher integration for graph compilation.
mod graph_patterns;
/// Framework-graph shape, contract, and asset validation.
mod graph_validation;
/// Live build-state protocol shared by the app binary and the build crate.
pub mod live_state;
/// Committed runtime manifest contracts.
pub mod runtime_manifest;
/// TypeScript type collection and drafting model.
pub mod tsgen;
/// Wire-contract types and constants shared with the browser runtime.
pub mod wire;

#[cfg(test)]
mod test_support;
