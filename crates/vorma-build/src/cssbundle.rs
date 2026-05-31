use std::collections::BTreeMap;
use std::fmt;
use std::path::{Path, PathBuf};

use lightningcss::bundler::{Bundler, FileProvider};
use lightningcss::stylesheet::{MinifyOptions, ParserOptions, PrinterOptions, StyleSheet};
use lightningcss::values::url::Url as CssUrl;
use lightningcss::visit_types;
use lightningcss::visitor::{Visit, VisitTypes, Visitor};
use url::Url;

pub(crate) const PUBLIC_URL_PREFIX: &str = "@public/";

#[derive(Clone, Debug, Default, Eq, PartialEq)]
pub(crate) struct BundleOutput {
	pub(crate) css: String,
	pub(crate) imports: Vec<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) struct BundleArgs<'a> {
	pub(crate) entry_path: PathBuf,
	pub(crate) public_url_map: &'a BTreeMap<String, String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct ParsedCssUrl {
	path: String,
	query: Option<String>,
	fragment: Option<String>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub(crate) enum Error {
	Css(String),
	InvalidCssUrlPath { raw: String },
	UnresolvedPublicAsset { path: String },
}

pub(crate) fn bundle(args: BundleArgs<'_>) -> Result<BundleOutput, Error> {
	let provider = FileProvider::new();
	let mut bundler = Bundler::new(&provider, None, ParserOptions::default());
	let mut stylesheet = bundler
		.bundle(&args.entry_path)
		.map_err(|error| Error::Css(error.to_string()))?;
	let imports = stylesheet
		.sources
		.iter()
		.map(|source| normalized_path_string(source))
		.collect();

	rewrite_urls(&mut stylesheet, args.public_url_map)?;

	stylesheet
		.minify(MinifyOptions::default())
		.map_err(|error| Error::Css(error.to_string()))?;
	let css = stylesheet
		.to_css(PrinterOptions {
			minify: true,
			..PrinterOptions::default()
		})
		.map_err(|error| Error::Css(error.to_string()))?
		.code
		.trim()
		.to_owned();

	Ok(BundleOutput { css, imports })
}

fn rewrite_urls(
	stylesheet: &mut StyleSheet<'_, '_>,
	public_url_map: &BTreeMap<String, String>,
) -> Result<(), Error> {
	let mut visitor = PublicUrlRewriteVisitor { public_url_map };
	stylesheet.visit(&mut visitor)?;
	Ok(())
}

struct PublicUrlRewriteVisitor<'a> {
	public_url_map: &'a BTreeMap<String, String>,
}

impl<'i> Visitor<'i> for PublicUrlRewriteVisitor<'_> {
	type Error = Error;

	fn visit_types(&self) -> VisitTypes {
		visit_types!(URLS)
	}

	fn visit_url(&mut self, url: &mut CssUrl<'i>) -> Result<(), Self::Error> {
		let raw = url.url.trim().to_owned();
		let Some(parsed) = parse_rewritable_url(&raw) else {
			return Ok(());
		};
		if let Some(resolved) = resolve_critical_css_url(&raw, &parsed, self.public_url_map)? {
			url.url = resolved.into();
		}
		Ok(())
	}
}

fn parse_rewritable_url(raw: &str) -> Option<ParsedCssUrl> {
	if raw.starts_with("//") {
		return None;
	}
	if Url::parse(raw).is_ok() {
		return None;
	}

	let (without_fragment, fragment) = match raw.split_once('#') {
		Some((before, after)) => (before, Some(after.to_owned())),
		None => (raw, None),
	};
	let (path, query) = match without_fragment.split_once('?') {
		Some((before, after)) => (before.to_owned(), Some(after.to_owned())),
		None => (without_fragment.to_owned(), None),
	};

	if path.is_empty() && fragment.is_some() && raw.trim_start().starts_with('#') {
		return None;
	}

	Some(ParsedCssUrl {
		path,
		query,
		fragment,
	})
}

fn resolve_critical_css_url(
	raw: &str,
	parsed: &ParsedCssUrl,
	public_url_map: &BTreeMap<String, String>,
) -> Result<Option<String>, Error> {
	if parsed.path.starts_with('/') {
		return Ok(Some(raw.to_owned()));
	}
	if !parsed.path.starts_with(PUBLIC_URL_PREFIX) {
		return Err(Error::InvalidCssUrlPath {
			raw: raw.to_owned(),
		});
	}

	let lookup = parsed
		.path
		.strip_prefix(PUBLIC_URL_PREFIX)
		.expect("public URL path should have framework prefix");
	let Some(public_url) = public_url_map.get(lookup) else {
		return Err(Error::UnresolvedPublicAsset {
			path: parsed.path.clone(),
		});
	};

	let mut resolved = public_url.clone();
	if let Some(query) = &parsed.query {
		resolved.push('?');
		resolved.push_str(query);
	}
	if let Some(fragment) = &parsed.fragment {
		resolved.push('#');
		resolved.push_str(fragment);
	}
	Ok(Some(resolved))
}

