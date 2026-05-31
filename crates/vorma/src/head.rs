use std::collections::{BTreeMap, BTreeSet};
use std::sync::OnceLock;

use crate::htmlutil::{Element, escape_into_trusted, render_element_to_string};

pub(crate) const ATTR_ANY_VALUE: &str = "\x00__vorma_headels_any__";

#[derive(Debug)]
pub(crate) struct Renderer {
	meta_start: String,
	meta_end: String,
	rest_start: String,
	rest_end: String,
	unique_rules_by_tag: OnceLock<BTreeMap<String, Vec<RuleAttrs>>>,
}

impl Clone for Renderer {
	fn clone(&self) -> Self {
		let cloned = Self {
			meta_start: self.meta_start.clone(),
			meta_end: self.meta_end.clone(),
			rest_start: self.rest_start.clone(),
			rest_end: self.rest_end.clone(),
			unique_rules_by_tag: OnceLock::new(),
		};
		if let Some(rules) = self.unique_rules_by_tag.get() {
			let _ = cloned.unique_rules_by_tag.set(rules.clone());
		}
		cloned
	}
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub(crate) struct Prepared {
	pub(crate) title: Option<Element>,
	pub(crate) meta: Vec<Element>,
	pub(crate) rest: Vec<Element>,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
struct RuleAttrs {
	attrs: BTreeMap<String, String>,
	trusted: BTreeMap<String, String>,
	boolean: Vec<String>,
}

#[derive(Clone, Debug, Eq, Ord, PartialEq, PartialOrd)]
struct DedupeKey {
	tag: String,
	rule_idx: usize,
}

impl Renderer {
	pub(crate) fn new(namespace: &str) -> Self {
		Self {
			meta_start: comment(namespace, "meta-start"),
			meta_end: comment(namespace, "meta-end"),
			rest_start: comment(namespace, "rest-start"),
			rest_end: comment(namespace, "rest-end"),
			unique_rules_by_tag: OnceLock::new(),
		}
	}

	pub(crate) fn init_dedupe_rules(&self, b: Option<&HeadBuilder>) {
		self.unique_rules_by_tag.get_or_init(|| {
			let mut defaults = HeadBuilder::new();
			defaults
				.add([HeadTag("title").into()])
				.expect("default head dedupe rule includes tag");
			defaults.meta([defaults.name("description").into()]);
			defaults.meta([defaults.name("viewport").into()]);
			defaults.meta([defaults.name("robots").into()]);
			defaults.meta([defaults.attr_exists("charset").into()]);
			defaults.link([defaults.rel("icon").into()]);
			defaults.link([defaults.rel("canonical").into()]);
			defaults.meta([defaults.property("og:title").into()]);
			defaults.meta([defaults.property("og:description").into()]);
			defaults.meta([defaults.property("og:url").into()]);
			defaults.meta([defaults.property("og:type").into()]);
			defaults.meta([defaults.property("og:locale").into()]);
			defaults.meta([defaults.property("og:site_name").into()]);
			defaults.meta([defaults.property("og:determiner").into()]);

			let mut sources = defaults.elements().to_vec();
			if let Some(b) = b {
				sources.extend_from_slice(b.elements());
			}

			let mut seen_rules = BTreeMap::<String, BTreeSet<Element>>::new();
			let mut rules_by_tag = BTreeMap::<String, Vec<RuleAttrs>>::new();
			for rule in sources {
				let seen = seen_rules.entry(rule.tag.clone()).or_default();
				if seen.insert(rule.clone()) {
					rules_by_tag
						.entry(rule.tag.clone())
						.or_default()
						.push(extract_rule_attrs(&rule));
				}
			}
			rules_by_tag
		});
	}

	pub(crate) fn render(&self, input: &Prepared) -> Result<String, String> {
		self.init_dedupe_rules(None);

		let mut out = String::new();
		let estimated = self.meta_start.len()
			+ self.meta_end.len()
			+ self.rest_start.len()
			+ self.rest_end.len()
			+ 4 + usize::from(input.title.is_some()) * 80
			+ input.meta.len() * 80
			+ input.rest.len() * 80;
		out.reserve(estimated);

		if let Some(title) = &input.title {
			render_element_to_string(title, &mut out)
				.map_err(|err| format!("error rendering title: {err}"))?;
			out.push('\n');
		}

		out.push_str(&self.meta_start);
		out.push('\n');
		for el in &input.meta {
			render_element_to_string(el, &mut out)
				.map_err(|err| format!("error rendering meta head el: {err}"))?;
			out.push('\n');
		}
		out.push_str(&self.meta_end);
		out.push('\n');

		out.push_str(&self.rest_start);
		out.push('\n');
		for el in &input.rest {
			render_element_to_string(el, &mut out)
				.map_err(|err| format!("error rendering rest head el: {err}"))?;
			out.push('\n');
		}
		out.push_str(&self.rest_end);

		Ok(out)
	}

	pub(crate) fn prepare(&self, els: &[Element]) -> Prepared {
		self.init_dedupe_rules(None);

		let deduped = self.dedup_head_els(els);
		let mut out = Prepared {
			meta: Vec::with_capacity(deduped.len()),
			rest: Vec::with_capacity(deduped.len()),
			..Prepared::default()
		};

		for el in deduped {
			let safe = escape_into_trusted(&el);
			match safe.tag.as_str() {
				"title" => out.title = Some(safe),
				"meta" => out.meta.push(safe),
				_ => out.rest.push(safe),
			}
		}

		out
	}

	fn dedup_head_els(&self, els: &[Element]) -> Vec<Element> {
		let rules_by_tag = self.unique_rules_by_tag.get_or_init(BTreeMap::new);
		let mut result = Vec::with_capacity(els.len());
		let mut seen_rule = BTreeMap::<DedupeKey, usize>::new();
		let mut seen_el = BTreeMap::<Element, usize>::new();

		for el in els {
			if let Some(rules) = rules_by_tag.get(&el.tag) {
				let mut matched = false;
				for (rule_idx, rule) in rules.iter().enumerate() {
					if matches_rule(el, rule) {
						let key = DedupeKey {
							tag: el.tag.clone(),
							rule_idx,
						};
						if let Some(pos) = seen_rule.get(&key) {
							result[*pos] = el.clone();
						} else {
							seen_rule.insert(key, result.len());
							result.push(el.clone());
						}
						matched = true;
						break;
					}
				}
				if matched {
					continue;
				}
			}

			if let Some(pos) = seen_el.get(el) {
				result[*pos] = el.clone();
			} else {
				seen_el.insert(el.clone(), result.len());
				result.push(el.clone());
			}
		}

		result
	}
}

fn comment(namespace: &str, val: &str) -> String {
	format!("<!-- {namespace}-{val} -->")
}

fn extract_rule_attrs(rule: &Element) -> RuleAttrs {
	RuleAttrs {
		attrs: rule.attributes.clone(),
		trusted: rule.attributes_known_safe.clone(),
		boolean: rule.boolean_attributes.clone(),
	}
}

fn matches_rule(el: &Element, rule: &RuleAttrs) -> bool {
	let check = |key: &str, expected: &str| -> bool {
		if expected == ATTR_ANY_VALUE {
			return el.attributes.contains_key(key) || el.attributes_known_safe.contains_key(key);
		}
		el.attributes
			.get(key)
			.is_some_and(|value| value == expected)
			|| el
				.attributes_known_safe
				.get(key)
				.is_some_and(|value| value == expected)
	};

	for (key, value) in &rule.attrs {
		if !check(key, value) {
			return false;
		}
	}
	for (key, value) in &rule.trusted {
		if !check(key, value) {
			return false;
		}
	}
	for attr in &rule.boolean {
		if !el.boolean_attributes.contains(attr) {
			return false;
		}
	}
	true
}

/// Head element tag marker for low-level head builder definitions.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HeadTag(pub &'static str);

/// Head element attribute marker for low-level head builder definitions.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HeadAttr {
	attr: [String; 2],
	known_safe: bool,
}

/// Boolean head element attribute marker.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HeadBooleanAttribute(pub String);

/// Trusted inner HTML marker for low-level head builder definitions.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HeadInnerHtml(pub String);

/// Escaped text-content marker for low-level head builder definitions.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct HeadTextContent(pub String);

/// Whether a low-level head element renders as self-closing.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct HeadSelfClosing(pub bool);

/// Low-level head element definition part.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum HtmlElementDef {
	/// Element tag.
	HeadTag(HeadTag),
	/// Element attribute.
	HeadAttr(HeadAttr),
	/// Boolean element attribute.
	HeadBooleanAttribute(HeadBooleanAttribute),
	/// Trusted inner HTML.
	HeadInnerHtml(HeadInnerHtml),
	/// Escaped text content.
	HeadTextContent(HeadTextContent),
	/// Self-closing flag.
	HeadSelfClosing(HeadSelfClosing),
}

