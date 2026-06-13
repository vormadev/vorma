//! Complete app assembly over the fresh declaration facade.

use crate::asset_body_provider::PublicAssetBodyProvider;
use crate::config::{Config, ConfigError};
use crate::document_builder::{Document, DocumentBuilder};
use crate::execution_engine::HandlerRegistry;
use crate::facade::{CompiledAppFacade, FacadeError, FacadeRuntimeHostError, StatefulAppFacade};
use crate::framework_graph::{FrameworkConfig, FrameworkGraph};
use crate::runtime_host::CommittedRuntimeHost;
use crate::runtime_manifest::RuntimeManifest;
use vorma_tasks::{Tasks, TasksOptions};

/// Default maximum request body size collected by Vorma handlers.
pub const DEFAULT_REQUEST_BODY_LIMIT: usize = 16 * 1024 * 1024;

/// Complete app assembly before graph compilation.
pub struct AppAssembly<S> {
	facade: StatefulAppFacade<S>,
	document_builder: DocumentBuilder,
	request_body_limit: usize,
}

impl<S> AppAssembly<S>
where
	S: Send + Sync + 'static,
{
	/// Create an app assembly with default document and request body settings.
	pub fn new(config: FrameworkConfig, state: S) -> Self {
		Self::new_with_tasks(config, state, Tasks::new(TasksOptions::default()))
	}

	/// Create an app assembly from public framework configuration.
	pub fn from_config(config: &Config, state: S) -> Result<Self, ConfigError> {
		Ok(Self::new(config.framework_config()?, state))
	}
}

impl<S> AppAssembly<S>
where
	S: Send + Sync + 'static,
{
	/// Create an app assembly with default document, request body settings, and task runtime.
	pub fn new_with_tasks(config: FrameworkConfig, state: S, tasks: Tasks<crate::Error>) -> Self {
		let mut facade = StatefulAppFacade::new_with_tasks(config, state, tasks);
		let default_document = Document::new();
		facade.set_document(default_document.clone().into_contract());
		Self {
			facade,
			document_builder: static_document_builder(default_document),
			request_body_limit: DEFAULT_REQUEST_BODY_LIMIT,
		}
	}

	/// Create an app assembly from public framework configuration and task runtime.
	pub fn from_config_with_tasks(
		config: &Config,
		state: S,
		tasks: Tasks<crate::Error>,
	) -> Result<Self, ConfigError> {
		Ok(Self::new_with_tasks(
			config.framework_config()?,
			state,
			tasks,
		))
	}

	/// Stateful declaration facade.
	pub fn facade(&self) -> &StatefulAppFacade<S> {
		&self.facade
	}

	/// Mutable stateful declaration facade.
	pub fn facade_mut(&mut self) -> &mut StatefulAppFacade<S> {
		&mut self.facade
	}

	/// Runtime document builder.
	pub fn document_builder(&self) -> &DocumentBuilder {
		&self.document_builder
	}

	/// Replace the static document contract and runtime document builder together.
	pub fn set_static_document(&mut self, document: Document) -> &mut Self {
		self.facade.set_document(document.clone().into_contract());
		self.document_builder = static_document_builder(document);
		self
	}

	/// Replace only the runtime document builder.
	pub fn set_runtime_document_builder(&mut self, document_builder: DocumentBuilder) -> &mut Self {
		self.document_builder = document_builder;
		self
	}

	/// Maximum request body bytes collected by the runtime service.
	pub fn request_body_limit(&self) -> usize {
		self.request_body_limit
	}

	/// Set the maximum request body bytes collected by the runtime service.
	pub fn set_request_body_limit(&mut self, request_body_limit: usize) -> &mut Self {
		self.request_body_limit = request_body_limit;
		self
	}

	/// Compile this app assembly.
	pub fn compile(self) -> Result<CompiledAppAssembly, FacadeError> {
		Ok(CompiledAppAssembly {
			facade: self.facade.compile()?,
			document_builder: self.document_builder,
			request_body_limit: self.request_body_limit,
		})
	}
}

/// Complete app assembly after graph compilation.
pub struct CompiledAppAssembly {
	facade: CompiledAppFacade,
	document_builder: DocumentBuilder,
	request_body_limit: usize,
}

impl CompiledAppAssembly {
	/// Compiled declaration facade.
	pub fn facade(&self) -> &CompiledAppFacade {
		&self.facade
	}

	/// Runtime document builder.
	pub fn document_builder(&self) -> &DocumentBuilder {
		&self.document_builder
	}

	/// Maximum request body bytes collected by the runtime service.
	pub fn request_body_limit(&self) -> usize {
		self.request_body_limit
	}

	/// Consume this assembly into its compiled graph and runtime handlers.
	pub fn into_graph_and_handlers(self) -> (FrameworkGraph, HandlerRegistry) {
		self.facade.into_graph_and_handlers()
	}

	/// Consume this assembly into build-owned graph and document-builder facts.
	pub fn into_build_parts(self) -> (FrameworkGraph, DocumentBuilder) {
		let (graph, _) = self.facade.into_graph_and_handlers();
		(graph, self.document_builder)
	}

	/// Consume this assembly into a committed runtime host.
	pub fn into_runtime_host<P>(
		self,
		manifest: RuntimeManifest,
		asset_provider: P,
	) -> Result<CommittedRuntimeHost<P>, FacadeRuntimeHostError>
	where
		P: PublicAssetBodyProvider + 'static,
	{
		Ok(self
			.facade
			.into_runtime_host(manifest, asset_provider, self.request_body_limit)?
			.with_document_provider(self.document_builder))
	}
}

fn static_document_builder(document: Document) -> DocumentBuilder {
	DocumentBuilder::new(move |_| {
		let document = document.clone();
		async move { Ok(document) }
	})
}

