//! Route input decoding and validation.

use std::collections::BTreeMap;

use http::Method;
use serde_json::{Number, Value};
use url::form_urlencoded;

use crate::contracts::{FieldDef, TypeDef, TypeRefContract};
use crate::execution_engine::RequestInput;
use crate::form_data::{FormData, FormDataDecodeError, decode_form_data};
use crate::response_finalizer::VORMA_JSON_QUERY_KEY;

/// Decoded route handler input value.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DecodedRouteInput {
	value: Value,
	form_data: Option<FormData>,
}

impl DecodedRouteInput {
	/// Create decoded route input from a validated value.
	pub fn new(value: Value) -> Self {
		Self {
			value,
			form_data: None,
		}
	}

	/// Create decoded route input from parsed form data.
	pub fn form_data(form_data: FormData) -> Self {
		Self {
			value: Value::Null,
			form_data: Some(form_data),
		}
	}

	/// Decoded input value.
	pub fn value(&self) -> &Value {
		&self.value
	}

	/// Parsed form data, when the route input contract is `FormData`.
	pub fn parsed_form_data(&self) -> Option<&FormData> {
		self.form_data.as_ref()
	}
}

impl Default for DecodedRouteInput {
	fn default() -> Self {
		Self::new(Value::Null)
	}
}

/// Decode a view input from URL search params after removing framework-internal query keys.
pub fn decode_view_input(
	request: &RequestInput,
	input_type: &TypeRefContract,
	type_defs: &[TypeDef],
) -> Result<DecodedRouteInput, InputDecodeError> {
	decode_query_input(
		request.query().unwrap_or_default(),
		input_type,
		type_defs,
		QueryDecodeMode::StripInternalViewKeys,
	)
}

/// Decode a resource input from query params for GET/HEAD or JSON body for other methods.
pub async fn decode_resource_input(
	request: &RequestInput,
	input_type: &TypeRefContract,
	type_defs: &[TypeDef],
) -> Result<DecodedRouteInput, InputDecodeError> {
	if matches!(input_type, TypeRefContract::FormData) {
		let form_data = decode_form_data(
			request.method(),
			request.query(),
			request.headers(),
			request.body(),
		)
		.await
		.map_err(|source| InputDecodeError::FormData { source })?;
		return Ok(DecodedRouteInput::form_data(form_data));
	}
	if *request.method() == Method::GET || *request.method() == Method::HEAD {
		return decode_query_input(
			request.query().unwrap_or_default(),
			input_type,
			type_defs,
			QueryDecodeMode::PreserveAllKeys,
		);
	}
	if matches!(input_type, TypeRefContract::Unit) && request.body().is_empty() {
		return Ok(DecodedRouteInput::default());
	}
	let value = if request.body().is_empty() {
		Value::Null
	} else {
		serde_json::from_slice(request.body()).map_err(|error| {
			InputDecodeError::InvalidJsonBody {
				message: error.to_string(),
			}
		})?
	};
	validate_json_value(&value, input_type, type_defs)?;
	Ok(DecodedRouteInput::new(value))
}

/// Input decoding error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum InputDecodeError {
	/// JSON request body could not be decoded.
	InvalidJsonBody {
		/// JSON parser error message.
		message: String,
	},
	/// Query parameter failed scalar coercion.
	InvalidQueryValue {
		/// Query key.
		key: String,
		/// Rejected value.
		value: String,
		/// Expected type description.
		expected: &'static str,
	},
	/// Required input field was missing.
	MissingField {
		/// Missing field name.
		field: String,
	},
	/// Input value did not match its declared contract.
	TypeMismatch {
		/// JSON path or field name.
		path: String,
		/// Expected type description.
		expected: &'static str,
	},
	/// Named input type was not present in the graph type definitions.
	UnknownNamedType {
		/// Missing type key.
		key: String,
		/// Missing type name.
		name: String,
	},
	/// FormData input decoding failed.
	FormData {
		/// Source form-data decoder error.
		source: FormDataDecodeError,
	},
}