impl HeadAttr {
	/// Create a normal escaped attribute.
	pub fn new(name: impl Into<String>, value: impl Into<String>) -> Self {
		Self {
			attr: [name.into(), value.into()],
			known_safe: false,
		}
	}

	/// Mark this attribute value as already trusted.
	pub fn known_safe(mut self) -> Self {
		self.known_safe = true;
		self
	}
}

impl From<HeadTag> for HtmlElementDef {
	fn from(value: HeadTag) -> Self {
		Self::HeadTag(value)
	}
}

impl From<HeadAttr> for HtmlElementDef {
	fn from(value: HeadAttr) -> Self {
		Self::HeadAttr(value)
	}
}

impl From<HeadBooleanAttribute> for HtmlElementDef {
	fn from(value: HeadBooleanAttribute) -> Self {
		Self::HeadBooleanAttribute(value)
	}
}

impl From<HeadInnerHtml> for HtmlElementDef {
	fn from(value: HeadInnerHtml) -> Self {
		Self::HeadInnerHtml(value)
	}
}

impl From<HeadTextContent> for HtmlElementDef {
	fn from(value: HeadTextContent) -> Self {
		Self::HeadTextContent(value)
	}
}

impl From<HeadSelfClosing> for HtmlElementDef {
	fn from(value: HeadSelfClosing) -> Self {
		Self::HeadSelfClosing(value)
	}
}

