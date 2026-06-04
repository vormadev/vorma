use std::collections::{BTreeMap, HashMap};
use std::error;
use std::fmt;

use serde::Serialize;

mod identifier;
mod render;
mod resolve;

use identifier::validate_ts_identifier;
pub use render::{property_name, render_type_ref, render_types, string_literal};
pub use resolve::{ResolvedTypeDef, ResolvedTypes, TypeResolver};

/// TypeScript generation error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Error {
	message: String,
}

impl Error {
	/// Create a new TypeScript generation error.
	pub fn new(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
		}
	}
}

impl fmt::Display for Error {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		f.write_str(&self.message)
	}
}

impl error::Error for Error {}

/// Result type for TypeScript generation helpers.
pub type Result<T> = std::result::Result<T, Error>;

/// Whether a Rust type is being used for serialized output or deserialized input.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TypePhase {
	/// Server-to-client output shape.
	Serialize,
	/// Client-to-server input shape.
	Deserialize,
}

/// Extra Rust type to include in generated TypeScript output.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TsExtraType {
	type_ref: TypeRef,
	type_defs: Vec<TypeDef>,
}

impl TsExtraType {
	/// Include the normal serialized TypeScript shape for `T`.
	pub fn of<T>() -> Result<Self>
	where
		T: Type,
	{
		Self::of_for::<T>(TypePhase::Serialize)
	}

	/// Include `T` using a specific input/output phase.
	pub fn of_for<T>(phase: TypePhase) -> Result<Self>
	where
		T: Type,
	{
		let mut registry = TypeRegistry::default();
		T::collect_type_defs_for(phase, &mut registry)?;
		Ok(Self {
			type_ref: T::type_ref_for(phase),
			type_defs: registry.into_defs(),
		})
	}

	#[doc(hidden)]
	pub fn __type_ref(&self) -> &TypeRef {
		&self.type_ref
	}

	#[doc(hidden)]
	pub fn __type_defs(&self) -> &[TypeDef] {
		&self.type_defs
	}
}

/// Builder for supplemental TypeScript declarations.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct TsDrafter {
	entries: Vec<String>,
}

impl TsDrafter {
	/// Create an empty TypeScript drafter.
	pub fn new() -> Self {
		Self::default()
	}

	/// Append an exported `const` initialized from a serializable Rust value.
	pub fn export_const(
		&mut self,
		name: impl Into<String>,
		value: impl Serialize,
	) -> Result<&mut Self> {
		self.add_const("export ", name, value)
	}

	/// Append a non-exported `const` initialized from a serializable Rust value.
	pub fn const_(&mut self, name: impl Into<String>, value: impl Serialize) -> Result<&mut Self> {
		self.add_const("", name, value)
	}

	/// Append an exported TypeScript type alias.
	pub fn export_type(
		&mut self,
		name: impl Into<String>,
		value: impl Into<String>,
	) -> Result<&mut Self> {
		self.add_type("export ", name, value)
	}

	/// Append a non-exported TypeScript type alias.
	pub fn type_(
		&mut self,
		name: impl Into<String>,
		value: impl Into<String>,
	) -> Result<&mut Self> {
		self.add_type("", name, value)
	}

	/// Append an exported string-literal union type.
	pub fn export_string_enum(
		&mut self,
		name: impl Into<String>,
		variants: impl IntoIterator<Item = impl Into<String>>,
	) -> Result<&mut Self> {
		let mut value = String::new();
		for (index, variant) in variants.into_iter().enumerate() {
			if index > 0 {
				value.push_str(" | ");
			}
			value.push_str(&string_literal(&variant.into()));
		}
		if value.is_empty() {
			value.push_str("never");
		}
		self.export_type(name, value)
	}

	/// Append an exported const object and exported union type over its values.
	pub fn export_string_enum_object(
		&mut self,
		const_name: impl Into<String>,
		type_name: impl Into<String>,
		variants: impl IntoIterator<Item = (impl Into<String>, impl Into<String>)>,
	) -> Result<&mut Self> {
		let const_name = const_name.into();
		let type_name = type_name.into();
		validate_ts_identifier(&type_name, "TypeScript declaration name")?;
		let mut object = BTreeMap::<String, String>::new();
		for (key, value) in variants {
			object.insert(key.into(), value.into());
		}
		self.export_const(&const_name, object)?;
		self.entries.push(format!(
			"export type {type_name} = (typeof {const_name})[keyof typeof {const_name}];"
		));
		Ok(self)
	}

