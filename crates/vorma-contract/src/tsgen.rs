//! Public TypeScript type collection surface.
//!
//! App-facing: this is the model behind `vorma`'s `#[derive(TsGen)]`
//! (`vorma-macros`), which is how application code ordinarily reaches
//! this module — a view or resource input/output type derives `TsGen`,
//! and the derive generates a [`Type`](crate::tsgen::Type) implementation
//! from the type's own `#[derive(Serialize)]`/`#[derive(Deserialize)]`
//! shape and `serde` attributes (see `vorma-macros`' own docs for exactly
//! which shapes and attributes are supported).
//!
//! # Getting started with a manual `Type` implementation
//!
//! `#[derive(TsGen)]` covers named-field structs and unit-only enums.
//! Anything else — a data-carrying enum, a `serde(flatten)`ed field, a
//! type with no Rust `Deserialize`/`Serialize` shape at all — needs a
//! hand-written [`Type`](crate::tsgen::Type) implementation instead:
//!
//! ```
//! use vorma_contract::tsgen::{Result, Type, TypeRef};
//!
//! struct Timestamp(i64);
//!
//! impl Type for Timestamp {
//!     fn type_ref() -> TypeRef {
//!         TypeRef::Integer
//!     }
//! }
//!
//! # fn main() -> Result<()> {
//! assert_eq!(Timestamp::type_ref(), TypeRef::Integer);
//! # Ok(())
//! # }
//! ```
//!
//! A type with its own named shape (rather than reusing a primitive like
//! `TypeRef::Integer` above) additionally implements
//! [`collect_type_defs`](crate::tsgen::Type::collect_type_defs) to
//! register a [`TypeDef`](crate::tsgen::TypeDef) — see
//! [`classify_shared_name_with`](crate::contracts::TypeDef::classify_shared_name_with)
//! for what happens when two different `TypeDef`s would both try to
//! register the same exported name (whether two genuinely distinct Rust
//! types, or one `TsGen`-derived type's own Serialize/Deserialize
//! phases).
//!
//! # Extra generated TypeScript beyond route contracts
//!
//! Not every type or value an app wants in its generated client is
//! reachable from a route's input/output —
//! [`TsExtraType`](crate::tsgen::TsExtraType) and
//! [`TsDrafter`](crate::tsgen::TsDrafter) cover that
//! (`AppConfig::ts_gen_config` in `vorma`).
//! [`TsExtraType::of`](crate::tsgen::TsExtraType::of) includes a Rust
//! type's generated shape even when no route references it;
//! [`TsDrafter`](crate::tsgen::TsDrafter) appends hand-authored
//! constants, type aliases, and string-literal unions built from Rust
//! values — useful for shared constants (a page size, a header name) that
//! should travel through the same generated module the route contracts
//! do,
//! rather than being duplicated by hand on the TypeScript side.

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
///
/// The same Rust type frequently needs two distinct TypeScript shapes:
/// its `Serialize` shape is what the server actually sends (a
/// `skip_serializing_if` field vanishes when the predicate holds), while
/// its `Deserialize` shape is what the client must accept (`serde(default)`
/// fields become optional on the TypeScript side, since the client is not
/// required to supply them). [`Type::type_ref_for`]/[`Type::collect_type_defs_for`]
/// take a phase explicitly rather than assuming one; the `TsGen` derive
/// always registers a phase's [`TypeDef`] under its own distinct key
/// (never sharing a key across phases, even when the two shapes end up
/// identical), so [`TypeRegistry::try_define`]'s key-based dedup — which
/// runs once per route contract, before any other route's `TypeDef`s are
/// even in scope — never collapses them into one definition on its own.
/// A later, cross-route stage does: see
/// [`crate::contracts::TypeDef::classify_shared_name_with`] for where a
/// type reached at both phases across an app's declared routes is
/// resolved to one exported TypeScript type (when the phases agree
/// structurally) or a teaching error (when they do not).
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum TypePhase {
	/// Server-to-client output shape.
	Serialize,
	/// Client-to-server input shape.
	Deserialize,
}

