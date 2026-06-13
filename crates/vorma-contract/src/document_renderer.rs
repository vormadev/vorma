//! Document contract rendering for server HTML responses.

use std::collections::{BTreeMap, BTreeSet};

use crate::contracts::{DocumentAttributeContract, DocumentContract, DocumentElementContract};

const DOCTYPE_AND_HTML_OPEN: &str = "<!doctype html>\n<html";
const HEAD_OPEN: &str = ">\n<head>\n";
const HEAD_CLOSE_AND_BODY_OPEN: &str = "</head>\n<body";
const BODY_CLOSE: &str = "</body>\n</html>\n";
const VOID_TAGS: &[&str] = &[
	"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track",
	"wbr",
];

/// Trusted internal markup inserted into a rendered document shell.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct DocumentRenderInput<'a> {
	head_markup: &'a str,
	body_markup: &'a str,
}

impl<'a> DocumentRenderInput<'a> {
	/// Create document render input from trusted internal head and body markup.
	pub fn new(head_markup: &'a str, body_markup: &'a str) -> Self {
		Self {
			head_markup,
			body_markup,
		}
	}

	/// Trusted head markup inserted after default document head elements.
	pub fn head_markup(&self) -> &'a str {
		self.head_markup
	}

	/// Trusted body markup inserted after document body-prefix elements.
	pub fn body_markup(&self) -> &'a str {
		self.body_markup
	}
}

/// Render a document contract into the fixed HTML shell.
pub fn render_document(
	document: &DocumentContract,
	input: DocumentRenderInput<'_>,
) -> Result<String, DocumentRenderError> {
	let mut out = String::new();
	out.push_str(DOCTYPE_AND_HTML_OPEN);
	render_document_attributes(document.html_attributes(), &mut out)?;
	out.push_str(HEAD_OPEN);
	for element in document.head_defaults() {
		render_element(element, &mut out)?;
		out.push('\n');
	}
	out.push_str(input.head_markup());
	out.push_str(HEAD_CLOSE_AND_BODY_OPEN);
	render_document_attributes(document.body_attributes(), &mut out)?;
	out.push_str(">\n");
	for element in document.body_prefix() {
		render_element(element, &mut out)?;
		out.push('\n');
	}
	out.push_str(input.body_markup());
	out.push_str(BODY_CLOSE);
	Ok(out)
}

/// Render one document element through the same validated HTML path as a full document.
pub fn render_document_element(
	element: &DocumentElementContract,
) -> Result<String, DocumentRenderError> {
	let mut out = String::new();
	render_element(element, &mut out)?;
	Ok(out)
}

/// Validate and escape one document element into trusted render parts.
pub fn trusted_element_parts(
	element: &DocumentElementContract,
) -> Result<TrustedElementParts, DocumentRenderError> {
	validate_tag_name(element.tag())?;
	let mut seen_names = BTreeSet::new();
	let mut attributes_known_safe = BTreeMap::new();
	for (name, value) in element.attributes() {
		validate_attribute_name(name)?;
		if !seen_names.insert(name.as_str()) {
			return Err(DocumentRenderError::DuplicateAttribute {
				name: name.to_owned(),
			});
		}
		let mut escaped = String::new();
		escape_attribute_value(value, &mut escaped);
		attributes_known_safe.insert(name.clone(), escaped);
	}
	for (name, value) in element.attributes_known_safe() {
		validate_attribute_name(name)?;
		if !seen_names.insert(name.as_str()) {
			return Err(DocumentRenderError::DuplicateAttribute {
				name: name.to_owned(),
			});
		}
		attributes_known_safe.insert(name.clone(), value.clone());
	}
	let mut boolean_attributes = Vec::with_capacity(element.boolean_attributes().len());
	for name in element.boolean_attributes() {
		validate_attribute_name(name)?;
		if !seen_names.insert(name.as_str()) {
			return Err(DocumentRenderError::DuplicateAttribute {
				name: name.to_owned(),
			});
		}
		boolean_attributes.push(name.clone());
	}
	let mut dangerous_inner_html = String::new();
	if element.tag().eq_ignore_ascii_case("style") {
		let raw = if element.dangerous_inner_html().is_empty() {
			element.text_content()
		} else {
			element.dangerous_inner_html()
		};
		escape_style_raw_text(raw, &mut dangerous_inner_html);
	} else {
		escape_text(element.text_content(), &mut dangerous_inner_html);
		dangerous_inner_html.push_str(element.dangerous_inner_html());
	}
	Ok(TrustedElementParts {
		tag: element.tag().to_owned(),
		attributes_known_safe,
		boolean_attributes,
		dangerous_inner_html,
		self_closing: element.self_closing(),
	})
}

