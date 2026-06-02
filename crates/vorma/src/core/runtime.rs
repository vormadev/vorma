use crate::constants::{
	DYNAMIC_PARAM_PREFIX, EXPLICIT_INDEX_SEGMENT_IDENTIFIER, SPLAT_SEGMENT_IDENTIFIER,
};
use crate::mux::{self, NestedOptions, NestedRouter, Options};

use super::{Middlewares, Resources, Views};

pub(crate) struct RuntimeRoutes<S, E = Box<dyn std::error::Error + Send + Sync>> {
	pub(crate) resources: mux::Router<S, E>,
	pub(crate) views: mux::NestedRouter<S, E>,
}

impl<S, E> RuntimeRoutes<S, E>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	fn new(api_base: &str) -> Result<Self, mux::Error> {
		let resources = mux::Router::new(Options {
			mount_root: api_base.to_string(),
			dynamic_param_prefix: DYNAMIC_PARAM_PREFIX,
			splat_segment_identifier: SPLAT_SEGMENT_IDENTIFIER,
		})?;
		let views = NestedRouter::new(NestedOptions {
			dynamic_param_prefix: DYNAMIC_PARAM_PREFIX,
			splat_segment_identifier: SPLAT_SEGMENT_IDENTIFIER,
			explicit_index_segment_identifier: EXPLICIT_INDEX_SEGMENT_IDENTIFIER.to_string(),
		})?;
		Ok(Self { resources, views })
	}
}

pub(crate) fn runtime_routes_for<S, E>(
	views: &Views<S, E>,
	resources: &Resources<S, E>,
	middlewares: &Middlewares<S, E>,
	api_base: &str,
) -> Result<RuntimeRoutes<S, E>, mux::Error>
where
	S: Send + Sync + 'static,
	E: Send + Sync + 'static,
{
	let mut routes = RuntimeRoutes::new(api_base)?;
	views.register_runtime(&mut routes)?;
	resources.register_runtime(&mut routes)?;
	middlewares.register_runtime(&mut routes)?;
	Ok(routes)
}
