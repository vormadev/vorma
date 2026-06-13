//! Projection from route execution reports into fixed browser payloads.

use std::collections::{BTreeMap, BTreeSet};

use crate::execution_engine::{HandlerRole, RouteExecutionReport};
use crate::response_finalizer::ViewPayload;
use crate::runtime_manifest::RuntimeManifest;

/// Build a browser view payload from a completed view execution report.
pub fn project_view_payload(
	manifest: &RuntimeManifest,
	report: &RouteExecutionReport,
) -> Result<ViewPayload, PayloadProjectionError> {
	let mut import_urls = Vec::new();
	let mut deps = Vec::new();
	let mut seen_deps = BTreeSet::new();
	let mut css_bundles = Vec::new();
	let mut seen_css_bundles = BTreeSet::new();
	let mut search_schemas = Vec::new();
	for pattern in report.matched_patterns() {
		search_schemas.push(
			manifest
				.search_schemas()
				.get(pattern)
				.cloned()
				.ok_or_else(|| PayloadProjectionError::MissingSearchSchema {
					pattern: pattern.clone(),
				})?,
		);
	}
	append_unique(
		&mut deps,
		&mut seen_deps,
		manifest.client_entry().dep_urls(),
	);
	append_unique(
		&mut css_bundles,
		&mut seen_css_bundles,
		manifest.client_entry().css_bundle_urls(),
	);
	let module_patterns = module_patterns_for_report(report);
	for pattern in module_patterns {
		let module = manifest.view_module(pattern).ok_or_else(|| {
			PayloadProjectionError::MissingViewModule {
				pattern: pattern.to_owned(),
			}
		})?;
		import_urls.push(module.import_url().to_owned());
		append_unique(&mut deps, &mut seen_deps, module.dep_urls());
		append_unique(
			&mut css_bundles,
			&mut seen_css_bundles,
			module.css_bundle_urls(),
		);
	}
	let views_data = report
		.committed()
		.iter()
		.filter(|commit| commit.role() == HandlerRole::View)
		.filter_map(|commit| commit.output().data_value().cloned())
		.collect();
	Ok(ViewPayload {
		matched_patterns: report.matched_patterns().to_vec(),
		params: report
			.params()
			.iter()
			.map(|(key, value)| (key.to_string(), value.to_string()))
			.collect::<BTreeMap<_, _>>(),
		splat_values: report
			.splat_values()
			.iter()
			.map(|value| value.to_string())
			.collect(),
		search_schemas,
		title: report.effects().title().cloned(),
		meta_head_els: report.effects().meta_head_elements().to_vec(),
		rest_head_els: report.effects().rest_head_elements().to_vec(),
		import_urls,
		deps,
		css_bundles,
		views_data,
		outermost_server_err: report
			.server_error()
			.map(|error| error.client_message().to_owned())
			.unwrap_or_default(),
		outermost_server_err_idx: report.server_error().map(|error| error.view_index()),
	})
}

/// View payload projection error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum PayloadProjectionError {
	/// Runtime manifest missed a search schema for a matched view.
	MissingSearchSchema {
		/// Matched view pattern.
		pattern: String,
	},
	/// Runtime manifest missed a module for a matched view.
	MissingViewModule {
		/// Matched view pattern.
		pattern: String,
	},
}

fn append_unique(target: &mut Vec<String>, seen: &mut BTreeSet<String>, values: &[String]) {
	for value in values {
		if seen.insert(value.clone()) {
			target.push(value.clone());
		}
	}
}

