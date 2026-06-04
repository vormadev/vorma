use std::collections::BTreeMap;

use super::{FieldDef, RawTsPart, ResolvedTypeDef, ResolvedTypes, TypeDef, TypeRef};

#[doc(hidden)]
pub fn render_types(resolved: &ResolvedTypes) -> String {
	let mut out = String::new();
	for (index, resolved_def) in resolved.defs().iter().enumerate() {
		if index > 0 {
			out.push('\n');
		}
		push_type_def(&mut out, resolved_def, resolved.names_by_key());
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
