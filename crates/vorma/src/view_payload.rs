use std::collections::{BTreeMap, BTreeSet};

use serde::{Deserialize, Serialize};

use crate::document::Document;
use crate::envutil::is_dev;
use crate::error::ViewErrorClientMsg;
use crate::htmlutil::Element;
use crate::manifest::Manifest;
use crate::mux::{RouteExecutionError, ViewStackExecution};
use crate::response::{ResponseEffects, merge_response_effects};

#[derive(Clone, Debug, Default, Deserialize, PartialEq, Serialize)]
pub(crate) struct SsrPayload {
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub(crate) client_build_id: String,
	#[serde(default, skip_serializing_if = "is_false")]
	pub(crate) is_dev: bool,
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub(crate) deployment_id: String,

	#[serde(flatten)]
	pub(crate) view_payload: ViewPayload,
}

#[derive(Clone, Debug, Default, Deserialize, PartialEq, Serialize)]
pub(crate) struct ViewPayload {
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) matched_patterns: Vec<String>,
	#[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
	pub(crate) params: BTreeMap<String, String>,
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) splat_values: Vec<String>,
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) search_schemas: Vec<serde_json::Value>,

	#[serde(default, skip_serializing_if = "Option::is_none")]
	pub(crate) title: Option<Element>,
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) meta_head_els: Vec<Element>,
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) rest_head_els: Vec<Element>,

	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub(crate) outermost_server_err: String,
	#[serde(default, skip_serializing_if = "Option::is_none")]
	pub(crate) outermost_server_err_idx: Option<usize>,

	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) import_urls: Vec<String>,
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) deps: Vec<String>,
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) css_bundles: Vec<String>,

	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) views_data: Vec<serde_json::Value>,
}

fn is_false(value: &bool) -> bool {
	!*value
}

pub(crate) fn build_view_payload<E>(
	manifest: &Manifest,
	document: &Document,
	view_stack: &ViewStackExecution<E>,
	include_prod_preloads: bool,
) -> Result<(ViewPayload, ResponseEffects), String>
where
	E: ViewErrorClientMsg,
{
	let response_effects_refs = response_effects_refs_for_payload(view_stack);
	let merged_effects = merge_response_effects(&response_effects_refs);
	let effects_short_circuited = merged_effects.is_terminal_response();
	let matched_patterns = view_stack.matched_patterns().to_vec();
	let params = view_stack
		.params()
		.iter()
		.map(|(key, value)| (key.clone(), value.clone()))
		.collect();
	let splat_values = view_stack.splat_values().to_vec();

	if effects_short_circuited {
		let mut raw_head_els = Vec::new();
		if let Some(head_builder) = merged_effects.head_builder_ref() {
			raw_head_els.extend_from_slice(head_builder.elements());
		}
		let prepared_head = document.prepare_head(&raw_head_els);
		return Ok((
			ViewPayload {
				matched_patterns,
				params,
				splat_values,
				title: prepared_head.title,
				meta_head_els: prepared_head.meta,
				rest_head_els: prepared_head.rest,
				..ViewPayload::default()
			},
			merged_effects,
		));
	}

	let search_schemas = matched_patterns
		.iter()
		.map(|pattern| {
			manifest
				.search_schemas
				.get(pattern)
				.cloned()
				.ok_or_else(|| format!("no search schema found for matched pattern: {pattern}"))
		})
		.collect::<Result<Vec<_>, _>>()?;

	let mut import_urls = Vec::with_capacity(matched_patterns.len());
	let mut views_data = Vec::new();
	let mut deps = Vec::new();
	let mut seen_deps = BTreeSet::new();
	let mut css_bundles = Vec::new();
	let mut seen_css_bundles = BTreeSet::new();

	append_unique(&mut deps, &mut seen_deps, &manifest.client_entry.dep_urls);
	append_unique(
		&mut css_bundles,
		&mut seen_css_bundles,
		&manifest.client_entry.css_bundle_urls,
	);

	let mut outermost_server_err = String::new();
	let mut outermost_server_err_idx = Option::None;

	let terminal_view_index = terminal_view_index(view_stack);
	for (idx, pattern) in matched_patterns.iter().enumerate() {
		if terminal_view_index.is_some_and(|terminal_view_index| idx > terminal_view_index) {
			break;
		}
		let route_mod = manifest
			.client_views
			.get(pattern)
			.ok_or_else(|| format!("no view module found for matched pattern: {pattern}"))?;
		import_urls.push(route_mod.url.clone());
		append_unique(&mut deps, &mut seen_deps, &route_mod.dep_urls);
		append_unique(
			&mut css_bundles,
			&mut seen_css_bundles,
			&route_mod.css_bundle_urls,
		);

		let result = view_stack
			.view_results()
			.get(idx)
			.ok_or_else(|| format!("missing view result for matched pattern: {pattern}"))?;
		if let Some(error) = result.error() {
			outermost_server_err = view_client_msg(error);
			outermost_server_err_idx = Some(idx);
			break;
		}
		if result.ran_task() && !effects_short_circuited {
			let data = result
				.data()
				.ok_or_else(|| format!("missing view data for executed view pattern: {pattern}"))?;
			views_data.push(data.clone());
		}
	}

	let mut raw_head_els = Vec::new();
	if let Some(head_builder) = merged_effects.head_builder_ref() {
		raw_head_els.extend_from_slice(head_builder.elements());
	}

	if !is_dev() && include_prod_preloads {
		for dep in &deps {
			raw_head_els.push(Element {
				tag: "link".to_owned(),
				attributes: BTreeMap::from([
					("rel".to_owned(), "modulepreload".to_owned()),
					("href".to_owned(), dep.clone()),
				]),
				self_closing: true,
				..Element::default()
			});
		}
		if let Some(assets) = &manifest.client_core_assets {
			if !seen_deps.contains(&assets.module_url) {
				raw_head_els.push(Element {
					tag: "link".to_owned(),
					attributes: BTreeMap::from([
						("rel".to_owned(), "modulepreload".to_owned()),
						("href".to_owned(), assets.module_url.clone()),
					]),
					self_closing: true,
					..Element::default()
				});
			}
			raw_head_els.push(Element {
				tag: "link".to_owned(),
				attributes: BTreeMap::from([
					("rel".to_owned(), "preload".to_owned()),
					("href".to_owned(), assets.wasm_url.clone()),
					("as".to_owned(), "fetch".to_owned()),
					("type".to_owned(), "application/wasm".to_owned()),
					("crossorigin".to_owned(), "anonymous".to_owned()),
				]),
				self_closing: true,
				..Element::default()
			});
		}
	}

	let prepared_head = document.prepare_head(&raw_head_els);
	let payload = ViewPayload {
		matched_patterns,
		params,
		splat_values,
		search_schemas,
		title: prepared_head.title,
		meta_head_els: prepared_head.meta,
		rest_head_els: prepared_head.rest,
		outermost_server_err,
		outermost_server_err_idx,
		import_urls,
		deps,
		css_bundles,
		views_data,
	};
	Ok((payload, merged_effects))
}

