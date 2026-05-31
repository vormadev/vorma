use std::collections::{BTreeMap, BTreeSet};

use serde_json::{Map, Value};

use crate::tsgen::{self, Type, TypeDef, TypePhase, TypeRef};

pub fn schema_for_type<T: Type>() -> Result<Value, String> {
	let mut registry = tsgen::TypeRegistry::default();
	T::collect_type_defs_for(TypePhase::Deserialize, &mut registry)
		.map_err(|err| err.to_string())?;
	let defs = registry
		.into_defs()
		.into_iter()
		.map(|def| (def.key().to_owned(), def))
		.collect::<BTreeMap<_, _>>();
	type_ref_to_schema(
		&T::type_ref_for(TypePhase::Deserialize),
		&defs,
		&mut BTreeSet::new(),
	)
	.ok_or_else(|| "type cannot be represented as URL search params".to_owned())
}

fn type_ref_to_schema(
	type_ref: &TypeRef,
	defs: &BTreeMap<String, TypeDef>,
	stack: &mut BTreeSet<String>,
) -> Option<Value> {
	match type_ref {
		TypeRef::Unit => Some(Value::Object(Map::new())),
		TypeRef::Bool => Some(Value::String("b".to_owned())),
		TypeRef::String | TypeRef::StringLiteral(_) => Some(Value::String("s".to_owned())),
		TypeRef::Number | TypeRef::Integer => Some(Value::String("n".to_owned())),
		TypeRef::Nullable(inner) => optional_schema(type_ref_to_schema(inner, defs, stack)?),
		TypeRef::Array(inner) => Some(Value::Array(vec![type_ref_to_scalar_schema(
			inner, defs, stack,
		)?])),
		TypeRef::Map(key, value) => {
			if !type_ref_is_string_key(key, defs, stack) {
				return None;
			}
			Some(Value::Array(vec![
				Value::String("*".to_owned()),
				type_ref_to_map_value_schema(value, defs, stack)?,
			]))
		}
		TypeRef::Named { key, .. } => named_type_to_schema(key, defs, stack),
		TypeRef::Null | TypeRef::Unknown | TypeRef::Union(_) | TypeRef::Raw(_) => None,
	}
}

fn optional_schema(schema: Value) -> Option<Value> {
	match schema {
		Value::String(value) => Some(Value::String(format!("?{value}"))),
		Value::Array(_) | Value::Object(_) => Some(schema),
		_ => None,
	}
}

fn named_type_to_schema(
	key: &str,
	defs: &BTreeMap<String, TypeDef>,
	stack: &mut BTreeSet<String>,
) -> Option<Value> {
	if !stack.insert(key.to_owned()) {
		return None;
	}
	let result = match defs.get(key)? {
		TypeDef::Alias { target, .. } => type_ref_to_schema(target, defs, stack),
		TypeDef::Record { fields, .. } => {
			let mut object = Map::new();
			for field in fields {
				let mut schema = type_ref_to_schema(field.type_ref(), defs, stack)?;
				if field.is_optional() {
					schema = optional_schema(schema)?;
				}
				object.insert(field.name().to_owned(), schema);
			}
			Some(Value::Object(object))
		}
		TypeDef::StringEnum { .. } => Some(Value::String("s".to_owned())),
		TypeDef::Raw { .. } => None,
	};
	stack.remove(key);
	result
}

fn type_ref_to_scalar_schema(
	type_ref: &TypeRef,
	defs: &BTreeMap<String, TypeDef>,
	stack: &mut BTreeSet<String>,
) -> Option<Value> {
	let schema = type_ref_to_schema(type_ref, defs, stack)?;
	if schema.is_string() {
		return Some(schema);
	}
	None
}

fn type_ref_to_map_value_schema(
	type_ref: &TypeRef,
	defs: &BTreeMap<String, TypeDef>,
	stack: &mut BTreeSet<String>,
) -> Option<Value> {
	let schema = type_ref_to_schema(type_ref, defs, stack)?;
	if schema.is_object() {
		return None;
	}
	Some(schema)
}

fn type_ref_is_string_key(
	type_ref: &TypeRef,
	defs: &BTreeMap<String, TypeDef>,
	stack: &mut BTreeSet<String>,
) -> bool {
	match type_ref {
		TypeRef::String | TypeRef::StringLiteral(_) => true,
		TypeRef::Named { key, .. } => {
			if !stack.insert(key.to_owned()) {
				return false;
			}
			let result = match defs.get(key) {
				Some(TypeDef::Alias { target, .. }) => type_ref_is_string_key(target, defs, stack),
				Some(TypeDef::StringEnum { .. }) => true,
				_ => false,
			};
			stack.remove(key);
			result
		}
		TypeRef::Unit
		| TypeRef::Null
		| TypeRef::Unknown
		| TypeRef::Bool
		| TypeRef::Number
		| TypeRef::Integer
		| TypeRef::Array(_)
		| TypeRef::Map(_, _)
		| TypeRef::Nullable(_)
		| TypeRef::Union(_)
		| TypeRef::Raw(_) => {
			let _ = stack;
			false
		}
	}
}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::tsgen::{Result as TsResult, TypeRegistry};

	struct MapWithAliasKey;

	impl Type for MapWithAliasKey {
		fn type_ref() -> TypeRef {
			TypeRef::map(TypeRef::named("KeyAlias"), TypeRef::String)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::alias("KeyAlias", TypeRef::String));
			Ok(())
		}
	}

	struct MapWithCyclicAliasKey;

	impl Type for MapWithCyclicAliasKey {
		fn type_ref() -> TypeRef {
			TypeRef::map(TypeRef::named("KeyA"), TypeRef::String)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::alias("KeyA", TypeRef::named("KeyB")));
			registry.define(TypeDef::alias("KeyB", TypeRef::named("KeyA")));
			Ok(())
		}
	}

	#[test]
	fn map_key_alias_to_string_is_search_param_schema() {
		assert_eq!(
			schema_for_type::<MapWithAliasKey>().unwrap(),
			serde_json::json!(["*", "s"])
		);
	}

	#[test]
	fn map_key_alias_cycles_fail_instead_of_recursing() {
		assert_eq!(
			schema_for_type::<MapWithCyclicAliasKey>().unwrap_err(),
			"type cannot be represented as URL search params"
		);
	}
}
