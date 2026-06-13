//! Public network helper functions.

use std::net::{Ipv4Addr, SocketAddr};

use crate::error::Error;

/// Read `PORT` and return `0.0.0.0:<PORT>` for deployment-style app servers.
pub fn bind_addr() -> crate::Result<SocketAddr> {
	bind_addr_from_port(
		&std::env::var("PORT").map_err(|_| Error::new("PORT environment variable is required"))?,
	)
}

fn bind_addr_from_port(port: &str) -> crate::Result<SocketAddr> {
	let port = port
		.parse::<u16>()
		.map_err(|error| Error::new(format!("invalid PORT {port:?}: {error}")))?;
	Ok(SocketAddr::from((Ipv4Addr::UNSPECIFIED, port)))
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn bind_addr_from_port_rejects_invalid_ports_and_uses_unspecified_ipv4() {
		assert_eq!(
			bind_addr_from_port("3000").unwrap(),
			SocketAddr::from((Ipv4Addr::UNSPECIFIED, 3000))
		);
		assert!(
			bind_addr_from_port("bad")
				.unwrap_err()
				.to_string()
				.contains("invalid PORT")
		);
	}
}