impl fmt::Display for Error {
	fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
		match self {
			Self::Css(error) => write!(f, "{error}"),
			Self::InvalidCssUrlPath { raw: _ } => write!(
				f,
				"CSS URL paths must be absolute, external, or start with {PUBLIC_URL_PREFIX:?}",
			),
			Self::UnresolvedPublicAsset { path } => {
				write!(f, "unresolved static public asset {path:?}")
			}
		}
	}
}

impl std::error::Error for Error {}

fn normalized_path_string(path: &str) -> String {
	Path::new(path)
		.components()
		.collect::<PathBuf>()
		.to_string_lossy()
		.into_owned()
}

#[cfg(test)]
mod tests {
	use std::fs;
	use std::time::{SystemTime, UNIX_EPOCH};

	use super::*;

	#[test]
	fn bundles_imports_minifies_and_rewrites_public_urls() {
		let root = temp_dir("bundle");
		fs::create_dir_all(&root).unwrap();
		let entry = root.join("critical.css");
		let imported = root.join("base.css");
		fs::write(
			&entry,
			r#"@import "./base.css"; .hero { background: url("@public/logo.svg?v=1#mark"); }"#,
		)
		.unwrap();
		fs::write(&imported, "body { margin: 0; }").unwrap();

		let output = bundle(BundleArgs {
			entry_path: entry.clone(),
			public_url_map: &BTreeMap::from([(
				"logo.svg".to_owned(),
				"/static/logo.abc.svg".to_owned(),
			)]),
		})
		.unwrap();

		assert!(output.css.contains("body{margin:0}"));
		assert!(output.css.contains("url(/static/logo.abc.svg?v=1#mark)"));
		assert!(
			output
				.imports
				.contains(&entry.to_string_lossy().into_owned())
		);
		assert!(
			output
				.imports
				.contains(&imported.to_string_lossy().into_owned())
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn rejects_relative_non_public_urls() {
		let root = temp_dir("relative-url");
		fs::create_dir_all(&root).unwrap();
		let entry = root.join("critical.css");
		fs::write(&entry, r#".hero { background: url("./logo.svg"); }"#).unwrap();

		let error = bundle(BundleArgs {
			entry_path: entry,
			public_url_map: &BTreeMap::new(),
		})
		.unwrap_err();

		assert!(matches!(error, Error::InvalidCssUrlPath { .. }));
		assert_eq!(
			error.to_string(),
			"CSS URL paths must be absolute, external, or start with \"@public/\""
		);

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn preserves_absolute_and_external_urls() {
		let root = temp_dir("absolute-url");
		fs::create_dir_all(&root).unwrap();
		let entry = root.join("critical.css");
		fs::write(
			&entry,
			r#".a{background:url("/logo.svg")}.b{background:url("https://example.com/logo.svg")}"#,
		)
		.unwrap();

		let output = bundle(BundleArgs {
			entry_path: entry,
			public_url_map: &BTreeMap::new(),
		})
		.unwrap();

		assert!(output.css.contains("url(/logo.svg)"));
		assert!(output.css.contains("url(https://example.com/logo.svg)"));

		fs::remove_dir_all(root).unwrap();
	}

	#[test]
	fn removes_only_one_public_url_prefix() {
		let root = temp_dir("one-public-prefix");
		fs::create_dir_all(&root).unwrap();
		let entry = root.join("critical.css");
		fs::write(&entry, r#".a{background:url("@public/@public/logo.svg")}"#).unwrap();

		let error = bundle(BundleArgs {
			entry_path: entry,
			public_url_map: &BTreeMap::new(),
		})
		.unwrap_err();

		assert_eq!(
			error,
			Error::UnresolvedPublicAsset {
				path: "@public/@public/logo.svg".to_owned(),
			}
		);

		fs::remove_dir_all(root).unwrap();
	}

	fn temp_dir(name: &str) -> PathBuf {
		let nonce = SystemTime::now()
			.duration_since(UNIX_EPOCH)
			.unwrap()
			.as_nanos();
		std::env::temp_dir().join(format!("vorma-critical-css-{name}-{nonce}"))
	}
}
