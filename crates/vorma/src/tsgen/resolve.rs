use std::collections::{BTreeMap, BTreeSet};

use super::identifier::validate_ts_identifier;
use super::{Error, RawTsPart, Result, TypeDef, TypeRef};
use crate::tsgen::render_type_ref;

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

	pub(crate) fn names_by_key(&self) -> &BTreeMap<String, String> {
		&self.names_by_key
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

#[cfg(test)]
mod tests {
	use super::*;
	use crate::tsgen::{FieldDef, TypeDef, TypeRef, render_types};

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