impl std::fmt::Display for InputDecodeError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		match self {
			Self::InvalidJsonBody { message } => write!(f, "error decoding JSON: {message}"),
			Self::InvalidQueryValue {
				key,
				value,
				expected,
			} => write!(
				f,
				"query parameter {key:?} value {value:?} is not {expected}"
			),
			Self::MissingField { field } => write!(f, "missing input field {field:?}"),
			Self::TypeMismatch { path, expected } => {
				write!(f, "input value at {path:?} is not {expected}")
			}
			Self::UnknownNamedType { key, name } => {
				write!(f, "input type {name:?} with key {key:?} was not declared")
			}
			Self::FormData { source } => write!(f, "{source}"),
		}
	}
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum QueryDecodeMode {
	PreserveAllKeys,
	StripInternalViewKeys,
}

fn decode_query_input(
	query: &str,
	input_type: &TypeRefContract,
	type_defs: &[TypeDef],
	mode: QueryDecodeMode,
) -> Result<DecodedRouteInput, InputDecodeError> {
	if matches!(input_type, TypeRefContract::Unit) {
		return Ok(DecodedRouteInput::default());
	}
	let query_params = query_params(query, mode);
	let value = decode_query_value("", &query_params, input_type, type_defs)?;
	Ok(DecodedRouteInput::new(value))
}

fn query_params(query: &str, mode: QueryDecodeMode) -> BTreeMap<String, Vec<String>> {
	let mut query_params = BTreeMap::<String, Vec<String>>::new();
	for (key, value) in form_urlencoded::parse(query.as_bytes()) {
		if mode == QueryDecodeMode::StripInternalViewKeys && key == VORMA_JSON_QUERY_KEY {
			continue;
		}
		query_params
			.entry(key.into_owned())
			.or_default()
			.push(value.into_owned());
	}
	query_params
}

fn decode_query_value(
	key: &str,
	query_params: &BTreeMap<String, Vec<String>>,
	input_type: &TypeRefContract,
	type_defs: &[TypeDef],
) -> Result<Value, InputDecodeError> {
	match input_type {
		TypeRefContract::Unit => Ok(Value::Null),
		TypeRefContract::Unknown | TypeRefContract::Raw(_) => Ok(raw_query_object(query_params)),
		TypeRefContract::Named {
			key: type_key,
			name,
		} => {
			let type_def = find_type_def(type_defs, type_key, name)?;
			decode_query_type_def(key, query_params, type_def, type_defs)
		}
		TypeRefContract::Array(inner) => query_params
			.get(key)
			.map(Vec::as_slice)
			.unwrap_or_default()
			.iter()
			.filter(|value| !value.is_empty())
			.map(|value| decode_query_scalar(key, value, inner, type_defs))
			.collect::<Result<Vec<_>, _>>()
			.map(Value::Array),
		TypeRefContract::Map(key_type, value_type) => {
			decode_query_map(key, query_params, key_type, value_type, type_defs)
		}
		TypeRefContract::Nullable(inner) => {
			if !query_key_present(key, query_params) {
				return Ok(Value::Null);
			}
			if query_key_has_null_marker(key, query_params)
				&& !matches!(
					inner.as_ref(),
					TypeRefContract::Array(_) | TypeRefContract::Map(_, _)
				) {
				return Ok(Value::Null);
			}
			decode_query_value(key, query_params, inner, type_defs)
		}
		TypeRefContract::Union(types) => decode_query_union(key, query_params, types, type_defs),
		_ => {
			let Some(value) = single_query_value(key, query_params) else {
				return Err(InputDecodeError::MissingField {
					field: key.to_owned(),
				});
			};
			decode_query_scalar(key, value, input_type, type_defs)
		}
	}
}

