use std::collections::BTreeMap;

use serde::Serialize;
use vorma::__private::document::{
	DocumentBuildIdentity, DocumentBuildIdentityAttribute, DocumentBuildIdentityElement,
};
use vorma::Document;

const FORMAT: &str = "vorma-document-hash-source-v1";

pub(crate) fn hash_source(document: &Document) -> Result<String, String> {
	let input = document.__build_identity()?;
	serde_json::to_string(&DocumentHashSource::from(input)).map_err(|err| err.to_string())
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

impl<'a> From<DocumentBuildIdentity<'a>> for DocumentHashSource<'a> {
	fn from(input: DocumentBuildIdentity<'a>) -> Self {
		Self {
			format: FORMAT,
			html_attributes: input
				.html_attributes
				.into_iter()
				.map(DocumentHashAttribute::from)
				.collect(),
			body_attributes: input
				.body_attributes
				.into_iter()
				.map(DocumentHashAttribute::from)
				.collect(),
			head_defaults: input
				.head_defaults
				.into_iter()
				.map(DocumentHashElement::from)
				.collect(),
			head_dedupe_rules: input
				.head_dedupe_rules
				.into_iter()
				.map(DocumentHashElement::from)
				.collect(),
			body_prefix: input
				.body_prefix
				.into_iter()
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

impl<'a> From<DocumentBuildIdentityAttribute<'a>> for DocumentHashAttribute<'a> {
	fn from(attr: DocumentBuildIdentityAttribute<'a>) -> Self {
		Self {
			name: attr.name,
			value: attr.value,
			known_safe: attr.known_safe,
			boolean: attr.boolean,
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

impl<'a> From<DocumentBuildIdentityElement<'a>> for DocumentHashElement<'a> {
	fn from(el: DocumentBuildIdentityElement<'a>) -> Self {
		Self {
			tag: el.tag,
			attributes: el.attributes,
			attributes_known_safe: el.attributes_known_safe,
			boolean_attributes: el.boolean_attributes,
			text_content: el.text_content,
			dangerous_inner_html: el.dangerous_inner_html,
			self_closing: el.self_closing,
		}
	}
}

#[cfg(test)]
mod tests {
	use serde_json::Value;
	use vorma::Document;

	use super::*;

	#[test]
	fn hash_source_includes_default_head() {
		let mut first = Document::new();
		first.head().title("One");
		let mut second = Document::new();
		second.head().title("Two");

		assert_ne!(hash_source(&first).unwrap(), hash_source(&second).unwrap());
	}

	#[test]
	fn hash_source_includes_head_dedupe_rules() {
		let mut first = Document::new();
		{
			let rules = first.head_dedupe_rules();
			let name = rules.name("keywords");
			rules.meta([name.into()]);
		}
		let mut second = Document::new();
		{
			let rules = second.head_dedupe_rules();
			let name = rules.name("author");
			rules.meta([name.into()]);
		}

		assert_ne!(hash_source(&first).unwrap(), hash_source(&second).unwrap());
	}

	#[test]
	fn hash_source_includes_document_attrs_and_element_modes() {
		let mut first = Document::new();
		first.html().lang("en");
		first.body().class("one");
		{
			let head = first.head();
			head.script([head.src("/one.js").into()]);
		}

		let mut second = Document::new();
		second.html().lang("fr");
		second.body().class("two");
		{
			let head = second.head();
			head.script([head.src("/one.js").into(), head.self_closing().into()]);
		}

		assert_ne!(hash_source(&first).unwrap(), hash_source(&second).unwrap());
	}

	#[test]
	fn hash_source_serializes_canonical_document_state() {
		let mut document = Document::new();
		document.head().title("Example");

		let value: Value = serde_json::from_str(&hash_source(&document).unwrap()).unwrap();

		assert_eq!(value["format"], FORMAT);
		assert_eq!(value["head_defaults"][0]["tag"], "title");
		assert_eq!(value["head_defaults"][0]["text_content"], "Example");
	}
}
