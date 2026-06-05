use std::time::{Duration, Instant};

/// Monotonic timestamp used for cross-execution-context cache expiry.
#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct ClockInstant {
	nanos_since_origin: u128,
}

impl ClockInstant {
	/// Create a timestamp from a duration since an arbitrary monotonic origin.
	pub fn from_duration_since_origin(duration: Duration) -> Self {
		Self {
			nanos_since_origin: duration.as_nanos(),
		}
	}

	pub(crate) fn saturating_add_duration(self, duration: Duration) -> Self {
		Self {
			nanos_since_origin: self.nanos_since_origin.saturating_add(duration.as_nanos()),
		}
	}

	pub(crate) fn saturating_duration_since(self, earlier: Self) -> Duration {
		let nanos = self
			.nanos_since_origin
			.saturating_sub(earlier.nanos_since_origin);
		Duration::from_nanos(nanos.min(u64::MAX.into()) as u64)
	}
}

/// Monotonic clock used by [`crate::Tasks`] for TTL expiry.
pub trait Clock: Send + Sync + 'static {
	/// Return the current monotonic timestamp.
	fn now(&self) -> ClockInstant;
}

/// Native monotonic clock backed by [`std::time::Instant`].
#[derive(Clone, Debug)]
pub struct SystemClock {
	origin: Instant,
}

impl SystemClock {
	/// Create a system monotonic clock with a fresh origin.
	pub fn new() -> Self {
		Self {
			origin: Instant::now(),
		}
	}
}

impl Default for SystemClock {
	fn default() -> Self {
		Self::new()
	}
}

impl Clock for SystemClock {
	fn now(&self) -> ClockInstant {
		ClockInstant::from_duration_since_origin(self.origin.elapsed())
	}
}
