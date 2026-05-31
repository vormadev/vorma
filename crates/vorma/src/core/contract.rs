use serde_json::Value;

use super::runtime::runtime_routes_for;
use super::{ApiRouteKind, ApiRoutes, TaskMiddlewares, Views};
use crate::tsgen::{TypeDef, TypeRef, TypeRegistry};

#[derive(Clone, Debug, Eq, PartialEq)]
#[doc(hidden)]
pub struct ApiRouteEntry {
	pub method: String,
	pub pattern: String,
	pub kind: Option<ApiRouteKind>,
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
	api_routes: Vec<ApiRouteEntry>,
	views: Vec<ViewEntry>,
	type_defs: Vec<TypeDef>,
}

impl Contract {
	pub fn api_routes(&self) -> &[ApiRouteEntry] {
		&self.api_routes
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
	pub(super) api_routes: Vec<ApiRouteEntry>,
	pub(super) views: Vec<ViewEntry>,
	pub(super) types: TypeRegistry,
}

impl RouteCollector {
	fn resolve(self) -> Contract {
		Contract {
			api_routes: self.api_routes,
			views: self.views,
			type_defs: self.types.into_defs(),
		}
	}
}

#[doc(hidden)]
pub fn contract_for<S, E>(
	views: &Views<S, E>,
	api_routes: &ApiRoutes<S, E>,
) -> Result<Contract, String>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	validate_route_contract(views, api_routes)?;
	let mut collector = RouteCollector::default();
	views.register_contract(&mut collector)?;
	api_routes.register_contract(&mut collector)?;
	Ok(collector.resolve())
}

fn validate_route_contract<S, E>(
	views: &Views<S, E>,
	api_routes: &ApiRoutes<S, E>,
) -> Result<(), String>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	let task_middlewares = TaskMiddlewares::new();
	runtime_routes_for(views, api_routes, &task_middlewares, "/api/")
		.map(|_| ())
		.map_err(|err| format!("error validating app route contract: {err}"))
}
