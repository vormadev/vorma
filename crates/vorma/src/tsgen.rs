use std::collections::{BTreeMap, BTreeSet, HashMap};
use std::error;
use std::fmt;

use serde::Serialize;

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

#[doc(hidden)]
#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub struct TypeResolver {
	roots: Vec<TypeRef>,
	defs: Vec<TypeDef>,
}

impl TypeResolver {
	#[doc(hidden)]
	pub fn add_root(&mut self, root: TypeRef) {
		self.roots.push(root);
	}

	#[doc(hidden)]
	pub fn add_defs(&mut self, defs: impl IntoIterator<Item = TypeDef>) {
		self.defs.extend(defs);
	}

	#[doc(hidden)]
	pub fn resolve(self) -> Result<ResolvedTypes> {
		let defs_by_key = defs_by_key(self.defs)?;
		let mut retained = BTreeSet::new();

		for root in &self.roots {
			retain_type_ref(root, &defs_by_key, &mut retained)?;
		}

		let retained_defs = retained
			.into_iter()
			.filter_map(|key| defs_by_key.get(&key).cloned())
			.collect::<Vec<_>>();
		let canonical = canonicalize_defs(&retained_defs);
		let names_by_canonical_key = assign_export_names(&canonical.defs)?;
		let names_by_key = canonical
			.key_to_canonical_key
			.into_iter()
			.map(|(key, canonical_key)| {
				let name = names_by_canonical_key
					.get(&canonical_key)
					.expect("canonical type should have assigned TypeScript name")
					.clone();
				(key, name)
			})
			.collect::<BTreeMap<_, _>>();
		let mut defs = retained_defs
			.into_iter()
			.filter(|def| canonical.canonical_keys.contains(def.key()))
			.map(|def| {
				let key = def.key().to_owned();
				let name = names_by_key
					.get(&key)
					.expect("retained type should have assigned TypeScript name")
					.clone();
				ResolvedTypeDef { name, def }
			})
			.collect::<Vec<_>>();
		defs.sort_by(|a, b| a.name.cmp(&b.name));

		Ok(ResolvedTypes { defs, names_by_key })
	}
}

struct CanonicalDefs {
	defs: Vec<TypeDef>,
	canonical_keys: BTreeSet<String>,
	key_to_canonical_key: BTreeMap<String, String>,
}

fn canonicalize_defs(defs: &[TypeDef]) -> CanonicalDefs {
	let mut canonical_defs = Vec::<TypeDef>::new();
	let mut canonical_keys = BTreeSet::<String>::new();
	let mut key_to_canonical_key = BTreeMap::<String, String>::new();

	for def in defs {
		let key = def.key().to_owned();
		let canonical_key = canonical_defs
			.iter()
			.find(|candidate| candidate.name() == def.name() && candidate.has_same_body_as(def))
			.map(|candidate| candidate.key().to_owned());

		match canonical_key {
			Some(canonical_key) => {
				key_to_canonical_key.insert(key, canonical_key);
			}
			None => {
				canonical_keys.insert(key.clone());
				key_to_canonical_key.insert(key.clone(), key);
				canonical_defs.push(def.clone());
			}
		}
	}

	CanonicalDefs {
		defs: canonical_defs,
		canonical_keys,
		key_to_canonical_key,
	}
}

#[doc(hidden)]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResolvedTypes {
	defs: Vec<ResolvedTypeDef>,
	names_by_key: BTreeMap<String, String>,
}

impl ResolvedTypes {
	#[doc(hidden)]
	pub fn defs(&self) -> &[ResolvedTypeDef] {
		&self.defs
	}

	#[doc(hidden)]
	pub fn render_type_ref(&self, type_ref: &TypeRef) -> String {
		render_type_ref(type_ref, &self.names_by_key)
	}
}

#[doc(hidden)]
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ResolvedTypeDef {
	name: String,
	def: TypeDef,
}

impl ResolvedTypeDef {
	#[doc(hidden)]
	pub fn name(&self) -> &str {
		&self.name
	}

	#[doc(hidden)]
	pub fn def(&self) -> &TypeDef {
		&self.def
	}
}

