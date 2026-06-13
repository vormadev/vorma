//! Public route input marker traits and graph contract resolvers.

use serde::de::DeserializeOwned;
use serde_json::Value;

use crate::contracts::{RouteTypeContract, TypeDef, TypeRefContract};
use crate::form_data::FormData;
use crate::search_params;
use crate::tsgen::{Type, TypePhase, TypeRegistry};

/// Function pointer that resolves a Rust type into a TypeScript route contract.
pub type TypeResolver = fn(TypePhase, &mut TypeRegistry) -> Result<TypeRefContract, String>;

/// Function pointer that resolves a view input into a search-parameter schema.
pub type SearchSchemaResolver = fn() -> Result<Value, String>;

/// Marker trait for view inputs accepted by public route declarations.
pub trait ViewInput: DeserializeOwned + Type + Send + Sync + 'static {}

impl<I> ViewInput for I where I: DeserializeOwned + Type + Send + Sync + 'static {}

/// Marker trait for resource inputs accepted by public route declarations.
pub trait ResourceInput: Send + Sync + 'static {}

impl<I> ResourceInput for I where I: DeserializeOwned + Type + Send + Sync + 'static {}

impl ResourceInput for FormData {}

/// Route type contract plus the named type definitions it requires.
pub struct RouteContractFacts {
	/// Input/output type contract for the route.
	pub type_contract: RouteTypeContract,
	/// Named type definitions collected while resolving the contract.
	pub type_defs: Vec<TypeDef>,
}

/// Assemble one route's type contract from its input/output resolvers.
pub fn route_contract_from_resolvers(
	input_type: TypeResolver,
	output_type: TypeResolver,
) -> Result<RouteContractFacts, String> {
	let mut registry = TypeRegistry::default();
	let input = input_type(TypePhase::Deserialize, &mut registry)?;
	let output = output_type(TypePhase::Serialize, &mut registry)?;
	Ok(RouteContractFacts {
		type_contract: RouteTypeContract::new(input, output),
		type_defs: registry.into_defs(),
	})
}

/// Resolve the route type reference and collect required named type definitions.
pub fn type_resolver<T>(
	phase: TypePhase,
	registry: &mut TypeRegistry,
) -> Result<TypeRefContract, String>
where
	T: Type,
{
	T::collect_type_defs_for(phase, registry).map_err(|error| error.to_string())?;
	Ok(T::type_ref_for(phase))
}

/// Resolve a public view input search schema.
pub fn search_schema_resolver<T>() -> Result<Value, String>
where
	T: Type,
{
	search_params::schema_for_type::<T>()
}

#[cfg(test)]
mod tests {
	use serde::Deserialize;

	use super::*;
	use crate::tsgen::{FieldDef, Result as TsResult, TypeDef, TypeRef};

	const TEST_INPUT_TYPE_NAME: &str = "SearchInput";

	#[derive(Deserialize)]
	struct SearchInput;

	impl Type for SearchInput {
		fn type_ref() -> TypeRef {
			TypeRef::named(TEST_INPUT_TYPE_NAME)
		}

		fn collect_type_defs(registry: &mut TypeRegistry) -> TsResult<()> {
			registry.define(TypeDef::record(
				TEST_INPUT_TYPE_NAME,
				vec![FieldDef::required::<String>("q")],
			));
			Ok(())
		}
	}

	fn accepts_view_input<I: ViewInput>() {}

	fn accepts_resource_input<I: ResourceInput>() {}

	#[test]
	fn route_input_traits_accept_serde_and_type_inputs_plus_form_data_resources() {
		accepts_view_input::<SearchInput>();
		accepts_resource_input::<SearchInput>();
		accepts_resource_input::<FormData>();
	}

	#[test]
	fn type_and_search_resolvers_collect_contract_facts() {
		let mut registry = TypeRegistry::default();
		let type_ref = type_resolver::<SearchInput>(TypePhase::Deserialize, &mut registry).unwrap();
		let search_schema = search_schema_resolver::<SearchInput>().unwrap();

		assert_eq!(type_ref, TypeRef::named(TEST_INPUT_TYPE_NAME));
		assert_eq!(registry.into_defs()[0].name(), TEST_INPUT_TYPE_NAME);
		assert_eq!(search_schema, serde_json::json!({"q": "s"}));
	}
}
