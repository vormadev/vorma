use vorma::TaskClock;

#[test]
fn root_head_helpers_are_usable_externally() {
	let mut head = vorma::HeadBuilder::new();
	head.script([vorma::HtmlAttribute::type_("module")]);
	head.style(vorma::SafeHtml::style_content(
		":root{color-scheme:light dark}",
	));

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
}

#[test]
fn root_ts_helpers_are_usable_externally() {
	let mut drafter = vorma::TsDrafter::new();
	let result: vorma::TsResult<_> = drafter.export_const("FLAGS", ["a", "b"]);

	result.unwrap();
	drafter.export_type("Extra", "{ ok: true }").unwrap();

	let _: vorma::TsExtraType = vorma::TsExtraType::of::<()>().unwrap();
}

#[test]
fn root_task_helpers_are_usable_externally() {
	let clock = vorma::SystemTaskClock::new();
	let _: vorma::TaskClockInstant = clock.now();

	let task_error: vorma::TaskError<vorma::Error> = vorma::Error::runtime("boom").into();
	assert!(matches!(task_error, vorma::TaskError::Failed(_)));

	let _: vorma::TaskResult<(), vorma::Error> = Ok(());
	let _: vorma::TasksOptions<vorma::Error> = vorma::TasksOptions::default();
}
