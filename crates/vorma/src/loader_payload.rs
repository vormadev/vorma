use std::collections::{BTreeMap, BTreeSet};

use serde::{Deserialize, Serialize};

use crate::document::Document;
use crate::envutil::is_dev;
use crate::error::LoaderErrorClientMsg;
use crate::htmlutil::Element;
use crate::manifest::Manifest;
use crate::mux::{NestedTasksResults, RouteExecutionError};
use crate::response::{Proxy, merge_proxy_responses};

#[derive(Clone, Debug, Default, Deserialize, PartialEq, Serialize)]
pub(crate) struct SsrPayload {
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub(crate) client_build_id: String,
	#[serde(default, skip_serializing_if = "is_false")]
	pub(crate) is_dev: bool,
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub(crate) deployment_id: String,

	#[serde(flatten)]
	pub(crate) loader_payload: LoaderPayload,
}

#[derive(Clone, Debug, Default, Deserialize, PartialEq, Serialize)]
pub(crate) struct LoaderPayload {
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
	pub(crate) loaders_data: Vec<serde_json::Value>,
}

fn is_false(value: &bool) -> bool {
	!*value
}

pub(crate) fn build_loader_payload<E>(
	manifest: &Manifest,
	document: &Document,
	match_results: &vorma_matcher::NestedMatches,
	tasks_results: &NestedTasksResults<E>,
	include_prod_preloads: bool,
) -> Result<(LoaderPayload, Proxy), String>
where
	E: LoaderErrorClientMsg,
{
	let terminal_idx = first_terminal_loader_idx(tasks_results);
	let matched_patterns = match_results
		.matches
		.iter()
		.map(|matched| matched.pattern.original_pattern().to_owned())
		.collect::<Vec<_>>();
	let response_proxy_refs = response_proxy_refs_for_payload(tasks_results, terminal_idx);
	let merged_proxy = merge_proxy_responses(&response_proxy_refs);
	let proxy_short_circuited = merged_proxy.is_terminal_response();
	let params = match_results
		.params
		.iter()
		.map(|(key, value)| (key.clone(), value.clone()))
		.collect();
	let splat_values = match_results.splat_values.clone();

	if proxy_short_circuited {
		let mut raw_head_els = Vec::new();
		if let Some(head_builder) = merged_proxy.head_builder_ref() {
			raw_head_els.extend_from_slice(head_builder.elements());
		}
		let prepared_head = document.prepare_head(&raw_head_els);
		return Ok((
			LoaderPayload {
				matched_patterns,
				params,
				splat_values,
				title: prepared_head.title,
				meta_head_els: prepared_head.meta,
				rest_head_els: prepared_head.rest,
				..LoaderPayload::default()
			},
			merged_proxy,
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
	let mut loaders_data = Vec::new();
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

	for (idx, pattern) in matched_patterns.iter().enumerate() {
		if terminal_idx.is_some_and(|terminal_idx| idx > terminal_idx) {
			break;
		}
		let route_mod = manifest
			.client_routes
			.get(pattern)
			.ok_or_else(|| format!("no route module found for matched pattern: {pattern}"))?;
		import_urls.push(route_mod.url.clone());
		append_unique(&mut deps, &mut seen_deps, &route_mod.dep_urls);
		append_unique(
			&mut css_bundles,
			&mut seen_css_bundles,
			&route_mod.css_bundle_urls,
		);

		let result = tasks_results
			.results()
			.get(idx)
			.ok_or_else(|| format!("missing loader result for matched pattern: {pattern}"))?;
		if let Some(error) = result.error() {
			outermost_server_err = loader_client_msg(error);
			outermost_server_err_idx = Some(idx);
			break;
		}
		if result.ran_task() && !proxy_short_circuited {
			let data = result.data().ok_or_else(|| {
				format!("missing loader data for executed loader pattern: {pattern}")
			})?;
			loaders_data.push(data.clone());
		}
	}

	let mut raw_head_els = Vec::new();
	if let Some(head_builder) = merged_proxy.head_builder_ref() {
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
	let payload = LoaderPayload {
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
		loaders_data,
	};
	Ok((payload, merged_proxy))
}

fn first_terminal_loader_idx<E>(tasks_results: &NestedTasksResults<E>) -> Option<usize> {
	if tasks_results
		.middleware_proxy()
		.is_some_and(Proxy::is_terminal_response)
	{
		return Some(0);
	}

	tasks_results
		.results()
		.iter()
		.enumerate()
		.find_map(|(idx, result)| {
			if result.error().is_some()
				|| result
					.response_proxy()
					.is_some_and(Proxy::is_terminal_response)
			{
				return Some(idx);
			}
			Option::None
		})
}

fn response_proxy_refs_for_payload<E>(
	tasks_results: &NestedTasksResults<E>,
	terminal_idx: Option<usize>,
) -> Vec<Option<&Proxy>> {
	let mut proxies = Vec::new();
	if let Some(proxy) = tasks_results.middleware_proxy() {
		proxies.push(Some(proxy));
	}

	for (idx, result) in tasks_results.results().iter().enumerate() {
		if terminal_idx.is_some_and(|terminal_idx| idx > terminal_idx) {
			break;
		}
		proxies.push(result.response_proxy());
	}

	proxies
}

fn loader_client_msg<E>(error: &RouteExecutionError<E>) -> String
where
	E: LoaderErrorClientMsg,
{
	if let RouteExecutionError::Input(input_error) = error
		&& input_error.is_bad_request()
	{
		return input_error.to_string();
	}
	if let RouteExecutionError::Task(vorma_tasks::Error::Failed(error)) = error
		&& let Some(client_msg) = error.loader_error_client_msg()
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
	fn loader_payload_json_keys_are_snake_case() {
		let payload = SsrPayload {
			client_build_id: "build-id".to_owned(),
			is_dev: true,
			loader_payload: LoaderPayload {
				matched_patterns: vec!["/".to_owned()],
				params: BTreeMap::from([("id".to_owned(), "123".to_owned())]),
				title: Some(Element {
					tag: "title".to_owned(),
					text_content: "Hello".to_owned(),
					..Element::default()
				}),
				import_urls: vec!["/entry.js".to_owned()],
				loaders_data: vec![serde_json::json!({"ok": true})],
				..LoaderPayload::default()
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
		assert_eq!(json["loaders_data"][0]["ok"], true);
	}
}
