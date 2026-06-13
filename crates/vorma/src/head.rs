//! Public low-level head element builders.

use std::collections::BTreeMap;

use vorma_contract::wire::HeadElement;

use crate::contracts::DocumentElementContract;
use crate::document_renderer::{DocumentRenderError, trusted_element_parts};

pub(crate) const ATTR_ANY_VALUE: &str = "\0__vorma_headels_any__";

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
	elements: Vec<DocumentElementContract>,
}

impl HeadBuilder {
	/// Create an empty head builder.
	pub fn new() -> Self {
		Self {
			elements: Vec::new(),
		}
	}

	/// Add one low-level element from definition parts.
	pub fn add(
		&mut self,
		defs: impl IntoIterator<Item = impl Into<HtmlElementDef>>,
	) -> Result<&mut Self, String> {
		self.elements.push(element_contract_from_defs(defs)?);
		Ok(self)
	}

	/// Append another builder's elements.
	pub fn append(&mut self, other: &HeadBuilder) -> &mut Self {
		self.elements.extend_from_slice(other.elements());
		self
	}

	/// Low-level document element contracts.
	pub fn elements(&self) -> &[DocumentElementContract] {
		&self.elements
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
		let defs: [HtmlElementDef; 2] = [
			HeadTag("title").into(),
			HeadTextContent(title.into()).into(),
		];
		self.add(defs).expect("title helper includes tag");
		self
	}

	/// Add a meta description.
	pub fn description(&mut self, desc: impl Into<String>) -> &mut Self {
		let desc = desc.into();
		self.meta([self.name("description"), self.content(desc)]);
		self
	}

	/// Add a `<meta>` element from definition parts.
	pub fn meta(&mut self, defs: impl IntoIterator<Item = impl Into<HtmlElementDef>>) -> &mut Self {
		let mut defs = defs
			.into_iter()
			.map(Into::into)
			.collect::<Vec<HtmlElementDef>>();
		defs.push(HeadTag("meta").into());
		self.add(defs).expect("meta helper includes tag");
		self
	}

	/// Add a `<link>` element from definition parts.
	pub fn link(&mut self, defs: impl IntoIterator<Item = impl Into<HtmlElementDef>>) -> &mut Self {
		let mut defs = defs
			.into_iter()
			.map(Into::into)
			.collect::<Vec<HtmlElementDef>>();
		defs.push(HeadTag("link").into());
		self.add(defs).expect("link helper includes tag");
		self
	}

	/// Add a `<script>` element from definition parts.
	pub fn script(
		&mut self,
		defs: impl IntoIterator<Item = impl Into<HtmlElementDef>>,
	) -> &mut Self {
		let mut defs = defs
			.into_iter()
			.map(Into::into)
			.collect::<Vec<HtmlElementDef>>();
		defs.push(HeadTag("script").into());
		self.add(defs).expect("script helper includes tag");
		self
	}

	/// Add a `<style>` element from definition parts.
	pub fn style(
		&mut self,
		defs: impl IntoIterator<Item = impl Into<HtmlElementDef>>,
	) -> &mut Self {
		let mut defs = defs
			.into_iter()
			.map(Into::into)
			.collect::<Vec<HtmlElementDef>>();
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
		self.link([self.rel("icon"), self.href(href)]);
		self
	}