#[cfg(test)]
mod tests {
	use std::collections::BTreeMap;

	use bytes::Bytes;
	use http::Method;
	use http_body_util::{BodyExt, Full};
	use serde::Serialize;
	use tower_service::Service;

	use super::*;
	use crate::asset_body_provider::PublicAssetBodyError;
	use crate::config::{
		Config, DevWatchConfig, FrontendConfig, ServerTarget, TsGenConfig, UiVariant,
	};
	use crate::facade::FacadeRuntimeHostError;
	use crate::framework_graph::HandlerId;
	use crate::runtime_manifest::{RuntimeManifest, RuntimeViewModule};
	use crate::test_support::route_type_contract;
	use crate::typed_handler::TypedHandlerContext;

	#[derive(Clone, Debug, Serialize)]
	struct TestOutput {
		ok: bool,
	}

	fn handler_id(value: &str) -> HandlerId {
		HandlerId::new(value).unwrap()
	}

	fn public_config() -> Config {
		Config {
			root_dir: "app".into(),
			server_target: ServerTarget {
				cargo_package: "server-package".to_owned(),
				cargo_bin: "server-bin".to_owned(),
			},
			dist_dir: "dist".to_owned(),
			public_static_base: "assets".to_owned(),
			frontend_config: FrontendConfig {
				ui_variant: UiVariant::React,
				js_package_manager_base_cmd: "pnpm".to_owned(),
				js_package_manager_dir: ".".to_owned(),
				vite_config_file: "vite.config.ts".to_owned(),
				entry_file: "src/entry.tsx".to_owned(),
				public_static_src_dir: "public".to_owned(),
				critical_css_file: "src/critical.css".to_owned(),
			},
			ts_gen_config: TsGenConfig {
				out_file: "src/vorma.gen.ts".to_owned(),
				extra_types: Vec::new(),
				extra_ts: crate::tsgen::TsDrafter::new(),
			},
			dev_watch_config: DevWatchConfig::default(),
		}
	}

	#[test]
	fn app_assembly_defaults_static_document_and_request_body_limit() {
		let assembly = AppAssembly::new(FrameworkConfig::default(), ());

		assert_eq!(assembly.request_body_limit(), DEFAULT_REQUEST_BODY_LIMIT);
		assert_eq!(
			assembly
				.facade()
				.declarations()
				.document()
				.html_attributes()[0]
				.value(),
			"en"
		);
	}

	#[test]
	fn app_assembly_lowers_public_config_into_graph_build_inputs() {
		let assembly = AppAssembly::from_config(&public_config(), ()).unwrap();
		let build_inputs = assembly
			.facade()
			.declarations()
			.config()
			.build_inputs()
			.unwrap();

		assert_eq!(
			assembly
				.facade()
				.declarations()
				.config()
				.public_static_base(),
			"/assets/"
		);
		assert_eq!(
			build_inputs.server_target().cargo_package(),
			"server-package"
		);
	}

	#[tokio::test]
	async fn app_assembly_compiles_into_runtime_host_with_document_builder() {
		let mut assembly: AppAssembly<()> = AppAssembly::new_with_tasks(
			FrameworkConfig::default(),
			(),
			vorma_tasks::Tasks::new(vorma_tasks::TasksOptions::default()),
		);
		assembly
			.facade_mut()
			.add_typed_view_route::<(), TestOutput, _, _>(
				"/",
				"root.tsx",
				serde_json::json!({}),
				route_type_contract(),
				handler_id("root"),
				|_ctx: TypedHandlerContext<(), ()>| async move { Ok(TestOutput { ok: true }) },
			)
			.unwrap();
		assembly.set_runtime_document_builder(DocumentBuilder::new(|context| async move {
			let mut document = Document::new();
			document.body().data("path", context.request().path());
			Ok(document)
		}));
		assembly.set_request_body_limit(1024);
		let manifest = RuntimeManifest::new(
			"build-id",
			"/static/",
			"",
			BTreeMap::from([("/".to_owned(), serde_json::json!({}))]),
			vec!["/static/root.js".to_owned()],
			BTreeMap::new(),
			vec![RuntimeViewModule::new(
				"/",
				"/static/root.js",
				Vec::new(),
				Vec::new(),
			)],
		);
		let mut service = assembly
			.compile()
			.unwrap()
			.into_runtime_host(manifest, |_| async {
				Err(PublicAssetBodyError::new("asset provider should not run"))
			})
			.unwrap()
			.into_service();
		let request = http::Request::builder()
			.method(Method::GET)
			.uri("/")
			.body(Full::new(Bytes::new()))
			.unwrap();

		let response = service.call(request).await.unwrap();
		let html = response.into_body().collect().await.unwrap().to_bytes();
		let html = String::from_utf8(html.to_vec()).unwrap();

		assert!(html.contains("data-path=\"/\""));
		assert!(html.contains("\"ok\":true"));
	}

	#[test]
	fn compiled_app_assembly_surfaces_runtime_host_errors() {
		let assembly = AppAssembly::new(FrameworkConfig::default(), ());
		let result = assembly.compile().unwrap().into_runtime_host(
			RuntimeManifest::new(
				"",
				"/static/",
				"",
				BTreeMap::new(),
				Vec::new(),
				BTreeMap::from([("app.css".to_owned(), "/static/app.hash.css".to_owned())]),
				Vec::new(),
			),
			|_| async { Err(PublicAssetBodyError::new("asset provider should not run")) },
		);
		let error = match result {
			Ok(_) => panic!("expected runtime host assembly to fail"),
			Err(error) => error,
		};

		assert!(matches!(
			error,
			FacadeRuntimeHostError::RuntimeSnapshot { .. }
		));
	}
}
