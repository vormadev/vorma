use vorma::tasks::Clock;

#[test]
fn root_head_helpers_are_usable_externally() {
	let mut head = vorma::HeadBuilder::new();
	head.script([vorma::HtmlAttribute::r#type("module")]);
	head.style([vorma::SafeHtml::style_content(
		":root{color-scheme:light dark}",
	)]);
	head.script([
		vorma::HtmlAttribute::r#type("module"),
		vorma::SafeHtml::script_content("document.documentElement.dataset.booted = '1';"),
	]);

	let _: vorma::HtmlElementDef = vorma::HtmlAttribute::attr("data-test", "1");
	let _: vorma::HeadAttr = head.href("/app.css");
	let _: vorma::HeadBooleanAttribute = head.bool_attr("async");
	let _: vorma::HeadInnerHtml = head.dangerous_inner_html("body{}");
	let _: vorma::HeadSelfClosing = head.self_closing();
	let _: vorma::HeadTag = vorma::HeadTag("meta");
	let _: vorma::HeadTextContent = head.text_content("hello");
}

#[test]
fn root_document_helpers_are_usable_externally() {
	let mut document = vorma::Document::new();
	let mut attrs: vorma::DocumentAttributes<'_> = document.html();

	attrs.lang("en");

	let identity: vorma::build_interface::contracts::DocumentBuildIdentity<'_> =
		document.__build_identity().unwrap();
	assert_eq!(identity.html_attributes[0].name, "lang");
	assert_eq!(identity.html_attributes[0].value, "en");
}

// `known_safe_attribute`/`boolean_attribute` have no honest board call site
// (census F-21: no non-contrived real-app use for a boolean or
// trusted-unescaped attribute on the root `<html>`/`<body>` element
// presented itself) and are ruled discharged here instead, matching how
// `HeadBuilder`'s equally low-level `known_safe`/`bool_attr` defs are
// handled in the test above. This exercises both the real public call site
// (proving the identity it produces carries the right flags) and the
// rendered HTML form those flags are documented to produce (proving what
// "known safe" and "boolean" actually mean, not just that a flag got set).
#[test]
fn root_document_helpers_known_safe_and_boolean_attributes_render_as_documented() {
	let mut document = vorma::Document::new();
	document
		.html()
		.known_safe_attribute("data-brand", "<b>Vorma</b>")
		.boolean_attribute("data-preview");

	let identity: vorma::build_interface::contracts::DocumentBuildIdentity<'_> =
		document.__build_identity().unwrap();
	let known_safe = identity
		.html_attributes
		.iter()
		.find(|attribute| attribute.name == "data-brand")
		.expect("known_safe_attribute must appear on the built identity");
	assert_eq!(known_safe.value, "<b>Vorma</b>");
	assert!(known_safe.known_safe);
	assert!(!known_safe.boolean);
	let boolean = identity
		.html_attributes
		.iter()
		.find(|attribute| attribute.name == "data-preview")
		.expect("boolean_attribute must appear on the built identity");
	assert!(boolean.boolean);
	assert!(!boolean.known_safe);

	// The identity's flags are exactly what the renderer contract
	// documents: a known-safe value is emitted unescaped (raw `<b>...</b>`,
	// not `&lt;b&gt;`), and a boolean attribute is emitted bare, with no
	// `="..."` at all, never its value.
	let contract = vorma_contract::contracts::DocumentContract::new(
		vec![
			vorma_contract::contracts::DocumentAttributeContract::new(
				known_safe.name,
				known_safe.value,
				known_safe.known_safe,
				known_safe.boolean,
			),
			vorma_contract::contracts::DocumentAttributeContract::new(
				boolean.name,
				boolean.value,
				boolean.known_safe,
				boolean.boolean,
			),
		],
		Vec::new(),
		Vec::new(),
		Vec::new(),
		Vec::new(),
	);
	let html = vorma_contract::document_renderer::render_document(
		&contract,
		vorma_contract::document_renderer::DocumentRenderInput::new("", ""),
	)
	.unwrap();
	assert!(
		html.contains("data-brand=\"<b>Vorma</b>\""),
		"known-safe value must render unescaped: {html}"
	);
	assert!(
		!html.contains("&lt;b&gt;"),
		"known-safe value must not also be escaped: {html}"
	);
	assert!(
		html.contains(" data-preview>") || html.contains(" data-preview "),
		"boolean attribute must render bare, with no value: {html}"
	);
	assert!(
		!html.contains("data-preview="),
		"boolean attribute must never render its (empty) value: {html}"
	);
}

#[tokio::test]
async fn document_builder_doc_hidden_build_helper_is_usable_externally() {
	let builder = vorma::DocumentBuilder::new(|ctx| async move {
		let mut document = vorma::Document::new();
		document
			.html()
			.data("asset", ctx.public_url("favicon.ico")?);
		Ok(document)
	});

	let document = builder
		.build(vorma::DocumentBuildCtx::build())
		.await
		.unwrap();
	let identity_source = builder.build_root_document_hash_source().await.unwrap();
	let mut document = document;

	document.body().class("returned-document-is-public");
	assert!(identity_source.contains("/favicon.ico"));
}

#[test]
fn root_ts_helpers_are_usable_externally() {
	let mut drafter = vorma::tsgen::TsDrafter::new();
	let result: vorma::tsgen::Result<_> = drafter.export_const("FLAGS", ["a", "b"]);

	result.unwrap();
	drafter.export_type("Extra", "{ ok: true }").unwrap();

	let _: vorma::tsgen::TsExtraType = vorma::tsgen::TsExtraType::of::<()>().unwrap();
}

#[test]
fn root_task_helpers_are_usable_externally() {
	let clock = vorma::tasks::SystemClock::new();
	let _: vorma::tasks::ClockInstant = clock.now();

	let task_error: vorma::tasks::Error<vorma::Error> = vorma::Error::new("boom").into();
	assert!(matches!(task_error, vorma::tasks::Error::Failed(_)));

	let _: vorma::tasks::Result<(), vorma::Error> = Ok(());
	let _: vorma::tasks::TasksOptions<vorma::Error> = vorma::tasks::TasksOptions::default();
}

#[allow(dead_code)]
fn runtime_host_collected_request_entrypoint_is_usable_externally(host: &vorma::RuntimeHost<()>) {
	let request = http::Request::new(bytes::Bytes::new());

	let _future = host.handle_request(request);
}

#[test]
fn app_error_type_requires_client_message_trait_not_display() {
	let config: vorma::AppConfig<()> = vorma::AppConfig::default();

	let result = vorma::App::from_config(config);

	assert!(result.is_err());
}
