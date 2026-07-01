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
