//! Convenience emitter for the theme bootstrap snippet documented in
//! packages/vorma/kit/theme/theme.ts.
/*
The TS kit is the source of truth and never references this module; the
docs there show the snippet for ANY backend, and this is merely the
vorma-native way to inject the same thing. The key prefix is plain
duplication on purpose — pass the same value here and to `initTheme()`;
keeping two literals in sync beats config-sharing machinery for
obviousness.
*/

use crate::HtmlElementDef;

/// Pre-paint theme bootstrap for the document head:
/// `head.script([vorma::kit::theme::script(None)])`.
///
/// `None` uses the same default key prefix as the TS kit's
/// `initTheme()`; pass `Some(...)`/a literal only if you override it
/// there too — the two must match.
pub fn script<'a>(key_prefix: impl Into<Option<&'a str>>) -> HtmlElementDef {
	let key_prefix = key_prefix.into().unwrap_or("__vorma_kit");
	crate::SafeHtml::script_content(format!(
		r#"const key_prefix = "{key_prefix}";
const key = `${{key_prefix}}_theme`;
const resolved_key = `${{key_prefix}}_resolved_theme`;
const themes = {{ System: "system", Dark: "dark", Light: "light" }};
const raw = localStorage.getItem(key) ?? themes.System;
const theme = Object.values(themes).includes(raw) ? raw : themes.System;
let resolved_theme = theme;
if (resolved_theme === themes.System) {{
	resolved_theme = window.matchMedia("(prefers-color-scheme: dark)").matches
		? themes.Dark
		: themes.Light;
}}
document.documentElement.classList.add(theme);
if (theme === themes.System) {{
	document.documentElement.classList.add(resolved_theme);
}}
localStorage.setItem(key, theme);
localStorage.setItem(resolved_key, resolved_theme);"#
	))
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn script_injects_the_prefix_and_keeps_the_documented_shape() {
		/*
		Debug output escapes quotes, so the pins use quote-free tokens.
		*/
		let rendered = format!("{:?}", script(None));
		for token in [
			"const key_prefix = ",
			"__vorma_kit",
			"localStorage.getItem(key)",
			"Object.values(themes).includes(raw)",
			"prefers-color-scheme: dark",
			"localStorage.setItem(resolved_key, resolved_theme);",
		] {
			assert!(rendered.contains(token), "missing {token:?}");
		}

		let custom = format!("{:?}", script("__acme"));
		assert!(custom.contains("__acme"));
		assert!(!custom.contains("__vorma_kit"));
	}
}
