//! Public-style document builder lowered into document contracts.

use std::collections::BTreeMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use http::Method;
use serde::Serialize;

use crate::asset_capabilities::public_source_path_key;
use crate::contracts::{DocumentAttributeContract, DocumentContract, DocumentElementContract};
use crate::document_renderer::{DocumentRenderInput, render_document};
use crate::execution_engine::RequestInput;
use crate::runtime_document::{
	RuntimeDocumentError, RuntimeDocumentFuture, RuntimeDocumentInput, RuntimeDocumentProvider,
};

const DOCUMENT_HASH_SOURCE_FORMAT: &str = "vorma-document-hash-source-v1";
const DEFAULT_HTML_LANG: &str = "en";
const DOCUMENT_ATTR_CLASS: &str = "class";
const DOCUMENT_ATTR_ID: &str = "id";
const DOCUMENT_ATTR_LANG: &str = "lang";
const DOCUMENT_DATA_ATTR_PREFIX: &str = "data-";

/// Build-safe document shell and default head declaration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Document {
	html_attributes: Vec<DocumentAttributeContract>,
	body_attributes: Vec<DocumentAttributeContract>,
	head_defaults: crate::HeadBuilder,
	head_dedupe_rules: crate::HeadBuilder,
	body_prefix: Vec<DocumentElementContract>,
}

impl Document {
	/// Create the default document shell.
	pub fn new() -> Self {
		Self {
			html_attributes: vec![DocumentAttributeContract::new(
				DOCUMENT_ATTR_LANG,
				DEFAULT_HTML_LANG,
				false,
				false,
			)],
			body_attributes: Vec::new(),
			head_defaults: crate::HeadBuilder::new(),
			head_dedupe_rules: crate::HeadBuilder::new(),
			body_prefix: Vec::new(),
		}
	}

	/// Mutate attributes on the root `<html>` element.
	pub fn html(&mut self) -> DocumentAttributes<'_> {
		DocumentAttributes {
			attributes: &mut self.html_attributes,
		}
	}

	/// Mutate attributes on the root `<body>` element.
	pub fn body(&mut self) -> DocumentAttributes<'_> {
		DocumentAttributes {
			attributes: &mut self.body_attributes,
		}
	}

	/// Default head entries applied to every HTML view response.
	pub fn head(&mut self) -> &mut crate::HeadBuilder {
		&mut self.head_defaults
	}

	/// Additional head dedupe rules applied when preparing merged head entries.
	pub fn head_dedupe_rules(&mut self) -> &mut crate::HeadBuilder {
		&mut self.head_dedupe_rules
	}

	/// Append an element before framework-owned body markup.
	#[cfg(test)]
	pub(crate) fn push_body_prefix(&mut self, element: DocumentElementContract) -> &mut Self {
		self.body_prefix.push(element);
		self
	}

	/// Convert this builder document into a committed document contract.
	pub(crate) fn into_contract(self) -> DocumentContract {
		DocumentContract::new(
			self.html_attributes,
			self.body_attributes,
			self.head_defaults.elements().to_vec(),
			self.head_dedupe_rules.elements().to_vec(),
			self.body_prefix,
		)
	}

	/// Build-stable identity facts consumed by build tooling.
	#[doc(hidden)]
	pub fn __build_identity(&self) -> Result<DocumentBuildIdentity<'_>, String> {
		let contract = DocumentContract::new(
			self.html_attributes.clone(),
			self.body_attributes.clone(),
			self.head_defaults.elements().to_vec(),
			self.head_dedupe_rules.elements().to_vec(),
			self.body_prefix.clone(),
		);
		render_document(&contract, DocumentRenderInput::new("", ""))
			.map_err(|source| source.to_string())?;
		Ok(DocumentBuildIdentity {
			html_attributes: self
				.html_attributes
				.iter()
				.map(DocumentBuildIdentityAttribute::from)
				.collect(),
			body_attributes: self
				.body_attributes
				.iter()
				.map(DocumentBuildIdentityAttribute::from)
				.collect(),
			head_defaults: self
				.head_defaults
				.elements()
				.iter()
				.map(DocumentBuildIdentityElement::from)
				.collect(),
			head_dedupe_rules: self
				.head_dedupe_rules
				.elements()
				.iter()
				.map(DocumentBuildIdentityElement::from)
				.collect(),
			body_prefix: self
				.body_prefix
				.iter()
				.map(DocumentBuildIdentityElement::from)
				.collect(),
		})
	}
}