fn decode_query_type_def(
	key: &str,
	query_params: &BTreeMap<String, Vec<String>>,
	type_def: &TypeDef,
	type_defs: &[TypeDef],
) -> Result<Value, InputDecodeError> {
	match type_def {
		TypeDef::Alias { target, .. } => decode_query_value(key, query_params, target, type_defs),
		TypeDef::Record { fields, .. } => {
			let mut object = serde_json::Map::new();
			for field in fields {
				let field_key = query_child_key(key, field.name());
				if field.is_optional() && !query_key_present(&field_key, query_params) {
					continue;
				}
				let value =
					decode_query_value(&field_key, query_params, field.type_ref(), type_defs)?;
				object.insert(field.name().to_owned(), value);
			}
			Ok(Value::Object(object))
		}
		TypeDef::StringEnum { variants, .. } => {
			let Some(value) = single_query_value(key, query_params) else {
				return Err(InputDecodeError::MissingField {
					field: key.to_owned(),
				});
			};
			if variants.iter().any(|variant| variant == value) {
				return Ok(Value::String(value.to_owned()));
			}
			Err(InputDecodeError::InvalidQueryValue {
				key: key.to_owned(),
				value: value.to_owned(),
				expected: "declared string enum variant",
			})
		}
		TypeDef::Raw { .. } => Ok(raw_query_object(query_params)),
	}
}

fn decode_query_map(
	key: &str,
	query_params: &BTreeMap<String, Vec<String>>,
	key_type: &TypeRefContract,
	value_type: &TypeRefContract,
	type_defs: &[TypeDef],
) -> Result<Value, InputDecodeError> {
	let mut object = serde_json::Map::new();
	let prefix = query_prefix(key);
	for query_key in query_params.keys() {
		let Some(map_key) = map_key_for_query_key(key, &prefix, query_key) else {
			continue;
		};
		if map_key.is_empty() {
			continue;
		}
		decode_query_scalar(query_key, map_key, key_type, type_defs)?;
		let value = decode_query_value(query_key, query_params, value_type, type_defs)?;
		object.insert(map_key.to_owned(), value);
	}
	Ok(Value::Object(object))
}

fn decode_query_union(
	key: &str,
	query_params: &BTreeMap<String, Vec<String>>,
	types: &[TypeRefContract],
	type_defs: &[TypeDef],
) -> Result<Value, InputDecodeError> {
	for input_type in types {
		if let Ok(value) = decode_query_value(key, query_params, input_type, type_defs) {
			return Ok(value);
		}
	}
	Err(InputDecodeError::TypeMismatch {
		path: key.to_owned(),
		expected: "one of the declared union variants",
	})
}

fn decode_query_scalar(
	key: &str,
	value: &str,
	input_type: &TypeRefContract,
	type_defs: &[TypeDef],
) -> Result<Value, InputDecodeError> {
	match input_type {
		TypeRefContract::Null => {
			if value.is_empty() || value == "null" {
				Ok(Value::Null)
			} else {
				Err(InputDecodeError::InvalidQueryValue {
					key: key.to_owned(),
					value: value.to_owned(),
					expected: "null",
				})
			}
		}
		TypeRefContract::Bool => match value {
			"true" => Ok(Value::Bool(true)),
			"false" => Ok(Value::Bool(false)),
			_ => Err(InputDecodeError::InvalidQueryValue {
				key: key.to_owned(),
				value: value.to_owned(),
				expected: "boolean",
			}),
		},
		TypeRefContract::String | TypeRefContract::Unknown | TypeRefContract::Raw(_) => {
			Ok(Value::String(value.to_owned()))
		}
		TypeRefContract::Number => value
			.parse::<f64>()
			.ok()
			.and_then(Number::from_f64)
			.map(Value::Number)
			.ok_or_else(|| InputDecodeError::InvalidQueryValue {
				key: key.to_owned(),
				value: value.to_owned(),
				expected: "number",
			}),
		TypeRefContract::Integer => value
			.parse::<i64>()
			.map(|value| Value::Number(value.into()))
			.map_err(|_| InputDecodeError::InvalidQueryValue {
				key: key.to_owned(),
				value: value.to_owned(),
				expected: "integer",
			}),
		TypeRefContract::StringLiteral(expected) => {
			if value == expected {
				Ok(Value::String(value.to_owned()))
			} else {
				Err(InputDecodeError::InvalidQueryValue {
					key: key.to_owned(),
					value: value.to_owned(),
					expected: "declared string literal",
				})
			}
		}
		TypeRefContract::Named {
			key: type_key,
			name,
		} => {
			let type_def = find_type_def(type_defs, type_key, name)?;
			decode_query_type_def(
				key,
				&BTreeMap::from([(key.to_owned(), vec![value.to_owned()])]),
				type_def,
				type_defs,
			)
		}
		TypeRefContract::Nullable(inner) => {
			if value.is_empty() || value == "null" {
				return Ok(Value::Null);
			}
			decode_query_scalar(key, value, inner, type_defs)
		}
		TypeRefContract::Union(types) => {
			for input_type in types {
				if let Ok(value) = decode_query_scalar(key, value, input_type, type_defs) {
					return Ok(value);
				}
			}
			Err(InputDecodeError::InvalidQueryValue {
				key: key.to_owned(),
				value: value.to_owned(),
				expected: "one of the declared union variants",
			})
		}
		TypeRefContract::FormData => Err(InputDecodeError::InvalidQueryValue {
			key: key.to_owned(),
			value: value.to_owned(),
			expected: "FormData resource input",
		}),
		TypeRefContract::Unit | TypeRefContract::Array(_) | TypeRefContract::Map(_, _) => {
			Err(InputDecodeError::InvalidQueryValue {
				key: key.to_owned(),
				value: value.to_owned(),
				expected: "scalar value",
			})
		}
	}
}

