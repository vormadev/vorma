//! Public TypeScript type collection surface.

use std::collections::{BTreeMap, HashMap};
use std::error;
use std::fmt;

use serde::Serialize;

pub use crate::contracts::{FieldDef, RawTsPart, TypeDef, TypeRefContract as TypeRef};

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
			.map_err(|error| Error::new(format!("error serializing TypeScript const: {error}")))?;
		let mut rendered = crate::contracts::to_tab_indented_json_string(&value)
			.map_err(|error| Error::new(format!("error rendering TypeScript const: {error}")))?;
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

fn string_literal(value: &str) -> String {
	let encoded = serde_json::to_string(value).expect("JSON string encoding should not fail");
	encoded
		.replace('\u{2028}', "\\u2028")
		.replace('\u{2029}', "\\u2029")
}

fn validate_ts_identifier(name: &str, label: &str) -> Result<()> {
	if name.is_empty() {
		return Err(Error::new(format!("{label} cannot be empty")));
	}
	for (index, ch) in name.chars().enumerate() {
		let valid = if index == 0 {
			ch.is_ascii_alphabetic() || ch == '_' || ch == '$'
		} else {
			ch.is_ascii_alphanumeric() || ch == '_' || ch == '$'
		};
		if !valid {
			return Err(Error::new(format!("invalid {label} {name:?}")));
		}
	}
	if ts_identifier_is_reserved(name) {
		return Err(Error::new(format!(
			"invalid {label} {name:?}: reserved word"
		)));
	}
	Ok(())
}

fn ts_identifier_is_reserved(name: &str) -> bool {
	matches!(
		name,
		"abstract"
			| "any" | "as"
			| "async" | "await"
			| "boolean"
			| "break" | "case"
			| "catch" | "class"
			| "const" | "constructor"
			| "continue"
			| "debugger"
			| "declare"
			| "default"
			| "delete"
			| "do" | "else"
			| "enum" | "export"
			| "extends"
			| "false" | "finally"
			| "for" | "from"
			| "function"
			| "get" | "if"
			| "implements"
			| "import"
			| "in" | "infer"
			| "instanceof"
			| "interface"
			| "is" | "keyof"
			| "let" | "module"
			| "namespace"
			| "never" | "new"
			| "null" | "number"
			| "object"
			| "of" | "package"
			| "private"
			| "protected"
			| "public"
			| "readonly"
			| "require"
			| "return"
			| "set" | "static"
			| "string"
			| "super" | "switch"
			| "symbol"
			| "this" | "throw"
			| "true" | "try"
			| "type" | "typeof"
			| "undefined"
			| "unique"
			| "unknown"
			| "var" | "void"
			| "while" | "with"
			| "yield"
	)
}

#[cfg(test)]
mod tests {
	use super::*;

	const TEST_TYPE_NAME: &str = "User";

	#[test]
	fn ts_extra_type_collects_referenced_definitions() {
		struct User;

		impl Type for User {
			fn type_ref() -> TypeRef {
				TypeRef::named(TEST_TYPE_NAME)
			}

			fn collect_type_defs(registry: &mut TypeRegistry) -> Result<()> {
				registry.try_define(TypeDef::record(
					TEST_TYPE_NAME,
					vec![FieldDef::required::<String>("name")],
				))?;
				Ok(())
			}
		}

		let extra_type = TsExtraType::of::<User>().unwrap();

		assert_eq!(extra_type.__type_ref(), &TypeRef::named(TEST_TYPE_NAME));
		assert_eq!(extra_type.__type_defs()[0].name(), TEST_TYPE_NAME);
	}

	#[test]
	fn type_registry_rejects_conflicting_definitions() {
		let mut registry = TypeRegistry::default();
		assert!(
			registry
				.try_define(TypeDef::alias(TEST_TYPE_NAME, TypeRef::String))
				.unwrap()
		);
		assert!(
			!registry
				.try_define(TypeDef::alias(TEST_TYPE_NAME, TypeRef::String))
				.unwrap()
		);
		let error = registry
			.try_define(TypeDef::alias(TEST_TYPE_NAME, TypeRef::Integer))
			.unwrap_err();

		assert!(
			error
				.to_string()
				.contains("conflicting TypeScript type definitions")
		);
	}

	#[test]
	fn drafter_validates_and_renders_supplemental_declarations() {
		let mut drafter = TsDrafter::new();
		drafter
			.export_const("FLAGS", BTreeMap::from([("enabled", true)]))
			.unwrap()
			.export_string_enum("Mode", ["read", "write"])
			.unwrap();

		let rendered = drafter.to_string();
		assert!(rendered.contains("export const FLAGS"));
		assert!(rendered.contains("export type Mode = \"read\" | \"write\";"));
		assert!(
			drafter
				.export_type("type", "string")
				.unwrap_err()
				.to_string()
				.contains("reserved word")
		);
	}
}