/// Extra Rust type to include in generated TypeScript output.
///
/// Registered through `AppConfig::ts_gen_config.extra_types` in `vorma`
/// for a type that has generated-TypeScript-worthy shape but is not
/// reachable from any route's input/output — [`Self::of`] captures both
/// the type's own [`TypeRef`] and every [`TypeDef`] it (transitively)
/// requires in one call, ready to be emitted alongside route-derived
/// contracts.
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
///
/// Every method here validates `name` as a TypeScript identifier
/// (non-empty, identifier characters only, not a reserved word) before
/// appending anything, and every method returns `&mut Self` so calls
/// chain. Renders (via its [`std::fmt::Display`] implementation) as one
/// blank-line-separated block of statements, in the order they were
/// added — `vorma`'s build crate appends that rendered text after the
/// route-derived generated contracts.
///
/// ```
/// use vorma_contract::tsgen::TsDrafter;
///
/// let mut drafter = TsDrafter::new();
/// drafter
///     .export_const("PAGE_SIZE", 20)
///     .unwrap()
///     .export_string_enum("Theme", ["light", "dark"])
///     .unwrap();
///
/// let rendered = drafter.to_string();
/// assert!(rendered.contains("export const PAGE_SIZE = 20;"));
/// assert!(rendered.contains("export type Theme = \"light\" | \"dark\";"));
/// ```
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
	///
	/// Renders through the same [`crate::contracts::to_tab_indented_json_string`]
	/// path any other JSON artifact in this crate does, with `as const`
	/// appended for array/object values so TypeScript infers the
	/// narrowest literal type rather than widening to `string[]` or a
	/// loose object shape.
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
	///
	/// `value` is trusted, unvalidated TypeScript text — the same trust
	/// model [`crate::contracts::TypeRefContract::raw`] uses, appropriate
	/// for hand-written type expressions the caller already controls.
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
	///
	/// ```
	/// use vorma_contract::tsgen::TsDrafter;
	///
	/// let mut drafter = TsDrafter::new();
	/// drafter.export_string_enum("Mode", ["read", "write"]).unwrap();
	///
	/// assert_eq!(
	///     drafter.to_string(),
	///     "export type Mode = \"read\" | \"write\";"
	/// );
	/// ```
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
	///
	/// Emits two statements: `export const {const_name} = {"key": "value",
	/// ...} as const;` followed by `export type {type_name} = (typeof
	/// {const_name})[keyof typeof {const_name}];` — the common TypeScript
	/// idiom for a runtime-inspectable enum-like object (iterable, usable
	/// in a `for...of`) whose value type is still narrowed to the literal
	/// union of its values, unlike [`Self::export_string_enum`], which
	/// only produces the type with no matching runtime object.
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
	///
	/// No validation at all — unlike every other method here, `content`
	/// is written into generated output byte-for-byte, including its
	/// name if it declares one. Reach for the named methods above
	/// whenever they cover the shape needed; use this only for content
	/// the caller already controls, never for end-user or
	/// request-derived data.
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
///
/// Accumulates one flat list of [`TypeDef`]s while a [`Type::collect_type_defs`]/
/// [`Type::collect_type_defs_for`] call walks a type's shape — a nested
/// type's own `collect_type_defs*` call receives the same registry, so a
/// deeply nested type contributes its definitions exactly once regardless
/// of how many times it is reached, checked by [`Self::try_define`]'s
/// key-based dedup. Note what this registry does *not* check: two
/// definitions under different keys that happen to share the same
/// [`TypeDef::name`] are not rejected here (identity is `key`-scoped,
/// not `name`-scoped) — a broader uniqueness check over the exported
/// TypeScript name happens later, at the point definitions are actually
/// registered onto an app's declarations.
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
	///
	/// `true` when `def` was genuinely new; `false` when an
	/// identical definition under the same key was already present
	/// (harmless — the caller can stop recursing into that type's own
	/// dependencies, since they were already collected the first time).
	/// A *different* definition under the same key is [`Error`].
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
///
/// The four methods split into two pairs: [`Self::type_ref`]/[`Self::collect_type_defs`]
/// are the phase-agnostic originals (kept for types with only one shape —
/// every primitive impl in this module implements only these two, via the
/// default [`Self::type_ref_for`]/[`Self::collect_type_defs_for`] that
/// simply ignore the phase); [`Self::type_ref_for`]/[`Self::collect_type_defs_for`]
/// are the phase-aware overrides the `TsGen` derive actually generates,
/// since a struct's serialize and deserialize shapes commonly differ (see
/// [`TypePhase`]). Implement the `_for` pair directly when a type
/// genuinely has two shapes; implementing only the phase-agnostic pair is
/// sufficient — and correct — for anything whose shape never varies by
/// phase.
///
/// A named type sharing one exported TypeScript identifier across both
/// phases (the common case for a hand-written or `TsGen`-derived struct)
/// may be *reached* through either phase anywhere in an app's declared
/// routes — used as a route input in one place and a route output in
/// another, the ordinary shape of a type like a shared `User` accepted as
/// a partial-update input and returned as a full-record output. The two
/// phases register under different [`TypeDef`] keys but the same exported
/// name; when the two phases' rendered shapes agree, the pair resolves to
/// one emitted TypeScript type rather than a registration conflict. When
/// they genuinely disagree (the natural case: a `#[serde(default)]` field
/// is optional on Deserialize but required on Serialize), the app compile
/// step still rejects it — with an error naming the fix (declare two
/// distinct Rust types, one per phase) rather than a generic name
/// collision. A type genuinely reached at only one phase is unaffected
/// either way. See [`crate::contracts::TypeDef::classify_shared_name_with`]
/// for exactly how "agree" is decided and where this resolution actually
/// runs (an app-graph-compile concern, downstream of anything in this
/// module) — a hand-written `Type` implementation opts in by keying its
/// two phases with the same convention the `TsGen` derive uses (see that
/// method's docs).
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

	/*
	Rider coverage for P013 finding 2 (`tsgen-shared-type-phase-name-collision`
	ticket, consumed by packet P019): `#[serde(transparent)]` structs and
	`#[serde(default = "path")]` had zero test coverage repo-wide before
	these two tests — the derive parses and accepts both, but nothing
	asserted the resulting `Type` implementation's shape was actually
	correct.
	*/

	#[derive(Clone, Debug, Default, serde::Deserialize, serde::Serialize, vorma_macros::TsGen)]
	#[serde(transparent)]
	struct TransparentScore {
		value: i64,
	}

	#[test]
	fn transparent_struct_derives_an_alias_to_its_one_field() {
		let mut serialize_registry = TypeRegistry::default();
		let serialize_ref = <TransparentScore as Type>::collect_type_defs_for(
			TypePhase::Serialize,
			&mut serialize_registry,
		)
		.map(|()| <TransparentScore as Type>::type_ref_for(TypePhase::Serialize))
		.unwrap();

		let mut deserialize_registry = TypeRegistry::default();
		let deserialize_ref = <TransparentScore as Type>::collect_type_defs_for(
			TypePhase::Deserialize,
			&mut deserialize_registry,
		)
		.map(|()| <TransparentScore as Type>::type_ref_for(TypePhase::Deserialize))
		.unwrap();

		assert_eq!(
			serialize_ref,
			TypeRef::Named {
				key: "vorma_contract::tsgen::tests::TransparentScore::serialize".to_owned(),
				name: "TransparentScore".to_owned(),
			}
		);
		let serialize_defs = serialize_registry.into_defs();
		assert_eq!(serialize_defs.len(), 1);
		assert_eq!(
			serialize_defs[0],
			TypeDef::Alias {
				key: "vorma_contract::tsgen::tests::TransparentScore::serialize".to_owned(),
				name: "TransparentScore".to_owned(),
				target: TypeRef::Integer,
			},
			"a transparent struct's Serialize-phase definition must alias \
			 its one field's own TypeScript type, matching how serde \
			 serializes it as if the wrapper were not there"
		);

		assert_eq!(
			deserialize_ref,
			TypeRef::Named {
				key: "vorma_contract::tsgen::tests::TransparentScore::deserialize".to_owned(),
				name: "TransparentScore".to_owned(),
			}
		);
		let deserialize_defs = deserialize_registry.into_defs();
		assert_eq!(deserialize_defs.len(), 1);
		assert_eq!(
			deserialize_defs[0],
			TypeDef::Alias {
				key: "vorma_contract::tsgen::tests::TransparentScore::deserialize".to_owned(),
				name: "TransparentScore".to_owned(),
				target: TypeRef::Integer,
			},
			"a transparent struct's Deserialize-phase definition must also \
			 alias its one field's own type, matching serde: a bare \
			 transparent struct with no per-field defaults never diverges \
			 by phase, unlike the #[serde(default = \"path\")] case below"
		);
	}

	fn record_with_default_path_field_default_score() -> i64 {
		42
	}

	#[derive(Clone, Debug, Default, serde::Deserialize, serde::Serialize, vorma_macros::TsGen)]
	struct RecordWithDefaultPathField {
		#[serde(default = "record_with_default_path_field_default_score")]
		score: i64,
	}

	#[test]
	fn serde_default_with_explicit_path_makes_the_field_deserialize_optional_only() {
		let mut serialize_registry = TypeRegistry::default();
		<RecordWithDefaultPathField as Type>::collect_type_defs_for(
			TypePhase::Serialize,
			&mut serialize_registry,
		)
		.unwrap();
		let serialize_defs = serialize_registry.into_defs();

		let mut deserialize_registry = TypeRegistry::default();
		<RecordWithDefaultPathField as Type>::collect_type_defs_for(
			TypePhase::Deserialize,
			&mut deserialize_registry,
		)
		.unwrap();
		let deserialize_defs = deserialize_registry.into_defs();

		let TypeDef::Record {
			fields: serialize_fields,
			..
		} = &serialize_defs[0]
		else {
			panic!("expected a record definition");
		};
		assert_eq!(serialize_fields.len(), 1);
		assert_eq!(serialize_fields[0].name(), "score");
		assert!(
			!serialize_fields[0].is_optional(),
			"#[serde(default = \"path\")] must not affect Serialize-phase \
			 optionality — the field is always actually sent"
		);

		let TypeDef::Record {
			fields: deserialize_fields,
			..
		} = &deserialize_defs[0]
		else {
			panic!("expected a record definition");
		};
		assert_eq!(deserialize_fields.len(), 1);
		assert_eq!(deserialize_fields[0].name(), "score");
		assert!(
			deserialize_fields[0].is_optional(),
			"#[serde(default = \"path\")] (the explicit-function form, not \
			 bare #[serde(default)]) must still make the field \
			 Deserialize-phase optional, matching what the field actually \
			 guarantees a caller: the client may omit it and \
			 record_with_default_path_field_default_score() fills it in"
		);

		assert_eq!(
			serialize_defs[0].classify_shared_name_with(&deserialize_defs[0]),
			crate::contracts::SharedTypeNameRelation::DivergentPhaseShape,
			"a #[serde(default = \"path\")] field is exactly the case P019's \
			 ruling says must NOT collapse: the two phases genuinely differ \
			 (required on output, optional on input)"
		);
	}
}