fn validate_json_value(
	value: &Value,
	input_type: &TypeRefContract,
	type_defs: &[TypeDef],
) -> Result<(), InputDecodeError> {
	validate_json_value_at("$", value, input_type, type_defs)
}

fn validate_json_value_at(
	path: &str,
	value: &Value,
	input_type: &TypeRefContract,
	type_defs: &[TypeDef],
) -> Result<(), InputDecodeError> {
	match input_type {
		TypeRefContract::Unit | TypeRefContract::Null => {
			if value.is_null() {
				return Ok(());
			}
			Err(InputDecodeError::TypeMismatch {
				path: path.to_owned(),
				expected: "null",
			})
		}
		TypeRefContract::Unknown | TypeRefContract::Raw(_) => Ok(()),
		TypeRefContract::Bool => expect_json(value.is_boolean(), path, "boolean"),
		TypeRefContract::String => expect_json(value.is_string(), path, "string"),
		TypeRefContract::Number => expect_json(value.is_number(), path, "number"),
		TypeRefContract::Integer => expect_json(value.as_i64().is_some(), path, "integer"),
		TypeRefContract::Named { key, name } => {
			let type_def = find_type_def(type_defs, key, name)?;
			validate_json_type_def(path, value, type_def, type_defs)
		}
		TypeRefContract::Array(inner) => {
			let Value::Array(values) = value else {
				return Err(InputDecodeError::TypeMismatch {
					path: path.to_owned(),
					expected: "array",
				});
			};
			for (index, value) in values.iter().enumerate() {
				validate_json_value_at(&format!("{path}[{index}]"), value, inner, type_defs)?;
			}
			Ok(())
		}
		TypeRefContract::Map(key_type, value_type) => {
			let Value::Object(values) = value else {
				return Err(InputDecodeError::TypeMismatch {
					path: path.to_owned(),
					expected: "object",
				});
			};
			for (key, value) in values {
				validate_json_value_at(
					&format!("{path}.<key>"),
					&Value::String(key.clone()),
					key_type,
					type_defs,
				)?;
				validate_json_value_at(&format!("{path}.{key}"), value, value_type, type_defs)?;
			}
			Ok(())
		}
		TypeRefContract::FormData => Err(InputDecodeError::TypeMismatch {
			path: path.to_owned(),
			expected: "FormData resource input",
		}),
		TypeRefContract::Nullable(inner) => {
			if value.is_null() {
				return Ok(());
			}
			validate_json_value_at(path, value, inner, type_defs)
		}
		TypeRefContract::Union(types) => {
			if types.iter().any(|input_type| {
				validate_json_value_at(path, value, input_type, type_defs).is_ok()
			}) {
				return Ok(());
			}
			Err(InputDecodeError::TypeMismatch {
				path: path.to_owned(),
				expected: "one of the declared union variants",
			})
		}
		TypeRefContract::StringLiteral(expected) => {
			if value.as_str() == Some(expected) {
				return Ok(());
			}
			Err(InputDecodeError::TypeMismatch {
				path: path.to_owned(),
				expected: "declared string literal",
			})
		}
	}
}

