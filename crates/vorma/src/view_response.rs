//! View execution report response projection.

use std::borrow::Cow;
use std::collections::BTreeMap;

use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64_STANDARD;
use bytes::Bytes;
use http::Response;
use sha2::{Digest, Sha256};
use url::Url;

use crate::config::UiVariant;
use crate::contracts::{DocumentContract, DocumentElementContract};
use crate::document_renderer::{
	DocumentRenderError, DocumentRenderInput, render_document_element, trusted_element_parts,
};
use crate::execution_engine::RouteExecutionReport;
use crate::head::{
	HEAD_ATTR_AS, HEAD_ATTR_CHARSET, HEAD_ATTR_CROSSORIGIN, HEAD_ATTR_HREF, HEAD_ATTR_ID,
	HEAD_ATTR_NAME, HEAD_ATTR_PROPERTY, HEAD_ATTR_REL, HEAD_ATTR_SRC, HEAD_ATTR_TYPE,
	HEAD_REL_ICON, HEAD_REL_PRELOAD, HEAD_TAG_LINK, HEAD_TAG_META, HEAD_TAG_SCRIPT, HEAD_TAG_TITLE,
	META_DESCRIPTION_NAME,
};
use crate::payload_projection::{PayloadProjectionError, project_view_payload};
use crate::response_finalizer::{
	FinalizerError, HeadElement, SsrPayload, finalize_terminal_response,
	finalize_view_html_response, finalize_view_json_response, ssr_payload_json,
	suppress_response_body_preserving_content_length,
};
use crate::runtime_manifest::RuntimeManifest;

/// HTML script element id containing the fixed SSR payload.
pub const SSR_PAYLOAD_SCRIPT_ID: &str = crate::constants::VORMA_DATA_JSON_SCRIPT_EL_ID;
/// Attribute used to identify Vorma-managed CSS bundle links.
pub const CSS_BUNDLE_ATTR: &str = "data-vorma-css-bundle";

const HEAD_REL_CANONICAL: &str = "canonical";
const HEAD_REL_MODULEPRELOAD: &str = "modulepreload";
const HEAD_REL_STYLESHEET: &str = "stylesheet";
const HEAD_AS_FETCH: &str = "fetch";
const HEAD_TYPE_WASM: &str = "application/wasm";
const SCRIPT_TYPE_MODULE: &str = "module";
const SCRIPT_TYPE_JSON: &str = "application/json";
const HEAD_CROSSORIGIN_ANONYMOUS: &str = "anonymous";
const DEV_REACT_REFRESH_PATH: &str = "/@react-refresh";
const DEV_VITE_CLIENT_PATH: &str = "/@vite/client";
const DEV_REFRESH_PORT_PLACEHOLDER: &str = "__REPLACE_ME_WITH_REFRESH_PORT__";
const DEV_REFRESH_TOKEN_PLACEHOLDER: &str = "__REPLACE_ME_WITH_REFRESH_TOKEN__";
const META_VIEWPORT_NAME: &str = "viewport";
const META_ROBOTS_NAME: &str = "robots";
const OPEN_GRAPH_TITLE_PROPERTY: &str = "og:title";
const OPEN_GRAPH_DESCRIPTION_PROPERTY: &str = "og:description";
const OPEN_GRAPH_URL_PROPERTY: &str = "og:url";
const OPEN_GRAPH_TYPE_PROPERTY: &str = "og:type";
const OPEN_GRAPH_LOCALE_PROPERTY: &str = "og:locale";
const OPEN_GRAPH_SITE_NAME_PROPERTY: &str = "og:site_name";
const OPEN_GRAPH_DETERMINER_PROPERTY: &str = "og:determiner";
const VERCEL_SKEW_PROTECTION_ENABLED_ENV_KEY: &str = "VERCEL_SKEW_PROTECTION_ENABLED";
const VERCEL_DEPLOYMENT_ID_ENV_KEY: &str = "VERCEL_DEPLOYMENT_ID";

/// Trusted SSR HTML shell inputs.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ViewHtmlResponseInput<'a> {
	body_markup: Cow<'a, str>,
	is_dev: bool,
	deployment_id: Cow<'a, str>,
}

impl<'a> ViewHtmlResponseInput<'a> {
	/// Create SSR HTML input from already-rendered trusted application body markup.
	pub fn new(body_markup: impl Into<Cow<'a, str>>) -> Self {
		Self {
			body_markup: body_markup.into(),
			is_dev: false,
			deployment_id: vercel_deployment_id().into(),
		}
	}

	/// Set whether this response belongs to a dev generation.
	pub fn with_is_dev(mut self, is_dev: bool) -> Self {
		self.is_dev = is_dev;
		self
	}

