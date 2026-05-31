fn main() {
	if let Err(error) = vorma_build::run(
		vorma_minimal_example::app_config,
		vorma_build::BuildOptions {
			cargo_package: env!("CARGO_PKG_NAME"),
			cargo_bin: "minimal-build",
		},
	) {
		eprintln!("{error}");
		std::process::exit(1);
	}
}
