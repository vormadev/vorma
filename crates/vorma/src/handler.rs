use std::collections::BTreeMap;
#[cfg(test)]
use std::sync::Arc;

use bytes::Bytes;
use http::header::{CACHE_CONTROL, CONTENT_TYPE, HeaderValue};
use http::{Response, StatusCode, Uri};
use serde::Serialize;
#[cfg(test)]
use serde_json::Value;
use url::{Url, form_urlencoded};
#[cfg(test)]
use vorma_tasks::ExecCtx;

use crate::constants::{
	CSS_BUNDLE_ATTR, QUERY_KEY_VORMA_JSON, VORMA_DATA_JSON_SCRIPT_EL_ID, VORMA_ROOT_EL_ID,
	X_VORMA_BUILD_SKEW,
};
use crate::document::{Document, DocumentRenderInput};
use crate::envutil::is_dev;
use crate::error::ViewErrorClientMsg;
use crate::htmlutil::{
	Element, render_element, render_element_to_string, render_module_script_to_string,
};
use crate::manifest::Manifest;
#[cfg(test)]
use crate::mux::NestedRouter;
use crate::mux::{RawRequest, RouteExecutionError, TaskRouteResult, ViewStackExecution};
use crate::response::{
	ResponsePlan, ResponseStatusPolicy, finalize_response_plan, insert_header,
	internal_server_error_response, json_response, plain_text_response,
	response_with_client_build_id,
};
use crate::view_payload::{SsrPayload, ViewPayload, build_view_payload};

/////////////////////////////////////////////////////////////////////
/////// View Handler
/////////////////////////////////////////////////////////////////////

pub(crate) fn is_json_request(uri: &Uri) -> bool {
	submitted_client_build_id(uri).is_some()
}

pub(crate) fn build_view_skew_response(
	request_uri: &Uri,
	expected_client_build_id: &str,
) -> Result<Option<Response<Bytes>>, String> {
	if !is_json_request(request_uri) {
		return Ok(None);
	}

	let submitted_client_build_id = submitted_client_build_id(request_uri);
	if submitted_client_build_id.as_deref() == Some(expected_client_build_id)
		|| submitted_client_build_id.as_deref() == Some("_")
	{
		return Ok(None);
	}

	let mut response = response_with_client_build_id(
		json_response(StatusCode::OK, Bytes::from_static(br#"{"ok":true}"#)),
		expected_client_build_id,
	)?;
	insert_header(response.headers_mut(), X_VORMA_BUILD_SKEW, "1")?;
	response
		.headers_mut()
		.insert(CACHE_CONTROL, HeaderValue::from_static("no-store"));
	Ok(Some(response))
}

#[cfg(test)]
pub(crate) struct ViewResponseInput<'a, S, E> {
	pub(crate) expected_client_build_id: &'a str,
	pub(crate) request: RawRequest,
	pub(crate) state: Arc<S>,
	pub(crate) exec_ctx: ExecCtx<E>,
	pub(crate) views: &'a NestedRouter<S, E>,
	pub(crate) manifest: &'a Manifest,
	pub(crate) document: &'a Document,
	pub(crate) public_filemap: Arc<BTreeMap<String, String>>,
}

#[cfg(test)]
pub(crate) async fn build_view_response<S, E>(
	input: ViewResponseInput<'_, S, E>,
) -> Result<Response<Bytes>, String>
where
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
{
	if let Some(response) =
		build_view_skew_response(input.request.uri(), input.expected_client_build_id)?
	{
		return Ok(response);
	}

	let Some(match_results) = input
		.views
		.find_nested_matches(input.request.path())
		.map_err(|err| err.to_string())?
	else {
		return view_not_found_response(input.expected_client_build_id);
	};

	let view_stack = input
		.views
		.execute_view_stack(
			input.state,
			input.exec_ctx,
			input.request.clone(),
			match_results.clone(),
			input.public_filemap.clone(),
		)
		.await
		.map_err(|err| err.to_string())?;
	build_view_response_from_results(ViewResponseResultsInput {
		expected_client_build_id: input.expected_client_build_id,
		request: &input.request,
		manifest: input.manifest,
		document: input.document,
		view_stack: &view_stack,
	})
}

pub(crate) struct ViewResponseResultsInput<'a, E> {
	pub(crate) expected_client_build_id: &'a str,
	pub(crate) request: &'a RawRequest,
	pub(crate) manifest: &'a Manifest,
	pub(crate) document: &'a Document,
	pub(crate) view_stack: &'a ViewStackExecution<E>,
}

pub(crate) fn view_not_found_response(
	expected_client_build_id: &str,
) -> Result<Response<Bytes>, String> {
	response_with_client_build_id(
		plain_text_response(StatusCode::NOT_FOUND, Bytes::new()),
		expected_client_build_id,
	)
}

