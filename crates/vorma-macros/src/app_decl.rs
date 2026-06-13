use quote::quote;
use syn::parse::{Parse, ParseStream};
use syn::{Expr, Ident, LitStr, Pat, Result, Token, Type};
use vorma_matcher::{MatcherBuilder, Options as MatcherOptions, SegmentKind};

pub(crate) struct ViewMacroInput {
	client_file: LitStr,
	pattern: LitStr,
	input_type: Type,
	output_type: Type,
	handler: ClosureExpr,
}

pub(crate) struct ResourceMacroInput {
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

pub(crate) fn expand_view(input: ViewMacroInput) -> Result<proc_macro2::TokenStream> {
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

pub(crate) fn expand_resource(input: ResourceMacroInput) -> Result<proc_macro2::TokenStream> {
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
	let matcher = MatcherBuilder::new(MatcherOptions {
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
				.map(|value| value.to_string())
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