/// Builder for default head, route head effects, and head dedupe rules.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct HeadBuilder {
	els: Vec<Element>,
}

impl HeadBuilder {
	pub(crate) fn from_elements(els: Vec<Element>) -> Self {
		Self { els }
	}

	/// Create an empty head builder.
	pub fn new() -> Self {
		Self { els: Vec::new() }
	}

	/// Add one low-level element from definition parts.
	pub fn add(
		&mut self,
		defs: impl IntoIterator<Item = HtmlElementDef>,
	) -> Result<&mut Self, String> {
		let mut el = Element::default();
		for def in defs {
			match def {
				HtmlElementDef::HeadTag(tag) => el.tag = tag.0.to_owned(),
				HtmlElementDef::HeadAttr(attr) => {
					if attr.known_safe {
						el.attributes_known_safe
							.insert(attr.attr[0].clone(), attr.attr[1].clone());
					} else {
						el.attributes
							.insert(attr.attr[0].clone(), attr.attr[1].clone());
					}
				}
				HtmlElementDef::HeadBooleanAttribute(attr) => el.boolean_attributes.push(attr.0),
				HtmlElementDef::HeadInnerHtml(inner) => el.dangerous_inner_html = inner.0,
				HtmlElementDef::HeadTextContent(text) => el.text_content = text.0,
				HtmlElementDef::HeadSelfClosing(self_closing) => el.self_closing = self_closing.0,
			}
		}
		if el.tag.is_empty() {
			return Err("head element added without a tag".to_owned());
		}
		self.els.push(el);
		Ok(self)
	}

	/// Append another builder's elements.
	pub fn append(&mut self, other: &HeadBuilder) -> &mut Self {
		self.els.extend_from_slice(&other.els);
		self
	}

	pub(crate) fn elements(&self) -> &[Element] {
		&self.els
	}

	/// Marker for a self-closing low-level element.
	pub fn self_closing(&self) -> HeadSelfClosing {
		HeadSelfClosing(true)
	}

	/// Marker for trusted inner HTML.
	pub fn dangerous_inner_html(&self, content: impl Into<String>) -> HeadInnerHtml {
		HeadInnerHtml(content.into())
	}

	/// Marker for escaped text content.
	pub fn text_content(&self, content: impl Into<String>) -> HeadTextContent {
		HeadTextContent(content.into())
	}

	/// Attribute marker that matches any value in dedupe rules.
	pub fn attr_exists(&self, name: impl Into<String>) -> HeadAttr {
		self.attr(name, ATTR_ANY_VALUE)
	}

	/// Add a `<title>` element.
	pub fn title(&mut self, title: impl Into<String>) -> &mut Self {
		self.add([
			HeadTag("title").into(),
			HeadTextContent(title.into()).into(),
		])
		.expect("title helper includes tag");
		self
	}

	/// Add a meta description.
	pub fn description(&mut self, desc: impl Into<String>) -> &mut Self {
		let desc = desc.into();
		self.meta([self.name("description").into(), self.content(desc).into()]);
		self
	}

	/// Add a `<meta>` element from definition parts.
	pub fn meta(&mut self, defs: impl IntoIterator<Item = HtmlElementDef>) -> &mut Self {
		let mut defs = defs.into_iter().collect::<Vec<_>>();
		defs.push(HeadTag("meta").into());
		self.add(defs).expect("meta helper includes tag");
		self
	}

