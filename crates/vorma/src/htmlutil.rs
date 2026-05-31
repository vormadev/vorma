use std::collections::BTreeMap;
use std::fmt;

use base64::Engine;
use base64::engine::general_purpose::STANDARD as BASE64_STANDARD;
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};

#[derive(Clone, Debug, Default, Deserialize, Eq, Ord, PartialEq, PartialOrd, Serialize)]
pub(crate) struct Element {
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub(crate) tag: String,
	#[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
	pub(crate) attributes: BTreeMap<String, String>,
	#[serde(default, skip_serializing_if = "BTreeMap::is_empty")]
	pub(crate) attributes_known_safe: BTreeMap<String, String>,
	#[serde(default, skip_serializing_if = "Vec::is_empty")]
	pub(crate) boolean_attributes: Vec<String>,
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub(crate) text_content: String,
	#[serde(default, skip_serializing_if = "String::is_empty")]
	pub(crate) dangerous_inner_html: String,
	#[serde(skip)]
	pub(crate) self_closing: bool,
}

const SELF_CLOSING_TAGS: &[&str] = &[
	"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta", "source", "track",
	"wbr",
];

pub(crate) fn compute_content_sha256(el: &Element) -> String {
	let hash = Sha256::digest(combine_inner_html(el).as_bytes());
	BASE64_STANDARD.encode(hash)
}

pub(crate) fn render_element(el: &Element) -> Result<String, String> {
	let mut out = String::new();
	render_element_to_string(el, &mut out)?;
	Ok(out)
}

pub(crate) fn render_element_to_string(el: &Element, out: &mut String) -> Result<(), String> {
	validate_tag_name(&el.tag)?;

	let is_self_closing = SELF_CLOSING_TAGS.contains(&el.tag.as_str()) || el.self_closing;
	let escaped_attrs = combine_attributes(el);

	out.push('<');
	out.push_str(&el.tag);

	for (key, value) in escaped_attrs {
		write_attribute(out, &key, &value)?;
	}

	for bool_attr in &el.boolean_attributes {
		validate_attribute_name(bool_attr)?;
		out.push(' ');
		out.push_str(bool_attr);
	}

	if is_self_closing {
		out.push_str(" />");
		return Ok(());
	}

	out.push('>');
	out.push_str(&combine_inner_html(el));
	out.push_str("</");
	out.push_str(&el.tag);
	out.push('>');
	Ok(())
}

pub(crate) fn render_module_script_to_string(src: &str, out: &mut String) -> Result<(), String> {
	render_element_to_string(
		&Element {
			tag: "script".to_owned(),
			attributes: BTreeMap::from([
				("type".to_owned(), "module".to_owned()),
				("src".to_owned(), src.to_owned()),
			]),
			..Element::default()
		},
		out,
	)
}

pub(crate) fn escape_into_trusted(el: &Element) -> Element {
	Element {
		tag: el.tag.clone(),
		attributes: BTreeMap::new(),
		attributes_known_safe: combine_attributes(el),
		boolean_attributes: el.boolean_attributes.clone(),
		text_content: String::new(),
		dangerous_inner_html: combine_inner_html(el),
		self_closing: el.self_closing,
	}
}

fn write_attribute(out: &mut String, key: &str, value: &str) -> Result<(), String> {
	validate_attribute_name(key)?;
	out.push(' ');
	out.push_str(key);
	out.push_str("=\"");
	out.push_str(value);
	out.push('"');
	Ok(())
}

pub(crate) fn combine_attributes(el: &Element) -> BTreeMap<String, String> {
	let mut out = BTreeMap::new();
	for (key, value) in &el.attributes {
		out.insert(key.clone(), escape_html_attr(value));
	}
	for (key, value) in &el.attributes_known_safe {
		out.insert(key.clone(), value.clone());
	}
	out
}

pub(crate) fn validate_tag_name(name: &str) -> Result<(), String> {
	if name.is_empty() {
		return Err("element has no tag".to_owned());
	}
	if is_html_name(name) {
		return Ok(());
	}
	Err(format!("tag {name:?} is not a valid HTML tag name"))
}

pub(crate) fn validate_attribute_name(name: &str) -> Result<(), String> {
	if name.is_empty() {
		return Err("attribute has no key".to_owned());
	}
	if is_html_name(name) {
		return Ok(());
	}
	Err(format!(
		"attribute {name:?} is not a valid HTML attribute name"
	))
}

fn is_html_name(name: &str) -> bool {
	name.chars()
		.all(|ch| ch.is_ascii_alphanumeric() || matches!(ch, '-' | '_' | ':' | '.'))
}

fn combine_inner_html(el: &Element) -> String {
	let raw = if !el.dangerous_inner_html.is_empty() {
		el.dangerous_inner_html.clone()
	} else if !el.text_content.is_empty() {
		escape_html(&el.text_content)
	} else {
		String::new()
	};
	if el.tag.eq_ignore_ascii_case("style") {
		return escape_style_raw_text(&raw);
	}
	raw
}