impl Default for Document {
	fn default() -> Self {
		Self::new()
	}
}

/// Builder for document element attributes.
pub struct DocumentAttributes<'a> {
	attributes: &'a mut Vec<DocumentAttributeContract>,
}

impl DocumentAttributes<'_> {
	/// Set an escaped attribute.
	pub fn attribute(&mut self, name: impl Into<String>, value: impl Into<String>) -> &mut Self {
		self.upsert(DocumentAttributeContract::new(name, value, false, false));
		self
	}

	/// Set an attribute whose value is already trusted HTML.
	pub fn known_safe_attribute(
		&mut self,
		name: impl Into<String>,
		value: impl Into<String>,
	) -> &mut Self {
		self.upsert(DocumentAttributeContract::new(name, value, true, false));
		self
	}

	/// Set a boolean attribute.
	pub fn boolean_attribute(&mut self, name: impl Into<String>) -> &mut Self {
		self.upsert(DocumentAttributeContract::new(name, "", false, true));
		self
	}

	/// Set the `lang` attribute.
	pub fn lang(&mut self, value: impl Into<String>) -> &mut Self {
		self.attribute(DOCUMENT_ATTR_LANG, value)
	}

	/// Set the `id` attribute.
	pub fn id(&mut self, value: impl Into<String>) -> &mut Self {
		self.attribute(DOCUMENT_ATTR_ID, value)
	}

	/// Set the `class` attribute.
	pub fn class(&mut self, value: impl Into<String>) -> &mut Self {
		self.attribute(DOCUMENT_ATTR_CLASS, value)
	}

	/// Set a `data-*` attribute.
	pub fn data(&mut self, name: impl AsRef<str>, value: impl Into<String>) -> &mut Self {
		self.attribute(
			format!("{DOCUMENT_DATA_ATTR_PREFIX}{}", name.as_ref()),
			value,
		)
	}

	fn upsert(&mut self, attribute: DocumentAttributeContract) {
		self.attributes
			.retain(|existing| existing.name() != attribute.name());
		self.attributes.push(attribute);
	}
}

/// Borrowed build-stable document identity facts.
#[doc(hidden)]
pub struct DocumentBuildIdentity<'a> {
	/// Root `<html>` attributes.
	pub html_attributes: Vec<DocumentBuildIdentityAttribute<'a>>,
	/// Root `<body>` attributes.
	pub body_attributes: Vec<DocumentBuildIdentityAttribute<'a>>,
	/// Default head elements.
	pub head_defaults: Vec<DocumentBuildIdentityElement<'a>>,
	/// Head dedupe-rule elements.
	pub head_dedupe_rules: Vec<DocumentBuildIdentityElement<'a>>,
	/// Body prefix elements.
	pub body_prefix: Vec<DocumentBuildIdentityElement<'a>>,
}

/// Borrowed document attribute identity.
#[doc(hidden)]
pub struct DocumentBuildIdentityAttribute<'a> {
	/// Attribute name.
	pub name: &'a str,
	/// Attribute value.
	pub value: &'a str,
	/// Whether the value is already trusted HTML.
	pub known_safe: bool,
	/// Whether this is a boolean attribute.
	pub boolean: bool,
}

impl<'a> From<&'a DocumentAttributeContract> for DocumentBuildIdentityAttribute<'a> {
	fn from(attribute: &'a DocumentAttributeContract) -> Self {
		Self {
			name: attribute.name(),
			value: attribute.value(),
			known_safe: attribute.known_safe(),
			boolean: attribute.boolean(),
		}
	}
}

