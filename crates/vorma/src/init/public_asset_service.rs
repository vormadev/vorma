use bytes::Bytes;
use http::header::{CACHE_CONTROL, CONTENT_LENGTH, CONTENT_TYPE, HeaderValue};
use http::{Method, Response, StatusCode};

use crate::error::ViewErrorClientMsg;
use crate::r#static::{RuntimeAssets, path_is_under_public_static_base};

use super::{RuntimeHost, empty_response};

impl<S, E> RuntimeHost<S, E>
where
	S: Send + Sync + 'static,
	E: ViewErrorClientMsg + Send + Sync + 'static,
{
	pub(super) fn public_asset_response(
		&self,
		method: &Method,
		path: &str,
	) -> Result<Option<Response<Bytes>>, String> {
		if method != Method::GET && method != Method::HEAD {
			return Ok(None);
		}
		let Some(assets) = &self.assets else {
			return Ok(None);
		};
		if method == Method::HEAD {
			let Some(asset) = assets.stat_public_asset(path)? else {
				return self.public_asset_missing_response(assets, path);
			};
			return public_asset_found_response(
				Bytes::new(),
				asset.content_length,
				asset.cache_control,
				asset.content_type,
			)
			.map(Some);
		}
		let Some(asset) = assets.read_public_asset(path)? else {
			return self.public_asset_missing_response(assets, path);
		};
		let content_length = asset.bytes.len() as u64;
		public_asset_found_response(
			Bytes::from(asset.bytes),
			content_length,
			asset.cache_control,
			asset.content_type,
		)
		.map(Some)
	}

	fn public_asset_missing_response(
		&self,
		assets: &RuntimeAssets,
		path: &str,
	) -> Result<Option<Response<Bytes>>, String> {
		let snapshot = assets.snapshot()?;
		if path_is_under_public_static_base(snapshot.manifest(), path) {
			return empty_response(StatusCode::NOT_FOUND).map(Some);
		}
		Ok(None)
	}
}

fn public_asset_found_response(
	body: Bytes,
	content_length: u64,
	cache_control: &'static str,
	content_type: String,
) -> Result<Response<Bytes>, String> {
	let mut response = Response::new(body);
	response
		.headers_mut()
		.insert(CACHE_CONTROL, HeaderValue::from_static(cache_control));
	response.headers_mut().insert(
		CONTENT_TYPE,
		HeaderValue::from_str(&content_type).map_err(|err| err.to_string())?,
	);
	response.headers_mut().insert(
		CONTENT_LENGTH,
		HeaderValue::from_str(&content_length.to_string()).map_err(|err| err.to_string())?,
	);
	Ok(response)
}