fn defs_by_key(defs: Vec<TypeDef>) -> Result<BTreeMap<String, TypeDef>> {
	let mut by_key = BTreeMap::new();
	for def in defs {
		match by_key.get(def.key()) {
			Some(existing) if existing == &def => {}
			Some(_) => {
				return Err(Error::new(format!(
					"conflicting TypeScript type definitions for {}",
					def.name()
				)));
			}
			None => {
				by_key.insert(def.key().to_owned(), def);
			}
		}
	}
	Ok(by_key)
}

fn assign_export_names(defs: &[TypeDef]) -> Result<BTreeMap<String, String>> {
	let mut keys_by_name = BTreeMap::<String, Vec<String>>::new();
	for def in defs {
		keys_by_name
			.entry(def.name().to_owned())
			.or_default()
			.push(def.key().to_owned());
	}

	let mut names_by_key = BTreeMap::new();
	for (name, mut keys) in keys_by_name {
		keys.sort();
		for (index, key) in keys.into_iter().enumerate() {
			let resolved_name = if index == 0 {
				name.clone()
			} else {
				format!("{name}_{}", index + 1)
			};
			validate_export_type_name(&resolved_name)?;
			names_by_key.insert(key, resolved_name);
		}
	}
	Ok(names_by_key)
}

fn validate_export_type_name(name: &str) -> Result<()> {
	validate_ts_identifier(name, "TypeScript export type name")
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

fn retain_type_ref(
	type_ref: &TypeRef,
	defs_by_key: &BTreeMap<String, TypeDef>,
	retained: &mut BTreeSet<String>,
) -> Result<()> {
	match type_ref {
		TypeRef::Named { key, name } => retain_named_type(key, name, defs_by_key, retained),
		TypeRef::Array(inner) | TypeRef::Nullable(inner) => {
			retain_type_ref(inner, defs_by_key, retained)
		}
		TypeRef::Map(key, value) => {
			retain_type_ref(key, defs_by_key, retained)?;
			retain_type_ref(value, defs_by_key, retained)
		}
		TypeRef::Union(types) => {
			for type_ref in types {
				retain_type_ref(type_ref, defs_by_key, retained)?;
			}
			Ok(())
		}
		TypeRef::Raw(parts) => {
			for part in parts {
				if let RawTsPart::TypeRef(type_ref) = part {
					retain_type_ref(type_ref, defs_by_key, retained)?;
				}
			}
			Ok(())
		}
		TypeRef::Unit
		| TypeRef::Null
		| TypeRef::Unknown
		| TypeRef::Bool
		| TypeRef::String
		| TypeRef::Number
		| TypeRef::Integer
		| TypeRef::StringLiteral(_) => Ok(()),
	}
}

fn retain_named_type(
	key: &str,
	name: &str,
	defs_by_key: &BTreeMap<String, TypeDef>,
	retained: &mut BTreeSet<String>,
) -> Result<()> {
	let Some(def) = defs_by_key.get(key) else {
		return Err(Error::new(format!(
			"unresolved TypeScript type reference {name}"
		)));
	};

	if !retained.insert(key.to_owned()) {
		return Ok(());
	}

	match def {
		TypeDef::Alias { target, .. } => retain_type_ref(target, defs_by_key, retained),
		TypeDef::Record { fields, .. } => {
			for field in fields {
				retain_type_ref(field.type_ref(), defs_by_key, retained)?;
			}
			Ok(())
		}
		TypeDef::StringEnum { .. } => Ok(()),
		TypeDef::Raw { body, .. } => {
			for part in body {
				if let RawTsPart::TypeRef(type_ref) = part {
					retain_type_ref(type_ref, defs_by_key, retained)?;
				}
			}
			Ok(())
		}
	}
}

#[doc(hidden)]
pub fn render_types(resolved: &ResolvedTypes) -> String {
	let mut out = String::new();
	for (index, resolved_def) in resolved.defs().iter().enumerate() {
		if index > 0 {
			out.push('\n');
		}
		push_type_def(&mut out, resolved_def, &resolved.names_by_key);
	}
	out
}

fn push_type_def(
	out: &mut String,
	resolved_def: &ResolvedTypeDef,
	names_by_key: &BTreeMap<String, String>,
) {
	let name = resolved_def.name();
	match resolved_def.def() {
		TypeDef::Alias { target, .. } => {
			out.push_str("export type ");
			out.push_str(name);
			out.push_str(" = ");
			out.push_str(&render_type_ref(target, names_by_key));
			out.push_str(";\n");
		}
		TypeDef::Record { fields, .. } => {
			out.push_str("export type ");
			out.push_str(name);
			out.push_str(" = ");
			if fields.is_empty() {
				out.push_str("Record<never, never>");
			} else {
				out.push_str("{\n");
				for field in fields {
					push_field_to_string(out, field, names_by_key);
				}
				out.push('}');
			}
			out.push_str(";\n");
		}
		TypeDef::StringEnum { variants, .. } => {
			out.push_str("export type ");
			out.push_str(name);
			out.push_str(" = ");
			if variants.is_empty() {
				out.push_str("never");
			} else {
				for (index, variant) in variants.iter().enumerate() {
					if index > 0 {
						out.push_str(" | ");
					}
					out.push_str(&string_literal(variant));
				}
			}
			out.push_str(";\n");
		}
		TypeDef::Raw { body, .. } => {
			out.push_str("export type ");
			out.push_str(name);
			out.push_str(" = ");
			out.push_str(&render_raw_parts(body, names_by_key));
			out.push_str(";\n");
		}
	}
}

#[doc(hidden)]
pub fn render_type_ref(type_ref: &TypeRef, names_by_key: &BTreeMap<String, String>) -> String {
	match type_ref {
		TypeRef::Unit => "undefined".to_owned(),
		TypeRef::Null => "null".to_owned(),
		TypeRef::Unknown => "unknown".to_owned(),
		TypeRef::Bool => "boolean".to_owned(),
		TypeRef::String => "string".to_owned(),
		TypeRef::Number | TypeRef::Integer => "number".to_owned(),
		TypeRef::Named { key, name } => names_by_key
			.get(key)
			.unwrap_or_else(|| {
				panic!("unresolved TypeScript type reference {name:?} with key {key:?}")
			})
			.clone(),
		TypeRef::Array(inner) => format!("Array<{}>", render_type_ref(inner, names_by_key)),
		TypeRef::Map(key, value) => {
			format!(
				"Record<{}, {}>",
				render_type_ref(key, names_by_key),
				render_type_ref(value, names_by_key)
			)
		}
		TypeRef::Nullable(inner) => format!("{} | null", render_type_ref(inner, names_by_key)),
		TypeRef::Union(types) if types.is_empty() => "never".to_owned(),
		TypeRef::Union(types) => types
			.iter()
			.map(|type_ref| render_type_ref(type_ref, names_by_key))
			.collect::<Vec<_>>()
			.join(" | "),
		TypeRef::StringLiteral(value) => string_literal(value),
		TypeRef::Raw(parts) => render_raw_parts(parts, names_by_key),
	}
}

fn render_raw_parts(parts: &[RawTsPart], names_by_key: &BTreeMap<String, String>) -> String {
	let mut out = String::new();
	for part in parts {
		match part {
			RawTsPart::Text(value) => out.push_str(value),
			RawTsPart::TypeRef(type_ref) => {
				out.push_str(&render_type_ref(type_ref, names_by_key));
			}
		}
	}
	out
}

fn push_field_to_string(
	out: &mut String,
	field: &FieldDef,
	names_by_key: &BTreeMap<String, String>,
) {
	out.push('\t');
	out.push_str(&property_name(field.name()));
	if field.is_optional() {
		out.push('?');
	}
	out.push_str(": ");
	out.push_str(&render_type_ref(field.type_ref(), names_by_key));
	out.push_str(";\n");
}

#[doc(hidden)]
pub fn string_literal(value: &str) -> String {
	let mut out = String::with_capacity(value.len() + 2);
	out.push('"');
	for ch in value.chars() {
		match ch {
			'"' => out.push_str("\\\""),
			'\\' => out.push_str("\\\\"),
			'\n' => out.push_str("\\n"),
			'\r' => out.push_str("\\r"),
			'\t' => out.push_str("\\t"),
			'\u{2028}' => out.push_str("\\u2028"),
			'\u{2029}' => out.push_str("\\u2029"),
			'\u{08}' => out.push_str("\\b"),
			'\u{0c}' => out.push_str("\\f"),
			ch if ch < ' ' => {
				out.push_str("\\u");
				out.push_str(&format!("{:04x}", ch as u32));
			}
			ch => out.push(ch),
		}
	}
	out.push('"');
	out
}

#[doc(hidden)]
pub fn property_name(name: &str) -> String {
	if name.is_empty() {
		return string_literal(name);
	}

	for (index, ch) in name.chars().enumerate() {
		if index == 0 {
			if ch.is_ascii_alphabetic() || ch == '_' || ch == '$' {
				continue;
			}
			return string_literal(name);
		}
		if ch.is_ascii_alphanumeric() || ch == '_' || ch == '$' {
			continue;
		}
		return string_literal(name);
	}

	name.to_owned()
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn resolver_rejects_invalid_export_type_names() {
		let mut resolver = TypeResolver::default();
		resolver.add_root(TypeRef::named_with_key(
			"bad",
			"Bad; export type Evil = string",
		));
		resolver.add_defs([TypeDef::alias_with_key(
			"bad",
			"Bad; export type Evil = string",
			TypeRef::String,
		)]);

		let err = resolver.resolve().unwrap_err();

		assert!(
			err.to_string()
				.contains("invalid TypeScript export type name")
		);
	}

	#[test]
	fn resolver_rejects_reserved_export_type_names() {
		let mut resolver = TypeResolver::default();
		resolver.add_root(TypeRef::named_with_key("bad", "type"));
		resolver.add_defs([TypeDef::alias_with_key("bad", "type", TypeRef::String)]);

		let err = resolver.resolve().unwrap_err();

		assert!(
			err.to_string()
				.contains("invalid TypeScript export type name")
		);
	}

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

	#[test]
	fn resolver_collapses_phase_specific_defs_when_bodies_match() {
		let mut resolver = TypeResolver::default();
		resolver.add_root(TypeRef::named_with_key("user::deserialize", "User"));
		resolver.add_root(TypeRef::named_with_key("user::serialize", "User"));
		resolver.add_defs([
			TypeDef::record_with_key(
				"user::deserialize",
				"User",
				vec![FieldDef::required::<String>("name")],
			),
			TypeDef::record_with_key(
				"user::serialize",
				"User",
				vec![FieldDef::required::<String>("name")],
			),
		]);

		let resolved = resolver.resolve().unwrap();

		assert_eq!(
			resolved.render_type_ref(&TypeRef::named_with_key("user::deserialize", "User")),
			"User"
		);
		assert_eq!(
			resolved.render_type_ref(&TypeRef::named_with_key("user::serialize", "User")),
			"User"
		);
		assert_eq!(
			render_types(&resolved),
			"export type User = {\n\tname: string;\n};\n"
		);
	}

	#[test]
	fn resolver_keeps_phase_specific_defs_when_bodies_differ() {
		let mut resolver = TypeResolver::default();
		resolver.add_root(TypeRef::named_with_key("user::deserialize", "User"));
		resolver.add_root(TypeRef::named_with_key("user::serialize", "User"));
		resolver.add_defs([
			TypeDef::record_with_key(
				"user::deserialize",
				"User",
				vec![FieldDef::optional::<String>("name")],
			),
			TypeDef::record_with_key(
				"user::serialize",
				"User",
				vec![FieldDef::required::<String>("name")],
			),
		]);

		let resolved = resolver.resolve().unwrap();

		assert_eq!(
			resolved.render_type_ref(&TypeRef::named_with_key("user::deserialize", "User")),
			"User"
		);
		assert_eq!(
			resolved.render_type_ref(&TypeRef::named_with_key("user::serialize", "User")),
			"User_2"
		);
		assert_eq!(
			render_types(&resolved),
			"export type User = {\n\tname?: string;\n};\n\nexport type User_2 = {\n\tname: string;\n};\n"
		);
	}
}