fn validate_json_type_def(
	path: &str,
	value: &Value,
	type_def: &TypeDef,
	type_defs: &[TypeDef],
) -> Result<(), InputDecodeError> {
	match type_def {
		TypeDef::Alias { target, .. } => validate_json_value_at(path, value, target, type_defs),
		TypeDef::Record { fields, .. } => {
			let Value::Object(values) = value else {
				return Err(InputDecodeError::TypeMismatch {
					path: path.to_owned(),
					expected: "object",
				});
			};
			for field in fields {
				validate_json_field(path, values, field, type_defs)?;
			}
			Ok(())
		}
		TypeDef::StringEnum { variants, .. } => {
			if value
				.as_str()
				.is_some_and(|value| variants.iter().any(|variant| variant == value))
			{
				return Ok(());
			}
			Err(InputDecodeError::TypeMismatch {
				path: path.to_owned(),
				expected: "declared string enum variant",
			})
		}
		TypeDef::Raw { .. } => Ok(()),
	}
}

fn validate_json_field(
	path: &str,
	values: &serde_json::Map<String, Value>,
	field: &FieldDef,
	type_defs: &[TypeDef],
) -> Result<(), InputDecodeError> {
	let Some(value) = values.get(field.name()) else {
		if field.is_optional() {
			return Ok(());
		}
		return Err(InputDecodeError::MissingField {
			field: field.name().to_owned(),
		});
	};
	validate_json_value_at(
		&format!("{path}.{}", field.name()),
		value,
		field.type_ref(),
		type_defs,
	)
}

fn expect_json(
	condition: bool,
	path: &str,
	expected: &'static str,
) -> Result<(), InputDecodeError> {
	if condition {
		return Ok(());
	}
	Err(InputDecodeError::TypeMismatch {
		path: path.to_owned(),
		expected,
	})
}

fn single_query_value<'a>(
	key: &str,
	query_params: &'a BTreeMap<String, Vec<String>>,
) -> Option<&'a str> {
	query_params
		.get(key)
		.and_then(|values| values.first())
		.map(String::as_str)
}

fn query_child_key(parent: &str, child: &str) -> String {
	if parent.is_empty() {
		child.to_owned()
	} else {
		format!("{parent}.{child}")
	}
}

fn query_prefix(key: &str) -> String {
	if key.is_empty() {
		String::new()
	} else {
		format!("{key}.")
	}
}

fn query_key_present(key: &str, query_params: &BTreeMap<String, Vec<String>>) -> bool {
	if key.is_empty() {
		return !query_params.is_empty();
	}
	query_params.contains_key(key)
		|| query_params.keys().any(|query_key| {
			query_key
				.strip_prefix(key)
				.is_some_and(|suffix| suffix.starts_with('.'))
		})
}

fn query_key_has_null_marker(key: &str, query_params: &BTreeMap<String, Vec<String>>) -> bool {
	query_params
		.get(key)
		.is_some_and(|values| values.first().is_some_and(String::is_empty))
}

fn map_key_for_query_key<'a>(key: &str, prefix: &str, query_key: &'a str) -> Option<&'a str> {
	if key.is_empty() {
		return Some(query_key);
	}
	query_key.strip_prefix(prefix)
}

fn raw_query_object(query_params: &BTreeMap<String, Vec<String>>) -> Value {
	let mut object = serde_json::Map::new();
	for (key, values) in query_params {
		let value = match values.as_slice() {
			[] => Value::Null,
			[value] => Value::String(value.clone()),
			values => Value::Array(values.iter().cloned().map(Value::String).collect()),
		};
		object.insert(key.clone(), value);
	}
	Value::Object(object)
}