/// Borrowed document element identity.
#[doc(hidden)]
pub struct DocumentBuildIdentityElement<'a> {
	/// Element tag.
	pub tag: &'a str,
	/// Normal escaped attributes.
	pub attributes: &'a BTreeMap<String, String>,
	/// Known-safe attributes.
	pub attributes_known_safe: &'a BTreeMap<String, String>,
	/// Boolean attributes.
	pub boolean_attributes: &'a [String],
	/// Escaped text content.
	pub text_content: &'a str,
	/// Trusted inner HTML.
	pub dangerous_inner_html: &'a str,
	/// Whether this element is self-closing.
	pub self_closing: bool,
}

impl<'a> From<&'a DocumentElementContract> for DocumentBuildIdentityElement<'a> {
	fn from(element: &'a DocumentElementContract) -> Self {
		Self {
			tag: element.tag(),
			attributes: element.attributes(),
			attributes_known_safe: element.attributes_known_safe(),
			boolean_attributes: element.boolean_attributes(),
			text_content: element.text_content(),
			dangerous_inner_html: element.dangerous_inner_html(),
			self_closing: element.self_closing(),
		}
	}
}

/// Context passed to runtime document builders.
#[derive(Clone, Debug)]
pub struct DocumentBuildContext {
	request: RequestInput,
	runtime_input: Option<RuntimeDocumentInput>,
}

impl DocumentBuildContext {
	#[doc(hidden)]
	pub fn build() -> Self {
		Self {
			request: RequestInput::new(Method::GET, "/"),
			runtime_input: None,
		}
	}

	fn runtime(input: RuntimeDocumentInput) -> Self {
		Self {
			request: input.request().clone(),
			runtime_input: Some(input),
		}
	}

	/// Request used for the current document/default-head render.
	pub fn request(&self) -> crate::HttpRequest<'_> {
		crate::HttpRequest::new(&self.request)
	}

	/// Resolve a public static source path through committed asset capabilities.
	pub fn public_url(&self, src_path: &str) -> Result<String, String> {
		if let Some(input) = &self.runtime_input {
			return input
				.public_url(src_path)
				.map_err(|source| source.to_string());
		}
		let source_path = public_source_path_key(src_path)
			.ok_or_else(|| "public URL source path is empty".to_owned())?;
		Ok(format!("/{source_path}"))
	}
}

type DocumentBuilderFuture =
	Pin<Box<dyn Future<Output = Result<Document, String>> + Send + 'static>>;

/// Async document builder used by runtime HTML rendering.
#[derive(Clone)]
pub struct DocumentBuilder {
	build: Arc<dyn Fn(DocumentBuildContext) -> DocumentBuilderFuture + Send + Sync>,
}

impl DocumentBuilder {
	/// Create a document builder from an async function.
	pub fn new<F, Fut>(build: F) -> Self
	where
		F: Fn(DocumentBuildContext) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<Document, String>> + Send + 'static,
	{
		Self {
			build: Arc::new(move |context| Box::pin(build(context))),
		}
	}

	/// Build one public document from an explicit context.
	#[doc(hidden)]
	pub async fn build(&self, context: DocumentBuildContext) -> Result<Document, String> {
		(self.build)(context).await
	}

	/// Build one document contract from runtime input.
	pub(crate) async fn build_contract(
		&self,
		input: RuntimeDocumentInput,
	) -> Result<DocumentContract, DocumentBuilderError> {
		self.build_contract_from_context(DocumentBuildContext::runtime(input))
			.await
	}

	#[doc(hidden)]
	pub async fn build_root_document_hash_source(&self) -> Result<String, DocumentBuilderError> {
		let document = self
			.build(DocumentBuildContext::build())
			.await
			.map_err(DocumentBuilderError::new)?;
		document_hash_source(&document.into_contract())
	}

	async fn build_contract_from_context(
		&self,
		context: DocumentBuildContext,
	) -> Result<DocumentContract, DocumentBuilderError> {
		let document = (self.build)(context)
			.await
			.map_err(DocumentBuilderError::new)?;
		Ok(document.into_contract())
	}
}

