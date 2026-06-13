//! Projection from resource execution reports into HTTP responses.

use bytes::Bytes;
use http::Response;
use http::header::{CONTENT_TYPE, HeaderValue};
use serde_json::Value;

use crate::execution_engine::{HandlerRole, RouteExecutionReport};
use crate::response_finalizer::{
	FinalizerError, JSON_CONTENT_TYPE, finalize_response, finalize_terminal_response,
	suppress_response_body_preserving_content_length,
};

/// Finalize a resource execution report into an HTTP response.
pub fn finalize_resource_report(
	report: &RouteExecutionReport,
	client_build_id: &str,
) -> Result<Response<Bytes>, ResourceResponseError> {
	if report.effects().is_terminal() {
		/*
		Resources are a JSON API: terminal ERRORS become the JSON error
		envelope the TS client parses. Terminal redirects keep the plain
		redirect response.
		*/
		let response = if report.effects().is_terminal_error() {
			finalize_resource_error_response(report.effects(), client_build_id)
				.map_err(|source| ResourceResponseError::Finalizer { source })?
		} else {
			finalize_terminal_response(report.effects(), client_build_id)
				.map_err(|source| ResourceResponseError::Finalizer { source })?
		};
		return if report.suppress_body() {
			suppress_response_body_preserving_content_length(response)
				.map_err(|source| ResourceResponseError::Finalizer { source })
		} else {
			Ok(response)
		};
	}
	let output_body = report
		.committed()
		.iter()
		.rev()
		.find(|commit| commit.role() == HandlerRole::Resource)
		.map(|commit| resource_output_body(commit.output()))
		.transpose()?
		.unwrap_or_default();
	let mut response = finalize_response(output_body.bytes, report.effects(), client_build_id)
		.map_err(|source| ResourceResponseError::Finalizer { source })?;
	if output_body.json && !response.headers().contains_key(CONTENT_TYPE) {
		response
			.headers_mut()
			.insert(CONTENT_TYPE, HeaderValue::from_static(JSON_CONTENT_TYPE));
	}
	if report.suppress_body() {
		response = suppress_response_body_preserving_content_length(response)
			.map_err(|source| ResourceResponseError::Finalizer { source })?;
	}
	Ok(response)
}

/// Resource response projection error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum ResourceResponseError {
	/// HTTP finalization failed.
	Finalizer {
		/// Source finalizer error.
		source: FinalizerError,
	},
	/// Resource data JSON serialization failed.
	Json {
		/// JSON serialization error message.
		message: String,
	},
}

#[derive(Default)]
struct ResourceOutputBody {
	bytes: Bytes,
	json: bool,
}

fn finalize_resource_error_response(
	effects: &crate::response_finalizer::ResponseEffects,
	client_build_id: &str,
) -> Result<Response<Bytes>, crate::response_finalizer::FinalizerError> {
	let error_text = effects
		.status_text()
		.unwrap_or(crate::response_finalizer::INTERNAL_SERVER_ERROR_STATUS_TEXT);
	let body = serde_json::json!({ "error": error_text });
	let body = serde_json::to_vec(&body)
		.expect("resource error envelope serialization cannot fail for a string field");
	let mut response = finalize_response(Bytes::from(body), effects, client_build_id)?;
	response.headers_mut().insert(
		CONTENT_TYPE,
		http::HeaderValue::from_static(JSON_CONTENT_TYPE),
	);
	Ok(response)
}

fn resource_output_body(
	output: &crate::execution_engine::HandlerOutput,
) -> Result<ResourceOutputBody, ResourceResponseError> {
	if let Some(body) = output.body_value() {
		return Ok(ResourceOutputBody {
			bytes: body.clone(),
			json: false,
		});
	}
	let Some(data) = output.data_value() else {
		return Ok(ResourceOutputBody {
			bytes: Bytes::new(),
			json: false,
		});
	};
	let bytes = json_bytes(data)?;
	Ok(ResourceOutputBody { bytes, json: true })
}

