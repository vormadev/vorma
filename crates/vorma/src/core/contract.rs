use serde_json::Value;

use super::runtime::runtime_routes_for;
use super::{Middlewares, ResourceKind, Resources, Views};
use crate::tsgen::{TypeDef, TypeRef, TypeRegistry};

#[derive(Clone, Debug, Eq, PartialEq)]
#[doc(hidden)]
pub struct ResourceEntry {
	pub method: String,
	pub pattern: String,
	pub kind: Option<ResourceKind>,
	pub input: TypeRef,
	pub output: TypeRef,
}

#[derive(Clone, Debug, Eq, PartialEq)]
#[doc(hidden)]
pub struct ViewEntry {
	pub pattern: String,
	pub client_file: String,
	pub input: TypeRef,
	pub output: TypeRef,
	pub search_schema: Value,
}

#[derive(Clone, Debug, Default, Eq, PartialEq)]
#[doc(hidden)]
pub struct Contract {
	resources: Vec<ResourceEntry>,
	views: Vec<ViewEntry>,
	type_defs: Vec<TypeDef>,
}

impl Contract {
	pub fn resources(&self) -> &[ResourceEntry] {
		&self.resources
	}

	pub fn views(&self) -> &[ViewEntry] {
		&self.views
	}

	pub fn type_defs(&self) -> &[TypeDef] {
		&self.type_defs
	}
}

#[derive(Default)]
pub(super) struct RouteCollector {
	pub(super) resources: Vec<ResourceEntry>,
	pub(super) views: Vec<ViewEntry>,
	pub(super) types: TypeRegistry,
}

impl RouteCollector {
	fn resolve(self) -> Contract {
		Contract {
			resources: self.resources,
			views: self.views,
			type_defs: self.types.into_defs(),
		}
	}
}

#[doc(hidden)]
pub fn contract_for<S, E>(
	views: &Views<S, E>,
	resources: &Resources<S, E>,
) -> Result<Contract, String>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	validate_route_contract(views, resources)?;
	let mut collector = RouteCollector::default();
	views.register_contract(&mut collector)?;
	resources.register_contract(&mut collector)?;
	Ok(collector.resolve())
}

fn validate_route_contract<S, E>(
	views: &Views<S, E>,
	resources: &Resources<S, E>,
) -> Result<(), String>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	let middlewares = Middlewares::new();
	runtime_routes_for(views, resources, &middlewares, "/api/")
		.map(|_| ())
		.map_err(|err| format!("error validating app route contract: {err}"))
}
