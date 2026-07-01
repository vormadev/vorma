//! Typed raw resource response body support.

use std::sync::{Arc, Mutex};

use bytes::Bytes;
use http::header::{CONTENT_TYPE, HeaderName, HeaderValue};
use serde::Serialize;
use vorma_contract::wire::RESOURCE_BODY_HEADER;

use crate::execution_engine::{HandlerExecutionError, HandlerOutput};
use crate::response_finalizer::ResponseEffects;

const RESOURCE_BODY_HEADER_NAME: HeaderName = HeaderName::from_static(RESOURCE_BODY_HEADER);

/// Raw bytes returned by a resource handler.
pub struct ResourceBody {
	content_type: HeaderValue,
	body: Bytes,
}

impl ResourceBody {
	/// Create a raw resource response body with an explicit content type.
	pub fn new(content_type: HeaderValue, body: impl Into<Bytes>) -> Self {
		Self {
			content_type,
			body: body.into(),
		}
	}

	/// Response body content type.
	pub fn content_type(&self) -> &HeaderValue {
		&self.content_type
	}

	/// Response body bytes.
	pub fn body(&self) -> &Bytes {
		&self.body
	}
}

impl crate::tsgen::Type for ResourceBody {
	fn type_ref() -> crate::tsgen::TypeRef {
		crate::tsgen::TypeRef::Blob
	}
}

/// Resource handler output type.
pub trait ResourceOutput: sealed::ResourceOutputSealed + Send + Sync + 'static {}

impl<O> ResourceOutput for O where O: Serialize + Send + Sync + 'static {}

impl ResourceOutput for ResourceBody {}

pub(crate) fn resource_output_with_effects<O>(
	output: O,
	effects: &Arc<Mutex<ResponseEffects>>,
) -> Result<HandlerOutput, HandlerExecutionError>
where
	O: ResourceOutput,
{
	output.into_handler_output(effects)
}

mod sealed {
	use super::*;

	pub trait ResourceOutputSealed {
		fn into_handler_output(
			self,
			effects: &Arc<Mutex<ResponseEffects>>,
		) -> Result<HandlerOutput, HandlerExecutionError>;
	}

	impl<O> ResourceOutputSealed for O
	where
		O: Serialize,
	{
		fn into_handler_output(
			self,
			effects: &Arc<Mutex<ResponseEffects>>,
		) -> Result<HandlerOutput, HandlerExecutionError> {
			let data = serde_json::to_value(self).map_err(|error| {
				HandlerExecutionError::new(format!("serialize route output: {error}"))
			})?;
			let effects = effects
				.lock()
				.expect("typed handler response effects lock poisoned")
				.clone();
			Ok(HandlerOutput::data(data).with_effects(effects))
		}
	}

	impl ResourceOutputSealed for ResourceBody {
		fn into_handler_output(
			self,
			effects: &Arc<Mutex<ResponseEffects>>,
		) -> Result<HandlerOutput, HandlerExecutionError> {
			let mut effects = effects
				.lock()
				.expect("typed handler response effects lock poisoned")
				.clone();
			effects.set_header(CONTENT_TYPE, self.content_type);
			effects.set_header(RESOURCE_BODY_HEADER_NAME, HeaderValue::from_static("1"));
			Ok(HandlerOutput::body(self.body).with_effects(effects))
		}
	}
}

#[cfg(test)]
mod tests {
	use super::*;
	use crate::response_finalizer::finalize_response;

	#[test]
	fn resource_body_output_marks_response_for_blob_decode() {
		let effects = Arc::new(Mutex::new(ResponseEffects::default()));

		let output = resource_output_with_effects(
			ResourceBody::new(HeaderValue::from_static("text/plain"), b"raw".to_vec()),
			&effects,
		)
		.unwrap();
		let response = finalize_response(
			output.body_value().expect("resource body").clone(),
			output.effects(),
			"build-id",
		)
		.unwrap();

		assert_eq!(response.body().as_ref(), b"raw");
		assert_eq!(response.headers()[CONTENT_TYPE], "text/plain");
		assert_eq!(response.headers()[RESOURCE_BODY_HEADER_NAME], "1");
	}
}