	/// Add a `<link>` element from definition parts.
	pub fn link(&mut self, defs: impl IntoIterator<Item = HtmlElementDef>) -> &mut Self {
		let mut defs = defs.into_iter().collect::<Vec<_>>();
		defs.push(HeadTag("link").into());
		self.add(defs).expect("link helper includes tag");
		self
	}

	/// Add a `<script>` element from definition parts.
	pub fn script(&mut self, defs: impl IntoIterator<Item = HtmlElementDef>) -> &mut Self {
		let mut defs = defs.into_iter().collect::<Vec<_>>();
		defs.push(HeadTag("script").into());
		self.add(defs).expect("script helper includes tag");
		self
	}

	/// Add a `<style>` element from definition parts.
	pub fn style(&mut self, defs: impl IntoIterator<Item = HtmlElementDef>) -> &mut Self {
		let mut defs = defs.into_iter().collect::<Vec<_>>();
		defs.push(HeadTag("style").into());
		self.add(defs).expect("style helper includes tag");
		self
	}

	/// Create an escaped attribute marker.
	pub fn attr(&self, name: impl Into<String>, value: impl Into<String>) -> HeadAttr {
		HeadAttr::new(name, value)
	}

	/// Create a boolean attribute marker.
	pub fn bool_attr(&self, name: impl Into<String>) -> HeadBooleanAttribute {
		HeadBooleanAttribute(name.into())
	}

	/// Create a `property="..."` attribute marker.
	pub fn property(&self, prop: impl Into<String>) -> HeadAttr {
		self.attr("property", prop)
	}

	/// Create a `name="..."` attribute marker.
	pub fn name(&self, name: impl Into<String>) -> HeadAttr {
		self.attr("name", name)
	}

	/// Create a `content="..."` attribute marker.
	pub fn content(&self, content: impl Into<String>) -> HeadAttr {
		self.attr("content", content)
	}

	/// Create a `rel="..."` attribute marker.
	pub fn rel(&self, rel: impl Into<String>) -> HeadAttr {
		self.attr("rel", rel)
	}

	/// Create a `href="..."` attribute marker.
	pub fn href(&self, href: impl Into<String>) -> HeadAttr {
		self.attr("href", href)
	}

	/// Create a `src="..."` attribute marker.
	pub fn src(&self, src: impl Into<String>) -> HeadAttr {
		self.attr("src", src)
	}