fn terminal_view_index<E>(view_stack: &ViewStackExecution<E>) -> Option<usize> {
	match view_stack.terminal_boundary() {
		Some(crate::mux::ViewStackTerminalBoundary::View { index }) => Some(index),
		Some(crate::mux::ViewStackTerminalBoundary::Middleware) | Option::None => Option::None,
	}
}

fn response_effects_refs_for_payload<E>(
	view_stack: &ViewStackExecution<E>,
) -> Vec<Option<&ResponseEffects>> {
	let mut effects_list = Vec::new();
	effects_list.push(Some(view_stack.middleware_effects()));

	if matches!(
		view_stack.terminal_boundary(),
		Some(crate::mux::ViewStackTerminalBoundary::Middleware)
	) {
		return effects_list;
	}

	let terminal_view_index = terminal_view_index(view_stack);
	for (idx, result) in view_stack.view_results().iter().enumerate() {
		if terminal_view_index.is_some_and(|terminal_view_index| idx > terminal_view_index) {
			break;
		}
		effects_list.push(result.response_effects());
	}

	effects_list
}

fn view_client_msg<E>(error: &RouteExecutionError<E>) -> String
where
	E: ViewErrorClientMsg,
{
	if let RouteExecutionError::Input(input_error) = error
		&& input_error.is_bad_request()
	{
		return input_error.to_string();
	}
	if let RouteExecutionError::Task(vorma_tasks::Error::Failed(error)) = error
		&& let Some(client_msg) = error.view_error_client_msg()
	{
		return client_msg.to_owned();
	}
	"An unexpected error occurred.".to_owned()
}

fn append_unique(out: &mut Vec<String>, seen: &mut BTreeSet<String>, items: &[String]) {
	for item in items {
		if seen.insert(item.clone()) {
			out.push(item.clone());
		}
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn view_payload_json_keys_are_snake_case() {
		let payload = SsrPayload {
			client_build_id: "build-id".to_owned(),
			is_dev: true,
			view_payload: ViewPayload {
				matched_patterns: vec!["/".to_owned()],
				params: BTreeMap::from([("id".to_owned(), "123".to_owned())]),
				title: Some(Element {
					tag: "title".to_owned(),
					text_content: "Hello".to_owned(),
					..Element::default()
				}),
				import_urls: vec!["/entry.js".to_owned()],
				views_data: vec![serde_json::json!({"ok": true})],
				..ViewPayload::default()
			},
			..SsrPayload::default()
		};

		let json = serde_json::to_value(&payload).unwrap();

		assert_eq!(json["client_build_id"], "build-id");
		assert_eq!(json["is_dev"], true);
		assert_eq!(json["matched_patterns"][0], "/");
		assert_eq!(json["params"]["id"], "123");
		assert!(json.get("title").is_some());
		assert_eq!(json["import_urls"][0], "/entry.js");
		assert_eq!(json["views_data"][0]["ok"], true);
	}
}
