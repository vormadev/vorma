//! Procedural macros for Vorma.
//!
//! This crate is an implementation detail of `vorma`'s public API: it is
//! not meant to be depended on directly. Its one genuinely public,
//! app-facing item is [`macro@TsGen`] (re-exported as `vorma::TsGen`);
//! `__vorma_view`/`__vorma_resource` are `#[doc(hidden)]` expansion
//! targets for `vorma`'s own `app!`/`view!`/`resource!` macros and carry
//! no independent contract of their own — their behavior is `vorma`'s to
//! document, through those macros, not this crate's.
//!
//! # The `TsGen` derive's contract
//!
//! `#[derive(TsGen)]` generates an implementation of
//! `vorma_contract::tsgen::Type` (re-exported as `vorma::tsgen::Type`)
//! from a type's existing `#[derive(Serialize)]`/`#[derive(Deserialize)]`
//! shape — see [`macro@TsGen`]'s own docs for exactly which Rust shapes
//! and `serde` attributes it supports, what it rejects and why, and the
//! `vorma-contract` doctrine (named type identity, the Serialize/Deserialize
//! phase split) the generated implementation follows.

#![deny(missing_docs)]
#![deny(rustdoc::broken_intra_doc_links)]
#![forbid(unsafe_code)]

mod app_decl;
mod ts_gen_derive;

use proc_macro::TokenStream;
use syn::parse_macro_input;

#[doc(hidden)]
#[proc_macro]
pub fn __vorma_view(input: TokenStream) -> TokenStream {
	let input = parse_macro_input!(input as app_decl::ViewMacroInput);
	app_decl::expand_view(input)
		.unwrap_or_else(syn::Error::into_compile_error)
		.into()
}

#[doc(hidden)]
#[proc_macro]
pub fn __vorma_resource(input: TokenStream) -> TokenStream {
	let input = parse_macro_input!(input as app_decl::ResourceMacroInput);
	app_decl::expand_resource(input)
		.unwrap_or_else(syn::Error::into_compile_error)
		.into()
}

/// Derive Vorma's TypeScript generation trait for a serializable Rust type.
///
/// Generates a `vorma::tsgen::Type` implementation by reading the same
/// shape and `serde` attributes that already drive
/// `#[derive(Serialize)]`/`#[derive(Deserialize)]`, so a route's
/// input/output type's generated TypeScript shape stays a direct
/// reflection of what it actually serializes/deserializes as — never a
/// separately hand-maintained shape that can drift from the real one.
///
/// # Supported shapes
///
/// - **Named-field structs.** Every field becomes one TypeScript object
///   property. A field's Rust name is used unless overridden by
///   `#[serde(rename = "...")]` or a container-level
///   `#[serde(rename_all = "...")]` (both accept the same casing rules
///   `serde` does: `lowercase`, `UPPERCASE`, `PascalCase`, `camelCase`,
///   `snake_case`, `SCREAMING_SNAKE_CASE`, `kebab-case`,
///   `SCREAMING-KEBAB-CASE`; the phase-specific
///   `#[serde(rename(serialize = "...", deserialize = "..."))]` and
///   `#[serde(rename_all(serialize = "...", deserialize = "..."))]` forms
///   are also honored, letting the two phases use different names for
///   the same field). `#[serde(skip)]` drops a field from both phases;
///   `#[serde(skip_serializing)]`/`#[serde(skip_deserializing)]` drop it
///   from one phase only.
/// - **Unit-only enums.** Every variant becomes one string literal in a
///   TypeScript union, with the same rename/`rename_all` support as
///   struct fields.
/// - **`#[serde(transparent)]` structs.** A single-field transparent
///   struct becomes a TypeScript alias for its one field's own type,
///   matching how `serde` serializes it as if the wrapper were not
///   there.
///
/// # Field optionality
///
/// A field's TypeScript optionality (`field?: T` vs `field: T`) is
/// derived independently per phase, matching what each phase actually
/// guarantees a caller: on the **serialize** side, a field is optional
/// exactly when it carries `#[serde(skip_serializing_if = "...")]` (the
/// predicate itself is not evaluated at generation time — its mere
/// presence means the field is not guaranteed to be sent); on the
/// **deserialize** side, a field is optional when the type itself is
/// `Option<T>`, or the field carries `#[serde(default)]`, or the
/// container carries `#[serde(default)]`.
///
/// # What this derive rejects
///
/// Every rejection is a compile error naming a manual `vorma::tsgen::Type`
/// implementation as the fallback, never a silent best-effort guess:
///
/// - Generic types (`struct Wrapper<T>`) — not supported yet.
/// - Tuple structs and unit structs — named fields only.
/// - Unions and data-carrying enum variants — unit-only enums only; a
///   `#[serde(tag = "...")]`-style externally/internally/adjacently
///   tagged enum needs a manual implementation.
/// - `#[serde(flatten)]` fields — deferred; flattening changes a type's
///   shape in a way this derive does not yet project.
/// - Non-`String` `HashMap`/`BTreeMap` keys (including through
///   `Option`/`Vec`/`Box`/fixed-size-array wrapping) — TypeScript object
///   keys are always strings, so a non-string Rust map key has no direct
///   representation this derive will guess at.
/// - `#[serde(serialize_with = "...")]`/`#[serde(deserialize_with =
///   "...")]`/`#[serde(with = "...")]` — a custom codec means the
///   TypeScript shape can no longer be inferred from the Rust type alone.
///
/// # A named type shared across both phases
///
/// This derive always registers a type's serialize and deserialize
/// `vorma_contract::contracts::TypeDef`s under two distinct identity keys
/// (`{module_path}::{TypeIdent}::serialize`/`{module_path}::{TypeIdent}::deserialize`),
/// even when a type's two phases render identically. Using one derived
/// type as a route input in one place and a route output in another —
/// the ordinary shape of a type like a shared `User` accepted as a
/// partial-update input and returned as a full-record output — resolves
/// to exactly one exported TypeScript type when the two phases' rendered
/// shapes agree. When they genuinely differ (the natural cause:
/// `#[serde(default)]` makes a field optional on Deserialize only, or
/// `#[serde(skip_serializing_if = "...")]` makes a field optional on
/// Serialize only), the app compile step rejects it with an error naming
/// the fix: declare two distinct Rust types, one per phase, instead of
/// sharing one derived type across a shape it does not actually share.
/// See `vorma_contract::contracts::TypeDef::classify_shared_name_with`
/// for exactly how "agree" is decided and where this resolution runs (an
/// app-graph-compile concern, not something this derive or
/// `vorma_contract::tsgen::Type` decides on its own).
#[proc_macro_derive(TsGen, attributes(serde))]
pub fn derive_ts_gen(input: TokenStream) -> TokenStream {
	let input = parse_macro_input!(input as syn::DeriveInput);
	ts_gen_derive::expand_ts_gen(input)
		.unwrap_or_else(syn::Error::into_compile_error)
		.into()
}