	/// Create a `type="..."` attribute marker.
	pub fn r#type(&self, t: impl Into<String>) -> HeadAttr {
		self.attr("type", t)
	}

	/// Create a `charset="..."` attribute marker.
	pub fn charset(&self, charset: impl Into<String>) -> HeadAttr {
		self.attr("charset", charset)
	}

	/// Create an `as="..."` attribute marker.
	pub fn r#as(&self, r#as: impl Into<String>) -> HeadAttr {
		self.attr("as", r#as)
	}

	/// Create a `crossorigin="..."` attribute marker.
	pub fn cross_origin(&self, co: impl Into<String>) -> HeadAttr {
		self.attr("crossorigin", co)
	}

	/// Add an icon link.
	pub fn icon(&mut self, href: impl Into<String>) -> &mut Self {
		let href = href.into();
		self.link([self.rel("icon").into(), self.href(href).into()]);
		self
	}

	/// Add a preload link.
	pub fn preload(&mut self, href: impl Into<String>, r#as: impl Into<String>) -> &mut Self {
		let href = href.into();
		let r#as = r#as.into();
		self.link([
			self.rel("preload").into(),
			self.href(href).into(),
			self.r#as(r#as).into(),
		]);
		self
	}

	/// Add a `<meta property="..." content="...">` element.
	pub fn meta_property_content(
		&mut self,
		prop: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.meta([
			self.property(prop).into(),
			self.content(content.into()).into(),
		]);
		self
	}

	/// Add a `<meta name="..." content="...">` element.
	pub fn meta_name_content(
		&mut self,
		name: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.meta([self.name(name).into(), self.content(content.into()).into()]);
		self
	}

	/// Add a charset meta element.
	pub fn meta_charset(&mut self, charset: impl Into<String>) -> &mut Self {
		self.meta([self.charset(charset).into()]);
		self
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn render_outputs_title_meta_and_rest_sections() {
		let renderer = Renderer::new("bob");
		let html = renderer
			.render(&Prepared {
				title: Some(Element {
					tag: "title".to_owned(),
					text_content: "Test Title".to_owned(),
					..Element::default()
				}),
				meta: vec![Element {
					tag: "meta".to_owned(),
					attributes: BTreeMap::from([
						("name".to_owned(), "description".to_owned()),
						("content".to_owned(), "Test Description".to_owned()),
					]),
					..Element::default()
				}],
				rest: vec![Element {
					tag: "link".to_owned(),
					attributes: BTreeMap::from([
						("rel".to_owned(), "stylesheet".to_owned()),
						("href".to_owned(), "/style.css".to_owned()),
					]),
					..Element::default()
				}],
			})
			.unwrap();

		assert!(html.contains("<title>Test Title</title>"));
		assert!(html.contains("<!-- bob-meta-start -->"));
		assert!(html.contains("name=\"description\""));
		assert!(html.contains("<!-- bob-rest-start -->"));
		assert!(html.contains("rel=\"stylesheet\""));
	}

	#[test]
	fn prepare_dedupes_default_head_rules_and_deepest_wins() {
		let renderer = Renderer::new("test");
		let prepared = renderer.prepare(&[
			Element {
				tag: "title".to_owned(),
				text_content: "Old".to_owned(),
				..Element::default()
			},
			Element {
				tag: "title".to_owned(),
				text_content: "New".to_owned(),
				..Element::default()
			},
			Element {
				tag: "meta".to_owned(),
				attributes: BTreeMap::from([
					("name".to_owned(), "description".to_owned()),
					("content".to_owned(), "Old Desc".to_owned()),
				]),
				..Element::default()
			},
			Element {
				tag: "meta".to_owned(),
				attributes: BTreeMap::from([
					("name".to_owned(), "description".to_owned()),
					("content".to_owned(), "New Desc".to_owned()),
				]),
				..Element::default()
			},
		]);

		assert_eq!(
			prepared.title.unwrap().dangerous_inner_html,
			"New".to_owned()
		);
		assert_eq!(prepared.meta.len(), 1);
		assert_eq!(
			prepared.meta[0].attributes_known_safe["content"],
			"New Desc"
		);
	}

	#[test]
	fn caller_dedupe_rules_are_initialized_once() {
		let renderer = Renderer::new("test");
		let mut first = HeadBuilder::new();
		first.meta([first.name("keywords").into()]);
		renderer.init_dedupe_rules(Some(&first));

		let mut second = HeadBuilder::new();
		second.link([
			second.rel("stylesheet").into(),
			second.href("/style.css").into(),
		]);
		renderer.init_dedupe_rules(Some(&second));

		let prepared = renderer.prepare(&[
			Element {
				tag: "meta".to_owned(),
				attributes: BTreeMap::from([
					("name".to_owned(), "keywords".to_owned()),
					("content".to_owned(), "one".to_owned()),
				]),
				..Element::default()
			},
			Element {
				tag: "meta".to_owned(),
				attributes: BTreeMap::from([
					("name".to_owned(), "keywords".to_owned()),
					("content".to_owned(), "two".to_owned()),
				]),
				..Element::default()
			},
		]);

		assert_eq!(prepared.meta.len(), 1);
		assert_eq!(prepared.meta[0].attributes_known_safe["content"], "two");
	}

	#[test]
	fn builder_sets_expected_element_fields() {
		let mut b = HeadBuilder::new();
		b.add([
			HeadTag("link").into(),
			b.href("/style.css").known_safe().into(),
			b.rel("stylesheet").into(),
		])
		.unwrap();

		let el = &b.elements()[0];
		assert_eq!(el.tag, "link");
		assert_eq!(el.attributes_known_safe["href"], "/style.css");
		assert_eq!(el.attributes["rel"], "stylesheet");
	}

	#[test]
	fn builder_add_reports_missing_tag_without_mutating() {
		let mut b = HeadBuilder::new();
		let error = b
			.add([b.href("/style.css").known_safe().into()])
			.unwrap_err();

		assert_eq!(error, "head element added without a tag");
		assert!(b.elements().is_empty());
	}

	#[test]
	fn builder_shortcuts_are_chainable() {
		let mut b = HeadBuilder::new();
		b.title("Test Title")
			.description("Test Description")
			.icon("/favicon.ico")
			.preload("/font.woff2", "font");

		assert_eq!(b.elements().len(), 4);
		assert_eq!(b.elements()[0].tag, "title");

		let icon = &b.elements()[2];
		assert_eq!(icon.tag, "link");
		assert_eq!(icon.attributes["rel"], "icon");
		assert_eq!(icon.attributes["href"], "/favicon.ico");

		let preload = &b.elements()[3];
		assert_eq!(preload.tag, "link");
		assert_eq!(preload.attributes["rel"], "preload");
		assert_eq!(preload.attributes["href"], "/font.woff2");
		assert_eq!(preload.attributes["as"], "font");
	}
}
