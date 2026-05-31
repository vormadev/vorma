mod gate;

fn main() {
	let exit_code = match gate::run() {
		Ok(exit_code) => exit_code,
		Err(error) => {
			eprintln!("xtask: {error}");
			1
		}
	};

	std::process::exit(exit_code);
}