fn escape_style_raw_text(value: &str) -> String {
	let mut out = String::with_capacity(value.len());
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
	out
}

fn escape_html(value: &str) -> String {
	html_escape::encode_text(value).into_owned()
}

fn escape_html_attr(value: &str) -> String {
	html_escape::encode_double_quoted_attribute(value).into_owned()
}

impl fmt::Display for Element {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		let rendered = render_element(self).map_err(|_| fmt::Error)?;
		f.write_str(&rendered)
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn render_element_escapes_attributes_and_text_content() {
		let html = render_element(&Element {
			tag: "div".to_owned(),
			attributes: BTreeMap::from([(
				"data-info".to_owned(),
				"This is a \"quote\" and a <tag>".to_owned(),
			)]),
			text_content: "Content with <b>bold</b>".to_owned(),
			..Element::default()
		})
		.unwrap();

		assert!(html.contains("data-info=\"This is a &quot;quote&quot; and a &lt;tag&gt;\""));
		assert!(html.contains("Content with &lt;b&gt;bold&lt;/b&gt;"));
	}

	#[test]
	fn render_element_preserves_known_safe_attribute_values_and_dangerous_inner_html() {
		let html = render_element(&Element {
			tag: "div".to_owned(),
			attributes: BTreeMap::from([("class".to_owned(), "unsafe".to_owned())]),
			attributes_known_safe: BTreeMap::from([("class".to_owned(), "safe".to_owned())]),
			dangerous_inner_html: "Content with <b>bold</b>".to_owned(),
			..Element::default()
		})
		.unwrap();

		assert_eq!(html, "<div class=\"safe\">Content with <b>bold</b></div>");
	}

	#[test]
	fn render_element_escapes_style_raw_text_end_tag() {
		let html = render_element(&Element {
			tag: "style".to_owned(),
			dangerous_inner_html: "body::before{content:\"</style><script>x</script>\"}".to_owned(),
			..Element::default()
		})
		.unwrap();

		assert!(!html.contains("</style><script>"));
		assert!(html.contains("\\3C /style><script>x</script>"));
	}

	#[test]
	fn render_element_handles_self_closing_tags_and_boolean_attributes() {
		let html = render_element(&Element {
			tag: "input".to_owned(),
			attributes: BTreeMap::from([("type".to_owned(), "text".to_owned())]),
			boolean_attributes: vec!["checked".to_owned()],
			..Element::default()
		})
		.unwrap();

		assert_eq!(html, "<input type=\"text\" checked />");
	}

	#[test]
	fn render_element_rejects_invalid_tag_names() {
		let err = render_element(&Element {
			tag: "script src=x".to_owned(),
			..Element::default()
		})
		.unwrap_err();

		assert!(err.contains("valid HTML tag name"));
	}

	#[test]
	fn render_element_rejects_invalid_attribute_names() {
		let err = render_element(&Element {
			tag: "div".to_owned(),
			attributes: BTreeMap::from([("data-x onclick".to_owned(), "bad".to_owned())]),
			..Element::default()
		})
		.unwrap_err();

		assert!(err.contains("valid HTML attribute name"));
	}

	#[test]
	fn render_element_rejects_invalid_known_safe_attribute_names() {
		let err = render_element(&Element {
			tag: "div".to_owned(),
			attributes_known_safe: BTreeMap::from([("href onerror".to_owned(), "/".to_owned())]),
			..Element::default()
		})
		.unwrap_err();

		assert!(err.contains("valid HTML attribute name"));
	}

	#[test]
	fn render_element_rejects_invalid_boolean_attribute_names() {
		let err = render_element(&Element {
			tag: "input".to_owned(),
			boolean_attributes: vec!["checked autofocus".to_owned()],
			..Element::default()
		})
		.unwrap_err();

		assert!(err.contains("valid HTML attribute name"));
	}

	#[test]
	fn render_module_script_escapes_src_attribute() {
		let mut out = String::new();
		render_module_script_to_string(r#"/entry.js" defer="false"#, &mut out).unwrap();

		assert_eq!(
			out,
			r#"<script src="/entry.js&quot; defer=&quot;false" type="module"></script>"#
		);
	}

	#[test]
	fn compute_content_sha256_hashes_dangerous_inner_html_without_mutation() {
		let el = Element {
			dangerous_inner_html: "<b>x</b>".to_owned(),
			..Element::default()
		};

		let hash = compute_content_sha256(&el);

		assert_eq!(hash, "4x46ju2qZVk3v+1+Zr5q8exbMbOFCsZp3t2mw95FPHk=");
		assert!(el.attributes_known_safe.is_empty());
	}

	#[test]
	fn escape_into_trusted_does_not_alias_boolean_attributes() {
		let mut el = Element {
			tag: "script".to_owned(),
			boolean_attributes: vec!["async".to_owned()],
			..Element::default()
		};

		let trusted = escape_into_trusted(&el);
		el.boolean_attributes[0] = "defer".to_owned();

		assert_eq!(trusted.boolean_attributes[0], "async");
	}
}
