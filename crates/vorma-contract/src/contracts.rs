//! Framework graph contract facts for generated TypeScript and document rendering.
//!
//! Framework-integration surface, with one app-facing exception: this
//! module owns [`DocumentElementContract`](crate::contracts::DocumentElementContract),
//! the validated HTML element shape underneath `vorma`'s higher-level
//! `Document`/head-builder API (see [`crate::document_renderer`] for the
//! rendering rules it enforces and the standing ticket
//! `contract-borrowed-element-construction` for the one recorded design
//! question about its owned-construction API). Everything else here —
//! [`RouteTypeContract`](crate::contracts::RouteTypeContract),
//! [`TypeRefContract`](crate::contracts::TypeRefContract),
//! [`TypeDef`](crate::contracts::TypeDef),
//! [`FieldDef`](crate::contracts::FieldDef),
//! [`DocumentContract`](crate::contracts::DocumentContract),
//! [`DocumentAttributeContract`](crate::contracts::DocumentAttributeContract) —
//! is the serializable shape [`crate::framework_graph`] carries between
//! the build crate and the runtime crate; an application author reaches
//! these only through the [`crate::tsgen`] model that renders type
//! references and definitions into TypeScript.

use std::collections::BTreeMap;

use serde::{Deserialize, Serialize};

/// Render a serializable value as tab-indented pretty JSON.
///
/// Every generated JSON artifact in this crate (committed manifests, JSON
/// literals embedded in generated TypeScript) goes through this instead
/// of `serde_json`'s default two-space `PrettyFormatter`, so the bytes
/// this crate produces already match what the repo's own formatter would
/// produce — regenerating a committed artifact never fights the
/// formatter on indentation alone.
///
/// ```
/// use vorma_contract::contracts::to_tab_indented_json_string;
///
/// let rendered = to_tab_indented_json_string(&serde_json::json!({"a": 1})).unwrap();
/// assert_eq!(rendered, "{\n\t\"a\": 1\n}");
/// ```
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
///
/// Every declared view and resource carries one of these: the
/// [`TypeRefContract`] its handler decodes input from and the one its
/// handler's output serializes as, as reported by the
/// [`crate::tsgen::Type`] implementations the `TsGen` derive generates.
/// [`crate::framework_graph`] validates that both references resolve to
/// an actually-declared [`TypeDef`] (or a built-in primitive) before a
/// graph compiles.
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
///
/// How one Rust type's shape is described at a use site — a field type, a
/// route's input/output — as opposed to [`TypeDef`], which is where a
/// *named* type's shape is actually declared once. `Named` is the bridge
/// between the two: it is a reference by identity, not an inline
/// definition, and [`crate::framework_graph`] rejects a graph where a
/// `Named` reference's `key`/`name` do not match an actually-declared
/// `TypeDef`. The `TsGen` derive (`vorma-macros`) is the primary producer
/// of these; reach for the constructor methods below only when hand-authoring
/// a [`crate::tsgen::Type`] implementation.
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
		///
		/// Compared for equality when checking whether two `Named`
		/// references point at the same [`TypeDef`]; never rendered into
		/// generated TypeScript. The `TsGen` derive builds this from the
		/// Rust type's fully-qualified module path plus its
		/// serialize/deserialize phase, so two distinct Rust types can
		/// never collide even if they happen to share a display `name`.
		key: String,
		/// Preferred TypeScript type name.
		///
		/// The identifier actually written into generated TypeScript.
		/// Unlike `key`, this is what a [`crate::framework_graph::GraphError::DuplicateTypeName`]
		/// check guards: two different `key`s are never allowed to render
		/// the same exported `name`.
		name: String,
	},
	/// `Array<T>`.
	Array(Box<TypeRefContract>),
	/// `Record<K, V>`.
	Map(Box<TypeRefContract>, Box<TypeRefContract>),
	/// Platform `FormData`.
	FormData,
	/// Platform `Blob`.
	Blob,
	/// `T | null`.
	Nullable(Box<TypeRefContract>),
	/// `A | B | ...`.
	Union(Vec<TypeRefContract>),
	/// String literal type.
	StringLiteral(String),
	/// Trusted raw TypeScript type expression.
	///
	/// An escape hatch for a shape the other variants cannot express
	/// (a TypeScript-only construct with no Rust equivalent). The
	/// "trusted" in the name means exactly that: nothing validates or
	/// escapes `Raw` text before it is written into generated output, so
	/// its content must be TypeScript source the caller already controls,
	/// never end-user or request-derived data.
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
///
/// Splits a [`TypeRefContract::Raw`] expression into trusted literal text
/// and nested type references that still get resolved and validated
/// normally — so a hand-authored raw expression can still name a
/// framework-declared type inside it rather than needing to spell that
/// type's own generated name as a text literal.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub enum RawTsPart {
	/// Literal TypeScript text.
	Text(String),
	/// Nested TypeScript type reference rendered through the resolver.
	TypeRef(TypeRefContract),
}

