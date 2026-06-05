//! Procedural macros for Vorma.

#![deny(missing_docs)]
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
#[proc_macro_derive(TsGen, attributes(serde))]
pub fn derive_ts_gen(input: TokenStream) -> TokenStream {
	let input = parse_macro_input!(input as syn::DeriveInput);
	ts_gen_derive::expand_ts_gen(input)
		.unwrap_or_else(syn::Error::into_compile_error)
		.into()
}