	/// Append trusted raw TypeScript.
	pub fn raw(&mut self, content: impl Into<String>) -> &mut Self {
		self.entries.push(content.into());
		self
	}

	/// Whether no declarations have been added.
	pub fn is_empty(&self) -> bool {
		self.entries.is_empty()
	}

	fn add_const(
		&mut self,
		prefix: &str,
		name: impl Into<String>,
		value: impl Serialize,
	) -> Result<&mut Self> {
		let name = name.into();
		validate_ts_identifier(&name, "TypeScript declaration name")?;
		let value = serde_json::to_value(value)
			.map_err(|err| Error::new(format!("error serializing TypeScript const: {err}")))?;
		let mut rendered = serde_json::to_string_pretty(&value)
			.map_err(|err| Error::new(format!("error rendering TypeScript const: {err}")))?;
		if matches!(
			value,
			serde_json::Value::Array(_) | serde_json::Value::Object(_)
		) {
			rendered.push_str(" as const");
		}
		self.entries
			.push(format!("{prefix}const {name} = {rendered};"));
		Ok(self)
	}

	fn add_type(
		&mut self,
		prefix: &str,
		name: impl Into<String>,
		value: impl Into<String>,
	) -> Result<&mut Self> {
		let name = name.into();
		validate_ts_identifier(&name, "TypeScript declaration name")?;
		self.entries
			.push(format!("{prefix}type {name} = {};", value.into()));
		Ok(self)
	}
}

impl fmt::Display for TsDrafter {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		for (index, entry) in self.entries.iter().enumerate() {
			if index > 0 {
				f.write_str("\n\n")?;
			}
			f.write_str(entry)?;
		}
		Ok(())
	}
}

/// TypeScript type reference used by Vorma's generated contract machinery.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum TypeRef {
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
	Array(Box<TypeRef>),
	/// `Record<K, V>`.
	Map(Box<TypeRef>, Box<TypeRef>),
	/// `T | null`.
	Nullable(Box<TypeRef>),
	/// `A | B | ...`.
	Union(Vec<TypeRef>),
	/// String literal type.
	StringLiteral(String),
	/// Raw TypeScript type expression.
	Raw(Vec<RawTsPart>),
}

impl TypeRef {
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
	pub fn array(inner: TypeRef) -> Self {
		Self::Array(Box::new(inner))
	}

	/// Create a map/record type reference.
	pub fn map(key: TypeRef, value: TypeRef) -> Self {
		Self::Map(Box::new(key), Box::new(value))
	}

	/// Create a nullable type reference.
	pub fn nullable(inner: TypeRef) -> Self {
		Self::Nullable(Box::new(inner))
	}

