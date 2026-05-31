use crate::constants::{ENV_KEY_IS_BUILD, ENV_KEY_IS_DEV};

pub fn is_dev() -> bool {
	get_bool(ENV_KEY_IS_DEV, false)
}

pub fn is_build() -> bool {
	get_bool(ENV_KEY_IS_BUILD, false)
}

fn get_bool(key: &str, default_value: bool) -> bool {
	match std::env::var(key) {
		Ok(value) => parse_bool(&value)
			.unwrap_or_else(|| panic!("{key} must be a boolean value, got {value:?}")),
		Err(std::env::VarError::NotPresent) => default_value,
		Err(err) => panic!("{key} is invalid: {err}"),
	}
}

fn parse_bool(value: &str) -> Option<bool> {
	match value {
		"1" | "t" | "T" | "TRUE" | "true" | "True" => Some(true),
		"0" | "f" | "F" | "FALSE" | "false" | "False" => Some(false),
		_ => None,
	}
}

#[cfg(test)]
mod tests {
	use std::sync::Mutex;

	use super::{get_bool, parse_bool};

	static ENV_LOCK: Mutex<()> = Mutex::new(());

	#[test]
	fn parse_bool_accepts_common_bool_values() {
		for value in ["1", "t", "T", "TRUE", "true", "True"] {
			assert_eq!(parse_bool(value), Some(true));
		}
		for value in ["0", "f", "F", "FALSE", "false", "False"] {
			assert_eq!(parse_bool(value), Some(false));
		}
		assert_eq!(parse_bool("yes"), None);
	}

	#[test]
	fn get_bool_panics_on_explicit_invalid_env_value() {
		let _guard = ENV_LOCK.lock().unwrap();
		let key = "__VORMA_TEST_INVALID_BOOL";
		// Rust 2024 marks environment mutation unsafe because concurrent env access can race.
		// This test serializes its own access with ENV_LOCK.
		unsafe {
			std::env::set_var(key, "yes");
		}
		let result = std::panic::catch_unwind(|| get_bool(key, false));
		unsafe {
			std::env::remove_var(key);
		}

		assert!(result.is_err());
	}
}