fn module_patterns_for_report(report: &RouteExecutionReport) -> &[String] {
	if let Some(error) = report.server_error() {
		return &report.matched_patterns()[..=error.view_index()];
	}
	report.matched_patterns()
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::Method;

	use super::*;
	use crate::asset_capabilities::AssetCapabilities;
	use crate::execution_engine::{
		ExecutionEngine, HandlerOutput, HandlerRegistry, RequestExecutionReport, RequestInput,
	};
	use crate::execution_plan::ExecutionPlan;
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, ViewDeclaration,
	};
	use crate::response_finalizer::{HeadElement, ResponseEffects};
	use crate::runtime_manifest::{ClientModule, RuntimeViewModule};
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn engine_for_story_views() -> ExecutionEngine {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({"root": true}),
			route_type_contract(),
			handler_id("root"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/stories/:id",
			"story.tsx",
			serde_json::json!({"story": true}),
			route_type_contract(),
			handler_id("story"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();
		let assets = AssetCapabilities::new("/static/", Vec::new(), BTreeMap::new()).unwrap();
		ExecutionEngine::new(plan, assets)
	}

	fn story_handlers() -> HandlerRegistry {
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("root"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"root": 1})))
		});
		handlers.insert(handler_id("story"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_title(HeadElement {
				tag: "title".to_owned(),
				dangerous_inner_html: "Story".to_owned(),
				..HeadElement::default()
			});
			Ok(HandlerOutput::data(serde_json::json!({"story": 2})).with_effects(effects))
		});
		handlers
	}

	fn story_manifest() -> RuntimeManifest {
		RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([
				("/".to_owned(), serde_json::json!({"root": true})),
				(
					"/stories/:id".to_owned(),
					serde_json::json!({"story": true}),
				),
			]),
			Vec::new(),
			BTreeMap::new(),
			vec![
				RuntimeViewModule::new(
					"/",
					"/assets/root.js",
					vec!["/assets/shared.js".to_owned()],
					Vec::new(),
				),
				RuntimeViewModule::new(
					"/stories/:id",
					"/assets/story.js",
					vec!["/assets/shared.js".to_owned()],
					vec!["/assets/story.css".to_owned()],
				),
			],
		)
		.with_client_entry(ClientModule::new(
			"/assets/entry.js",
			vec![
				"/assets/entry.js".to_owned(),
				"/assets/shared.js".to_owned(),
			],
			vec!["/assets/entry.css".to_owned()],
		))
	}

	#[tokio::test]
	async fn payload_projection_uses_report_and_runtime_manifest_only() {
		let engine = engine_for_story_views();
		let handlers = story_handlers();
		let report = engine
			.execute(RequestInput::new(Method::GET, "/stories/42"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};
		let manifest = story_manifest();

		let payload = project_view_payload(&manifest, &report).unwrap();

		assert_eq!(payload.matched_patterns, ["/", "/stories/:id"]);
		assert_eq!(
			payload.params,
			BTreeMap::from([("id".to_owned(), "42".to_owned())])
		);
		assert_eq!(payload.import_urls, ["/assets/root.js", "/assets/story.js"]);
		assert_eq!(payload.deps, ["/assets/entry.js", "/assets/shared.js"]);
		assert_eq!(
			payload.css_bundles,
			["/assets/entry.css", "/assets/story.css"]
		);
		assert_eq!(
			payload.views_data,
			[
				serde_json::json!({"root": 1}),
				serde_json::json!({"story": 2})
			]
		);
		assert_eq!(payload.title.unwrap().dangerous_inner_html, "Story");
	}

	#[tokio::test]
	async fn payload_projection_truncates_modules_and_data_on_parent_server_error() {
		let engine = engine_for_story_views();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("root"), |_| async {
			Err(crate::execution_engine::HandlerExecutionError::new(
				"parent failed",
			))
		});
		handlers.insert(handler_id("story"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"story": 2})))
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/stories/42"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};

		let payload = project_view_payload(&story_manifest(), &report).unwrap();

		assert_eq!(payload.matched_patterns, ["/", "/stories/:id"]);
		assert_eq!(
			payload.search_schemas,
			[
				serde_json::json!({"root": true}),
				serde_json::json!({"story": true})
			]
		);
		assert_eq!(payload.import_urls, ["/assets/root.js"]);
		assert!(payload.views_data.is_empty());
		assert_eq!(payload.outermost_server_err_idx, Some(0));
		assert_eq!(
			payload.outermost_server_err,
			"An unexpected error occurred."
		);
	}

	#[tokio::test]
	async fn payload_projection_keeps_prior_data_on_child_server_error() {
		let engine = engine_for_story_views();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("root"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"root": 1})))
		});
		handlers.insert(handler_id("story"), |_| async {
			Err(
				crate::execution_engine::HandlerExecutionError::with_client_message(
					"child failed",
					"client visible",
				),
			)
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/stories/42"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};

		let payload = project_view_payload(&story_manifest(), &report).unwrap();

		assert_eq!(payload.import_urls, ["/assets/root.js", "/assets/story.js"]);
		assert_eq!(payload.deps, ["/assets/entry.js", "/assets/shared.js"]);
		assert_eq!(
			payload.css_bundles,
			["/assets/entry.css", "/assets/story.css"]
		);
		assert_eq!(payload.views_data, [serde_json::json!({"root": 1})]);
		assert_eq!(payload.outermost_server_err_idx, Some(1));
		assert_eq!(payload.outermost_server_err, "client visible");
	}

	#[tokio::test]
	async fn payload_projection_keeps_error_index_when_client_message_is_empty() {
		let engine = engine_for_story_views();
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("root"), |_| async {
			Err(
				crate::execution_engine::HandlerExecutionError::with_client_message(
					"root failed",
					"",
				),
			)
		});
		handlers.insert(handler_id("story"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"story": 2})))
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/stories/42"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};

		let payload = project_view_payload(&story_manifest(), &report).unwrap();
		let payload_json = serde_json::to_value(&payload).unwrap();

		assert_eq!(payload.outermost_server_err_idx, Some(0));
		assert_eq!(payload.outermost_server_err, "");
		assert!(payload_json.get("outermost_server_err").is_none());
		assert_eq!(payload_json["outermost_server_err_idx"], 0);
	}

	#[tokio::test]
	async fn payload_projection_rejects_manifest_missing_matched_view_data() {
		let engine = engine_for_story_views();
		let handlers = story_handlers();
		let report = engine
			.execute(RequestInput::new(Method::GET, "/stories/42"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::new(),
			Vec::new(),
			BTreeMap::new(),
			Vec::new(),
		);

		let error = project_view_payload(&manifest, &report).unwrap_err();

		assert!(matches!(
			error,
			PayloadProjectionError::MissingSearchSchema { .. }
		));
	}
}
