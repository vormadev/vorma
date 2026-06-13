//! Framework graph contract facts for generated TypeScript and document rendering.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

/// Render a serializable value as tab-indented pretty JSON.
/*
Generated JSON artifacts are tab-indented so committed goldens and JSON
literals embedded in generated TypeScript hold the same bytes the repo
formatter produces, instead of fighting it on every regeneration.
*/
pub fn to_tab_indented_json_string<T: Serialize>(value: &T) -> serde_json::Result<String> {
	let mut bytes = Vec::new();
	let mut serializer = serde_json::Serializer::with_formatter(
		&mut bytes,
		serde_json::ser::PrettyFormatter::with_indent(b"\t"),
	);
	value.serialize(&mut serializer)?;
	Ok(String::from_utf8(bytes).expect("serde_json output is valid UTF-8"))
}

/// TypeScript input/output contract for one route.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct RouteTypeContract {
	input: TypeRefContract,
	output: TypeRefContract,
}

impl RouteTypeContract {
	/// Create a route type contract.
	pub fn new(input: TypeRefContract, output: TypeRefContract) -> Self {
		Self { input, output }
	}

	/// Input type reference.
	pub fn input(&self) -> &TypeRefContract {
		&self.input
	}

	/// Output type reference.
	pub fn output(&self) -> &TypeRefContract {
		&self.output
	}
}

/// TypeScript type reference contract.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub enum TypeRefContract {
	/// `undefined`.
	Unit,
	/// `null`.
	Null,
	/// `unknown`.
	Unknown,
	/// `boolean`.
	Bool,
	/// `string`.
	String,
	/// `number`.
	Number,
	/// Integer-like Rust number, rendered as TypeScript `number`.
	Integer,
	/// Named type reference.
	Named {
		/// Stable identity key used for definition resolution.
		key: String,
		/// Preferred TypeScript type name.
		name: String,
	},
	/// `Array<T>`.
	Array(Box<TypeRefContract>),
	/// `Record<K, V>`.
	Map(Box<TypeRefContract>, Box<TypeRefContract>),
	/// Platform `FormData`.
	FormData,
	/// `T | null`.
	Nullable(Box<TypeRefContract>),
	/// `A | B | ...`.
	Union(Vec<TypeRefContract>),
	/// String literal type.
	StringLiteral(String),
	/// Trusted raw TypeScript type expression.
	Raw(Vec<RawTsPart>),
}

impl TypeRefContract {
	/// Create a named type reference whose key and display name are the same.
	pub fn named(name: impl Into<String>) -> Self {
		let name = name.into();
		Self::named_with_key(name.clone(), name)
	}

	/// Create a named type reference with a separate stable key.
	pub fn named_with_key(key: impl Into<String>, name: impl Into<String>) -> Self {
		Self::Named {
			key: key.into(),
			name: name.into(),
		}
	}

	/// Create an array type reference.
	pub fn array(inner: TypeRefContract) -> Self {
		Self::Array(Box::new(inner))
	}

	/// Create a map/record type reference.
	pub fn map(key: TypeRefContract, value: TypeRefContract) -> Self {
		Self::Map(Box::new(key), Box::new(value))
	}

	/// Create a nullable type reference.
	pub fn nullable(inner: TypeRefContract) -> Self {
		Self::Nullable(Box::new(inner))
	}

	/// Create a union type reference.
	pub fn union(types: Vec<TypeRefContract>) -> Self {
		Self::Union(types)
	}

	/// Create a string literal type reference.
	pub fn string_literal(value: impl Into<String>) -> Self {
		Self::StringLiteral(value.into())
	}

	/// Create a trusted raw TypeScript type expression.
	pub fn raw(ts: impl Into<String>) -> Self {
		Self::Raw(vec![RawTsPart::Text(ts.into())])
	}
}

/// Part of a raw TypeScript type expression.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub enum RawTsPart {
	/// Literal TypeScript text.
	Text(String),
	/// Nested TypeScript type reference rendered through the resolver.
	TypeRef(TypeRefContract),
}

/// TypeScript definition emitted into generated contracts.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub enum TypeDef {
	/// Type alias definition.
	Alias {
		/// Stable identity key.
		key: String,
		/// Exported TypeScript name.
		name: String,
		/// Alias target type.
		target: TypeRefContract,
	},
	/// Object/record definition.
	Record {
		/// Stable identity key.
		key: String,
		/// Exported TypeScript name.
		name: String,
		/// Record fields.
		fields: Vec<FieldDef>,
	},
	/// String-literal union definition.
	StringEnum {
		/// Stable identity key.
		key: String,
		/// Exported TypeScript name.
		name: String,
		/// String literal variants.
		variants: Vec<String>,
	},
	/// Raw TypeScript definition.
	Raw {
		/// Stable identity key.
		key: String,
		/// Exported TypeScript name.
		name: String,
		/// Raw body parts.
		body: Vec<RawTsPart>,
	},
}

