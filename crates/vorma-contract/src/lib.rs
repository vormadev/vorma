//! Vorma build/runtime boundary contracts.
//!
//! Everything here is shared truth between the runtime crate (`vorma`) and the
//! build crate (`vorma-build`): the canonical framework graph, the compiled
//! execution plan, the committed runtime manifest, document/type declaration
//! contracts, shared constants, and the TypeScript type model that code
//! generation renders from.
//!
//! # Two audiences, one crate
//!
//! Most of this crate is **framework-integration surface**: types an
//! application author never names directly, but that `vorma` and
//! `vorma-build` pass between themselves as the compiled shape of an app —
//! [`framework_graph`], [`execution_plan`], [`runtime_manifest`],
//! [`live_state`], and [`document_renderer`]. If you are writing a Vorma
//! app rather than working on the framework itself, you will not construct
//! these types by hand ([`constants`] is a partial exception: `vorma`
//! re-exports the handful of its values, like
//! [`constants::PUBLIC_STATIC_OUT_NAME_PREFIX`], that app code has any
//! reason to read).
//!
//! Two parts are **app-facing**, reached indirectly through `vorma`'s own
//! public API:
//!
//! - [`tsgen`] — the [`tsgen::Type`] trait every `#[derive(TsGen)]` type
//!   implements, plus [`tsgen::TsExtraType`] and [`tsgen::TsDrafter`] for
//!   registering extra generated TypeScript by hand
//!   (`AppConfig::ts_gen_config` in `vorma`).
//! - [`contracts::DocumentElementContract`] — the validated HTML element
//!   shape underneath `vorma`'s `Document`/head-builder API. App code
//!   builds head elements through that higher-level builder, not this
//!   type directly, but the contract's own docs describe the escaping and
//!   validation rules that builder relies on.
//!
//! # Wire contracts are frozen
//!
//! [`wire`] and [`live_state::LIVE_BUILD_STATE_PROTOCOL`] are versioned
//! wire formats shared with the browser runtime and between build-entry
//! processes respectively. Their shapes are load-bearing for already-built
//! clients and already-running dev sessions; changing them is a framework
//! release decision, not a local edit — see [`wire::ViewPayload`] and
//! [`wire::SsrPayload`] for the browser-facing half of that contract and
//! [`live_state::LiveBuildState`] for the build-side half.

#![deny(missing_docs)]
#![deny(rustdoc::broken_intra_doc_links)]
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