fn find_type_def<'a>(
	type_defs: &'a [TypeDef],
	key: &str,
	name: &str,
) -> Result<&'a TypeDef, InputDecodeError> {
	type_defs
		.iter()
		.find(|type_def| type_def_key_and_name(type_def) == (key, name))
		.ok_or_else(|| InputDecodeError::UnknownNamedType {
			key: key.to_owned(),
			name: name.to_owned(),
		})
}

fn type_def_key_and_name(type_def: &TypeDef) -> (&str, &str) {
	match type_def {
		TypeDef::Alias { key, name, .. }
		| TypeDef::Record { key, name, .. }
		| TypeDef::StringEnum { key, name, .. }
		| TypeDef::Raw { key, name, .. } => (key, name),
	}
}

#[cfg(test)]
mod tests {
	use bytes::Bytes;

	use super::*;
	use crate::contracts::FieldDef;

	fn named(key: &str, name: &str) -> TypeRefContract {
		TypeRefContract::Named {
			key: key.to_owned(),
			name: name.to_owned(),
		}
	}

	fn search_type_defs() -> Vec<TypeDef> {
		vec![TypeDef::Record {
			key: "Search".to_owned(),
			name: "Search".to_owned(),
			fields: vec![
				FieldDef::new("q", TypeRefContract::String, false),
				FieldDef::new("page", TypeRefContract::Integer, false),
				FieldDef::new(
					"tag",
					TypeRefContract::Array(Box::new(TypeRefContract::String)),
					true,
				),
			],
		}]
	}

	fn nested_search_type_defs() -> Vec<TypeDef> {
		vec![
			TypeDef::Record {
				key: "Search".to_owned(),
				name: "Search".to_owned(),
				fields: vec![
					FieldDef::new("q", TypeRefContract::String, false),
					FieldDef::new("page", TypeRefContract::Integer, false),
				],
			},
			TypeDef::Record {
				key: "SearchEnvelope".to_owned(),
				name: "SearchEnvelope".to_owned(),
				fields: vec![FieldDef::new("filter", named("Search", "Search"), false)],
			},
		]
	}

	#[test]
	fn view_input_decodes_query_and_strips_internal_key() {
		let request = RequestInput::new(Method::GET, "/").with_query(format!(
			"q=ada&page=2&tag=rust&tag=vorma&{VORMA_JSON_QUERY_KEY}=build"
		));

		let decoded =
			decode_view_input(&request, &named("Search", "Search"), &search_type_defs()).unwrap();

		assert_eq!(
			decoded.value(),
			&serde_json::json!({"q": "ada", "page": 2, "tag": ["rust", "vorma"]})
		);
	}

	#[test]
	fn query_unit_input_ignores_query_values() {
		let request = RequestInput::new(Method::GET, "/").with_query("q=ada");

		let decoded = decode_view_input(&request, &TypeRefContract::Unit, &[]).unwrap();

		assert_eq!(decoded.value(), &Value::Null);
	}

	#[test]
	fn query_input_rejects_invalid_integer() {
		let request = RequestInput::new(Method::GET, "/").with_query("q=ada&page=nope");

		let error = decode_view_input(&request, &named("Search", "Search"), &search_type_defs())
			.unwrap_err();

		assert!(matches!(error, InputDecodeError::InvalidQueryValue { .. }));
	}

	#[test]
	fn query_input_decodes_present_top_level_nullable_record() {
		let request = RequestInput::new(Method::GET, "/").with_query("q=ada&page=2");

		let decoded = decode_view_input(
			&request,
			&TypeRefContract::Nullable(Box::new(named("Search", "Search"))),
			&search_type_defs(),
		)
		.unwrap();

		assert_eq!(decoded.value(), &serde_json::json!({"q": "ada", "page": 2}));
	}

