use std::collections::{BTreeMap, BTreeSet};

use serde::{Deserialize, Serialize};
use vorma::__private::core::Contract;

#[derive(Clone, Debug, Default, Deserialize, Eq, PartialEq, Serialize)]
pub(crate) struct TsViewModule {
	pub(crate) pattern: String,
	pub(crate) import_path: String,
	pub(crate) deps: Vec<String>,
}

pub(crate) fn get_dev_view_modules(
	views: &[TsViewModule],
) -> Result<BTreeMap<String, TsViewModule>, String> {
	let mut view_modules = BTreeMap::new();
	let mut seen_mods = BTreeSet::new();

	for view in views {
		let pattern = &view.pattern;
		let import_path = &view.import_path;
		if pattern.is_empty() {
			return Err("TS view pattern cannot be empty".to_owned());
		}
		if import_path.is_empty() {
			return Err(format!(
				"TS view import path cannot be empty for pattern: {pattern}"
			));
		}
		if view_modules.contains_key(pattern.as_str()) {
			return Err(format!("duplicate TS view pattern: {pattern}"));
		}
		if !seen_mods.insert(import_path.to_owned()) {
			return Err(format!("duplicate TS module: {import_path}"));
		}
		view_modules.insert(pattern.to_owned(), view.clone());
	}

	Ok(view_modules)
}

pub(crate) fn get_dev_view_modules_from_contract(
	contract: &Contract,
) -> Result<BTreeMap<String, TsViewModule>, String> {
	let views = contract
		.views()
		.iter()
		.map(|view| TsViewModule {
			pattern: view.pattern.clone(),
			import_path: view.client_file.clone(),
			deps: Vec::new(),
		})
		.collect::<Vec<_>>();
	get_dev_view_modules(&views)
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn get_dev_view_modules_rejects_missing_pattern_or_module_and_duplicates() {
		let error = get_dev_view_modules(&[TsViewModule {
			pattern: String::new(),
			import_path: "src/skip.tsx".to_owned(),
			deps: Vec::new(),
		}])
		.unwrap_err();
		assert_eq!(error, "TS view pattern cannot be empty");

		let error = get_dev_view_modules(&[TsViewModule {
			pattern: "/".to_owned(),
			import_path: String::new(),
			deps: Vec::new(),
		}])
		.unwrap_err();
		assert_eq!(error, "TS view import path cannot be empty for pattern: /");

		let error = get_dev_view_modules(&[
			TsViewModule {
				pattern: "/a".to_owned(),
				import_path: "src/a.tsx".to_owned(),
				deps: Vec::new(),
			},
			TsViewModule {
				pattern: "/b".to_owned(),
				import_path: "src/a.tsx".to_owned(),
				deps: Vec::new(),
			},
		])
		.unwrap_err();

		assert_eq!(error, "duplicate TS module: src/a.tsx");
	}
}