	/// Create a union type reference.
	pub fn union(types: Vec<TypeRef>) -> Self {
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
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum RawTsPart {
	/// Literal TypeScript text.
	Text(String),
	/// Nested TypeScript type reference rendered through the resolver.
	TypeRef(TypeRef),
}

/// TypeScript definition emitted into the generated file.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum TypeDef {
	/// Type alias definition.
	Alias {
		/// Stable identity key.
		key: String,
		/// Exported TypeScript name.
		name: String,
		/// Alias target type.
		target: TypeRef,
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
	pub fn alias(name: impl Into<String>, target: TypeRef) -> Self {
		let name = name.into();
		Self::alias_with_key(name.clone(), name, target)
	}

	/// Create an alias definition with a separate stable key.
	pub fn alias_with_key(
		key: impl Into<String>,
		name: impl Into<String>,
		target: TypeRef,
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

	/// Create a raw TypeScript definition.
	pub fn raw(name: impl Into<String>, body: impl Into<String>) -> Self {
		let name = name.into();
		Self::Raw {
			key: name.clone(),
			name,
			body: vec![RawTsPart::Text(body.into())],
		}
	}

	fn has_same_body_as(&self, other: &Self) -> bool {
		match (self, other) {
			(Self::Alias { target: left, .. }, Self::Alias { target: right, .. }) => left == right,
			(Self::Record { fields: left, .. }, Self::Record { fields: right, .. }) => {
				left == right
			}
			(
				Self::StringEnum { variants: left, .. },
				Self::StringEnum {
					variants: right, ..
				},
			) => left == right,
			(Self::Raw { body: left, .. }, Self::Raw { body: right, .. }) => left == right,
			_ => false,
		}
	}
}

/// Field definition for a TypeScript record.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FieldDef {
	name: String,
	type_ref: TypeRef,
	optional: bool,
}

impl FieldDef {
	/// Required field using `T`'s normal TypeScript shape.
	pub fn required<T: Type>(name: impl Into<String>) -> Self {
		Self::new(name, T::type_ref(), false)
	}

	/// Required field using `T` for a specific input/output phase.
	pub fn required_for<T: Type>(name: impl Into<String>, phase: TypePhase) -> Self {
		Self::new(name, T::type_ref_for(phase), false)
	}

	/// Optional field using `T`'s normal TypeScript shape.
	pub fn optional<T: Type>(name: impl Into<String>) -> Self {
		Self::new(name, T::type_ref(), true)
	}

	/// Optional field using `T` for a specific input/output phase.
	pub fn optional_for<T: Type>(name: impl Into<String>, phase: TypePhase) -> Self {
		Self::new(name, T::type_ref_for(phase), true)
	}

	/// Create a field from an explicit type reference.
	pub fn new(name: impl Into<String>, type_ref: TypeRef, optional: bool) -> Self {
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
	pub fn type_ref(&self) -> &TypeRef {
		&self.type_ref
	}

	/// Whether the field is optional.
	pub fn is_optional(&self) -> bool {
		self.optional
	}
}

/// Registry of TypeScript definitions collected while resolving app types.
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct TypeRegistry {
	defs: Vec<TypeDef>,
}

impl TypeRegistry {
	/// Define a type, panicking on conflicting definitions.
	pub fn define(&mut self, def: TypeDef) {
		self.try_define(def)
			.expect("conflicting Vorma type definitions");
	}

	/// Define a type and report whether it was newly inserted.
	pub fn try_define(&mut self, def: TypeDef) -> Result<bool> {
		if let Some(existing) = self
			.defs
			.iter()
			.find(|existing| existing.key() == def.key())
		{
			if existing == &def {
				return Ok(false);
			}
			return Err(Error::new(format!(
				"conflicting TypeScript type definitions for {}",
				def.name()
			)));
		}
		self.defs.push(def);
		Ok(true)
	}

	/// Consume the registry into its collected definitions.
	pub fn into_defs(self) -> Vec<TypeDef> {
		self.defs
	}
}

/// Trait implemented by Rust types that can describe their generated TypeScript shape.
pub trait Type: Send + Sync + 'static {
	/// Return this type's default TypeScript reference.
	fn type_ref() -> TypeRef;

	/// Collect any named definitions required by this type.
	fn collect_type_defs(_registry: &mut TypeRegistry) -> Result<()> {
		Ok(())
	}

	/// Return this type's TypeScript reference for a specific input/output phase.
	fn type_ref_for(_phase: TypePhase) -> TypeRef {
		Self::type_ref()
	}

	/// Collect named definitions required by this type for a specific phase.
	fn collect_type_defs_for(phase: TypePhase, registry: &mut TypeRegistry) -> Result<()> {
		let _ = phase;
		Self::collect_type_defs(registry)
	}
}

impl Type for () {
	fn type_ref() -> TypeRef {
		TypeRef::Unit
	}
}

impl Type for crate::mux::None {
	fn type_ref() -> TypeRef {
		TypeRef::Unit
	}
}

impl Type for bool {
	fn type_ref() -> TypeRef {
		TypeRef::Bool
	}
}

impl Type for String {
	fn type_ref() -> TypeRef {
		TypeRef::String
	}
}

impl Type for &'static str {
	fn type_ref() -> TypeRef {
		TypeRef::String
	}
}

impl Type for serde_json::Value {
	fn type_ref() -> TypeRef {
		TypeRef::Unknown
	}
}

macro_rules! integer_type {
    ($($ty:ty),* $(,)?) => {
        $(
            impl Type for $ty {
                fn type_ref() -> TypeRef {
                    TypeRef::Integer
                }
            }
        )*
    };
}

integer_type!(
	i8, i16, i32, i64, i128, isize, u8, u16, u32, u64, u128, usize
);

macro_rules! number_type {
    ($($ty:ty),* $(,)?) => {
        $(
            impl Type for $ty {
                fn type_ref() -> TypeRef {
                    TypeRef::Number
                }
            }
        )*
    };
}

number_type!(f32, f64);

impl<T: Type> Type for Option<T> {
	fn type_ref() -> TypeRef {
		TypeRef::nullable(T::type_ref())
	}

