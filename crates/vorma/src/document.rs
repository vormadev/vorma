use std::collections::BTreeMap;
use std::future::Future;
use std::pin::Pin;
use std::sync::Arc;

use crate::head;
use crate::htmlutil::{
	Element, combine_attributes, render_element_to_string, validate_attribute_name,
	validate_tag_name,
};
use crate::manifest::Manifest;
use crate::manifest::public_src_key;
use crate::mux::RawRequest;
use crate::request::HttpRequest;

/// Build-safe document shell and default head declaration.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Document {
	html_attributes: Vec<ElementAttribute>,
	body_attributes: Vec<ElementAttribute>,
	head_defaults: head::HeadBuilder,
	head_dedupe_rules: head::HeadBuilder,
	body_prefix: Vec<Element>,
}

impl Document {
	/// Create the default Vorma document shell.
	pub fn new() -> Self {
		Self {
			html_attributes: vec![ElementAttribute::new("lang", "en")],
			body_attributes: Vec::new(),
			head_defaults: head::HeadBuilder::new(),
			head_dedupe_rules: head::HeadBuilder::new(),
			body_prefix: Vec::new(),
		}
	}

	/// Mutate attributes on the root `<html>` element.
	pub fn html(&mut self) -> DocumentAttributes<'_> {
		DocumentAttributes {
			attrs: &mut self.html_attributes,
		}
	}

	/// Mutate attributes on the root `<body>` element.
	pub fn body(&mut self) -> DocumentAttributes<'_> {
		DocumentAttributes {
			attrs: &mut self.body_attributes,
		}
	}

	/// Default head entries applied to every HTML view response.
	pub fn head(&mut self) -> &mut head::HeadBuilder {
		&mut self.head_defaults
	}

	/// Additional head dedupe rules applied when preparing merged head entries.
	pub fn head_dedupe_rules(&mut self) -> &mut head::HeadBuilder {
		&mut self.head_dedupe_rules
	}

	#[cfg(test)]
	pub(crate) fn push_body_prefix(&mut self, el: Element) -> &mut Self {
		self.body_prefix.push(el);
		self
	}

	pub(crate) fn prepare_head(&self, els: &[Element]) -> head::Prepared {
		let mut head = self.head_defaults.clone();
		head.append(&head::HeadBuilder::from_elements(els.to_vec()));
		self.head_renderer().prepare(head.elements())
	}

	pub(crate) fn render_head(&self, prepared: &head::Prepared) -> Result<String, String> {
		self.head_renderer().render(prepared)
	}

	pub(crate) fn render(&self, input: DocumentRenderInput<'_>) -> Result<String, String> {
		let mut out = String::new();
		out.push_str("<!doctype html>\n<html");
		render_document_attrs(&self.html_attributes, &mut out)?;
		out.push_str(">\n<head>\n");
		out.push_str(input.vorma_head);
		out.push_str("</head>\n<body");
		render_document_attrs(&self.body_attributes, &mut out)?;
		out.push_str(">\n");
		for el in &self.body_prefix {
			render_element_to_string(el, &mut out)?;
			out.push('\n');
		}
		out.push_str(input.vorma_body);
		out.push_str("</body>\n</html>\n");
		Ok(out)
	}

	#[doc(hidden)]
	pub fn __build_identity(&self) -> Result<DocumentBuildIdentity<'_>, String> {
		Ok(DocumentBuildIdentity {
			html_attributes: build_identity_attributes(&self.html_attributes)?,
			body_attributes: build_identity_attributes(&self.body_attributes)?,
			head_defaults: build_identity_elements(self.head_defaults.elements())?,
			head_dedupe_rules: build_identity_elements(self.head_dedupe_rules.elements())?,
			body_prefix: build_identity_elements(&self.body_prefix)?,
		})
	}

	fn head_renderer(&self) -> head::Renderer {
		let renderer = head::Renderer::new("vorma");
		renderer.init_dedupe_rules(Some(&self.head_dedupe_rules));
		renderer
	}
}

