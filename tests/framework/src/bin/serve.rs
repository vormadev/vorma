#[tokio::main]
async fn main() {
	let result = if vorma::is_dev() {
		vorma_framework_tests::scenario::selected_variant()
			.serve_selected_from_disk()
			.await
	} else {
		vorma_framework_tests::scenario::selected_variant()
			.serve_from_disk()
			.await
	};

	if let Err(error) = result {
		eprintln!("{error}");
		std::process::exit(1);
	}
}