	fn type_ref_for(phase: TypePhase) -> TypeRef {
		TypeRef::nullable(T::type_ref_for(phase))
	}

	fn collect_type_defs(registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs(registry)
	}

	fn collect_type_defs_for(phase: TypePhase, registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs_for(phase, registry)
	}
}

impl<T: Type> Type for Vec<T> {
	fn type_ref() -> TypeRef {
		TypeRef::array(T::type_ref())
	}

	fn type_ref_for(phase: TypePhase) -> TypeRef {
		TypeRef::array(T::type_ref_for(phase))
	}

	fn collect_type_defs(registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs(registry)
	}

	fn collect_type_defs_for(phase: TypePhase, registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs_for(phase, registry)
	}
}

impl<T: Type, const N: usize> Type for [T; N] {
	fn type_ref() -> TypeRef {
		TypeRef::array(T::type_ref())
	}

	fn type_ref_for(phase: TypePhase) -> TypeRef {
		TypeRef::array(T::type_ref_for(phase))
	}

	fn collect_type_defs(registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs(registry)
	}

	fn collect_type_defs_for(phase: TypePhase, registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs_for(phase, registry)
	}
}

impl<T: Type> Type for Box<T> {
	fn type_ref() -> TypeRef {
		T::type_ref()
	}

	fn type_ref_for(phase: TypePhase) -> TypeRef {
		T::type_ref_for(phase)
	}

	fn collect_type_defs(registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs(registry)
	}

	fn collect_type_defs_for(phase: TypePhase, registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs_for(phase, registry)
	}
}

impl<T: Type> Type for BTreeMap<String, T> {
	fn type_ref() -> TypeRef {
		TypeRef::map(TypeRef::String, T::type_ref())
	}

	fn type_ref_for(phase: TypePhase) -> TypeRef {
		TypeRef::map(TypeRef::String, T::type_ref_for(phase))
	}

	fn collect_type_defs(registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs(registry)
	}

	fn collect_type_defs_for(phase: TypePhase, registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs_for(phase, registry)
	}
}

impl<T: Type> Type for HashMap<String, T> {
	fn type_ref() -> TypeRef {
		TypeRef::map(TypeRef::String, T::type_ref())
	}

	fn type_ref_for(phase: TypePhase) -> TypeRef {
		TypeRef::map(TypeRef::String, T::type_ref_for(phase))
	}

	fn collect_type_defs(registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs(registry)
	}

	fn collect_type_defs_for(phase: TypePhase, registry: &mut TypeRegistry) -> Result<()> {
		T::collect_type_defs_for(phase, registry)
	}
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn drafter_rejects_invalid_declaration_names() {
		let mut drafter = TsDrafter::new();

		let err = drafter.export_const("Bad-Name", true).unwrap_err();
		assert!(
			err.to_string()
				.contains("invalid TypeScript declaration name")
		);

		let err = drafter.export_type("type", "string").unwrap_err();
		assert!(err.to_string().contains("reserved word"));

		let err = drafter
			.export_string_enum_object("FLAGS", "Bad Name", [("Ok", "ok")])
			.unwrap_err();
		assert!(
			err.to_string()
				.contains("invalid TypeScript declaration name")
		);
	}

	#[test]
	fn render_type_ref_renders_empty_union_as_never() {
		assert_eq!(
			render_type_ref(&TypeRef::union(Vec::new()), &BTreeMap::new()),
			"never"
		);
	}

	#[test]
	#[should_panic(expected = "unresolved TypeScript type reference")]
	fn render_type_ref_panics_on_unresolved_named_type() {
		render_type_ref(
			&TypeRef::named_with_key("missing", "Missing"),
			&BTreeMap::new(),
		);
	}
}