impl TypeDef {
	/// Stable identity key for this definition.
	pub fn key(&self) -> &str {
		match self {
			Self::Alias { key, .. }
			| Self::Record { key, .. }
			| Self::StringEnum { key, .. }
			| Self::Raw { key, .. } => key,
		}
	}

	/// Preferred TypeScript export name.
	pub fn name(&self) -> &str {
		match self {
			Self::Alias { name, .. }
			| Self::Record { name, .. }
			| Self::StringEnum { name, .. }
			| Self::Raw { name, .. } => name,
		}
	}

	/// Create an alias definition whose key and export name are the same.
	pub fn alias(name: impl Into<String>, target: TypeRefContract) -> Self {
		let name = name.into();
		Self::alias_with_key(name.clone(), name, target)
	}

	/// Create an alias definition with a separate stable key.
	pub fn alias_with_key(
		key: impl Into<String>,
		name: impl Into<String>,
		target: TypeRefContract,
	) -> Self {
		Self::Alias {
			key: key.into(),
			name: name.into(),
			target,
		}
	}

	/// Create a record definition whose key and export name are the same.
	pub fn record(name: impl Into<String>, fields: Vec<FieldDef>) -> Self {
		let name = name.into();
		Self::record_with_key(name.clone(), name, fields)
	}

	/// Create a record definition with a separate stable key.
	pub fn record_with_key(
		key: impl Into<String>,
		name: impl Into<String>,
		fields: Vec<FieldDef>,
	) -> Self {
		Self::Record {
			key: key.into(),
			name: name.into(),
			fields,
		}
	}

	/// Create a string-literal union definition whose key and export name are the same.
	pub fn string_enum(
		name: impl Into<String>,
		variants: impl IntoIterator<Item = impl Into<String>>,
	) -> Self {
		let name = name.into();
		Self::string_enum_with_key(name.clone(), name, variants)
	}

	/// Create a string-literal union definition with a separate stable key.
	pub fn string_enum_with_key(
		key: impl Into<String>,
		name: impl Into<String>,
		variants: impl IntoIterator<Item = impl Into<String>>,
	) -> Self {
		Self::StringEnum {
			key: key.into(),
			name: name.into(),
			variants: variants.into_iter().map(Into::into).collect(),
		}
	}

	/// Create a trusted raw TypeScript definition.
	pub fn raw(name: impl Into<String>, body: impl Into<String>) -> Self {
		let name = name.into();
		Self::Raw {
			key: name.clone(),
			name,
			body: vec![RawTsPart::Text(body.into())],
		}
	}
}

/// Field definition for a TypeScript record contract.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct FieldDef {
	name: String,
	type_ref: TypeRefContract,
	optional: bool,
}

impl FieldDef {
	/// Required field using `T`'s normal TypeScript shape.
	pub fn required<T: crate::tsgen::Type>(name: impl Into<String>) -> Self {
		Self::new(name, T::type_ref(), false)
	}

	/// Required field using `T` for a specific input/output phase.
	pub fn required_for<T: crate::tsgen::Type>(
		name: impl Into<String>,
		phase: crate::tsgen::TypePhase,
	) -> Self {
		Self::new(name, T::type_ref_for(phase), false)
	}

	/// Optional field using `T`'s normal TypeScript shape.
	pub fn optional<T: crate::tsgen::Type>(name: impl Into<String>) -> Self {
		Self::new(name, T::type_ref(), true)
	}

	/// Optional field using `T` for a specific input/output phase.
	pub fn optional_for<T: crate::tsgen::Type>(
		name: impl Into<String>,
		phase: crate::tsgen::TypePhase,
	) -> Self {
		Self::new(name, T::type_ref_for(phase), true)
	}

	/// Create a TypeScript field contract.
	pub fn new(name: impl Into<String>, type_ref: TypeRefContract, optional: bool) -> Self {
		Self {
			name: name.into(),
			type_ref,
			optional,
		}
	}

	/// Field name.
	pub fn name(&self) -> &str {
		&self.name
	}

	/// Field type reference.
	pub fn type_ref(&self) -> &TypeRefContract {
		&self.type_ref
	}

	/// Whether the field is optional.
	pub fn is_optional(&self) -> bool {
		self.optional
	}
}

/// Build-stable document contract facts.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct DocumentContract {
	html_attributes: Vec<DocumentAttributeContract>,
	body_attributes: Vec<DocumentAttributeContract>,
	head_defaults: Vec<DocumentElementContract>,
	head_dedupe_rules: Vec<DocumentElementContract>,
	body_prefix: Vec<DocumentElementContract>,
}

impl DocumentContract {
	/// Create a document contract.
	pub fn new(
		html_attributes: Vec<DocumentAttributeContract>,
		body_attributes: Vec<DocumentAttributeContract>,
		head_defaults: Vec<DocumentElementContract>,
		head_dedupe_rules: Vec<DocumentElementContract>,
		body_prefix: Vec<DocumentElementContract>,
	) -> Self {
		Self {
			html_attributes,
			body_attributes,
			head_defaults,
			head_dedupe_rules,
			body_prefix,
		}
	}