impl Default for Document {
	fn default() -> Self {
		Self::new()
	}
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ElementAttribute {
	name: String,
	value: String,
	known_safe: bool,
	boolean: bool,
}

impl ElementAttribute {
	fn new(name: impl Into<String>, value: impl Into<String>) -> Self {
		Self {
			name: name.into(),
			value: value.into(),
			known_safe: false,
			boolean: false,
		}
	}

	fn known_safe(name: impl Into<String>, value: impl Into<String>) -> Self {
		Self {
			name: name.into(),
			value: value.into(),
			known_safe: true,
			boolean: false,
		}
	}

	fn boolean(name: impl Into<String>) -> Self {
		Self {
			name: name.into(),
			value: String::new(),
			known_safe: false,
			boolean: true,
		}
	}
}

/// Builder for document element attributes.
pub struct DocumentAttributes<'a> {
	attrs: &'a mut Vec<ElementAttribute>,
}

impl DocumentAttributes<'_> {
	/// Set an escaped attribute.
	pub fn attribute(&mut self, name: impl Into<String>, value: impl Into<String>) -> &mut Self {
		self.upsert(ElementAttribute::new(name, value));
		self
	}

	/// Set an attribute whose value is already trusted HTML.
	pub fn known_safe_attribute(
		&mut self,
		name: impl Into<String>,
		value: impl Into<String>,
	) -> &mut Self {
		self.upsert(ElementAttribute::known_safe(name, value));
		self
	}

	/// Set a boolean attribute.
	pub fn boolean_attribute(&mut self, name: impl Into<String>) -> &mut Self {
		self.upsert(ElementAttribute::boolean(name));
		self
	}

	/// Set the `lang` attribute.
	pub fn lang(&mut self, value: impl Into<String>) -> &mut Self {
		self.attribute("lang", value)
	}

	/// Set the `id` attribute.
	pub fn id(&mut self, value: impl Into<String>) -> &mut Self {
		self.attribute("id", value)
	}

	/// Set the `class` attribute.
	pub fn class(&mut self, value: impl Into<String>) -> &mut Self {
		self.attribute("class", value)
	}

	/// Set a `data-*` attribute.
	pub fn data(&mut self, name: impl AsRef<str>, value: impl Into<String>) -> &mut Self {
		self.attribute(format!("data-{}", name.as_ref()), value)
	}

	fn upsert(&mut self, attr: ElementAttribute) {
		self.attrs.retain(|existing| existing.name != attr.name);
		self.attrs.push(attr);
	}
}

pub(crate) struct DocumentRenderInput<'a> {
	pub(crate) vorma_head: &'a str,
	pub(crate) vorma_body: &'a str,
}

#[doc(hidden)]
pub struct DocumentBuildIdentity<'a> {
	pub html_attributes: Vec<DocumentBuildIdentityAttribute<'a>>,
	pub body_attributes: Vec<DocumentBuildIdentityAttribute<'a>>,
	pub head_defaults: Vec<DocumentBuildIdentityElement<'a>>,
	pub head_dedupe_rules: Vec<DocumentBuildIdentityElement<'a>>,
	pub body_prefix: Vec<DocumentBuildIdentityElement<'a>>,
}

#[doc(hidden)]
pub struct DocumentBuildIdentityAttribute<'a> {
	pub name: &'a str,
	pub value: &'a str,
	pub known_safe: bool,
	pub boolean: bool,
}

#[doc(hidden)]
pub struct DocumentBuildIdentityElement<'a> {
	pub tag: &'a str,
	pub attributes: &'a BTreeMap<String, String>,
	pub attributes_known_safe: &'a BTreeMap<String, String>,
	pub boolean_attributes: &'a [String],
	pub text_content: &'a str,
	pub dangerous_inner_html: &'a str,
	pub self_closing: bool,
}

/// Context passed to the document builder.
#[derive(Clone)]
pub struct DocumentBuildCtx {
	public_urls: DocumentPublicUrls,
	request: RawRequest,
}