	/// Set the deployment identifier embedded in the SSR payload.
	pub fn with_deployment_id(mut self, deployment_id: impl Into<Cow<'a, str>>) -> Self {
		self.deployment_id = deployment_id.into();
		self
	}

	/// Trusted application body markup.
	pub fn body_markup(&self) -> &str {
		&self.body_markup
	}

	/// Whether this response belongs to a dev generation.
	pub fn is_dev(&self) -> bool {
		self.is_dev
	}

	/// Deployment identifier embedded in the SSR payload.
	pub fn deployment_id(&self) -> &str {
		&self.deployment_id
	}
}

/// Finalize a JSON view response from a route execution report.
pub fn finalize_view_report_json_response(
	manifest: &RuntimeManifest,
	document: &DocumentContract,
	report: &RouteExecutionReport,
	client_build_id: &str,
) -> Result<Response<Bytes>, ViewResponseError> {
	if report.effects().is_terminal() {
		let response = finalize_terminal_response(report.effects(), client_build_id)
			.map_err(|source| ViewResponseError::Finalizer { source })?;
		return suppress_response_body_if_needed(response, report);
	}
	let mut payload = project_view_payload(manifest, report)
		.map_err(|source| ViewResponseError::PayloadProjection { source })?;
	prepare_payload_head(document, &mut payload, None)?;
	let response = finalize_view_json_response(&payload, report.effects(), client_build_id)
		.map_err(|source| ViewResponseError::Finalizer { source })?;
	suppress_response_body_if_needed(response, report)
}

/// Finalize an HTML view response from a route execution report.
pub fn finalize_view_report_html_response(
	manifest: &RuntimeManifest,
	document: &DocumentContract,
	report: &RouteExecutionReport,
	input: &ViewHtmlResponseInput<'_>,
	client_build_id: &str,
) -> Result<Response<Bytes>, ViewResponseError> {
	if report.effects().is_terminal() {
		let response = finalize_terminal_response(report.effects(), client_build_id)
			.map_err(|source| ViewResponseError::Finalizer { source })?;
		return suppress_response_body_if_needed(response, report);
	}
	let mut payload = project_view_payload(manifest, report)
		.map_err(|source| ViewResponseError::PayloadProjection { source })?;
	prepare_payload_head(
		document,
		&mut payload,
		if input.is_dev() { None } else { Some(manifest) },
	)?;
	let mut head_markup = String::new();
	if let Some(title) = &payload.title {
		append_payload_head_element(title, &mut head_markup)?;
	}
	for element in &payload.meta_head_els {
		append_payload_head_element(element, &mut head_markup)?;
	}
	for element in &payload.rest_head_els {
		append_payload_head_element(element, &mut head_markup)?;
	}
	append_critical_css_head_element(manifest, &mut head_markup)?;
	if !input.is_dev() {
		append_css_bundle_head_elements(&payload.css_bundles, &mut head_markup)?;
	}
	let ssr_payload = SsrPayload {
		client_build_id: client_build_id.to_owned(),
		is_dev: input.is_dev(),
		deployment_id: input.deployment_id().to_owned(),
		view_payload: payload,
	};
	let payload_json =
		ssr_payload_json(&ssr_payload).map_err(|source| ViewResponseError::Finalizer { source })?;
	let mut body_markup = String::new();
	body_markup.push_str(input.body_markup());
	append_json_payload_script(&payload_json, &mut body_markup)?;
	body_markup.push('\n');
	body_markup.push_str(&format!(
		r#"<div id="{}"></div>"#,
		crate::constants::VORMA_ROOT_EL_ID
	));
	body_markup.push('\n');
	if input.is_dev() {
		append_dev_scripts(manifest, &mut body_markup)?;
	} else if !manifest.client_entry().url().is_empty() {
		append_module_script(manifest.client_entry().url(), &mut body_markup)?;
	}
	let response = finalize_view_html_response(
		document,
		DocumentRenderInput::new(&head_markup, &body_markup),
		report.effects(),
		client_build_id,
	)
	.map_err(|source| ViewResponseError::Finalizer { source })?;
	suppress_response_body_if_needed(response, report)
}

/// View response projection error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ViewResponseError {
	/// View payload projection failed.
	PayloadProjection {
		/// Source payload projection error.
		source: PayloadProjectionError,
	},
	/// Response finalization failed.
	Finalizer {
		/// Source finalizer error.
		source: FinalizerError,
	},
	/// Document element rendering failed.
	Document {
		/// Source document rendering error.
		source: DocumentRenderError,
	},
}

