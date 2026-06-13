fn main() {
	if let Err(error) = vorma_build::run(vorma_notes_example::app_config) {
		eprintln!("{error}");
		std::process::exit(1);
	}
}