impl Default for DocumentBuilder {
	fn default() -> Self {
		Self::new(|_| async { Ok(Document::new()) })
	}
}

impl RuntimeDocumentProvider for DocumentBuilder {
	fn document_for_request(&self, input: RuntimeDocumentInput) -> RuntimeDocumentFuture {
		let builder = self.clone();
		Box::pin(async move {
			builder
				.build_contract(input)
				.await
				.map_err(|error| RuntimeDocumentError::new(error.to_string()))
		})
	}
}

pub(crate) fn document_hash_source(
	document: &DocumentContract,
) -> Result<String, DocumentBuilderError> {
	serde_json::to_string(&DocumentHashSource::from(document))
		.map_err(|error| DocumentBuilderError::new(error.to_string()))
}

#[derive(Serialize)]
struct DocumentHashSource<'a> {
	format: &'static str,
	html_attributes: Vec<DocumentHashAttribute<'a>>,
	body_attributes: Vec<DocumentHashAttribute<'a>>,
	head_defaults: Vec<DocumentHashElement<'a>>,
	head_dedupe_rules: Vec<DocumentHashElement<'a>>,
	body_prefix: Vec<DocumentHashElement<'a>>,
}

impl<'a> From<&'a DocumentContract> for DocumentHashSource<'a> {
	fn from(document: &'a DocumentContract) -> Self {
		Self {
			format: DOCUMENT_HASH_SOURCE_FORMAT,
			html_attributes: document
				.html_attributes()
				.iter()
				.map(DocumentHashAttribute::from)
				.collect(),
			body_attributes: document
				.body_attributes()
				.iter()
				.map(DocumentHashAttribute::from)
				.collect(),
			head_defaults: document
				.head_defaults()
				.iter()
				.map(DocumentHashElement::from)
				.collect(),
			head_dedupe_rules: document
				.head_dedupe_rules()
				.iter()
				.map(DocumentHashElement::from)
				.collect(),
			body_prefix: document
				.body_prefix()
				.iter()
				.map(DocumentHashElement::from)
				.collect(),
		}
	}
}

#[derive(Serialize)]
struct DocumentHashAttribute<'a> {
	name: &'a str,
	value: &'a str,
	known_safe: bool,
	boolean: bool,
}

impl<'a> From<&'a DocumentAttributeContract> for DocumentHashAttribute<'a> {
	fn from(attribute: &'a DocumentAttributeContract) -> Self {
		Self {
			name: attribute.name(),
			value: attribute.value(),
			known_safe: attribute.known_safe(),
			boolean: attribute.boolean(),
		}
	}
}

#[derive(Serialize)]
struct DocumentHashElement<'a> {
	tag: &'a str,
	attributes: &'a BTreeMap<String, String>,
	attributes_known_safe: &'a BTreeMap<String, String>,
	boolean_attributes: &'a [String],
	text_content: &'a str,
	dangerous_inner_html: &'a str,
	self_closing: bool,
}

impl<'a> From<&'a DocumentElementContract> for DocumentHashElement<'a> {
	fn from(element: &'a DocumentElementContract) -> Self {
		Self {
			tag: element.tag(),
			attributes: element.attributes(),
			attributes_known_safe: element.attributes_known_safe(),
			boolean_attributes: element.boolean_attributes(),
			text_content: element.text_content(),
			dangerous_inner_html: element.dangerous_inner_html(),
			self_closing: element.self_closing(),
		}
	}
}

/// Document builder error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DocumentBuilderError {
	message: String,
}

impl DocumentBuilderError {
	/// Create a document builder error.
	pub fn new(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
		}
	}

	/// Error message.
	pub fn message(&self) -> &str {
		&self.message
	}
}

impl std::fmt::Display for DocumentBuilderError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		f.write_str(&self.message)
	}
}

