//! Document shell for every page.

use crate::{APP_NAME, app};

pub(crate) fn document() -> app::DocumentBuilder {
	app::DocumentBuilder::new(|_ctx| async move {
		let mut document = vorma::Document::new();
		document.html().lang("en");
		document.body().data("app", "board");
		let head = document.head();
		head.meta([
			head.name("viewport"),
			head.content("width=device-width, initial-scale=1"),
		]);
		/*
		Pre-paint theme classes — the vorma-native injection of the snippet
		documented in vorma/kit/theme. The prefix must match what the
		client passes to initTheme().
		*/
		head.script([vorma::kit::theme::script(None)]);
		head.meta_charset("utf-8")
			.title(APP_NAME)
			.description("A small community board, built on Vorma.")
			.meta_property_content("og:site_name", APP_NAME)
			.meta_name_content("robots", "index,follow");
		Ok(document)
	})
}
