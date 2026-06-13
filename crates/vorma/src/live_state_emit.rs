//! Live build-state emission for app binaries running under the build's env key.
/*
The app binary is the only binary that embeds the declaration graph, so it
answers the build's live-state requests itself: when the env key is set,
`App::from_config` emits the serialized contract to stdout and exits instead
of constructing a runtime host. Errors are emitted as the protocol's error
envelope with a zero exit, exactly like a successful emission — the build
side distinguishes them by payload, not exit status.
*/

use std::io::Write;

use vorma_contract::live_state::{
	LIVE_BUILD_STATE_ENV_KEY, LIVE_BUILD_STATE_ENV_VALUE, LiveBuildState,
};

use crate::Result;
use crate::error::Error;
use crate::public_app::AppConfig;

/// Whether the current process was started in live build-state mode.
pub(crate) fn is_live_build_state_mode() -> bool {
	std::env::var(LIVE_BUILD_STATE_ENV_KEY).as_deref() == Ok(LIVE_BUILD_STATE_ENV_VALUE)
}

/// Emit live build-state JSON for this app config and exit the process.
pub(crate) fn emit_live_build_state_and_exit<S>(app_config: AppConfig<S>) -> !
where
	S: Send + Sync + 'static,
{
	let bytes = match live_build_state_bytes(app_config) {
		Ok(bytes) => bytes,
		Err(error) => {
			LiveBuildState::error_json_bytes(error.to_string()).unwrap_or_else(|serialize_error| {
				format!("{{\"error\":{:?}}}", serialize_error.to_string()).into_bytes()
			})
		}
	};
	let mut stdout = std::io::stdout();
	let _ = stdout.write_all(&bytes);
	let _ = stdout.flush();
	std::process::exit(0);
}

fn live_build_state_bytes<S>(app_config: AppConfig<S>) -> Result<Vec<u8>>
where
	S: Send + Sync + 'static,
{
	let app =
		crate::envutil::with_build_mode(|| crate::build_interface::app_build_contract(app_config))?;
	/*
	A dedicated thread with its own runtime keeps this safe regardless of
	the caller's async context: from_config is typically invoked inside the
	app's own tokio main.
	*/
	let state = std::thread::scope(|scope| {
		scope
			.spawn(|| {
				let runtime = tokio::runtime::Builder::new_current_thread()
					.build()
					.map_err(|source| Error::new(format!("build live-state runtime: {source}")))?;
				runtime
					.block_on(crate::build_interface::live_build_state_from_app_build_contract(app))
					.map_err(|source| Error::new(source.to_string()))
			})
			.join()
			.expect("live build-state emission thread panicked")
	})?;
	state
		.to_json_bytes()
		.map_err(|source| Error::new(source.to_string()))
}
