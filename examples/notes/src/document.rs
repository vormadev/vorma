//! Document shell: html/body attributes and default head for every page.

use crate::{APP_DESCRIPTION, APP_NAME, app};

pub(crate) fn document() -> app::DocumentBuilder {
	app::DocumentBuilder::new(|ctx| async move {
		let mark_url = ctx.public_url("mark.svg")?;
		let font_url = ctx.public_url("fonts/IoskeleyMono-400.woff2")?;
		let mut document = vorma::Document::new();
		document.html().lang("en");
		document.body().data("app", "notes");
		let head = document.head();
		head.meta([
			head.name("viewport"),
			head.content("width=device-width, initial-scale=1"),
		]);
		head.link([
			head.rel("preload"),
			head.href(font_url),
			head.r#as("font"),
			head.r#type("font/woff2"),
			head.cross_origin("anonymous"),
		]);
		/*
		Inline boot script and style demonstrate the explicitly-trusted
		fragment escape hatches; everything else stays escaped builders.
		*/
		head.script([
			vorma::HtmlAttribute::r#type("module"),
			vorma::HtmlAttribute::attr("data-purpose", "theme-boot"),
			vorma::SafeHtml::script_content("document.documentElement.dataset.booted = '1';"),
		]);
		head.style([vorma::SafeHtml::style_content(
			":root{color-scheme:light dark}",
		)]);
		head.meta_charset("utf-8")
			.title(APP_NAME)
			.description(APP_DESCRIPTION)
			.icon(mark_url.clone())
			.meta_property_content("og:title", APP_NAME)
			.meta_property_content("og:description", APP_DESCRIPTION)
			.meta_property_content("og:type", "website")
			.meta_property_content("og:image", mark_url.clone())
			.meta_property_content("og:site_name", APP_NAME)
			.meta_name_content("twitter:card", "summary")
			.meta_name_content("twitter:title", APP_NAME)
			.meta_name_content("twitter:description", APP_DESCRIPTION)
			.meta_name_content("twitter:image", mark_url)
			.meta_name_content("robots", "index,follow");
		Ok(document)
	})
}
