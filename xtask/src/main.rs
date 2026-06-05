mod gate;
mod rust_fuzz;
mod ts_publish;

fn main() {
	let mut args = std::env::args().skip(1);
	let result = match args.next().as_deref() {
		None | Some("gate") => gate::run(),
		Some("rust-fuzz") => rust_fuzz::run(),
		Some("ts-publish") => ts_publish::run(),
		Some(command) => Err(format!("unknown xtask command `{command}`")),
	};

	let exit_code = match result {
		Ok(exit_code) => exit_code,
		Err(error) => {
			eprintln!("xtask: {error}");
			1
		}
	};

	std::process::exit(exit_code);
}
