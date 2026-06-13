//! Opaque build/dev control token generation.

use data_encoding::BASE32_NOPAD;

const CONTROL_TOKEN_BYTES: usize = 32;

/// Generate an unguessable Vite plugin RPC token.
pub fn generate_vite_plugin_token() -> Result<String, TokenGenerationError> {
	generate_control_token()
}

/// Generate an unguessable dev browser refresh token.
pub fn generate_dev_refresh_token() -> Result<String, TokenGenerationError> {
	generate_control_token()
}

fn generate_control_token() -> Result<String, TokenGenerationError> {
	let mut bytes = [0; CONTROL_TOKEN_BYTES];
	getrandom::fill(&mut bytes).map_err(|source| TokenGenerationError {
		message: source.to_string(),
	})?;
	let mut token = BASE32_NOPAD.encode(&bytes);
	token.make_ascii_lowercase();
	Ok(token)
}

/// Token generation error.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TokenGenerationError {
	message: String,
}

impl std::fmt::Display for TokenGenerationError {
	fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
		write!(f, "generate control token: {}", self.message)
	}
}

impl std::error::Error for TokenGenerationError {}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn generated_control_tokens_are_lowercase_opaque_values() {
		let vite_token = generate_vite_plugin_token().unwrap();
		let refresh_token = generate_dev_refresh_token().unwrap();

		assert_eq!(vite_token.len(), 52);
		assert_eq!(refresh_token.len(), 52);
		assert!(
			vite_token
				.chars()
				.all(|ch| ch.is_ascii_lowercase() || ch.is_ascii_digit())
		);
		assert_ne!(vite_token, refresh_token);
	}
}