	/// Add a preload link.
	pub fn preload(&mut self, href: impl Into<String>, r#as: impl Into<String>) -> &mut Self {
		let href = href.into();
		let r#as = r#as.into();
		self.link([self.rel("preload"), self.href(href), self.r#as(r#as)]);
		self
	}

	/// Add a `<meta property="..." content="...">` element.
	pub fn meta_property_content(
		&mut self,
		prop: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.meta([self.property(prop), self.content(content.into())]);
		self
	}

	/// Add a `<meta name="..." content="...">` element.
	pub fn meta_name_content(
		&mut self,
		name: impl Into<String>,
		content: impl Into<String>,
	) -> &mut Self {
		self.meta([self.name(name), self.content(content.into())]);
		self
	}

	/// Add a charset meta element.
	pub fn meta_charset(&mut self, charset: impl Into<String>) -> &mut Self {
		self.meta([self.charset(charset)]);
		self
	}
}

pub(crate) fn element_contract_from_defs(
	defs: impl IntoIterator<Item = impl Into<HtmlElementDef>>,
) -> Result<DocumentElementContract, String> {
	let mut tag = String::new();
	let mut attributes = BTreeMap::new();
	let mut known_safe_attributes = BTreeMap::new();
	let mut boolean_attributes = Vec::new();
	let mut dangerous_inner_html = String::new();
	let mut text_content = String::new();
	let mut self_closing = false;
	for def in defs {
		let def = def.into();
		match def {
			HtmlElementDef::HeadTag(head_tag) => tag = head_tag.0.to_owned(),
			HtmlElementDef::HeadAttr(attr) => {
				if attr.known_safe {
					known_safe_attributes.insert(attr.attr[0].clone(), attr.attr[1].clone());
				} else {
					attributes.insert(attr.attr[0].clone(), attr.attr[1].clone());
				}
			}
			HtmlElementDef::HeadBooleanAttribute(attr) => boolean_attributes.push(attr.0),
			HtmlElementDef::HeadInnerHtml(inner) => dangerous_inner_html = inner.0,
			HtmlElementDef::HeadTextContent(text) => text_content = text.0,
			HtmlElementDef::HeadSelfClosing(value) => self_closing = value.0,
		}
	}
	if tag.is_empty() {
		return Err("head element added without a tag".to_owned());
	}
	Ok(DocumentElementContract::new(tag)
		.with_attributes(attributes)
		.with_known_safe_attributes(known_safe_attributes)
		.with_boolean_attributes(boolean_attributes)
		.with_dangerous_inner_html(dangerous_inner_html)
		.with_text_content(text_content)
		.with_self_closing(self_closing))
}

/*
Shared head-element semantics for the handler effect APIs: one constructor
per framework-owned head element and one trusted preparation step. Effect
routing lives on ResponseEffects::apply_head_element.
*/
pub(crate) const HEAD_REL_ICON: &str = "icon";
pub(crate) const HEAD_REL_PRELOAD: &str = "preload";
pub(crate) const META_DESCRIPTION_NAME: &str = "description";

pub(crate) fn title_element(title: impl Into<String>) -> DocumentElementContract {
	DocumentElementContract::new(HEAD_TAG_TITLE).with_text_content(title)
}

pub(crate) fn description_element(description: impl Into<String>) -> DocumentElementContract {
	meta_name_content_element(META_DESCRIPTION_NAME, description)
}

pub(crate) fn icon_element(href: impl Into<String>) -> DocumentElementContract {
	DocumentElementContract::new(HEAD_TAG_LINK).with_attributes(BTreeMap::from([
		(HEAD_ATTR_REL.to_owned(), HEAD_REL_ICON.to_owned()),
		(HEAD_ATTR_HREF.to_owned(), href.into()),
	]))
}

pub(crate) fn preload_element(
	href: impl Into<String>,
	r#as: impl Into<String>,
) -> DocumentElementContract {
	DocumentElementContract::new(HEAD_TAG_LINK).with_attributes(BTreeMap::from([
		(HEAD_ATTR_REL.to_owned(), HEAD_REL_PRELOAD.to_owned()),
		(HEAD_ATTR_HREF.to_owned(), href.into()),
		(HEAD_ATTR_AS.to_owned(), r#as.into()),
	]))
}

pub(crate) fn meta_name_content_element(
	name: impl Into<String>,
	content: impl Into<String>,
) -> DocumentElementContract {
	DocumentElementContract::new(HEAD_TAG_META).with_attributes(BTreeMap::from([
		(HEAD_ATTR_NAME.to_owned(), name.into()),
		(HEAD_ATTR_CONTENT.to_owned(), content.into()),
	]))
}

pub(crate) fn meta_property_content_element(
	property: impl Into<String>,
	content: impl Into<String>,
) -> DocumentElementContract {
	DocumentElementContract::new(HEAD_TAG_META).with_attributes(BTreeMap::from([
		(HEAD_ATTR_PROPERTY.to_owned(), property.into()),
		(HEAD_ATTR_CONTENT.to_owned(), content.into()),
	]))
}

pub(crate) fn meta_charset_element(charset: impl Into<String>) -> DocumentElementContract {
	DocumentElementContract::new(HEAD_TAG_META).with_attributes(BTreeMap::from([(
		HEAD_ATTR_CHARSET.to_owned(),
		charset.into(),
	)]))
}

pub(crate) fn prepare_head_element(
	element: DocumentElementContract,
) -> Result<HeadElement, DocumentRenderError> {
	let parts = trusted_element_parts(&element)?;
	Ok(HeadElement {
		tag: parts.tag,
		attributes_known_safe: parts.attributes_known_safe,
		boolean_attributes: parts.boolean_attributes,
		dangerous_inner_html: parts.dangerous_inner_html,
		self_closing: parts.self_closing,
	})
}

pub(crate) fn prepare_static_head_element(element: DocumentElementContract) -> HeadElement {
	prepare_head_element(element).expect("static head element contract is valid")
}

/*
Shared head-element vocabulary used by handler contexts and view-response
head merging.
*/
pub(crate) const HEAD_TAG_LINK: &str = "link";
pub(crate) const HEAD_TAG_META: &str = "meta";
pub(crate) const HEAD_TAG_SCRIPT: &str = "script";
pub(crate) const HEAD_TAG_TITLE: &str = "title";
pub(crate) const HEAD_ATTR_AS: &str = "as";
pub(crate) const HEAD_ATTR_CHARSET: &str = "charset";
pub(crate) const HEAD_ATTR_CONTENT: &str = "content";
pub(crate) const HEAD_ATTR_CROSSORIGIN: &str = "crossorigin";
pub(crate) const HEAD_ATTR_HREF: &str = "href";
pub(crate) const HEAD_ATTR_ID: &str = "id";
pub(crate) const HEAD_ATTR_NAME: &str = "name";
pub(crate) const HEAD_ATTR_PROPERTY: &str = "property";
pub(crate) const HEAD_ATTR_REL: &str = "rel";
pub(crate) const HEAD_ATTR_SRC: &str = "src";
pub(crate) const HEAD_ATTR_TYPE: &str = "type";

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn head_builder_compiles_low_level_element_defs_into_contracts() {
		let mut builder = HeadBuilder::new();
		/*
		Mixed-with-homogeneous coverage: arrays of raw markers (no .into())
		and pre-converted defs must both satisfy the element-def bound.
		*/
		builder.meta([builder.name("description"), builder.content("About")]);
		builder.link([builder.rel("icon"), builder.href("/favicon.ico")]);
		builder.style([HeadInnerHtml("body{}".to_owned())]);

		assert_eq!(builder.elements()[0].tag(), "meta");
		assert_eq!(builder.elements()[0].attributes()["content"], "About");
		assert_eq!(builder.elements()[1].tag(), "link");
		assert_eq!(builder.elements()[2].dangerous_inner_html(), "body{}");
	}
}
