use std::sync::Mutex;

static OUTPUT: Mutex<Vec<u8>> = Mutex::new(Vec::new());

pub(crate) fn set_output(output: Vec<u8>) {
	*OUTPUT
		.lock()
		.expect("client matcher output should not be poisoned") = output;
}

pub(crate) fn output_ptr() -> *const u8 {
	OUTPUT
		.lock()
		.expect("client matcher output should not be poisoned")
		.as_ptr()
}

pub(crate) fn output_len() -> usize {
	OUTPUT
		.lock()
		.expect("client matcher output should not be poisoned")
		.len()
}