/// Relationship between two [`TypeDef`]s that export the same TypeScript name.
///
/// Returned by [`TypeDef::classify_shared_name_with`] — see that method's
/// docs for exactly how each variant is decided.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum SharedTypeNameRelation {
	/// The same declared Rust type's Serialize-phase and Deserialize-phase
	/// definitions render an equivalent TypeScript shape — the pair
	/// collapses into one emitted definition under the shared name.
	SamePhaseShape,
	/// The same declared Rust type's two phases render different
	/// TypeScript shapes (the natural case: a `#[serde(default)]` field is
	/// optional on Deserialize but required on Serialize) — a genuine
	/// authoring error, not a name to collapse.
	DivergentPhaseShape,
	/// Not recognizable as two phases of one declared type at all — two
	/// unrelated `TypeDef`s are contending for one exported name.
	UnrelatedTypes,
}

/// TypeScript definition emitted into generated contracts.
///
/// While [`TypeRefContract`] is how a type is *referenced* at a use site,
/// `TypeDef` is where a named type's shape is declared exactly once. A
/// graph carries a flat list of these ([`crate::framework_graph::FrameworkGraph::type_defs`]);
/// [`crate::framework_graph`] always rejects a graph containing two
/// definitions with the same `key` (identity collision). Two definitions
/// with the same exported `name` but different `key`s are rejected too,
/// with one deliberate exception: [`Self::classify_shared_name_with`]
/// recognizes when both are the Serialize-phase and Deserialize-phase
/// `TypeDef`s of one `TsGen`-derived Rust type (see [`crate::tsgen::TypePhase`]),
/// in which case a shape-equivalent pair collapses into one emitted
/// definition rather than erroring — see that method's docs for exactly
/// what "the same declared type" and "shape-equivalent" mean.
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

	/// Classify the relationship between two `TypeDef`s that export the
	/// same TypeScript `name` under different `key`s.
	///
	/// Every `TsGen`-derived Rust type registers its Serialize-phase and
	/// Deserialize-phase shapes under two distinct keys built as
	/// `{module_path}::{TypeIdent}::serialize`/`{module_path}::{TypeIdent}::deserialize`
	/// (see `vorma-macros`' derive), so two `TypeDef`s are recognized as
	/// **the same declared Rust type, opposite phase** exactly when
	/// stripping one of those two suffixes from each key leaves the same
	/// remaining string — never merely because their rendered shapes
	/// happen to coincide. A hand-written [`crate::tsgen::Type`]
	/// implementation that wants a shared-name collision resolved this way
	/// must key its two phases with this same convention; a phase-agnostic
	/// hand-written key (no such suffix) never participates and any
	/// collision it has with another `TypeDef` is always
	/// [`SharedTypeNameRelation::UnrelatedTypes`].
	///
	/// Once recognized as the same declared type, the two definitions are
	/// compared for **shape equivalence**: identical in every respect
	/// except that a nested [`TypeRefContract::Named`] reference compares
	/// only by its `name` field, never its `key` — a nested `TsGen`-derived
	/// field's own key legitimately differs by phase even when the outer
	/// type is genuinely shared, so comparing nested keys literally would
	/// reject the common case of a shared type containing a shared nested
	/// type. Field/variant order matters (skipping a field from only one
	/// phase is itself a structural difference, not noise to ignore).
	/// `crate::graph_validation` (a private module) is the sole caller
	/// once graph compile wires it through the registration/uniqueness
	/// boundary — an app author reaches this only indirectly, through the
	/// resulting [`crate::framework_graph::GraphError::DuplicateTypeName`]/`DivergentTypePhaseShapes`
	/// outcomes.
	///
	/// ```
	/// use vorma_contract::contracts::{FieldDef, SharedTypeNameRelation, TypeDef, TypeRefContract};
	///
	/// let serialize_phase = TypeDef::record_with_key(
	///     "app::model::User::serialize",
	///     "User",
	///     vec![FieldDef::new("name", TypeRefContract::String, false)],
	/// );
	/// let deserialize_phase = TypeDef::record_with_key(
	///     "app::model::User::deserialize",
	///     "User",
	///     vec![FieldDef::new("name", TypeRefContract::String, false)],
	/// );
	/// let unrelated = TypeDef::record_with_key(
	///     "app::other::Account",
	///     "User",
	///     vec![FieldDef::new("name", TypeRefContract::String, false)],
	/// );
	///
	/// assert_eq!(
	///     serialize_phase.classify_shared_name_with(&deserialize_phase),
	///     SharedTypeNameRelation::SamePhaseShape
	/// );
	/// assert_eq!(
	///     serialize_phase.classify_shared_name_with(&unrelated),
	///     SharedTypeNameRelation::UnrelatedTypes
	/// );
	/// ```
	pub fn classify_shared_name_with(&self, other: &Self) -> SharedTypeNameRelation {
		match phase_independent_key(self.key()).zip(phase_independent_key(other.key())) {
			Some((self_base, other_base)) if self_base == other_base => {
				if type_defs_have_equivalent_shape(self, other) {
					SharedTypeNameRelation::SamePhaseShape
				} else {
					SharedTypeNameRelation::DivergentPhaseShape
				}
			}
			_ => SharedTypeNameRelation::UnrelatedTypes,
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
	///
	/// ```
	/// use vorma_contract::contracts::{FieldDef, TypeDef};
	///
	/// let user = TypeDef::record("User", vec![FieldDef::required::<String>("name")]);
	///
	/// assert_eq!(user.key(), "User");
	/// assert_eq!(user.name(), "User");
	/// ```
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

/*
The `TsGen` derive (`vorma-macros`) mints every stable key as
`{module_path}::{TypeIdent}::serialize` or
`{module_path}::{TypeIdent}::deserialize` — these two suffixes are the
entire contract [`TypeDef::classify_shared_name_with`] relies on to
recognize "same declared Rust type, opposite phase" without ever
mistaking two unrelated types that merely coincide in rendered shape.
The derive's own key literals cannot reference these constants directly
(`concat!` only accepts literal tokens, not `const` paths), so the
literals below are a documented, tested contract with `ts_gen_derive.rs`
rather than a shared Rust symbol; the shared-type dedup tests exercise the
real derive output against this stripping logic end to end, which catches
drift between the two more reliably than a standalone string-equality
assertion would.
*/
pub(crate) const SERIALIZE_PHASE_KEY_SUFFIX: &str = "::serialize";
pub(crate) const DESERIALIZE_PHASE_KEY_SUFFIX: &str = "::deserialize";

/// Strip a recognized phase suffix from a `TypeDef` key, returning the
/// key that identifies the declared type independent of which phase
/// registered it — or `None` when `key` carries neither suffix (a
/// phase-agnostic hand-written key, which never participates in phase
/// pairing).
fn phase_independent_key(key: &str) -> Option<&str> {
	key.strip_suffix(SERIALIZE_PHASE_KEY_SUFFIX)
		.or_else(|| key.strip_suffix(DESERIALIZE_PHASE_KEY_SUFFIX))
}

/// Whether `key` carries the `TsGen` derive's Serialize-phase key suffix.
///
/// [`crate::graph_validation`] uses this only to order a divergent phase
/// pair's keys for [`crate::framework_graph::GraphError::DivergentTypePhaseShapes`]'s
/// named fields — the identity test itself is
/// [`TypeDef::classify_shared_name_with`], not this.
pub(crate) fn is_serialize_phase_key(key: &str) -> bool {
	key.ends_with(SERIALIZE_PHASE_KEY_SUFFIX)
}

/// Whether two `TypeDef`s render an equivalent TypeScript shape, treating
/// a nested [`TypeRefContract::Named`] reference as equivalent whenever
/// its `name` matches — never comparing nested `key`s, which legitimately
/// differ by phase even for a genuinely shared nested type. Field/variant
/// order still matters (unlike unordered-set comparison, which would blur
/// a skip-only-one-phase divergence into false equivalence).
fn type_defs_have_equivalent_shape(a: &TypeDef, b: &TypeDef) -> bool {
	match (a, b) {
		(TypeDef::Alias { target: a, .. }, TypeDef::Alias { target: b, .. }) => {
			type_refs_have_equivalent_shape(a, b)
		}
		(TypeDef::Record { fields: a, .. }, TypeDef::Record { fields: b, .. }) => {
			a.len() == b.len()
				&& a.iter().zip(b).all(|(a, b)| {
					a.name == b.name
						&& a.optional == b.optional
						&& type_refs_have_equivalent_shape(&a.type_ref, &b.type_ref)
				})
		}
		(TypeDef::StringEnum { variants: a, .. }, TypeDef::StringEnum { variants: b, .. }) => {
			a == b
		}
		(TypeDef::Raw { body: a, .. }, TypeDef::Raw { body: b, .. }) => {
			a.len() == b.len()
				&& a.iter().zip(b).all(|(a, b)| match (a, b) {
					(RawTsPart::Text(a), RawTsPart::Text(b)) => a == b,
					(RawTsPart::TypeRef(a), RawTsPart::TypeRef(b)) => {
						type_refs_have_equivalent_shape(a, b)
					}
					(RawTsPart::Text(_), RawTsPart::TypeRef(_))
					| (RawTsPart::TypeRef(_), RawTsPart::Text(_)) => false,
				})
		}
		// A different `TypeDef` shape at the same key-pair is never
		// reachable through the `TsGen` derive (every phase of one
		// invocation emits one consistent variant), but a hand-written
		// `Type` implementation could still mint mismatched variants
		// under a phase-suffixed key pair; treated as divergent, not a
		// panic or a silent guess.
		_ => false,
	}
}

/// Whether two `TypeRefContract`s render an equivalent TypeScript shape,
/// recursing structurally and comparing a nested [`TypeRefContract::Named`]
/// only by `name` (see [`type_defs_have_equivalent_shape`]).
fn type_refs_have_equivalent_shape(a: &TypeRefContract, b: &TypeRefContract) -> bool {
	match (a, b) {
		(TypeRefContract::Named { name: a, .. }, TypeRefContract::Named { name: b, .. }) => a == b,
		(TypeRefContract::Array(a), TypeRefContract::Array(b))
		| (TypeRefContract::Nullable(a), TypeRefContract::Nullable(b)) => {
			type_refs_have_equivalent_shape(a, b)
		}
		(TypeRefContract::Map(a_key, a_value), TypeRefContract::Map(b_key, b_value)) => {
			type_refs_have_equivalent_shape(a_key, b_key)
				&& type_refs_have_equivalent_shape(a_value, b_value)
		}
		(TypeRefContract::Union(a), TypeRefContract::Union(b)) => {
			a.len() == b.len()
				&& a.iter()
					.zip(b)
					.all(|(a, b)| type_refs_have_equivalent_shape(a, b))
		}
		(TypeRefContract::Raw(a), TypeRefContract::Raw(b)) => {
			a.len() == b.len()
				&& a.iter().zip(b).all(|(a, b)| match (a, b) {
					(RawTsPart::Text(a), RawTsPart::Text(b)) => a == b,
					(RawTsPart::TypeRef(a), RawTsPart::TypeRef(b)) => {
						type_refs_have_equivalent_shape(a, b)
					}
					(RawTsPart::Text(_), RawTsPart::TypeRef(_))
					| (RawTsPart::TypeRef(_), RawTsPart::Text(_)) => false,
				})
		}
		(TypeRefContract::StringLiteral(a), TypeRefContract::StringLiteral(b)) => a == b,
		(
			TypeRefContract::Unit
			| TypeRefContract::Null
			| TypeRefContract::Unknown
			| TypeRefContract::Bool
			| TypeRefContract::String
			| TypeRefContract::Number
			| TypeRefContract::Integer
			| TypeRefContract::FormData
			| TypeRefContract::Blob,
			_,
		) => a == b,
		_ => false,
	}
}

/// Field definition for a TypeScript record contract.
///
/// One named field of a [`TypeDef::Record`], carrying the field's TypeScript
/// type and whether it is optional (`field?: T` vs `field: T`). The
/// `required`/`optional` constructors below take a Rust type parameter
/// `T: `[`crate::tsgen::Type`] and derive the field's [`TypeRefContract`]
/// from it, which is how the `TsGen` derive builds one `FieldDef` per
/// struct field without hand-writing a `TypeRefContract` at every call
/// site.
#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
pub struct FieldDef {
	name: String,
	type_ref: TypeRefContract,
	optional: bool,
}

impl FieldDef {
	/// Required field using `T`'s normal TypeScript shape.
	///
	/// ```
	/// use vorma_contract::contracts::FieldDef;
	///
	/// let field = FieldDef::required::<String>("name");
	///
	/// assert_eq!(field.name(), "name");
	/// assert!(!field.is_optional());
	/// ```
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
///
/// The build-time-fixed part of an app's document shell: attributes on
/// the root `<html>`/`<body>` elements, default head elements every page
/// starts with, the dedupe-rule shapes used to merge per-view head
/// effects on top of those defaults (see
/// [`Self::head_dedupe_rules`]), and body-prefix markup rendered right
/// after `<body ...>` opens. Framework-integration surface: app code
/// builds one of these through `vorma`'s `Document`/`DocumentBuilder` API,
/// not by constructing a `DocumentContract` directly.
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
	///
	/// Each element here is a *shape*, not literal content: a matched
	/// route's chain of nested views can each contribute head elements
	/// (a `<title>`, an Open Graph `<meta>`), and when more than one
	/// contribution matches the same rule shape (for example, "any
	/// `<title>` element" or "a `<meta name="description">` regardless of
	/// its `content` value"), the innermost view's contribution wins and
	/// replaces the outer one rather than both being rendered. A handful
	/// of common rules (title, description, viewport, robots, charset,
	/// icon, canonical, Open Graph tags) apply automatically; these are
	/// additional app-declared rules layered on top.
	pub fn head_dedupe_rules(&self) -> &[DocumentElementContract] {
		&self.head_dedupe_rules
	}

	/// Body prefix elements.
	pub fn body_prefix(&self) -> &[DocumentElementContract] {
		&self.body_prefix
	}
}

/// Document attribute contract.
///
/// One name/value attribute on the root `<html>` or `<body>` element (see
/// [`DocumentContract::html_attributes`]/[`DocumentContract::body_attributes`]).
/// Carries the same trust/boolean distinctions as
/// [`DocumentElementContract`]'s own attributes, at the single-attribute
/// granularity the document shell's root elements need since they are not
/// full [`DocumentElementContract`] values themselves.
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
	///
	/// Ignored when [`Self::boolean`] is `true`: a boolean attribute
	/// renders as its bare name with no `="value"` at all (`<body
	/// hidden>`, never `<body hidden="">`), matching how HTML itself
	/// treats boolean attributes.
	pub fn value(&self) -> &str {
		&self.value
	}

	/// Whether the value is already trusted HTML.
	///
	/// `false` (the common case) means [`Self::value`] is escaped before
	/// rendering, exactly like [`DocumentElementContract::attributes`].
	/// `true` skips escaping — reach for it only when the value is
	/// already known-safe framework-produced content, never end-user or
	/// request-derived data.
	pub fn known_safe(&self) -> bool {
		self.known_safe
	}

	/// Whether this is a boolean attribute.
	pub fn boolean(&self) -> bool {
		self.boolean
	}
}

/// Document element contract.
///
/// The validated, build-a-builder-then-render shape one HTML element
/// (`<title>`, `<meta>`, `<link>`, `<style>`, a body-prefix element) takes
/// on its way to [`crate::document_renderer::render_document`] or
/// [`crate::document_renderer::render_document_element`], which are the
/// only paths that turn one of these into actual HTML — construction here
/// never escapes or validates anything itself. Framework-integration
/// surface with app-facing docs: `vorma`'s head-builder API constructs
/// these on an application's behalf from a smaller, more ergonomic
/// surface, but understanding the escaped-vs-trusted split below is what
/// that builder's own contract rests on. Two independent trust axes:
///
/// - **Attributes**: [`Self::with_attributes`] values are HTML-escaped at
///   render time; [`Self::with_known_safe_attributes`] values are not.
///   Reach for the known-safe path only for framework-produced content
///   that is already guaranteed free of `"` and `<`/`>`/`&`, never for
///   end-user or request-derived strings.
/// - **Content**: [`Self::with_text_content`] is escaped;
///   [`Self::with_dangerous_inner_html`] is not — the name says so on
///   purpose. A `<style>` element's dangerous inner HTML additionally
///   gets its own escaping pass for a `</style>` end-tag boundary (see
///   [`crate::document_renderer`]), since raw CSS can legitimately
///   contain the literal text `</style>` inside a string value.
///
/// See the standing ticket `contract-borrowed-element-construction` for
/// the recorded design question about this type's owned-only builder API
/// (every `with_*` method here takes and returns an owned collection,
/// which means per-request dynamic elements pay a clone to reuse
/// already-owned attribute data).
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
	///
	/// ```
	/// use vorma_contract::contracts::DocumentElementContract;
	/// use vorma_contract::document_renderer::render_document_element;
	///
	/// let element = DocumentElementContract::new("meta")
	///     .with_attributes([("name".to_owned(), "vorma".to_owned())].into())
	///     .with_self_closing(true);
	///
	/// assert_eq!(
	///     render_document_element(&element).unwrap(),
	///     "<meta name=\"vorma\" />"
	/// );
	/// ```
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
	///
	/// Elements on HTML's own void-element list (`meta`, `link`, `img`,
	/// `br`, and similar) already render self-closing regardless of this
	/// setting — see [`crate::document_renderer`]. This exists for
	/// elements outside that fixed list that should still render without
	/// a body.
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