/// Validated, escaped parts of one trusted document element.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TrustedElementParts {
	/// Validated tag name.
	pub tag: String,
	/// Escaped attribute name/value pairs.
	pub attributes_known_safe: BTreeMap<String, String>,
	/// Validated boolean attribute names.
	pub boolean_attributes: Vec<String>,
	/// Trusted inner HTML produced by framework-owned rendering.
	pub dangerous_inner_html: String,
	/// Whether the element renders self-closing.
	pub self_closing: bool,
}

/// Document rendering error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DocumentRenderError {
	/// Element tag name was invalid.
	InvalidTagName {
		/// Rejected tag name.
		name: String,
	},
	/// Attribute name was invalid.
	InvalidAttributeName {
		/// Rejected attribute name.
		name: String,
	},
	/// One element or document attribute set declared the same attribute twice.
	DuplicateAttribute {
		/// Repeated attribute name.
		name: String,
	},
}

impl std::fmt::Display for DocumentRenderError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidTagName { name } => write!(f, "invalid HTML tag name {name:?}"),
			Self::InvalidAttributeName { name } => {
				write!(f, "invalid HTML attribute name {name:?}")
			}
			Self::DuplicateAttribute { name } => {
				write!(f, "duplicate HTML attribute name {name:?}")
			}
		}
	}
}

impl std::error::Error for DocumentRenderError {}

fn render_document_attributes(
	attributes: &[DocumentAttributeContract],
	out: &mut String,
) -> Result<(), DocumentRenderError> {
	let mut seen_names = BTreeSet::new();
	for attribute in attributes {
		validate_attribute_name(attribute.name())?;
		if !seen_names.insert(attribute.name()) {
			return Err(DocumentRenderError::DuplicateAttribute {
				name: attribute.name().to_owned(),
			});
		}
		out.push(' ');
		out.push_str(attribute.name());
		if attribute.boolean() {
			continue;
		}
		out.push_str("=\"");
		if attribute.known_safe() {
			out.push_str(attribute.value());
		} else {
			escape_attribute_value(attribute.value(), out);
		}
		out.push('"');
	}
	Ok(())
}

fn render_element(
	element: &DocumentElementContract,
	out: &mut String,
) -> Result<(), DocumentRenderError> {
	let parts = trusted_element_parts(element)?;
	out.push('<');
	out.push_str(&parts.tag);
	for (name, value) in &parts.attributes_known_safe {
		out.push(' ');
		out.push_str(name);
		out.push_str("=\"");
		out.push_str(value);
		out.push('"');
	}
	for name in &parts.boolean_attributes {
		out.push(' ');
		out.push_str(name);
	}
	if parts.self_closing || VOID_TAGS.contains(&parts.tag.as_str()) {
		out.push_str(" />");
		return Ok(());
	}
	out.push('>');
	out.push_str(&parts.dangerous_inner_html);
	out.push_str("</");
	out.push_str(&parts.tag);
	out.push('>');
	Ok(())
}

fn validate_tag_name(name: &str) -> Result<(), DocumentRenderError> {
	if name.is_empty() || !name.bytes().all(valid_html_name_byte) {
		return Err(DocumentRenderError::InvalidTagName {
			name: name.to_owned(),
		});
	}
	Ok(())
}

fn validate_attribute_name(name: &str) -> Result<(), DocumentRenderError> {
	if name.is_empty() || !name.bytes().all(valid_html_name_byte) {
		return Err(DocumentRenderError::InvalidAttributeName {
			name: name.to_owned(),
		});
	}
	Ok(())
}

fn valid_html_name_byte(byte: u8) -> bool {
	byte.is_ascii_alphanumeric() || matches!(byte, b':' | b'-' | b'_' | b'.')
}

fn escape_attribute_value(value: &str, out: &mut String) {
	for ch in value.chars() {
		match ch {
			'&' => out.push_str("&amp;"),
			'"' => out.push_str("&quot;"),
			'<' => out.push_str("&lt;"),
			'>' => out.push_str("&gt;"),
			_ => out.push(ch),
		}
	}
}

fn escape_text(value: &str, out: &mut String) {
	for ch in value.chars() {
		match ch {
			'&' => out.push_str("&amp;"),
			'<' => out.push_str("&lt;"),
			'>' => out.push_str("&gt;"),
			_ => out.push(ch),
		}
	}
}

