fn app_config() -> vorma::Result<vorma::AppConfig<()>> {
	Ok(vorma::AppConfig::default())
}

#[test]
fn public_entrypoints_accept_client_message_error_contract() {
	let run_result = vorma_build::run(app_config);

	assert!(run_result.is_err());
}