fn json_bytes(value: &Value) -> Result<Bytes, ResourceResponseError> {
	serde_json::to_vec(value)
		.map(Bytes::from)
		.map_err(|error| ResourceResponseError::Json {
			message: error.to_string(),
		})
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use bytes::Bytes;
	use http::header::{CONTENT_LENGTH, CONTENT_TYPE, HeaderName, HeaderValue};
	use http::{Method, StatusCode};

	use super::*;
	use crate::asset_capabilities::AssetCapabilities;
	use crate::execution_engine::{
		ExecutionEngine, HandlerOutput, HandlerRegistry, RequestExecutionReport, RequestInput,
	};
	use crate::execution_plan::ExecutionPlan;
	use crate::framework_graph::{
		FrameworkDeclarations, FrameworkGraph, HandlerId, MiddlewareDeclaration,
		ResourceDeclaration,
	};
	use crate::response_finalizer::{CLIENT_BUILD_ID_HEADER, JSON_CONTENT_TYPE, ResponseEffects};
	use crate::test_support::route_type_contract;

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn resource_engine(with_middleware: bool) -> ExecutionEngine {
		let mut declarations = FrameworkDeclarations::default();
		if with_middleware {
			declarations.add_middleware(MiddlewareDeclaration::new(handler_id("middleware")));
		}
		declarations.add_resource(ResourceDeclaration::new(
			Method::GET,
			"/api/ping",
			None,
			Some(serde_json::json!({})),
			route_type_contract(),
			handler_id("resource"),
		));
		let graph = FrameworkGraph::compile(declarations).unwrap();
		let plan = ExecutionPlan::compile(&graph).unwrap();
		let assets = AssetCapabilities::new("/static/", Vec::new(), BTreeMap::new()).unwrap();
		ExecutionEngine::new(plan, assets)
	}

	#[tokio::test]
	async fn resource_response_finalizes_resource_body_effects_and_build_id() {
		let engine = resource_engine(false);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_status(StatusCode::CREATED);
			effects.set_header(
				HeaderName::from_static("x-resource"),
				HeaderValue::from_static("ok"),
			);
			Ok(HandlerOutput::body(Bytes::from_static(b"pong")).with_effects(effects))
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		let response = finalize_resource_report(&report, "build-id").unwrap();

		assert_eq!(response.status(), StatusCode::CREATED);
		assert_eq!(response.headers()["x-resource"], "ok");
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
		assert_eq!(response.body(), &Bytes::from_static(b"pong"));
	}

	#[tokio::test]
	async fn resource_response_serializes_resource_data_as_json() {
		let engine = resource_engine(false);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |_| async {
			Ok(HandlerOutput::data(serde_json::json!({"pong": true})))
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		let response = finalize_resource_report(&report, "build-id").unwrap();

		assert_eq!(response.headers()[CONTENT_TYPE], JSON_CONTENT_TYPE);
		assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	}

	#[tokio::test]
	async fn resource_response_preserves_explicit_json_content_type() {
		let engine = resource_engine(false);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_header(
				CONTENT_TYPE,
				HeaderValue::from_static("application/problem+json"),
			);
			Ok(HandlerOutput::data(serde_json::json!({"pong": true})).with_effects(effects))
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		let response = finalize_resource_report(&report, "build-id").unwrap();

		assert_eq!(response.headers()[CONTENT_TYPE], "application/problem+json");
		assert_eq!(response.headers().get_all(CONTENT_TYPE).iter().count(), 1);
		assert_eq!(response.body(), &Bytes::from_static(br#"{"pong":true}"#));
	}

	#[tokio::test]
	async fn resource_response_finalizes_terminal_middleware_without_resource_body() {
		let engine = resource_engine(true);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("middleware"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_status_with_text(StatusCode::NOT_FOUND, "not found");
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		handlers.insert(handler_id("resource"), |_| async {
			Ok(HandlerOutput::body(Bytes::from_static(
				b"should not commit",
			)))
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		let response = finalize_resource_report(&report, "build-id").unwrap();

		assert_eq!(response.status(), StatusCode::NOT_FOUND);
		assert_eq!(
			response.body(),
			&Bytes::from_static(br#"{"error":"not found"}"#)
		);
		assert_eq!(response.headers()[CONTENT_TYPE], JSON_CONTENT_TYPE);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
	}

	#[tokio::test]
	async fn resource_response_short_circuits_terminal_resource_output() {
		let engine = resource_engine(false);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_status_with_text(StatusCode::BAD_REQUEST, "bad input");
			Ok(
				HandlerOutput::data(serde_json::json!({"should": "not leak"}))
					.with_effects(effects),
			)
		});
		let report = engine
			.execute(RequestInput::new(Method::GET, "/api/ping"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		let response = finalize_resource_report(&report, "build-id").unwrap();

		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(
			response.body(),
			&Bytes::from_static(br#"{"error":"bad input"}"#)
		);
		assert_eq!(response.headers()[CONTENT_TYPE], JSON_CONTENT_TYPE);
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
	}

	#[tokio::test]
	async fn resource_response_suppresses_head_response_body_after_get_fallback() {
		let engine = resource_engine(false);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |_| async {
			Ok(HandlerOutput::body(Bytes::from_static(b"pong")))
		});
		let report = engine
			.execute(RequestInput::new(Method::HEAD, "/api/ping"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		let response = finalize_resource_report(&report, "build-id").unwrap();

		assert!(report.suppress_body());
		assert!(response.body().is_empty());
		assert_eq!(response.headers()[CONTENT_LENGTH], "4");
		assert_eq!(response.headers()[CLIENT_BUILD_ID_HEADER], "build-id");
	}

	#[tokio::test]
	async fn resource_response_suppresses_head_terminal_error_body() {
		let engine = resource_engine(false);
		let mut handlers: HandlerRegistry = HandlerRegistry::default();
		handlers.insert(handler_id("resource"), |_| async {
			let mut effects = ResponseEffects::default();
			effects.set_status_with_text(StatusCode::BAD_REQUEST, "bad input");
			Ok(HandlerOutput::empty().with_effects(effects))
		});
		let report = engine
			.execute(RequestInput::new(Method::HEAD, "/api/ping"), &handlers)
			.await
			.unwrap();
		let RequestExecutionReport::Resource(report) = report else {
			panic!("expected resource report");
		};

		let response = finalize_resource_report(&report, "build-id").unwrap();

		assert!(report.suppress_body());
		assert_eq!(response.status(), StatusCode::BAD_REQUEST);
		assert_eq!(response.headers()[CONTENT_LENGTH], "21");
		assert_eq!(response.headers()[CONTENT_TYPE], JSON_CONTENT_TYPE);
		assert!(response.body().is_empty());
	}
}
