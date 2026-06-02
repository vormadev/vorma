#![forbid(unsafe_code)]

use proc_macro::TokenStream;
use quote::{format_ident, quote};
use syn::parse::{Parse, ParseStream};
use syn::{
	Expr, GenericArgument, Ident, LitStr, Pat, PathArguments, Result, Token, Type,
	parse_macro_input,
};
use vorma_matcher::{Matcher, Options as MatcherOptions, SegmentKind};

#[proc_macro]
pub fn __vorma_view(input: TokenStream) -> TokenStream {
	let input = parse_macro_input!(input as ViewMacroInput);
	expand_view(input)
		.unwrap_or_else(syn::Error::into_compile_error)
		.into()
}

#[proc_macro]
pub fn __vorma_resource(input: TokenStream) -> TokenStream {
	let input = parse_macro_input!(input as ResourceMacroInput);
	expand_resource(input)
		.unwrap_or_else(syn::Error::into_compile_error)
		.into()
}

#[proc_macro_derive(TsGen, attributes(serde))]
pub fn derive_ts_gen(input: TokenStream) -> TokenStream {
	let input = parse_macro_input!(input as syn::DeriveInput);
	expand_ts_gen(input)
		.unwrap_or_else(syn::Error::into_compile_error)
		.into()
}

struct ViewMacroInput {
	client_file: LitStr,
	pattern: LitStr,
	input_type: Type,
	output_type: Type,
	handler: ClosureExpr,
}

struct ResourceMacroInput {
	kind: Option<Expr>,
	method: Expr,
	pattern: LitStr,
	input_type: Type,
	output_type: Type,
	handler: ClosureExpr,
}

struct ClosureExpr {
	pat: Pat,
	body: Expr,
}

impl Parse for ViewMacroInput {
	fn parse(input: ParseStream<'_>) -> Result<Self> {
		Ok(Self {
			client_file: parse_lit_str_field(input, "client_file")?,
			pattern: parse_lit_str_field(input, "pattern")?,
			input_type: parse_type_field(input, "input")?,
			output_type: parse_type_field(input, "output")?,
			handler: parse_closure_field(input, "handler")?,
		})
	}
}

impl Parse for ResourceMacroInput {
	fn parse(input: ParseStream<'_>) -> Result<Self> {
		let kind = if next_field_is(input, "kind") {
			Some(parse_expr_field(input, "kind")?)
		} else {
			None
		};

		Ok(Self {
			kind,
			method: parse_expr_field(input, "method")?,
			pattern: parse_lit_str_field(input, "pattern")?,
			input_type: parse_type_field(input, "input")?,
			output_type: parse_type_field(input, "output")?,
			handler: parse_closure_field(input, "handler")?,
		})
	}
}

fn expand_view(input: ViewMacroInput) -> Result<proc_macro2::TokenStream> {
	let client_file = input.client_file;
	let pattern = input.pattern;
	let input_type = input.input_type;
	let output_type = input.output_type;
	let handler_pat = input.handler.pat;
	let handler_body = input.handler.body;
	let params = params_from_pattern(&pattern)?;
	let params_def = params_definition(&params);

	Ok(quote! {{
		#params_def

		::vorma::View::from_static(
			#pattern,
			#client_file,
			::vorma::__private::type_resolver::<#input_type>,
			::vorma::__private::type_resolver::<#output_type>,
			::vorma::__private::search_schema_resolver::<#input_type>,
			|ctx| {
				::vorma::__private::run_static_view::<
					_,
					_,
					#input_type,
					__VormaParams,
					#output_type,
				>(
					ctx,
					|ctx| {
						::std::boxed::Box::pin(async move {
							let #handler_pat = ctx;
							#handler_body
						})
					},
				)
			},
		)
	}})
}