fn escape_style_raw_text(value: &str, out: &mut String) {
	let mut last = 0;
	for (idx, ch) in value.char_indices() {
		if ch != '<' {
			continue;
		}
		let Some(candidate) = value.get(idx..idx + "</style".len()) else {
			continue;
		};
		if !candidate.eq_ignore_ascii_case("</style") {
			continue;
		}
		out.push_str(&value[last..idx]);
		out.push_str("\\3C ");
		last = idx + ch.len_utf8();
	}
	out.push_str(&value[last..]);
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use super::*;

	#[test]
	fn document_renderer_escapes_untrusted_attributes_and_text() {
		let document = DocumentContract::new(
			vec![DocumentAttributeContract::new(
				"data-name",
				"Tom & \"Ada\"",
				false,
				false,
			)],
			Vec::new(),
			vec![DocumentElementContract::new("title").with_text_content("<Hello & Goodbye>")],
			Vec::new(),
			Vec::new(),
		);

		let html = render_document(
			&document,
			DocumentRenderInput::new("<meta name=\"vorma\" />\n", "<div id=\"app\"></div>\n"),
		)
		.unwrap();

		assert!(html.contains("<html data-name=\"Tom &amp; &quot;Ada&quot;\">"));
		assert!(html.contains("<title>&lt;Hello &amp; Goodbye&gt;</title>"));
		assert!(html.contains("<meta name=\"vorma\" />\n</head>"));
		assert!(html.contains("<div id=\"app\"></div>\n</body>"));
	}

	#[test]
	fn document_renderer_preserves_known_safe_and_boolean_attributes() {
		let document = DocumentContract::new(
			Vec::new(),
			vec![DocumentAttributeContract::new("hidden", "", false, true)],
			Vec::new(),
			Vec::new(),
			vec![
				DocumentElementContract::new("script")
					.with_known_safe_attributes(BTreeMap::from([(
						"nonce".to_owned(),
						"safe&raw".to_owned(),
					)]))
					.with_dangerous_inner_html("window.__vorma = true;"),
			],
		);

		let html = render_document(&document, DocumentRenderInput::new("", "")).unwrap();

		assert!(html.contains("<body hidden>"));
		assert!(html.contains("<script nonce=\"safe&raw\">window.__vorma = true;</script>"));
	}

	#[test]
	fn document_renderer_escapes_style_raw_text_end_tag() {
		let document = DocumentContract::new(
			Vec::new(),
			Vec::new(),
			vec![
				DocumentElementContract::new("style").with_dangerous_inner_html(
					"body::before{content:\"</StYlE><script>x</script>\"}",
				),
			],
			Vec::new(),
			Vec::new(),
		);

		let html = render_document(&document, DocumentRenderInput::new("", "")).unwrap();

		assert!(!html.contains("</StYlE><script>"));
		assert!(html.contains("\\3C /StYlE><script>x</script>"));
	}

	#[test]
	fn document_renderer_self_closes_void_elements() {
		let document = DocumentContract::new(
			Vec::new(),
			Vec::new(),
			vec![
				DocumentElementContract::new("meta")
					.with_attributes(BTreeMap::from([("charset".to_owned(), "utf-8".to_owned())])),
			],
			Vec::new(),
			Vec::new(),
		);

		let html = render_document(&document, DocumentRenderInput::new("", "")).unwrap();

		assert!(html.contains("<meta charset=\"utf-8\" />"));
		assert!(!html.contains("</meta>"));
	}

	#[test]
	fn document_renderer_rejects_invalid_names() {
		let document = DocumentContract::new(
			Vec::new(),
			Vec::new(),
			vec![DocumentElementContract::new("bad tag")],
			Vec::new(),
			Vec::new(),
		);

		let error = render_document(&document, DocumentRenderInput::new("", "")).unwrap_err();

		assert!(matches!(error, DocumentRenderError::InvalidTagName { .. }));
	}

	#[test]
	fn document_renderer_rejects_duplicate_attributes() {
		let document = DocumentContract::new(
			Vec::new(),
			Vec::new(),
			vec![
				DocumentElementContract::new("meta")
					.with_attributes(BTreeMap::from([("name".to_owned(), "a".to_owned())]))
					.with_known_safe_attributes(BTreeMap::from([(
						"name".to_owned(),
						"b".to_owned(),
					)]))
					.with_self_closing(true),
			],
			Vec::new(),
			Vec::new(),
		);

		let error = render_document(&document, DocumentRenderInput::new("", "")).unwrap_err();

		assert!(matches!(
			error,
			DocumentRenderError::DuplicateAttribute { .. }
		));
	}
}
