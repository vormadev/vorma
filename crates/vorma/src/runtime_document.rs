//! Dynamic runtime document provider contract.
//!
//! Contract seam between declaration-time document builders and the serving
//! layers: `runtime_service`/`runtime_app` resolve a document through this
//! provider on each request that needs one, so apps can vary the shell at
//! runtime without touching committed snapshots.

use std::future::Future;
use std::pin::Pin;

use crate::asset_capabilities::{AssetCapabilities, PublicUrlError};
use crate::contracts::DocumentContract;
use crate::execution_engine::RequestInput;
use crate::runtime_manifest::RuntimeManifest;

/// Future returned by a runtime document provider.
pub type RuntimeDocumentFuture =
	Pin<Box<dyn Future<Output = Result<DocumentContract, RuntimeDocumentError>> + Send + 'static>>;

/// Provider for per-request document contracts.
pub trait RuntimeDocumentProvider: Send + Sync {
	/// Build the document contract for one matched view request.
	fn document_for_request(&self, input: RuntimeDocumentInput) -> RuntimeDocumentFuture;
}

impl<F, Fut> RuntimeDocumentProvider for F
where
	F: Fn(RuntimeDocumentInput) -> Fut + Send + Sync,
	Fut: Future<Output = Result<DocumentContract, RuntimeDocumentError>> + Send + 'static,
{
	fn document_for_request(&self, input: RuntimeDocumentInput) -> RuntimeDocumentFuture {
		Box::pin(self(input))
	}
}

/// Input facts for per-request document construction.
#[derive(Clone, Debug)]
pub struct RuntimeDocumentInput {
	request: RequestInput,
	manifest: RuntimeManifest,
	asset_capabilities: AssetCapabilities,
}

impl RuntimeDocumentInput {
	/// Create document provider input.
	pub fn new(
		request: RequestInput,
		manifest: RuntimeManifest,
		asset_capabilities: AssetCapabilities,
	) -> Self {
		Self {
			request,
			manifest,
			asset_capabilities,
		}
	}

	/// Request facts for this document render.
	pub fn request(&self) -> &RequestInput {
		&self.request
	}

	/// Committed runtime manifest for this generation.
	pub fn manifest(&self) -> &RuntimeManifest {
		&self.manifest
	}

	/// Resolve a logical public source path to a committed public URL.
	pub fn public_url(&self, src_path: &str) -> Result<String, RuntimeDocumentError> {
		self.asset_capabilities
			.public_url(src_path)
			.map(ToOwned::to_owned)
			.map_err(RuntimeDocumentError::from)
	}
}

/// Runtime document construction error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct RuntimeDocumentError {
	message: String,
}

impl RuntimeDocumentError {
	/// Create a runtime document error.
	pub fn new(message: impl Into<String>) -> Self {
		Self {
			message: message.into(),
		}
	}

	/// Error message.
	pub fn message(&self) -> &str {
		&self.message
	}
}

impl From<PublicUrlError> for RuntimeDocumentError {
	fn from(error: PublicUrlError) -> Self {
		Self::new(error.to_string())
	}
}

impl std::fmt::Display for RuntimeDocumentError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		f.write_str(&self.message)
	}
}

impl std::error::Error for RuntimeDocumentError {}

/// Runtime document provider for a committed static document contract.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct StaticRuntimeDocumentProvider {
	document: DocumentContract,
}

impl StaticRuntimeDocumentProvider {
	/// Create a provider that always returns the same document contract.
	pub fn new(document: DocumentContract) -> Self {
		Self { document }
	}

	/// Static document contract.
	pub fn document(&self) -> &DocumentContract {
		&self.document
	}
}

impl RuntimeDocumentProvider for StaticRuntimeDocumentProvider {
	fn document_for_request(&self, _input: RuntimeDocumentInput) -> RuntimeDocumentFuture {
		let document = self.document.clone();
		Box::pin(async move { Ok(document) })
	}
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use http::Method;

	use super::*;

	#[tokio::test]
	async fn static_document_provider_returns_committed_document() {
		let document = DocumentContract::new(
			vec![crate::contracts::DocumentAttributeContract::new(
				"lang", "en", false, false,
			)],
			Vec::new(),
			Vec::new(),
			Vec::new(),
			Vec::new(),
		);
		let provider = StaticRuntimeDocumentProvider::new(document.clone());
		let input = RuntimeDocumentInput::new(
			RequestInput::new(Method::GET, "/"),
			RuntimeManifest::new(
				"build-id",
				"/static/",
				"",
				Default::default(),
				Vec::new(),
				Default::default(),
				Vec::new(),
			),
			AssetCapabilities::new("/static/", Vec::new(), BTreeMap::new()).unwrap(),
		);

		let rendered = provider.document_for_request(input).await.unwrap();

		assert_eq!(rendered, document);
	}

	#[test]
	fn runtime_document_input_resolves_public_urls_from_committed_capabilities() {
		let public_filemap =
			BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]);
		let input = RuntimeDocumentInput::new(
			RequestInput::new(Method::GET, "/"),
			RuntimeManifest::new(
				"build-id",
				"/static/",
				"",
				Default::default(),
				vec!["/static/app.hash.css".to_owned()],
				public_filemap.clone(),
				Vec::new(),
			),
			AssetCapabilities::new(
				"/static/",
				vec!["/static/app.hash.css".to_owned()],
				public_filemap,
			)
			.unwrap(),
		);

		assert_eq!(
			input.public_url(" //app.css ").unwrap(),
			"/static/app.hash.css"
		);
		assert!(input.public_url("missing.css").is_err());
		assert!(input.public_url(" ").is_err());
	}
}