fn expand_resource(input: ResourceMacroInput) -> Result<proc_macro2::TokenStream> {
	let method = input.method;
	let pattern = input.pattern;
	let input_type = input.input_type;
	let output_type = input.output_type;
	let handler_pat = input.handler.pat;
	let handler_body = input.handler.body;
	let params = params_from_pattern(&pattern)?;
	let params_def = params_definition(&params);
	let kind = input
		.kind
		.map(|kind| quote! { ::std::option::Option::Some(#kind) })
		.unwrap_or_else(|| quote! { ::std::option::Option::None });

	Ok(quote! {{
		#params_def

		::vorma::Resource::from_static(
			#method,
			#pattern,
			#kind,
			::vorma::__private::type_resolver::<#input_type>,
			::vorma::__private::type_resolver::<#output_type>,
			|ctx| {
				::vorma::__private::run_static_resource::<
					_,
					_,
					#input_type,
					__VormaParams,
					#output_type,
				>(
					ctx,
					|ctx| {
						::std::boxed::Box::pin(async move {
							let #handler_pat = ctx;
							#handler_body
						})
					},
				)
			},
		)
	}})
}

fn parse_lit_str_field(input: ParseStream<'_>, name: &str) -> Result<LitStr> {
	parse_field_name(input, name)?;
	input.parse::<Token![:]>()?;
	let value = input.parse::<LitStr>()?;
	input.parse::<Token![;]>()?;
	Ok(value)
}

fn parse_type_field(input: ParseStream<'_>, name: &str) -> Result<Type> {
	parse_field_name(input, name)?;
	input.parse::<Token![:]>()?;
	let value = input.parse::<Type>()?;
	input.parse::<Token![;]>()?;
	Ok(value)
}

fn parse_expr_field(input: ParseStream<'_>, name: &str) -> Result<Expr> {
	parse_field_name(input, name)?;
	input.parse::<Token![:]>()?;
	let value = input.parse::<Expr>()?;
	input.parse::<Token![;]>()?;
	Ok(value)
}

fn parse_closure_field(input: ParseStream<'_>, name: &str) -> Result<ClosureExpr> {
	parse_field_name(input, name)?;
	input.parse::<Token![:]>()?;
	input.parse::<Token![|]>()?;
	let pat = Pat::parse_single(input)?;
	input.parse::<Token![|]>()?;
	let body = input.parse::<Expr>()?;
	input.parse::<Token![;]>()?;
	Ok(ClosureExpr { pat, body })
}

fn parse_field_name(input: ParseStream<'_>, expected: &str) -> Result<()> {
	let ident = input.parse::<Ident>()?;
	if ident != expected {
		return Err(syn::Error::new(
			ident.span(),
			format!("expected `{expected}` field"),
		));
	}
	Ok(())
}

fn next_field_is(input: ParseStream<'_>, expected: &str) -> bool {
	input
		.fork()
		.parse::<Ident>()
		.is_ok_and(|ident| ident == expected)
}

fn params_from_pattern(pattern: &LitStr) -> Result<Vec<Ident>> {
	let pattern_value = pattern.value();
	let matcher = Matcher::builder(MatcherOptions {
		dynamic_param_prefix: ':',
		splat_segment_identifier: '*',
		explicit_index_segment_identifier: "_index".to_owned(),
	})
	.map_err(|err| syn::Error::new(pattern.span(), err))?;
	let pattern = matcher
		.normalize_pattern(&pattern_value)
		.map_err(|err| syn::Error::new(pattern.span(), err))?;
	let mut params = Vec::new();

	for segment in pattern.normalized_segments() {
		if segment.kind != SegmentKind::Dynamic {
			continue;
		}
		let name = segment.normalized_value.trim_start_matches(':');
		params.push(syn::parse_str::<Ident>(name).map_err(|_| {
			syn::Error::new(
				proc_macro2::Span::call_site(),
				format!("route param `{name}` cannot be used as a Rust field accessor"),
			)
		})?);
	}

	Ok(params)
}

fn params_definition(params: &[Ident]) -> proc_macro2::TokenStream {
	if params.is_empty() {
		return quote! {
			type __VormaParams = ();
		};
	}

	let fields = params.iter();
	let initializers = params.iter().map(|param| {
		let name = param.to_string();
		quote! {
			#param: params
				.get(#name)
				.cloned()
				.ok_or_else(|| {
					::vorma::__private::InputError::bad_request(
						::std::format!("missing route param `{}`", #name),
					)
				})?,
		}
	});

	quote! {
		#[allow(non_snake_case)]
		#[derive(Clone, Debug, Eq, PartialEq)]
		struct __VormaParams {
			#(pub #fields: ::std::string::String,)*
		}

		impl ::vorma::__private::PathParams for __VormaParams {
			fn from_raw_path_params(
				params: &::vorma::__private::Params,
			) -> ::std::result::Result<Self, ::vorma::__private::InputError> {
				Ok(Self {
					#(#initializers)*
				})
			}
		}
	}
}

fn expand_ts_gen(input: syn::DeriveInput) -> syn::Result<proc_macro2::TokenStream> {
	let ident = input.ident;
	let type_name = ident.to_string();
	let container = ContainerAttrs::from_attrs(&input.attrs)?;
	if !input.generics.params.is_empty() {
		return Err(syn::Error::new_spanned(
			input.generics,
			"generic TsGen derive is not supported yet; use a manual Type implementation for this shape",
		));
	}

	let data = input.data;
	let syn::Data::Struct(data) = data else {
		if let syn::Data::Enum(data) = data {
			return expand_ts_gen_enum(ident, input.generics, data, container);
		}
		return Err(syn::Error::new_spanned(
			ident,
			"TsGen derive currently supports structs and unit-only enums",
		));
	};

	if container.transparent {
		return expand_transparent_ts_gen_struct(ident, input.generics, data);
	}

	let syn::Fields::Named(fields) = data.fields else {
		return Err(syn::Error::new_spanned(
			ident,
			"TsGen derive currently supports named-field structs; use a manual Type implementation for this shape",
		));
	};

	let mut serialize_fields = Vec::new();
	let mut deserialize_fields = Vec::new();
	let mut collect_serialize = Vec::new();
	let mut collect_deserialize = Vec::new();

	for field in fields.named {
		let Some(field_ident) = &field.ident else {
			continue;
		};
		let attrs = FieldAttrs::from_attrs(&field.attrs)?;
		if attrs.flatten {
			return Err(syn::Error::new_spanned(
				field,
				"serde(flatten) is deferred for TsGen derive; use a manual Type implementation for this shape",
			));
		}
		if let Some(key_ty) = find_non_string_map_key(&field.ty) {
			return Err(syn::Error::new_spanned(
				key_ty,
				"non-string map keys are not supported by TsGen derive; use a manual Type implementation",
			));
		}

		let field_ty = &field.ty;
		let rust_field_name = field_ident.to_string();
		let serialize_field_name =
			attrs.serialize_name(&rust_field_name, container.rename_all_serialize);
		let deserialize_field_name =
			attrs.deserialize_name(&rust_field_name, container.rename_all_deserialize);
		let serialize_field_name_lit = syn::LitStr::new(&serialize_field_name, field_ident.span());
		let deserialize_field_name_lit =
			syn::LitStr::new(&deserialize_field_name, field_ident.span());

		if !attrs.skip && !attrs.skip_serializing {
			let optional = attrs.skip_serializing_if;
			serialize_fields.push(quote! {
				::vorma::FieldDef::new(
					#serialize_field_name_lit,
					<#field_ty as ::vorma::Type>::type_ref_for(::vorma::TypePhase::Serialize),
					#optional,
				)
			});
			collect_serialize.push(quote! {
				<#field_ty as ::vorma::Type>::collect_type_defs_for(
					::vorma::TypePhase::Serialize,
					registry,
				)?;
			});
		}

		if !attrs.skip && !attrs.skip_deserializing {
			let optional = attrs.default || is_option_type(field_ty);
			deserialize_fields.push(quote! {
				::vorma::FieldDef::new(
					#deserialize_field_name_lit,
					<#field_ty as ::vorma::Type>::type_ref_for(::vorma::TypePhase::Deserialize),
					#optional,
				)
			});
			collect_deserialize.push(quote! {
				<#field_ty as ::vorma::Type>::collect_type_defs_for(
					::vorma::TypePhase::Deserialize,
					registry,
				)?;
			});
		}
	}

	let type_name_lit = syn::LitStr::new(&type_name, ident.span());
	let serialize_type_key =
		quote! { concat!(module_path!(), "::", stringify!(#ident), "::serialize") };
	let deserialize_type_key =
		quote! { concat!(module_path!(), "::", stringify!(#ident), "::deserialize") };
	let (impl_generics, ty_generics, where_clause) = input.generics.split_for_impl();

	Ok(quote! {
		impl #impl_generics ::vorma::Type for #ident #ty_generics #where_clause {
			fn type_ref() -> ::vorma::TypeRef {
				::vorma::TypeRef::named_with_key(#serialize_type_key, #type_name_lit)
			}

			fn collect_type_defs(
				registry: &mut ::vorma::TypeRegistry,
			) -> ::std::result::Result<(), ::vorma::__private::tsgen::Error> {
				Self::collect_type_defs_for(::vorma::TypePhase::Serialize, registry)
			}

			fn type_ref_for(phase: ::vorma::TypePhase) -> ::vorma::TypeRef {
				match phase {
					::vorma::TypePhase::Serialize => {
						::vorma::TypeRef::named_with_key(#serialize_type_key, #type_name_lit)
					}
					::vorma::TypePhase::Deserialize => {
						::vorma::TypeRef::named_with_key(#deserialize_type_key, #type_name_lit)
					}
				}
			}

			fn collect_type_defs_for(
				phase: ::vorma::TypePhase,
				registry: &mut ::vorma::TypeRegistry,
			) -> ::std::result::Result<(), ::vorma::__private::tsgen::Error> {
				match phase {
					::vorma::TypePhase::Serialize => {
						let inserted = registry.try_define(::vorma::TypeDef::record_with_key(
							#serialize_type_key,
							#type_name_lit,
							vec![#(#serialize_fields),*],
						))?;
						if !inserted {
							return Ok(());
						}
						#(#collect_serialize)*
					}
					::vorma::TypePhase::Deserialize => {
						let inserted = registry.try_define(::vorma::TypeDef::record_with_key(
							#deserialize_type_key,
							#type_name_lit,
							vec![#(#deserialize_fields),*],
						))?;
						if !inserted {
							return Ok(());
						}
						#(#collect_deserialize)*
					}
				}
				Ok(())
			}
		}
	})
}

fn expand_ts_gen_enum(
	ident: syn::Ident,
	generics: syn::Generics,
	data: syn::DataEnum,
	container: ContainerAttrs,
) -> syn::Result<proc_macro2::TokenStream> {
	let type_name = ident.to_string();
	let type_name_lit = syn::LitStr::new(&type_name, ident.span());
	let serialize_type_key =
		quote! { concat!(module_path!(), "::", stringify!(#ident), "::serialize") };
	let deserialize_type_key =
		quote! { concat!(module_path!(), "::", stringify!(#ident), "::deserialize") };
	let (impl_generics, ty_generics, where_clause) = generics.split_for_impl();

	let mut serialize_variants = Vec::new();
	let mut deserialize_variants = Vec::new();
	for variant in data.variants {
		if !matches!(variant.fields, syn::Fields::Unit) {
			return Err(syn::Error::new_spanned(
				variant,
				"TsGen derive currently supports unit-only enums; use a manual Type implementation for data-carrying enum variants",
			));
		}

		let attrs = VariantAttrs::from_attrs(&variant.attrs)?;
		let rust_variant_name = variant.ident.to_string();
		let serialize_variant_name =
			attrs.serialize_name(&rust_variant_name, container.rename_all_serialize);
		let deserialize_variant_name =
			attrs.deserialize_name(&rust_variant_name, container.rename_all_deserialize);
		let serialize_variant_name_lit =
			syn::LitStr::new(&serialize_variant_name, variant.ident.span());
		let deserialize_variant_name_lit =
			syn::LitStr::new(&deserialize_variant_name, variant.ident.span());

		if !attrs.skip && !attrs.skip_serializing {
			serialize_variants.push(quote! { #serialize_variant_name_lit });
		}
		if !attrs.skip && !attrs.skip_deserializing {
			deserialize_variants.push(quote! { #deserialize_variant_name_lit });
		}
	}

	Ok(quote! {
		impl #impl_generics ::vorma::Type for #ident #ty_generics #where_clause {
			fn type_ref() -> ::vorma::TypeRef {
				::vorma::TypeRef::named_with_key(#serialize_type_key, #type_name_lit)
			}

			fn collect_type_defs(
				registry: &mut ::vorma::TypeRegistry,
			) -> ::std::result::Result<(), ::vorma::__private::tsgen::Error> {
				Self::collect_type_defs_for(::vorma::TypePhase::Serialize, registry)
			}

			fn type_ref_for(phase: ::vorma::TypePhase) -> ::vorma::TypeRef {
				match phase {
					::vorma::TypePhase::Serialize => {
						::vorma::TypeRef::named_with_key(#serialize_type_key, #type_name_lit)
					}
					::vorma::TypePhase::Deserialize => {
						::vorma::TypeRef::named_with_key(#deserialize_type_key, #type_name_lit)
					}
				}
			}

			fn collect_type_defs_for(
				phase: ::vorma::TypePhase,
				registry: &mut ::vorma::TypeRegistry,
			) -> ::std::result::Result<(), ::vorma::__private::tsgen::Error> {
				match phase {
					::vorma::TypePhase::Serialize => {
						registry.try_define(::vorma::TypeDef::string_enum_with_key(
							#serialize_type_key,
							#type_name_lit,
							vec![#(#serialize_variants),*],
						))?;
					}
					::vorma::TypePhase::Deserialize => {
						registry.try_define(::vorma::TypeDef::string_enum_with_key(
							#deserialize_type_key,
							#type_name_lit,
							vec![#(#deserialize_variants),*],
						))?;
					}
				}
				Ok(())
			}
		}
	})
}

fn expand_transparent_ts_gen_struct(
	ident: syn::Ident,
	generics: syn::Generics,
	data: syn::DataStruct,
) -> syn::Result<proc_macro2::TokenStream> {
	let fields = data.fields.iter().collect::<Vec<_>>();
	if fields.len() != 1 {
		return Err(syn::Error::new_spanned(
			ident,
			"serde(transparent) requires exactly one field for TsGen derive",
		));
	}
	let field_ty = &fields[0].ty;
	let type_name = ident.to_string();
	let type_name_lit = syn::LitStr::new(&type_name, ident.span());
	let serialize_type_key =
		quote! { concat!(module_path!(), "::", stringify!(#ident), "::serialize") };
	let deserialize_type_key =
		quote! { concat!(module_path!(), "::", stringify!(#ident), "::deserialize") };
	let (impl_generics, ty_generics, where_clause) = generics.split_for_impl();

	Ok(quote! {
		impl #impl_generics ::vorma::Type for #ident #ty_generics #where_clause {
			fn type_ref() -> ::vorma::TypeRef {
				::vorma::TypeRef::named_with_key(#serialize_type_key, #type_name_lit)
			}

			fn collect_type_defs(
				registry: &mut ::vorma::TypeRegistry,
			) -> ::std::result::Result<(), ::vorma::__private::tsgen::Error> {
				Self::collect_type_defs_for(::vorma::TypePhase::Serialize, registry)
			}

			fn type_ref_for(phase: ::vorma::TypePhase) -> ::vorma::TypeRef {
				match phase {
					::vorma::TypePhase::Serialize => {
						::vorma::TypeRef::named_with_key(#serialize_type_key, #type_name_lit)
					}
					::vorma::TypePhase::Deserialize => {
						::vorma::TypeRef::named_with_key(#deserialize_type_key, #type_name_lit)
					}
				}
			}

			fn collect_type_defs_for(
				phase: ::vorma::TypePhase,
				registry: &mut ::vorma::TypeRegistry,
			) -> ::std::result::Result<(), ::vorma::__private::tsgen::Error> {
				let inserted = registry.try_define(::vorma::TypeDef::alias_with_key(
					match phase {
						::vorma::TypePhase::Serialize => #serialize_type_key,
						::vorma::TypePhase::Deserialize => #deserialize_type_key,
					},
					#type_name_lit,
					<#field_ty as ::vorma::Type>::type_ref_for(phase),
				))?;
				if !inserted {
					return Ok(());
				}
				<#field_ty as ::vorma::Type>::collect_type_defs_for(phase, registry)
			}
		}
	})
}

#[derive(Clone, Copy, Default)]
struct ContainerAttrs {
	rename_all_serialize: Option<RenameRule>,
	rename_all_deserialize: Option<RenameRule>,
	transparent: bool,
}

impl ContainerAttrs {
	fn from_attrs(attrs: &[syn::Attribute]) -> syn::Result<Self> {
		let mut out = Self::default();
		for attr in attrs.iter().filter(|attr| attr.path().is_ident("serde")) {
			attr.parse_nested_meta(|meta| {
				if meta.path.is_ident("rename_all") {
					if meta.input.peek(syn::Token![=]) {
						let value = meta.value()?;
						let value: syn::LitStr = value.parse()?;
						let rule = parse_rename_rule(&value)?;
						out.rename_all_serialize = Some(rule);
						out.rename_all_deserialize = Some(rule);
						return Ok(());
					}
					meta.parse_nested_meta(|nested| {
						let value = nested.value()?;
						let value: syn::LitStr = value.parse()?;
						let rule = parse_rename_rule(&value)?;
						if nested.path.is_ident("serialize") {
							out.rename_all_serialize = Some(rule);
							return Ok(());
						}
						if nested.path.is_ident("deserialize") {
							out.rename_all_deserialize = Some(rule);
							return Ok(());
						}
						Err(nested.error("unsupported serde rename_all phase for TsGen derive"))
					})?;
					return Ok(());
				}
				if meta.path.is_ident("transparent") {
					out.transparent = true;
					return Ok(());
				}
				if meta.path.is_ident("rename") {
					let value = meta.value()?;
					let _: syn::LitStr = value.parse()?;
					return Ok(());
				}
				if meta.path.is_ident("default") {
					if meta.input.peek(syn::Token![=]) {
						let value = meta.value()?;
						let _: syn::LitStr = value.parse()?;
					}
					return Ok(());
				}
				Err(meta.error("unsupported serde container attribute for TsGen derive"))
			})?;
		}
		Ok(out)
	}
}

#[derive(Default)]
struct FieldAttrs {
	rename_serialize: Option<String>,
	rename_deserialize: Option<String>,
	skip: bool,
	skip_serializing: bool,
	skip_deserializing: bool,
	skip_serializing_if: bool,
	default: bool,
	flatten: bool,
}

impl FieldAttrs {
	fn from_attrs(attrs: &[syn::Attribute]) -> syn::Result<Self> {
		let mut out = Self::default();
		for attr in attrs.iter().filter(|attr| attr.path().is_ident("serde")) {
			attr.parse_nested_meta(|meta| {
				if meta.path.is_ident("rename") {
					if meta.input.peek(syn::Token![=]) {
						let value = meta.value()?;
						let value: syn::LitStr = value.parse()?;
						let value = value.value();
						out.rename_serialize = Some(value.clone());
						out.rename_deserialize = Some(value);
						return Ok(());
					}
					meta.parse_nested_meta(|nested| {
						let value = nested.value()?;
						let value: syn::LitStr = value.parse()?;
						if nested.path.is_ident("serialize") {
							out.rename_serialize = Some(value.value());
							return Ok(());
						}
						if nested.path.is_ident("deserialize") {
							out.rename_deserialize = Some(value.value());
							return Ok(());
						}
						Err(nested.error("unsupported serde rename phase for TsGen derive"))
					})?;
					return Ok(());
				}
				if meta.path.is_ident("skip") {
					out.skip = true;
					return Ok(());
				}
				if meta.path.is_ident("skip_serializing") {
					out.skip_serializing = true;
					return Ok(());
				}
				if meta.path.is_ident("skip_deserializing") {
					out.skip_deserializing = true;
					return Ok(());
				}
				if meta.path.is_ident("skip_serializing_if") {
					let value = meta.value()?;
					let _: syn::LitStr = value.parse()?;
					out.skip_serializing_if = true;
					return Ok(());
				}
				if meta.path.is_ident("default") {
					if meta.input.peek(syn::Token![=]) {
						let value = meta.value()?;
						let _: syn::LitStr = value.parse()?;
					}
					out.default = true;
					return Ok(());
				}
				if meta.path.is_ident("flatten") {
					out.flatten = true;
					return Ok(());
				}
				if meta.path.is_ident("serialize_with")
					|| meta.path.is_ident("deserialize_with")
					|| meta.path.is_ident("with")
				{
					return Err(meta.error(
						"custom serde codecs are not supported by TsGen derive; use a manual Type implementation",
					));
				}
				Err(meta.error("unsupported serde field attribute for TsGen derive"))
			})?;
		}
		Ok(out)
	}

	fn serialize_name(&self, rust_name: &str, rule: Option<RenameRule>) -> String {
		self.rename_serialize.clone().unwrap_or_else(|| {
			rule.map_or_else(
				|| rust_name.to_owned(),
				|rule| rule.apply_to_field(rust_name),
			)
		})
	}

	fn deserialize_name(&self, rust_name: &str, rule: Option<RenameRule>) -> String {
		self.rename_deserialize.clone().unwrap_or_else(|| {
			rule.map_or_else(
				|| rust_name.to_owned(),
				|rule| rule.apply_to_field(rust_name),
			)
		})
	}
}

#[derive(Default)]
struct VariantAttrs {
	rename_serialize: Option<String>,
	rename_deserialize: Option<String>,
	skip: bool,
	skip_serializing: bool,
	skip_deserializing: bool,
}

impl VariantAttrs {
	fn from_attrs(attrs: &[syn::Attribute]) -> syn::Result<Self> {
		let mut out = Self::default();
		for attr in attrs.iter().filter(|attr| attr.path().is_ident("serde")) {
			attr.parse_nested_meta(|meta| {
				if meta.path.is_ident("rename") {
					if meta.input.peek(syn::Token![=]) {
						let value = meta.value()?;
						let value: syn::LitStr = value.parse()?;
						let value = value.value();
						out.rename_serialize = Some(value.clone());
						out.rename_deserialize = Some(value);
						return Ok(());
					}
					meta.parse_nested_meta(|nested| {
						let value = nested.value()?;
						let value: syn::LitStr = value.parse()?;
						if nested.path.is_ident("serialize") {
							out.rename_serialize = Some(value.value());
							return Ok(());
						}
						if nested.path.is_ident("deserialize") {
							out.rename_deserialize = Some(value.value());
							return Ok(());
						}
						Err(nested.error("unsupported serde rename phase for TsGen derive"))
					})?;
					return Ok(());
				}
				if meta.path.is_ident("skip") {
					out.skip = true;
					return Ok(());
				}
				if meta.path.is_ident("skip_serializing") {
					out.skip_serializing = true;
					return Ok(());
				}
				if meta.path.is_ident("skip_deserializing") {
					out.skip_deserializing = true;
					return Ok(());
				}
				Err(meta.error("unsupported serde enum variant attribute for TsGen derive"))
			})?;
		}
		Ok(out)
	}

	fn serialize_name(&self, rust_name: &str, rule: Option<RenameRule>) -> String {
		self.rename_serialize.clone().unwrap_or_else(|| {
			rule.map_or_else(
				|| rust_name.to_owned(),
				|rule| rule.apply_to_variant(rust_name),
			)
		})
	}

	fn deserialize_name(&self, rust_name: &str, rule: Option<RenameRule>) -> String {
		self.rename_deserialize.clone().unwrap_or_else(|| {
			rule.map_or_else(
				|| rust_name.to_owned(),
				|rule| rule.apply_to_variant(rust_name),
			)
		})
	}
}

#[derive(Clone, Copy)]
enum RenameRule {
	Lower,
	Upper,
	Pascal,
	Camel,
	Snake,
	ScreamingSnake,
	Kebab,
	ScreamingKebab,
}

impl RenameRule {
	fn parse(value: &str) -> std::result::Result<Self, String> {
		match value {
			"lowercase" => Ok(Self::Lower),
			"UPPERCASE" => Ok(Self::Upper),
			"PascalCase" => Ok(Self::Pascal),
			"camelCase" => Ok(Self::Camel),
			"snake_case" => Ok(Self::Snake),
			"SCREAMING_SNAKE_CASE" => Ok(Self::ScreamingSnake),
			"kebab-case" => Ok(Self::Kebab),
			"SCREAMING-KEBAB-CASE" => Ok(Self::ScreamingKebab),
			_ => Err(format!("unsupported serde rename rule {value:?}")),
		}
	}

	fn apply_to_field(self, field: &str) -> String {
		match self {
			Self::Lower | Self::Snake => field.to_owned(),
			Self::Upper => field.to_ascii_uppercase(),
			Self::Pascal => pascal_case(field),
			Self::Camel => camel_case(&pascal_case(field)),
			Self::ScreamingSnake => field.to_ascii_uppercase(),
			Self::Kebab => field.replace('_', "-"),
			Self::ScreamingKebab => field.to_ascii_uppercase().replace('_', "-"),
		}
	}

	fn apply_to_variant(self, variant: &str) -> String {
		match self {
			Self::Pascal => variant.to_owned(),
			Self::Lower => variant.to_ascii_lowercase(),
			Self::Upper => variant.to_ascii_uppercase(),
			Self::Camel => camel_case(variant),
			Self::Snake => snake_case_variant(variant),
			Self::ScreamingSnake => snake_case_variant(variant).to_ascii_uppercase(),
			Self::Kebab => snake_case_variant(variant).replace('_', "-"),
			Self::ScreamingKebab => snake_case_variant(variant)
				.to_ascii_uppercase()
				.replace('_', "-"),
		}
	}
}

fn parse_rename_rule(value: &syn::LitStr) -> syn::Result<RenameRule> {
	RenameRule::parse(&value.value()).map_err(|err| syn::Error::new_spanned(value, err))
}

fn pascal_case(field: &str) -> String {
	let mut pascal = String::new();
	let mut capitalize = true;
	for ch in field.chars() {
		if ch == '_' {
			capitalize = true;
		} else if capitalize {
			pascal.push(ch.to_ascii_uppercase());
			capitalize = false;
		} else {
			pascal.push(ch);
		}
	}
	pascal
}

fn camel_case(value: &str) -> String {
	let Some(first) = value.chars().next() else {
		return String::new();
	};
	let first_len = first.len_utf8();
	first.to_ascii_lowercase().to_string() + &value[first_len..]
}

fn snake_case_variant(variant: &str) -> String {
	let mut snake = String::new();
	for (index, ch) in variant.char_indices() {
		if index > 0 && ch.is_uppercase() {
			snake.push('_');
		}
		snake.push(ch.to_ascii_lowercase());
	}
	snake
}

fn is_option_type(ty: &Type) -> bool {
	let Type::Path(path) = ty else {
		return false;
	};
	path.path
		.segments
		.last()
		.is_some_and(|segment| segment.ident == format_ident!("Option"))
}

fn find_non_string_map_key(ty: &Type) -> Option<&Type> {
	match ty {
		Type::Array(array) => find_non_string_map_key(&array.elem),
		Type::Path(path) => {
			let segment = path.path.segments.last()?;
			let ident = &segment.ident;
			if ident == &format_ident!("HashMap") || ident == &format_ident!("BTreeMap") {
				let key_ty = nth_generic_type(segment, 0)?;
				if is_string_type(key_ty) {
					return None;
				}
				return Some(key_ty);
			}
			if ident == &format_ident!("Option")
				|| ident == &format_ident!("Vec")
				|| ident == &format_ident!("Box")
			{
				return nth_generic_type(segment, 0).and_then(find_non_string_map_key);
			}
			None
		}
		_ => None,
	}
}

fn nth_generic_type(segment: &syn::PathSegment, index: usize) -> Option<&Type> {
	let PathArguments::AngleBracketed(args) = &segment.arguments else {
		return None;
	};
	args.args
		.iter()
		.filter_map(|arg| match arg {
			GenericArgument::Type(ty) => Some(ty),
			_ => None,
		})
		.nth(index)
}

fn is_string_type(ty: &Type) -> bool {
	let Type::Path(path) = ty else {
		return false;
	};
	path.path
		.segments
		.last()
		.is_some_and(|segment| segment.ident == format_ident!("String"))
}