pub(crate) fn build_view_response_from_results<E>(
	input: ViewResponseResultsInput<'_, E>,
) -> Result<Response<Bytes>, String>
where
	E: ViewErrorClientMsg,
{
	let is_json = is_json_request(input.request.uri());
	let (payload, merged_effects) =
		build_view_payload(input.manifest, input.document, input.view_stack, !is_json)?;

	if merged_effects.is_terminal_response() {
		return finalize_response_plan(
			ResponsePlan::ShortCircuit {
				effects: &merged_effects,
			},
			input.expected_client_build_id,
		);
	}

	let response = if is_json {
		json_response(
			StatusCode::OK,
			Bytes::from(serde_json::to_vec(&payload).map_err(|err| err.to_string())?),
		)
	} else {
		html_response(
			input.expected_client_build_id,
			input.manifest,
			input.document,
			&payload,
		)?
	};

	let status_policy = if payload.outermost_server_err_idx.is_some() {
		ResponseStatusPolicy::Suppress
	} else {
		ResponseStatusPolicy::Apply
	};
	let mut response = finalize_response_plan(
		ResponsePlan::Respond {
			effects: &merged_effects,
			response,
			status_policy,
		},
		input.expected_client_build_id,
	)?;
	if !response.headers().contains_key(CACHE_CONTROL) {
		response.headers_mut().insert(
			CACHE_CONTROL,
			HeaderValue::from_static("private, max-age=0, must-revalidate, no-cache"),
		);
	}
	Ok(response)
}

fn submitted_client_build_id(uri: &Uri) -> Option<String> {
	form_urlencoded::parse(uri.query().unwrap_or_default().as_bytes()).find_map(|(key, value)| {
		if key == QUERY_KEY_VORMA_JSON && !value.is_empty() {
			return Some(value.into_owned());
		}
		None
	})
}