	#[test]
	fn query_input_decodes_missing_top_level_nullable_record_as_null() {
		let request = RequestInput::new(Method::GET, "/");

		let decoded = decode_view_input(
			&request,
			&TypeRefContract::Nullable(Box::new(named("Search", "Search"))),
			&search_type_defs(),
		)
		.unwrap();

		assert_eq!(decoded.value(), &Value::Null);
	}

	#[test]
	fn query_input_decodes_nested_records_with_dotted_paths() {
		let request = RequestInput::new(Method::GET, "/").with_query("filter.q=ada&filter.page=2");

		let decoded = decode_view_input(
			&request,
			&named("SearchEnvelope", "SearchEnvelope"),
			&nested_search_type_defs(),
		)
		.unwrap();

		assert_eq!(
			decoded.value(),
			&serde_json::json!({"filter": {"q": "ada", "page": 2}})
		);
	}

	#[test]
	fn query_input_decodes_maps_from_dotted_entries_without_catching_sibling_prefixes() {
		let request = RequestInput::new(Method::GET, "/")
			.with_query("attrs.color=red&attrs.size=large&attribute=ignored");
		let type_defs = vec![TypeDef::Record {
			key: "AttrsInput".to_owned(),
			name: "AttrsInput".to_owned(),
			fields: vec![FieldDef::new(
				"attrs",
				TypeRefContract::Map(
					Box::new(TypeRefContract::String),
					Box::new(TypeRefContract::String),
				),
				false,
			)],
		}];

		let decoded =
			decode_view_input(&request, &named("AttrsInput", "AttrsInput"), &type_defs).unwrap();

		assert_eq!(
			decoded.value(),
			&serde_json::json!({"attrs": {"color": "red", "size": "large"}})
		);
	}

	#[test]
	fn query_input_decodes_empty_map_marker_as_empty_object() {
		let request = RequestInput::new(Method::GET, "/").with_query("attrs=");
		let type_defs = vec![TypeDef::Record {
			key: "AttrsInput".to_owned(),
			name: "AttrsInput".to_owned(),
			fields: vec![FieldDef::new(
				"attrs",
				TypeRefContract::Map(
					Box::new(TypeRefContract::String),
					Box::new(TypeRefContract::String),
				),
				false,
			)],
		}];

		let decoded =
			decode_view_input(&request, &named("AttrsInput", "AttrsInput"), &type_defs).unwrap();

		assert_eq!(decoded.value(), &serde_json::json!({"attrs": {}}));
	}

	#[test]
	fn query_input_decodes_empty_array_marker_as_empty_array() {
		let request = RequestInput::new(Method::GET, "/").with_query("tag=");
		let type_defs = vec![TypeDef::Record {
			key: "TagsInput".to_owned(),
			name: "TagsInput".to_owned(),
			fields: vec![FieldDef::new(
				"tag",
				TypeRefContract::Array(Box::new(TypeRefContract::String)),
				false,
			)],
		}];

		let decoded =
			decode_view_input(&request, &named("TagsInput", "TagsInput"), &type_defs).unwrap();

		assert_eq!(decoded.value(), &serde_json::json!({"tag": []}));
	}

	#[tokio::test]
	async fn resource_non_get_decodes_and_validates_json_body() {
		let type_defs = vec![TypeDef::Record {
			key: "CreateUser".to_owned(),
			name: "CreateUser".to_owned(),
			fields: vec![
				FieldDef::new("email", TypeRefContract::String, false),
				FieldDef::new("count", TypeRefContract::Integer, false),
			],
		}];
		let request = RequestInput::new(Method::POST, "/api/users").with_body(Bytes::from_static(
			br#"{"email":"a@example.com","count":2}"#,
		));

		let decoded =
			decode_resource_input(&request, &named("CreateUser", "CreateUser"), &type_defs)
				.await
				.unwrap();

		assert_eq!(
			decoded.value(),
			&serde_json::json!({"email": "a@example.com", "count": 2})
		);
	}

