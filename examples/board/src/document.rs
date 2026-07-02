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
			.data("request-path", request_path)
			/*
			`boolean_attribute` renders bare, name-only, with no `="..."` at
			all: the HTML sense of "boolean" is that presence alone is the
			signal, unlike `.data(...)` right above, which always carries a
			value. This document builder only ever runs while assembling a
			real server response, so this flag is genuinely true exactly
			when it appears: client CSS or JS can target
			`[data-server-rendered]` to tell a hard load apart from a route
			committed by client-side navigation, which never rebuilds the
			document shell.
			*/
			.boolean_attribute("data-server-rendered")
			/*
			`known_safe_attribute` skips the escaping `.attribute()` applies
			(quotes, `&`, angle brackets), so the value lands on the wire
			byte-for-byte. That is only safe for a value the app fully owns
			at compile time; reach for `.attribute()` by default, and never
			pass anything derived from user input here, since an unescaped
			`"` would let the value break out of the attribute and inject
			markup. This credit line is a hardcoded literal, chosen because
			it contains an ampersand: escaped, it would render as `&amp;`
			instead of the raw `&` below.
			*/
			.known_safe_attribute("data-built-with", format!("{APP_NAME} & Rust"));
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