#[derive(Clone)]
enum DocumentPublicUrls {
	BuildPassthrough,
	Manifest(Arc<Manifest>),
}

impl DocumentBuildCtx {
	#[doc(hidden)]
	pub fn build() -> Self {
		Self {
			public_urls: DocumentPublicUrls::BuildPassthrough,
			request: RawRequest::get("/"),
		}
	}

	pub(crate) fn manifest(manifest: Arc<Manifest>, request: RawRequest) -> Self {
		Self {
			public_urls: DocumentPublicUrls::Manifest(manifest),
			request,
		}
	}

	/// Request used for the current document/default-head render.
	pub fn request(&self) -> HttpRequest<'_> {
		HttpRequest::new(&self.request)
	}

	/// Resolve a public static source path.
	///
	/// Build mode returns a deterministic logical pass-through URL. Runtime mode resolves
	/// through the loaded manifest.
	pub fn public_url(&self, src_path: &str) -> Result<String, String> {
		match &self.public_urls {
			DocumentPublicUrls::BuildPassthrough => {
				let clean = public_src_key(src_path)
					.ok_or_else(|| "public URL source path is empty".to_owned())?;
				Ok(format!("/{clean}"))
			}
			DocumentPublicUrls::Manifest(manifest) => manifest
				.public_url(src_path)
				.map(ToOwned::to_owned)
				.ok_or_else(|| format!("file {src_path} not found in manifest public filemap")),
		}
	}
}

type DocumentFuture = Pin<Box<dyn Future<Output = Result<Document, String>> + Send + 'static>>;

/// Async document builder used by build/live-state and runtime HTML rendering.
#[derive(Clone)]
pub struct DocumentBuilder {
	build: Arc<dyn Fn(DocumentBuildCtx) -> DocumentFuture + Send + Sync>,
}

impl DocumentBuilder {
	/// Create a document builder from an async function.
	pub fn new<F, Fut>(build: F) -> Self
	where
		F: Fn(DocumentBuildCtx) -> Fut + Send + Sync + 'static,
		Fut: Future<Output = Result<Document, String>> + Send + 'static,
	{
		Self {
			build: Arc::new(move |ctx| Box::pin(build(ctx))),
		}
	}

	#[doc(hidden)]
	pub async fn build(&self, ctx: DocumentBuildCtx) -> Result<Document, String> {
		(self.build)(ctx).await
	}
}

impl Default for DocumentBuilder {
	fn default() -> Self {
		Self::new(|_| async { Ok(Document::new()) })
	}
}

fn render_document_attrs(attrs: &[ElementAttribute], out: &mut String) -> Result<(), String> {
	for attr in attrs {
		validate_attribute_name(&attr.name)?;
		out.push(' ');
		out.push_str(&attr.name);
		if attr.boolean {
			continue;
		}
		out.push_str("=\"");
		if attr.known_safe {
			out.push_str(&attr.value);
		} else {
			let el = Element {
				attributes: BTreeMap::from([(attr.name.clone(), attr.value.clone())]),
				..Element::default()
			};
			let combined = combine_attributes(&el);
			if let Some(value) = combined.get(&attr.name) {
				out.push_str(value);
			}
		}
		out.push('"');
	}
	Ok(())
}

fn build_identity_attributes(
	attrs: &[ElementAttribute],
) -> Result<Vec<DocumentBuildIdentityAttribute<'_>>, String> {
	attrs
		.iter()
		.map(|attr| {
			validate_attribute_name(&attr.name)?;
			Ok(DocumentBuildIdentityAttribute {
				name: &attr.name,
				value: &attr.value,
				known_safe: attr.known_safe,
				boolean: attr.boolean,
			})
		})
		.collect()
}

