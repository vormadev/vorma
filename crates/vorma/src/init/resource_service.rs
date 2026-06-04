use std::collections::BTreeMap;
use std::sync::Arc;

use bytes::Bytes;
use http::{Response, StatusCode};

use crate::error::ViewErrorClientMsg;
use crate::handler::{ApiResponseInput, build_api_response};
use crate::mux::RawRequest;
use crate::response::{internal_server_error_with_client_build_id, response_with_client_build_id};

use super::{RequestExecCtx, RuntimeHost, empty_response, method_not_allowed_response};

impl<S, E> RuntimeHost<S, E>
where
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
{
	pub(super) async fn handle_api_request(
		&self,
		request: RawRequest,
	) -> Result<Response<Bytes>, String> {
		if !self.routes.resources.method_is_allowed(request.method()) {
			let allow = self.api_allow_header();
			if !allow.is_empty() {
				return method_not_allowed_response(&allow);
			}
		}
		let snapshot = self
			.assets
			.as_ref()
			.map(crate::r#static::RuntimeAssets::snapshot)
			.transpose()?;
		let client_build_id = snapshot
			.as_ref()
			.map(|snapshot| snapshot.client_build_id().to_owned())
			.unwrap_or_default();
		let public_filemap = Arc::new(
			snapshot
				.as_ref()
				.map(|snapshot| snapshot.manifest().public_filemap.clone())
				.unwrap_or_else(BTreeMap::new),
		);
		let raw_path = request.path().to_owned();
		let RequestExecCtx {
			exec_ctx,
			_cancel_on_drop,
		} = self.request_exec_ctx();
		let result = match self
			.routes
			.resources
			.execute_route(request, self.state.clone(), exec_ctx, public_filemap)
			.await
		{
			Ok(result) => result,
			Err(_) => return internal_server_error_with_client_build_id(&client_build_id),
		};
		let Some(result) = result else {
			if let Some(allow) = self.api_allow_header_for_path(&raw_path) {
				let response = method_not_allowed_response(&allow)?;
				return response_with_client_build_id(response, &client_build_id);
			}
			let response = empty_response(StatusCode::NOT_FOUND)?;
			return response_with_client_build_id(response, &client_build_id);
		};
		match build_api_response(ApiResponseInput {
			expected_client_build_id: &client_build_id,
			result: &result,
		}) {
			Ok(response) => Ok(response),
			Err(_) => internal_server_error_with_client_build_id(&client_build_id),
		}
	}

	fn api_allow_header_for_path(&self, path: &str) -> Option<String> {
		let methods = self.routes.resources.allowed_methods_for_path(path);
		if methods.is_empty() {
			return None;
		}
		Some(super::allow_header_value(&methods))
	}

	fn api_allow_header(&self) -> String {
		super::allow_header_value(&self.routes.resources.allowed_methods())
	}
}