fn append_payload_head_element(
	element: &HeadElement,
	head_markup: &mut String,
) -> Result<(), ViewResponseError> {
	if element.tag.is_empty() {
		return Ok(());
	}
	let rendered = render_document_element(
		&DocumentElementContract::new(&element.tag)
			.with_known_safe_attributes(element.attributes_known_safe.clone())
			.with_boolean_attributes(element.boolean_attributes.clone())
			.with_dangerous_inner_html(element.dangerous_inner_html.clone())
			.with_self_closing(element.self_closing),
	)
	.map_err(|source| ViewResponseError::Document { source })?;
	head_markup.push_str(&rendered);
	head_markup.push('\n');
	Ok(())
}

fn append_json_payload_script(
	payload_json: &str,
	body_markup: &mut String,
) -> Result<(), ViewResponseError> {
	let rendered = render_document_element(
		&DocumentElementContract::new(HEAD_TAG_SCRIPT)
			.with_attributes(BTreeMap::from([
				(HEAD_ATTR_TYPE.to_owned(), SCRIPT_TYPE_JSON.to_owned()),
				(
					HEAD_ATTR_ID.to_owned(),
					crate::constants::VORMA_DATA_JSON_SCRIPT_EL_ID.to_owned(),
				),
			]))
			.with_dangerous_inner_html(payload_json),
	)
	.map_err(|source| ViewResponseError::Document { source })?;
	body_markup.push_str(&rendered);
	Ok(())
}

fn append_dev_scripts(
	manifest: &RuntimeManifest,
	body_markup: &mut String,
) -> Result<(), ViewResponseError> {
	body_markup.push_str(&dev_scripts(
		manifest.dev_vite_server_port(),
		manifest.client_entry().url(),
		manifest.ui_variant() == UiVariant::React.as_str(),
	)?);
	body_markup.push('\n');
	append_inline_script(&refresh_script_inner_html(manifest), body_markup)
}

