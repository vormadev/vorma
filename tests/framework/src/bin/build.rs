fn main() {
	if let Err(error) = vorma_framework_tests::scenario::selected_variant().build() {
		eprintln!("{error}");
		std::process::exit(1);
	}
}