impl std::error::Error for DocumentBuilderError {}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::Method;
	use serde_json::Value;

	use super::*;
	use crate::asset_capabilities::AssetCapabilities;
	use crate::contracts::DocumentElementContract;
	use crate::execution_engine::RequestInput;
	use crate::runtime_manifest::RuntimeManifest;

	const TEST_HEAD_TAG_META: &str = "meta";
	const TEST_HEAD_TAG_TITLE: &str = "title";

	#[test]
	fn document_builder_upserts_attributes_and_lowers_to_contract() {
		let mut document = Document::new();
		document.html().lang("fr").class("root");
		document.body().data("page", "home");
		document
			.head()
			.title("Home")
			.description("Welcome")
			.meta_charset("utf-8");
		document.push_body_prefix(DocumentElementContract::new("div").with_text_content("prefix"));

		let contract = document.into_contract();

		assert_eq!(contract.html_attributes()[0].name(), DOCUMENT_ATTR_LANG);
		assert_eq!(contract.html_attributes()[0].value(), "fr");
		assert_eq!(contract.html_attributes()[1].name(), DOCUMENT_ATTR_CLASS);
		assert_eq!(contract.body_attributes()[0].name(), "data-page");
		assert_eq!(contract.head_defaults()[0].tag(), TEST_HEAD_TAG_TITLE);
		assert_eq!(contract.head_defaults()[1].tag(), TEST_HEAD_TAG_META);
		assert_eq!(contract.body_prefix()[0].tag(), "div");
	}

	#[tokio::test]
	async fn document_builder_provider_builds_contract_from_runtime_request_and_public_urls() {
		let builder = DocumentBuilder::new(|context| async move {
			let mut document = Document::new();
			document.body().data("path", context.request().path());
			document.head().link([crate::HtmlAttribute::attr(
				"href",
				context.public_url("app.css")?,
			)]);
			Ok(document)
		});
		let public_filemap =
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]);
		let input = RuntimeDocumentInput::new(
			RequestInput::new(Method::GET, "/docs"),
			RuntimeManifest::new(
				"build-id",
				"/static/",
				"",
				BTreeMap::new(),
				vec!["/static/app.hash.css".to_owned()],
				public_filemap.clone(),
				Vec::new(),
			),
			AssetCapabilities::new(
				"/static/",
				vec!["/static/app.hash.css".to_owned()],
				public_filemap,
			)
			.unwrap(),
		);

		let contract = builder.document_for_request(input).await.unwrap();

		assert_eq!(contract.body_attributes()[0].value(), "/docs");
		assert_eq!(
			contract.head_defaults()[0].attributes()["href"],
			"/static/app.hash.css"
		);
	}

	#[tokio::test]
	async fn document_builder_build_hash_source_uses_logical_public_urls() {
		let builder = DocumentBuilder::new(|context| async move {
			let mut document = Document::new();
			let app_css = context.public_url(" app.css ")?;
			let head = document.head();
			head.link([head.rel("stylesheet"), head.href(app_css)]);
			Ok(document)
		});

		let source = builder.build_root_document_hash_source().await.unwrap();
		let value: Value = serde_json::from_str(&source).unwrap();

		assert_eq!(value["format"], DOCUMENT_HASH_SOURCE_FORMAT);
		assert_eq!(value["head_defaults"][0]["tag"], "link");
		assert_eq!(value["head_defaults"][0]["attributes"]["href"], "/app.css");
	}

	#[tokio::test]
	async fn document_builder_build_hash_source_uses_synthetic_root_request() {
		let builder = DocumentBuilder::new(|context| async move {
			let mut document = Document::new();
			document
				.head()
				.meta_property_content("og:url", context.request().path());
			Ok(document)
		});

		let source = builder.build_root_document_hash_source().await.unwrap();
		let value: Value = serde_json::from_str(&source).unwrap();

		assert_eq!(value["head_defaults"][0]["tag"], "meta");
		assert_eq!(
			value["head_defaults"][0]["attributes"]["property"],
			"og:url"
		);
		assert_eq!(value["head_defaults"][0]["attributes"]["content"], "/");
	}
}