fn html_response(
	expected_client_build_id: &str,
	manifest: &Manifest,
	document: &Document,
	payload: &ViewPayload,
) -> Result<Response<Bytes>, String> {
	let mut vorma_head = String::new();
	let prepared = crate::head::Prepared {
		title: payload.title.clone(),
		meta: payload.meta_head_els.clone(),
		rest: payload.rest_head_els.clone(),
	};
	vorma_head.push_str(&document.render_head(&prepared)?);
	vorma_head.push('\n');
	vorma_head.push_str(&render_element(&manifest.critical_css_el())?);
	vorma_head.push('\n');

	if !is_dev() {
		for css in &payload.css_bundles {
			let css_bundle_el = Element {
				tag: "link".to_owned(),
				attributes: BTreeMap::from([
					("rel".to_owned(), "stylesheet".to_owned()),
					("href".to_owned(), css.clone()),
					(CSS_BUNDLE_ATTR.to_owned(), css.clone()),
				]),
				self_closing: true,
				..Element::default()
			};
			vorma_head.push('\n');
			vorma_head.push_str(&render_element(&css_bundle_el)?);
		}
	}

	let mut vorma_body = String::new();
	let ssr_payload = SsrPayload {
		client_build_id: expected_client_build_id.to_owned(),
		is_dev: is_dev(),
		deployment_id: vercel_deployment_id(),
		view_payload: payload.clone(),
	};
	let payload_json = json_for_script(&ssr_payload)?;
	let data_json_el = Element {
		tag: "script".to_owned(),
		attributes: BTreeMap::from([
			("type".to_owned(), "application/json".to_owned()),
			("id".to_owned(), VORMA_DATA_JSON_SCRIPT_EL_ID.to_owned()),
		]),
		dangerous_inner_html: payload_json,
		..Element::default()
	};
	vorma_body.push_str(&render_element(&data_json_el)?);
	vorma_body.push('\n');
	vorma_body.push_str(&format!(r#"<div id="{VORMA_ROOT_EL_ID}"></div>"#));
	vorma_body.push('\n');
	if !is_dev() {
		render_module_script_to_string(&manifest.client_entry.url, &mut vorma_body)?;
	} else {
		vorma_body.push_str(&to_dev_scripts(
			manifest.dev_vite_server_port,
			&manifest.client_entry.url,
			manifest.ui_variant == crate::UiVariant::React.as_str(),
		)?);
		vorma_body.push('\n');
		vorma_body.push_str(&format!(
			"<script>{}</script>",
			refresh_script_inner_html(manifest)
		));
	}

	let html = document.render(DocumentRenderInput {
		vorma_head: &vorma_head,
		vorma_body: &vorma_body,
	})?;
	let mut response = Response::new(Bytes::from(html));
	response.headers_mut().insert(
		CONTENT_TYPE,
		HeaderValue::from_static("text/html; charset=utf-8"),
	);
	Ok(response)
}

fn vercel_deployment_id() -> String {
	let enabled = std::env::var("VERCEL_SKEW_PROTECTION_ENABLED")
		.ok()
		.is_some_and(|value| matches!(value.as_str(), "1" | "t" | "T" | "TRUE" | "true" | "True"));
	if enabled {
		return std::env::var("VERCEL_DEPLOYMENT_ID").unwrap_or_default();
	}
	String::new()
}

fn to_dev_scripts(port: i32, client_entry: &str, is_react: bool) -> Result<String, String> {
	let mut out = String::new();
	let vite_origin = vite_dev_origin(port, client_entry);
	if is_react {
		render_element_to_string(
			&Element {
				tag: "script".to_owned(),
				attributes: BTreeMap::from([("type".to_owned(), "module".to_owned())]),
				dangerous_inner_html: format!(
					"\nimport RefreshRuntime from \"{vite_origin}/@react-refresh\";\nRefreshRuntime.injectIntoGlobalHook(window);\nwindow.$RefreshReg$ = () => {{}};\nwindow.$RefreshSig$ = () => (type) => type;\nwindow.__vite_plugin_react_preamble_installed__ = true;\n"
				),
				..Element::default()
			},
			&mut out,
		)?;
		out.push('\n');
	}
	render_module_script_to_string(&format!("{vite_origin}/@vite/client"), &mut out)?;
	out.push('\n');
	render_module_script_to_string(client_entry, &mut out)?;
	Ok(out)
}

fn vite_dev_origin(port: i32, client_entry: &str) -> String {
	Url::parse(client_entry)
		.ok()
		.filter(|url| matches!(url.scheme(), "http" | "https"))
		.map(|url| url.origin().ascii_serialization())
		.filter(|origin| origin != "null")
		.unwrap_or_else(|| format!("http://127.0.0.1:{port}"))
}

pub(crate) fn refresh_script_inner_html(manifest: &Manifest) -> String {
	let script = include_str!("refresh_script.js")
		.replace(
			"__REPLACE_ME_WITH_REFRESH_PORT__",
			&manifest.dev_mux_port.to_string(),
		)
		.replace(
			"__REPLACE_ME_WITH_REFRESH_TOKEN__",
			&manifest.dev_refresh_token,
		);
	format!("\n{script}")
}

fn json_for_script(value: impl Serialize) -> Result<String, String> {
	let json = serde_json::to_string(&value).map_err(|err| err.to_string())?;
	Ok(json
		.replace('&', "\\u0026")
		.replace('<', "\\u003c")
		.replace('>', "\\u003e")
		.replace('\u{2028}', "\\u2028")
		.replace('\u{2029}', "\\u2029"))
}

/////////////////////////////////////////////////////////////////////
/////// API Handler
/////////////////////////////////////////////////////////////////////

pub(crate) struct ApiResponseInput<'a, E> {
	pub(crate) expected_client_build_id: &'a str,
	pub(crate) result: &'a TaskRouteResult<E>,
}

pub(crate) fn build_api_response<E>(
	input: ApiResponseInput<'_, E>,
) -> Result<Response<Bytes>, String> {
	let effects = input.result.response_effects();
	if let Some(error) = input.result.error() {
		if effects.is_terminal_response() {
			return finalize_response_plan(
				ResponsePlan::ShortCircuit { effects },
				input.expected_client_build_id,
			);
		}

		if matches!(error, RouteExecutionError::Input(input_error) if input_error.is_bad_request())
		{
			return finalize_response_plan(
				ResponsePlan::Respond {
					effects: input.result.middleware_effects(),
					response: plain_text_response(
						StatusCode::BAD_REQUEST,
						Bytes::from(format!("{error}\n")),
					),
					status_policy: ResponseStatusPolicy::Suppress,
				},
				input.expected_client_build_id,
			);
		}

		return finalize_response_plan(
			ResponsePlan::Respond {
				effects: input.result.middleware_effects(),
				response: internal_server_error_response(),
				status_policy: ResponseStatusPolicy::Suppress,
			},
			input.expected_client_build_id,
		);
	}

	let response = if effects.is_terminal_response() {
		return finalize_response_plan(
			ResponsePlan::ShortCircuit { effects },
			input.expected_client_build_id,
		);
	} else {
		let data = input
			.result
			.data()
			.ok_or_else(|| "missing resource data".to_owned())?;
		json_response(
			StatusCode::OK,
			Bytes::from(serde_json::to_vec(data).map_err(|err| err.to_string())?),
		)
	};

	finalize_response_plan(
		ResponsePlan::Respond {
			effects,
			response,
			status_policy: ResponseStatusPolicy::Apply,
		},
		input.expected_client_build_id,
	)
}

/////////////////////////////////////////////////////////////////////
/////// Response Helpers
/////////////////////////////////////////////////////////////////////

#[cfg(test)]
#[path = "handler_tests/mod.rs"]
mod handler_tests;
