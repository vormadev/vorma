use bytes::Bytes;
use http::{Response, StatusCode};
use std::sync::Arc;

use crate::document::DocumentBuildCtx;
use crate::error::ViewErrorClientMsg;
use crate::handler::{
	ViewResponseResultsInput, build_view_response_from_results, build_view_skew_response,
	view_not_found_response,
};
use crate::mux::RawRequest;
use crate::response::internal_server_error_with_client_build_id;

use super::{RequestExecCtx, RuntimeHost, empty_response};

impl<S, E> RuntimeHost<S, E>
where
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
{
	pub(super) async fn handle_view_request(
		&self,
		request: RawRequest,
	) -> Result<Response<Bytes>, String> {
		let Some(assets) = &self.assets else {
			return empty_response(StatusCode::NOT_FOUND);
		};
		let snapshot = assets.snapshot()?;
		match build_view_skew_response(request.uri(), snapshot.client_build_id()) {
			Ok(Some(response)) => return Ok(response),
			Ok(Option::None) => {}
			Err(_) => {
				return internal_server_error_with_client_build_id(snapshot.client_build_id());
			}
		}
		let match_results = match self.routes.views.find_nested_matches(request.path()) {
			Ok(Some(match_results)) => match_results,
			Ok(Option::None) => return view_not_found_response(snapshot.client_build_id()),
			Err(_) => {
				return internal_server_error_with_client_build_id(snapshot.client_build_id());
			}
		};
		let RequestExecCtx {
			exec_ctx,
			_cancel_on_drop,
		} = self.request_exec_ctx();
		let public_filemap = Arc::new(snapshot.manifest().public_filemap.clone());
		let document = self.document.build(DocumentBuildCtx::manifest(
			snapshot.manifest_handle(),
			request.clone(),
		));
		let view_report = self.routes.views.execute_view_matches(
			self.state.clone(),
			exec_ctx,
			request.clone(),
			match_results,
			public_filemap,
		);
		let (document, view_report) = match tokio::try_join!(document, async {
			view_report.await.map_err(|err| err.to_string())
		}) {
			Ok(results) => results,
			Err(_) => {
				return internal_server_error_with_client_build_id(snapshot.client_build_id());
			}
		};
		match build_view_response_from_results(ViewResponseResultsInput {
			expected_client_build_id: snapshot.client_build_id(),
			request: &request,
			manifest: snapshot.manifest(),
			document: &document,
			view_report: &view_report,
		}) {
			Ok(response) => Ok(response),
			Err(_) => internal_server_error_with_client_build_id(snapshot.client_build_id()),
		}
	}
}