fn build_identity_elements(
	els: &[Element],
) -> Result<Vec<DocumentBuildIdentityElement<'_>>, String> {
	for el in els {
		validate_tag_name(&el.tag)?;
		for name in el.attributes.keys() {
			validate_attribute_name(name)?;
		}
		for name in el.attributes_known_safe.keys() {
			validate_attribute_name(name)?;
		}
		for name in &el.boolean_attributes {
			validate_attribute_name(name)?;
		}
	}
	Ok(els
		.iter()
		.map(|el| DocumentBuildIdentityElement {
			tag: &el.tag,
			attributes: &el.attributes,
			attributes_known_safe: &el.attributes_known_safe,
			boolean_attributes: &el.boolean_attributes,
			text_content: &el.text_content,
			dangerous_inner_html: &el.dangerous_inner_html,
			self_closing: el.self_closing,
		})
		.collect())
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn default_document_renders_framework_shell() {
		let html = Document::new()
			.render(DocumentRenderInput {
				vorma_head: "HEAD",
				vorma_body: "BODY",
			})
			.unwrap();

		assert_eq!(
			html,
			"<!doctype html>\n<html lang=\"en\">\n<head>\nHEAD</head>\n<body>\nBODY</body>\n</html>\n"
		);
	}

	#[test]
	fn document_escapes_attributes_and_body_prefix() {
		let mut document = Document::new();
		document.html().data("x", "\"<&");
		document.body().class("main");
		document.push_body_prefix(Element {
			tag: "div".to_owned(),
			attributes: BTreeMap::from([("data-prefix".to_owned(), "<tag>".to_owned())]),
			text_content: "Hello <world>".to_owned(),
			..Element::default()
		});

		let html = document
			.render(DocumentRenderInput {
				vorma_head: "",
				vorma_body: "BODY",
			})
			.unwrap();

		assert!(html.contains("data-x=\"&quot;&lt;&amp;\""));
		assert!(html.contains("<body class=\"main\">"));
		assert!(html.contains("data-prefix=\"&lt;tag&gt;\""));
		assert!(html.contains("Hello &lt;world&gt;"));
	}

	#[test]
	fn document_attributes_replace_existing_keys() {
		let mut document = Document::new();
		document.html().lang("fr");

		let html = document
			.render(DocumentRenderInput {
				vorma_head: "",
				vorma_body: "",
			})
			.unwrap();

		assert!(html.contains(r#"<html lang="fr">"#));
		assert!(!html.contains(r#"lang="en" lang="fr""#));
	}

	#[test]
	fn document_build_identity_exposes_build_owned_parts() {
		let mut document = Document::new();
		document.html().lang("fr");
		document.body().class("app");
		document.head().title("Example");
		{
			let rules = document.head_dedupe_rules();
			let name = rules.name("keywords");
			rules.meta([name.into()]);
		}
		document.push_body_prefix(Element {
			tag: "script".to_owned(),
			attributes: BTreeMap::from([("src".to_owned(), "/one.js".to_owned())]),
			self_closing: false,
			..Element::default()
		});

		let identity = document.__build_identity().unwrap();

		assert_eq!(identity.html_attributes[0].name, "lang");
		assert_eq!(identity.html_attributes[0].value, "fr");
		assert_eq!(identity.body_attributes[0].name, "class");
		assert_eq!(identity.body_attributes[0].value, "app");
		assert_eq!(identity.head_defaults[0].tag, "title");
		assert_eq!(identity.head_defaults[0].text_content, "Example");
		assert_eq!(identity.head_dedupe_rules[0].tag, "meta");
		assert_eq!(identity.body_prefix[0].tag, "script");
		assert!(!identity.body_prefix[0].self_closing);
	}

	#[test]
	fn document_build_ctx_resolves_build_public_urls_as_logical_passthroughs() {
		let ctx = DocumentBuildCtx::build();

		assert_eq!(ctx.public_url(" /favicon.ico ").unwrap(), "/favicon.ico");
		assert!(ctx.public_url(" ").is_err());
		assert_eq!(ctx.request().method(), http::Method::GET);
		assert_eq!(ctx.request().path(), "/");
	}
}