	/// Root `<html>` attributes.
	pub fn html_attributes(&self) -> &[DocumentAttributeContract] {
		&self.html_attributes
	}

	/// Root `<body>` attributes.
	pub fn body_attributes(&self) -> &[DocumentAttributeContract] {
		&self.body_attributes
	}

	/// Default document head elements.
	pub fn head_defaults(&self) -> &[DocumentElementContract] {
		&self.head_defaults
	}

	/// Document head dedupe-rule elements.
	pub fn head_dedupe_rules(&self) -> &[DocumentElementContract] {
		&self.head_dedupe_rules
	}

	/// Body prefix elements.
	pub fn body_prefix(&self) -> &[DocumentElementContract] {
		&self.body_prefix
	}
}

/// Document attribute contract.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct DocumentAttributeContract {
	name: String,
	value: String,
	known_safe: bool,
	boolean: bool,
}

impl DocumentAttributeContract {
	/// Create a document attribute contract.
	pub fn new(
		name: impl Into<String>,
		value: impl Into<String>,
		known_safe: bool,
		boolean: bool,
	) -> Self {
		Self {
			name: name.into(),
			value: value.into(),
			known_safe,
			boolean,
		}
	}

	/// Attribute name.
	pub fn name(&self) -> &str {
		&self.name
	}

	/// Attribute value.
	pub fn value(&self) -> &str {
		&self.value
	}

	/// Whether the value is already trusted HTML.
	pub fn known_safe(&self) -> bool {
		self.known_safe
	}

	/// Whether this is a boolean attribute.
	pub fn boolean(&self) -> bool {
		self.boolean
	}
}

/// Document element contract.
#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub struct DocumentElementContract {
	tag: String,
	attributes: BTreeMap<String, String>,
	attributes_known_safe: BTreeMap<String, String>,
	boolean_attributes: Vec<String>,
	text_content: String,
	dangerous_inner_html: String,
	self_closing: bool,
}

impl DocumentElementContract {
	/// Create a document element contract.
	pub fn new(tag: impl Into<String>) -> Self {
		Self {
			tag: tag.into(),
			..Self::default()
		}
	}

	/// Replace normal escaped attributes.
	pub fn with_attributes(mut self, attributes: BTreeMap<String, String>) -> Self {
		self.attributes = attributes;
		self
	}

	/// Replace known-safe attributes.
	pub fn with_known_safe_attributes(mut self, attributes: BTreeMap<String, String>) -> Self {
		self.attributes_known_safe = attributes;
		self
	}

	/// Replace boolean attributes.
	pub fn with_boolean_attributes(mut self, attributes: Vec<String>) -> Self {
		self.boolean_attributes = attributes;
		self
	}

	/// Replace escaped text content.
	pub fn with_text_content(mut self, text_content: impl Into<String>) -> Self {
		self.text_content = text_content.into();
		self
	}

	/// Replace trusted inner HTML.
	pub fn with_dangerous_inner_html(mut self, dangerous_inner_html: impl Into<String>) -> Self {
		self.dangerous_inner_html = dangerous_inner_html.into();
		self
	}

	/// Set self-closing behavior.
	pub fn with_self_closing(mut self, self_closing: bool) -> Self {
		self.self_closing = self_closing;
		self
	}

	/// Element tag name.
	pub fn tag(&self) -> &str {
		&self.tag
	}

	/// Normal escaped attributes.
	pub fn attributes(&self) -> &BTreeMap<String, String> {
		&self.attributes
	}

	/// Known-safe attributes.
	pub fn attributes_known_safe(&self) -> &BTreeMap<String, String> {
		&self.attributes_known_safe
	}

	/// Boolean attributes.
	pub fn boolean_attributes(&self) -> &[String] {
		&self.boolean_attributes
	}

	/// Escaped text content.
	pub fn text_content(&self) -> &str {
		&self.text_content
	}

	/// Trusted inner HTML.
	pub fn dangerous_inner_html(&self) -> &str {
		&self.dangerous_inner_html
	}

	/// Whether the element is self-closing.
	pub fn self_closing(&self) -> bool {
		self.self_closing
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn route_type_contract_preserves_input_and_output_refs() {
		let contract = RouteTypeContract::new(
			TypeRefContract::String,
			TypeRefContract::Array(Box::new(TypeRefContract::Number)),
		);

		assert_eq!(contract.input(), &TypeRefContract::String);
		assert_eq!(
			contract.output(),
			&TypeRefContract::Array(Box::new(TypeRefContract::Number))
		);
	}

	#[test]
	fn document_contract_preserves_build_identity_facts() {
		let document = DocumentContract::new(
			vec![DocumentAttributeContract::new("lang", "en", false, false)],
			Vec::new(),
			vec![DocumentElementContract::new("title").with_text_content("Example")],
			Vec::new(),
			Vec::new(),
		);

		assert_eq!(document.html_attributes()[0].name(), "lang");
		assert_eq!(document.head_defaults()[0].tag(), "title");
		assert_eq!(document.head_defaults()[0].text_content(), "Example");
	}
}