	#[tokio::test]
	async fn resource_non_get_rejects_invalid_json_body() {
		let request =
			RequestInput::new(Method::POST, "/api/users").with_body(Bytes::from_static(b"{"));

		let error = decode_resource_input(&request, &TypeRefContract::Unknown, &[])
			.await
			.unwrap_err();

		assert!(matches!(error, InputDecodeError::InvalidJsonBody { .. }));
	}

	#[tokio::test]
	async fn resource_non_get_rejects_missing_required_json_field() {
		let type_defs = vec![TypeDef::Record {
			key: "CreateUser".to_owned(),
			name: "CreateUser".to_owned(),
			fields: vec![FieldDef::new("email", TypeRefContract::String, false)],
		}];
		let request = RequestInput::new(Method::POST, "/api/users")
			.with_body(Bytes::from_static(br#"{"name":"Ada"}"#));

		let error = decode_resource_input(&request, &named("CreateUser", "CreateUser"), &type_defs)
			.await
			.unwrap_err();

		assert!(matches!(error, InputDecodeError::MissingField { .. }));
	}

	#[tokio::test]
	async fn resource_non_get_validates_json_map_keys() {
		let request = RequestInput::new(Method::POST, "/api/map")
			.with_body(Bytes::from_static(br#"{"other":1}"#));

		let error = decode_resource_input(
			&request,
			&TypeRefContract::Map(
				Box::new(TypeRefContract::StringLiteral("known".to_owned())),
				Box::new(TypeRefContract::Integer),
			),
			&[],
		)
		.await
		.unwrap_err();

		assert!(matches!(error, InputDecodeError::TypeMismatch { .. }));
	}

	#[tokio::test]
	async fn resource_decodes_form_data_query_and_urlencoded_body() {
		let get = RequestInput::new(Method::GET, "/api/form").with_query("tag=a&tag=b");
		let mut headers = http::HeaderMap::new();
		headers.insert(
			http::header::CONTENT_TYPE,
			http::HeaderValue::from_static("application/x-www-form-urlencoded"),
		);
		let post = RequestInput::new(Method::POST, "/api/form")
			.with_headers(headers)
			.with_body(Bytes::from_static(b"name=ada"));

		let get_decoded = decode_resource_input(&get, &TypeRefContract::FormData, &[])
			.await
			.unwrap();
		let post_decoded = decode_resource_input(&post, &TypeRefContract::FormData, &[])
			.await
			.unwrap();

		assert_eq!(
			get_decoded
				.parsed_form_data()
				.unwrap()
				.texts("tag")
				.collect::<Vec<_>>(),
			["a", "b"]
		);
		assert_eq!(
			post_decoded.parsed_form_data().unwrap().text("name"),
			Some("ada")
		);
	}

	#[tokio::test]
	async fn resource_form_data_rejects_missing_form_content_type() {
		let request = RequestInput::new(Method::POST, "/api/form");

		let error = decode_resource_input(&request, &TypeRefContract::FormData, &[])
			.await
			.unwrap_err();

		assert!(matches!(error, InputDecodeError::FormData { .. }));
	}

	/// Golden vectors shared with the TypeScript serializer/parser suite.
	#[derive(serde::Deserialize)]
	struct QueryContractVectors {
		vectors: Vec<QueryContractVector>,
	}

	#[derive(serde::Deserialize)]
	struct QueryContractVector {
		name: String,
		input_type: TypeRefContract,
		type_defs: Vec<TypeDef>,
		query: String,
		expected: Value,
	}

	#[test]
	fn query_input_decoding_matches_shared_contract_vectors() {
		let vectors: QueryContractVectors = serde_json::from_str(include_str!(
			"../../../packages/vorma/kit/json/search_param_contract_vectors.json"
		))
		.expect("shared contract vectors parse");
		assert!(!vectors.vectors.is_empty());
		for vector in vectors.vectors {
			let request = RequestInput::new(Method::GET, "/").with_query(&vector.query);
			let decoded = decode_view_input(&request, &vector.input_type, &vector.type_defs)
				.unwrap_or_else(|error| panic!("vector {}: decode failed: {error:?}", vector.name));
			assert_eq!(decoded.value(), &vector.expected, "vector {}", vector.name);
		}
	}
}
