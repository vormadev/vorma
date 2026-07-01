//! Document shell for every page.

use crate::{APP_NAME, MARK_ASSET, app};

pub(crate) fn document() -> app::DocumentBuilder {
	app::DocumentBuilder::new(|ctx| async move {
		/*
		Document builders resolve public assets through the committed Vorma
		manifest, so the same source path works in dev, production, and tests.
		*/
		let mark_url = ctx.public_url(MARK_ASSET)?;
		let request_path = ctx.request().path().to_owned();
		let mut document = vorma::Document::new();
		document.html().lang("en").id("board-document");
		document
			.body()
			.class("board-shell")
			.data("app", "board")
			.data("request-path", request_path);
		let head = document.head();
		head.meta([
			head.name("viewport"),
			head.content("width=device-width, initial-scale=1"),
		]);
		/*
		Use the low-level `link` form when a normal helper is too narrow.
		`preload` is common enough to have a helper, but this variant also
		records the SVG type for the browser.
		*/
		head.link([
			head.rel("preload"),
			head.href(mark_url.clone()),
			head.r#as("image"),
			head.r#type("image/svg+xml"),
		]);
		/*
		Pre-paint theme classes — the vorma-native injection of the snippet
		documented in vorma/kit/theme. The prefix must match what the
		client passes to initTheme().
		*/
		head.script([vorma::kit::theme::script(None)]);
		/*
		Use `SafeHtml` only for trusted fragments the app authors control.
		Structured data is a realistic script-content use: it must be inline
		HTML, but it should not come from user input.
		*/
		head.script([
			vorma::HtmlAttribute::r#type("application/ld+json"),
			vorma::HtmlAttribute::attr("data-purpose", "structured-data"),
			vorma::SafeHtml::script_content(format!(
				r#"{{"@context":"https://schema.org","@type":"WebSite","name":"{APP_NAME}"}}"#
			)),
		]);
		/*
		Trusted inline styles are appropriate for tiny document-owned rules.
		Board enables view transitions, so the reduced-motion guard belongs in
		the head before any route code runs.
		*/
		head.style([vorma::SafeHtml::style_content(
			"@media (prefers-reduced-motion: reduce){::view-transition-old(root),::view-transition-new(root){animation:none}}",
		)]);
		head.meta_charset("utf-8")
			.title(APP_NAME)
			.description("A small community board, built on Vorma.")
			.icon(mark_url.clone())
			.meta_property_content("og:site_name", APP_NAME)
			.meta_property_content("og:image", mark_url)
			.meta_name_content("robots", "index,follow");
		Ok(document)
	})
}