fn dev_scripts(port: i32, client_entry: &str, is_react: bool) -> Result<String, ViewResponseError> {
	let mut out = String::new();
	let vite_origin = vite_dev_origin(port, client_entry);
	if is_react {
		append_inline_module_script(
			&format!(
				"\nimport RefreshRuntime from \"{vite_origin}{DEV_REACT_REFRESH_PATH}\";\nRefreshRuntime.injectIntoGlobalHook(window);\nwindow.$RefreshReg$ = () => {{}};\nwindow.$RefreshSig$ = () => (type) => type;\nwindow.__vite_plugin_react_preamble_installed__ = true;\n"
			),
			&mut out,
		)?;
		out.push('\n');
	}
	append_module_script(&format!("{vite_origin}{DEV_VITE_CLIENT_PATH}"), &mut out)?;
	out.push('\n');
	append_module_script(client_entry, &mut out)?;
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

fn vercel_deployment_id() -> String {
	vercel_deployment_id_from_env_values(
		std::env::var(VERCEL_SKEW_PROTECTION_ENABLED_ENV_KEY).ok(),
		std::env::var(VERCEL_DEPLOYMENT_ID_ENV_KEY).ok(),
	)
}

fn vercel_deployment_id_from_env_values(
	enabled: Option<String>,
	deployment_id: Option<String>,
) -> String {
	let enabled = enabled
		.as_deref()
		.is_some_and(|value| matches!(value, "1" | "t" | "T" | "TRUE" | "true" | "True"));
	if enabled {
		return deployment_id.unwrap_or_default();
	}
	String::new()
}

fn refresh_script_inner_html(manifest: &RuntimeManifest) -> String {
	let script = include_str!("refresh_script.js")
		.replace(
			DEV_REFRESH_PORT_PLACEHOLDER,
			&manifest.dev_mux_port().to_string(),
		)
		.replace(DEV_REFRESH_TOKEN_PLACEHOLDER, manifest.dev_refresh_token());
	format!("\n{script}")
}

/// CSP-compatible SHA-256 hash of the dev refresh script inner HTML.
/*
Empty outside dev: the refresh script only renders into dev documents, so a
production CSP built from this value must not whitelist an unused inline
script.
*/
pub(crate) fn dev_refresh_script_content_sha256(manifest: &RuntimeManifest) -> String {
	if !crate::envutil::is_dev() {
		return String::new();
	}
	BASE64_STANDARD.encode(Sha256::digest(
		refresh_script_inner_html(manifest).as_bytes(),
	))
}

fn append_module_script(src: &str, body_markup: &mut String) -> Result<(), ViewResponseError> {
	let rendered = render_document_element(
		&DocumentElementContract::new(HEAD_TAG_SCRIPT).with_attributes(BTreeMap::from([
			(HEAD_ATTR_TYPE.to_owned(), SCRIPT_TYPE_MODULE.to_owned()),
			(HEAD_ATTR_SRC.to_owned(), src.to_owned()),
		])),
	)
	.map_err(|source| ViewResponseError::Document { source })?;
	body_markup.push_str(&rendered);
	Ok(())
}

fn append_inline_module_script(
	script: &str,
	body_markup: &mut String,
) -> Result<(), ViewResponseError> {
	let rendered = render_document_element(
		&DocumentElementContract::new(HEAD_TAG_SCRIPT)
			.with_attributes(BTreeMap::from([(
				HEAD_ATTR_TYPE.to_owned(),
				SCRIPT_TYPE_MODULE.to_owned(),
			)]))
			.with_dangerous_inner_html(script),
	)
	.map_err(|source| ViewResponseError::Document { source })?;
	body_markup.push_str(&rendered);
	Ok(())
}

fn append_inline_script(script: &str, body_markup: &mut String) -> Result<(), ViewResponseError> {
	let rendered = render_document_element(
		&DocumentElementContract::new(HEAD_TAG_SCRIPT).with_dangerous_inner_html(script),
	)
	.map_err(|source| ViewResponseError::Document { source })?;
	body_markup.push_str(&rendered);
	Ok(())
}

fn append_critical_css_head_element(
	manifest: &RuntimeManifest,
	head_markup: &mut String,
) -> Result<(), ViewResponseError> {
	append_document_head_element(manifest.critical_css_element(), head_markup)
}

fn append_css_bundle_head_elements(
	css_bundles: &[String],
	head_markup: &mut String,
) -> Result<(), ViewResponseError> {
	for css_bundle in css_bundles {
		append_document_head_element(
			DocumentElementContract::new(HEAD_TAG_LINK)
				.with_attributes(BTreeMap::from([
					(CSS_BUNDLE_ATTR.to_owned(), css_bundle.clone()),
					(HEAD_ATTR_HREF.to_owned(), css_bundle.clone()),
					(HEAD_ATTR_REL.to_owned(), HEAD_REL_STYLESHEET.to_owned()),
				]))
				.with_self_closing(true),
			head_markup,
		)?;
	}
	Ok(())
}

fn append_document_head_element(
	element: DocumentElementContract,
	head_markup: &mut String,
) -> Result<(), ViewResponseError> {
	let rendered = render_document_element(&element)
		.map_err(|source| ViewResponseError::Document { source })?;
	head_markup.push_str(&rendered);
	head_markup.push('\n');
	Ok(())
}

fn suppress_response_body_if_needed(
	response: Response<Bytes>,
	report: &RouteExecutionReport,
) -> Result<Response<Bytes>, ViewResponseError> {
	if !report.suppress_body() {
		return Ok(response);
	}
	suppress_response_body_preserving_content_length(response)
		.map_err(|source| ViewResponseError::Finalizer { source })
}

fn prepare_payload_head(
	document: &DocumentContract,
	payload: &mut crate::response_finalizer::ViewPayload,
	prod_preload_manifest: Option<&RuntimeManifest>,
) -> Result<(), ViewResponseError> {
	let mut elements = document
		.head_defaults()
		.iter()
		.map(head_element_from_document_contract)
		.collect::<Result<Vec<_>, _>>()?;
	if let Some(manifest) = prod_preload_manifest {
		append_prod_preload_head_elements(manifest, &payload.deps, &mut elements);
	}
	if let Some(title) = payload.title.take() {
		elements.push(title);
	}
	elements.append(&mut payload.meta_head_els);
	elements.append(&mut payload.rest_head_els);
	let deduped = dedupe_head_elements(
		elements,
		document
			.head_dedupe_rules()
			.iter()
			.map(head_element_from_document_contract)
			.collect::<Result<Vec<_>, _>>()?,
	);
	for element in deduped {
		match element.tag.as_str() {
			HEAD_TAG_TITLE => payload.title = Some(element),
			HEAD_TAG_META => payload.meta_head_els.push(element),
			_ => payload.rest_head_els.push(element),
		}
	}
	Ok(())
}

fn append_prod_preload_head_elements(
	manifest: &RuntimeManifest,
	deps: &[String],
	elements: &mut Vec<HeadElement>,
) {
	for dep in deps {
		elements.push(modulepreload_head_element(dep));
	}
	if let Some(assets) = manifest.client_core_assets() {
		if !deps.iter().any(|dep| dep == assets.module_url()) {
			elements.push(modulepreload_head_element(assets.module_url()));
		}
		elements.push(HeadElement {
			tag: HEAD_TAG_LINK.to_owned(),
			attributes_known_safe: BTreeMap::from([
				(HEAD_ATTR_REL.to_owned(), HEAD_REL_PRELOAD.to_owned()),
				(HEAD_ATTR_HREF.to_owned(), assets.wasm_url().to_owned()),
				(HEAD_ATTR_AS.to_owned(), HEAD_AS_FETCH.to_owned()),
				(HEAD_ATTR_TYPE.to_owned(), HEAD_TYPE_WASM.to_owned()),
				(
					HEAD_ATTR_CROSSORIGIN.to_owned(),
					HEAD_CROSSORIGIN_ANONYMOUS.to_owned(),
				),
			]),
			self_closing: true,
			..HeadElement::default()
		});
	}
}

fn modulepreload_head_element(href: &str) -> HeadElement {
	HeadElement {
		tag: HEAD_TAG_LINK.to_owned(),
		attributes_known_safe: BTreeMap::from([
			(HEAD_ATTR_REL.to_owned(), HEAD_REL_MODULEPRELOAD.to_owned()),
			(HEAD_ATTR_HREF.to_owned(), href.to_owned()),
		]),
		self_closing: true,
		..HeadElement::default()
	}
}

fn head_element_from_document_contract(
	element: &DocumentElementContract,
) -> Result<HeadElement, ViewResponseError> {
	let parts =
		trusted_element_parts(element).map_err(|source| ViewResponseError::Document { source })?;
	Ok(HeadElement {
		tag: parts.tag,
		attributes_known_safe: parts.attributes_known_safe,
		boolean_attributes: parts.boolean_attributes,
		dangerous_inner_html: parts.dangerous_inner_html,
		self_closing: parts.self_closing,
	})
}

fn dedupe_head_elements(
	elements: Vec<HeadElement>,
	custom_rules: Vec<HeadElement>,
) -> Vec<HeadElement> {
	let rules = head_dedupe_rules(custom_rules);
	let mut result = Vec::with_capacity(elements.len());
	let mut seen_rule = std::collections::BTreeMap::<(String, usize), usize>::new();
	let mut seen_element = std::collections::BTreeMap::<HeadElement, usize>::new();
	for element in elements {
		if let Some(rules_for_tag) = rules.get(&element.tag) {
			let mut matched = false;
			for (rule_index, rule) in rules_for_tag.iter().enumerate() {
				if head_element_matches_rule(&element, rule) {
					let key = (element.tag.clone(), rule_index);
					if let Some(position) = seen_rule.get(&key) {
						result[*position] = element.clone();
					} else {
						seen_rule.insert(key, result.len());
						result.push(element.clone());
					}
					matched = true;
					break;
				}
			}
			if matched {
				continue;
			}
		}
		if let Some(position) = seen_element.get(&element) {
			result[*position] = element.clone();
		} else {
			seen_element.insert(element.clone(), result.len());
			result.push(element);
		}
	}
	result
}

fn head_dedupe_rules(
	custom_rules: Vec<HeadElement>,
) -> std::collections::BTreeMap<String, Vec<HeadElement>> {
	let mut rules = Vec::from([
		HeadElement {
			tag: HEAD_TAG_TITLE.to_owned(),
			..HeadElement::default()
		},
		head_meta_rule(HEAD_ATTR_NAME, META_DESCRIPTION_NAME),
		head_meta_rule(HEAD_ATTR_NAME, META_VIEWPORT_NAME),
		head_meta_rule(HEAD_ATTR_NAME, META_ROBOTS_NAME),
		head_meta_rule(HEAD_ATTR_CHARSET, HEAD_ATTR_ANY_VALUE),
		head_link_rule(HEAD_REL_ICON),
		head_link_rule(HEAD_REL_CANONICAL),
		head_meta_rule(HEAD_ATTR_PROPERTY, OPEN_GRAPH_TITLE_PROPERTY),
		head_meta_rule(HEAD_ATTR_PROPERTY, OPEN_GRAPH_DESCRIPTION_PROPERTY),
		head_meta_rule(HEAD_ATTR_PROPERTY, OPEN_GRAPH_URL_PROPERTY),
		head_meta_rule(HEAD_ATTR_PROPERTY, OPEN_GRAPH_TYPE_PROPERTY),
		head_meta_rule(HEAD_ATTR_PROPERTY, OPEN_GRAPH_LOCALE_PROPERTY),
		head_meta_rule(HEAD_ATTR_PROPERTY, OPEN_GRAPH_SITE_NAME_PROPERTY),
		head_meta_rule(HEAD_ATTR_PROPERTY, OPEN_GRAPH_DETERMINER_PROPERTY),
	]);
	rules.extend(custom_rules);
	let mut by_tag = std::collections::BTreeMap::<String, Vec<HeadElement>>::new();
	for rule in rules {
		let rules_for_tag = by_tag.entry(rule.tag.clone()).or_default();
		if !rules_for_tag.contains(&rule) {
			rules_for_tag.push(rule);
		}
	}
	by_tag
}

fn head_meta_rule(key: &str, value: &str) -> HeadElement {
	HeadElement {
		tag: HEAD_TAG_META.to_owned(),
		attributes_known_safe: std::collections::BTreeMap::from([(
			key.to_owned(),
			value.to_owned(),
		)]),
		..HeadElement::default()
	}
}

fn head_link_rule(rel: &str) -> HeadElement {
	HeadElement {
		tag: HEAD_TAG_LINK.to_owned(),
		attributes_known_safe: std::collections::BTreeMap::from([(
			HEAD_ATTR_REL.to_owned(),
			rel.to_owned(),
		)]),
		self_closing: true,
		..HeadElement::default()
	}
}

fn head_element_matches_rule(element: &HeadElement, rule: &HeadElement) -> bool {
	for (key, value) in &rule.attributes_known_safe {
		let Some(actual) = element.attributes_known_safe.get(key) else {
			return false;
		};
		if value != HEAD_ATTR_ANY_VALUE && actual != value {
			return false;
		}
	}
	for key in &rule.boolean_attributes {
		if !element.boolean_attributes.contains(key) {
			return false;
		}
	}
	true
}

const HEAD_ATTR_ANY_VALUE: &str = "\x00__vorma_headels_any__";

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::header::{CONTENT_LENGTH, HeaderName, HeaderValue, LOCATION};
	use http::{Method, StatusCode};

	use super::*;
	use crate::asset_capabilities::AssetCapabilities;
	use crate::contracts::DocumentElementContract;
	use crate::execution_engine::{
		ExecutionEngine, HandlerOutput, HandlerRegistry, RequestExecutionReport, RequestInput,
	};
	use crate::execution_plan::ExecutionPlan;
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, ViewDeclaration,
	};
	use crate::response_finalizer::{CLIENT_BUILD_ID_HEADER, ResponseEffects};
	use crate::runtime_manifest::{
		CRITICAL_CSS_STYLE_ELEMENT_ID, ClientCoreAssets, ClientModule, RuntimeViewModule,
	};
	use crate::test_support::route_type_contract;

	const TEST_CRITICAL_CSS: &str = "body{color:black}";

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	async fn executed_view_report() -> (RuntimeManifest, RouteExecutionReport) {
		executed_view_report_for_method(Method::GET).await
	}

	async fn executed_view_report_for_method(
		method: Method,
	) -> (RuntimeManifest, RouteExecutionReport) {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_view(ViewDeclaration::new(
			"/",
			"root.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("root"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();
		let assets = AssetCapabilities::new("/static/", Vec::new(), BTreeMap::new()).unwrap();
		let engine = ExecutionEngine::new(plan, assets);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("root"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_title(HeadElement {
				tag: "title".to_owned(),
				dangerous_inner_html: "Home".to_owned(),
				..HeadElement::default()
			});
			Ok(HandlerOutput::data(serde_json::json!({"ok": true})).with_effects(effects))
		});
		let report = engine
			.execute(RequestInput::new(method, "/"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::View(report) = report else {
			panic!("expected view report");
		};
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			TEST_CRITICAL_CSS,
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			Vec::new(),
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/assets/root.js",
				Vec::new(),
				vec!["/assets/root.css".to_owned()],
			)],
		)
		.with_client_entry(ClientModule::new(
			"/assets/entry.js",
			vec!["/assets/entry.js".to_owned(), "/assets/core.js".to_owned()],
			vec!["/assets/entry.css".to_owned()],
		))
		.with_client_core_assets(Some(ClientCoreAssets::new(
			"/assets/core.js",
			"/assets/core.wasm",
		)));
		(manifest, report)
	}

	#[tokio::test]
	async fn view_report_json_response_uses_payload_projection_and_effects() {
		let (manifest, report) = executed_view_report().await;

		let response = finalize_view_report_json_response(
			&manifest,
			&DocumentContract::default(),
			&report,
			"build-id",
		)
		.unwrap();
		let body: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert!(body.get("client_build_id").is_none());
		assert_eq!(body["views_data"], serde_json::json!([{"ok": true}]));
		assert_eq!(body["title"]["dangerous_inner_html"], "Home");
	}

	#[tokio::test]
	async fn view_report_html_response_embeds_head_and_ssr_payload() {
		let (manifest, report) = executed_view_report().await;
		let manifest = manifest
			.with_dev_metadata(5173, 4173, "refresh-token")
			.with_ui_variant(UiVariant::React.as_str());
		let document = DocumentContract::default();

		let response = finalize_view_report_html_response(
			&manifest,
			&document,
			&report,
			&ViewHtmlResponseInput::new("<main id=\"app\"></main>\n")
				.with_is_dev(true)
				.with_deployment_id("deploy-1"),
			"build-id",
		)
		.unwrap();
		let html = String::from_utf8(response.body().to_vec()).unwrap();

		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert!(html.contains("<title>Home</title>"));
		assert!(html.contains(&format!(
			"<style id=\"{CRITICAL_CSS_STYLE_ELEMENT_ID}\">{TEST_CRITICAL_CSS}</style>"
		)));
		assert!(!html.contains(CSS_BUNDLE_ATTR));
		assert!(!html.contains("modulepreload"));
		assert!(!html.contains("application/wasm"));
		assert!(html.contains("/@react-refresh"));
		assert!(html.contains("/@vite/client"));
		assert!(html.contains("refresh-token"));
		assert!(html.contains("<main id=\"app\"></main>"));
		assert!(html.contains(&format!(
			r#"<div id="{}"></div>"#,
			crate::constants::VORMA_ROOT_EL_ID
		)));
		assert!(html.contains(SSR_PAYLOAD_SCRIPT_ID));
		assert!(html.contains("\"client_build_id\":\"build-id\""));
		assert!(html.contains("\"is_dev\":true"));
		assert!(html.contains("\"deployment_id\":\"deploy-1\""));
	}

	#[test]
	fn view_html_response_input_derives_vercel_deployment_id_from_env_contract_values() {
		for enabled in ["1", "t", "T", "TRUE", "true", "True"] {
			assert_eq!(
				vercel_deployment_id_from_env_values(
					Some(enabled.to_owned()),
					Some("dpl_123".to_owned()),
				),
				"dpl_123"
			);
		}
		for disabled in [None, Some("".to_owned()), Some("false".to_owned())] {
			assert_eq!(
				vercel_deployment_id_from_env_values(disabled, Some("dpl_123".to_owned())),
				""
			);
		}

		let input = ViewHtmlResponseInput::new("").with_deployment_id("override");

		assert_eq!(input.deployment_id(), "override");
	}

	#[tokio::test]
	async fn view_report_html_response_embeds_production_css_bundle_links() {
		let (manifest, report) = executed_view_report().await;

		let response = finalize_view_report_html_response(
			&manifest,
			&DocumentContract::default(),
			&report,
			&ViewHtmlResponseInput::new("<main id=\"app\"></main>\n"),
			"build-id",
		)
		.unwrap();
		let html = String::from_utf8(response.body().to_vec()).unwrap();

		assert!(html.contains(&format!("{CSS_BUNDLE_ATTR}=\"/assets/entry.css\"")));
		assert!(html.contains(&format!("{CSS_BUNDLE_ATTR}=\"/assets/root.css\"")));
		assert!(html.contains("href=\"/assets/entry.js\""));
		assert!(html.contains("href=\"/assets/core.js\""));
		assert!(html.contains("rel=\"modulepreload\""));
		assert!(html.contains("href=\"/assets/core.wasm\""));
		assert!(html.contains("as=\"fetch\""));
		assert!(html.contains("type=\"application/wasm\""));
		assert!(html.contains("href=\"/assets/root.css\""));
		assert!(html.contains("rel=\"stylesheet\""));
		assert!(html.contains(&format!(
			r#"<div id="{}"></div>"#,
			crate::constants::VORMA_ROOT_EL_ID
		)));
		assert!(html.contains("src=\"/assets/entry.js\""));
	}

	#[tokio::test]
	async fn view_report_html_response_suppresses_head_body() {
		let (manifest, head_report) = executed_view_report_for_method(Method::HEAD).await;
		let (_, get_report) = executed_view_report().await;
		let document = DocumentContract::default();
		let input = ViewHtmlResponseInput::new("<main id=\"app\"></main>\n");
		let get_response = finalize_view_report_html_response(
			&manifest,
			&document,
			&get_report,
			&input,
			"build-id",
		)
		.unwrap();

		let head_response = finalize_view_report_html_response(
			&manifest,
			&document,
			&head_report,
			&input,
			"build-id",
		)
		.unwrap();

		assert!(head_report.suppress_body());
		assert_eq!(
			head_response.headers()[CONTENT_LENGTH],
			get_response.body().len().to_string().as_str()
		);
		assert!(head_response.body().is_empty());
	}

	#[tokio::test]
	async fn view_report_json_response_applies_document_head_defaults_and_dedupe() {
		let (manifest, report) = executed_view_report().await;
		let document = DocumentContract::new(
			Vec::new(),
			Vec::new(),
			vec![
				DocumentElementContract::new("title").with_text_content("Default"),
				DocumentElementContract::new("meta")
					.with_attributes(BTreeMap::from([
						("name".to_owned(), "description".to_owned()),
						("content".to_owned(), "Default Description".to_owned()),
					]))
					.with_self_closing(true),
			],
			Vec::new(),
			Vec::new(),
		);

		let response =
			finalize_view_report_json_response(&manifest, &document, &report, "build-id").unwrap();
		let body: serde_json::Value = serde_json::from_slice(response.body()).unwrap();

		assert_eq!(body["title"]["dangerous_inner_html"], "Home");
		assert_eq!(body["meta_head_els"].as_array().unwrap().len(), 1);
		assert_eq!(
			body["meta_head_els"][0]["attributes_known_safe"]["content"],
			"Default Description"
		);
	}

	fn minimal_runtime_manifest() -> RuntimeManifest {
		RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			Vec::new(),
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/assets/root.js",
				Vec::new(),
				Vec::new(),
			)],
		)
	}

	#[test]
	fn dev_refresh_script_hash_is_empty_outside_dev_and_present_in_dev() {
		let manifest = minimal_runtime_manifest();

		assert_eq!(dev_refresh_script_content_sha256(&manifest), "");
		let dev_hash =
			crate::envutil::with_dev_mode(|| dev_refresh_script_content_sha256(&manifest));
		assert!(!dev_hash.is_empty());
	}

	#[test]
	fn vite_dev_origin_uses_client_entry_origin_with_loopback_fallback() {
		assert_eq!(
			vite_dev_origin(5199, "http://localhost:5173/src/entry.tsx"),
			"http://localhost:5173"
		);
		assert_eq!(
			vite_dev_origin(5199, "http://[::1]:5173/src/entry.tsx"),
			"http://[::1]:5173"
		);
		assert_eq!(
			vite_dev_origin(5199, "/src/entry.tsx"),
			"http://127.0.0.1:5199"
		);
	}

	#[test]
	fn refresh_script_inner_html_substitutes_all_dev_manifest_placeholders() {
		let script = refresh_script_inner_html(&minimal_runtime_manifest());

		assert!(!script.contains(DEV_REFRESH_PORT_PLACEHOLDER));
		assert!(!script.contains(DEV_REFRESH_TOKEN_PLACEHOLDER));
	}

	#[tokio::test]
	async fn view_report_terminal_error_response_does_not_require_manifest_modules() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(crate::framework_graph::MiddlewareDeclaration::new(
			handler_id("middleware"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/items",
			"items.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("view"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();
		let assets = AssetCapabilities::new("/static/", Vec::new(), BTreeMap::new()).unwrap();
		let engine = ExecutionEngine::new(plan, assets);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("middleware"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_status_with_text(StatusCode::BAD_REQUEST, "bad input");
			effects.set_header(
				HeaderName::from_static("x-terminal"),
				HeaderValue::from_static("1"),
			);
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		handlers.insert(handler_id("view"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"unused": true})))
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/items"), &handlers)
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

		let response = finalize_view_report_json_response(
			&manifest,
			&DocumentContract::default(),
			&report,
			"build-id",
		)
		.unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(response.headers()["x-terminal"], "1");
		assert_eq!(response.body(), &Bytes::from_static(b"bad input\n"));
	}

	#[tokio::test]
	async fn view_report_terminal_redirect_response_does_not_require_manifest_modules() {
		let mut declarations = FrameworkDeclarations::default();
		declarations.add_middleware(crate::framework_graph::MiddlewareDeclaration::new(
			handler_id("middleware"),
		));
		declarations.add_view(ViewDeclaration::new(
			"/items",
			"items.tsx",
			serde_json::json!({}),
			route_type_contract(),
			handler_id("view"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();
		let assets = AssetCapabilities::new("/static/", Vec::new(), BTreeMap::new()).unwrap();
		let engine = ExecutionEngine::new(plan, assets);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("middleware"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.redirect(StatusCode::FOUND, "/login").unwrap();
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		handlers.insert(handler_id("view"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"unused": true})))
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/items"), &handlers)
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

		let response = finalize_view_report_html_response(
			&manifest,
			&DocumentContract::default(),
			&report,
			&ViewHtmlResponseInput::new("<main></main>"),
			"build-id",
		)
		.unwrap();

		assert_eq!(response.status(), StatusCode::FOUND);
		assert_eq!(response.headers()[LOCATION], "/login");
		assert!(response.body().is_empty());
	}
}
