#[derive(Clone, Debug, Default, Eq, Ord, PartialEq, PartialOrd)]
pub(crate) struct CargoBinTarget {
	pub(crate) cargo_package: String,
	pub(crate) cargo_bin: String,
}

pub(crate) fn validate_cargo_bin_target(
	target: &CargoBinTarget,
	label: &str,
) -> Result<CargoBinTarget, String> {
	let cargo_package = target.cargo_package.trim().to_owned();
	if cargo_package.is_empty() {
		return Err(format!("{label} cargo_package cannot be empty"));
	}
	let cargo_bin = target.cargo_bin.trim().to_owned();
	if cargo_bin.is_empty() {
		return Err(format!("{label} cargo_bin cannot be empty"));
	}
	Ok(CargoBinTarget {
		cargo_package,
		cargo_bin,
	})
}
