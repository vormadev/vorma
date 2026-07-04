use std::time::{Duration, Instant};

/// Opaque monotonic timestamp produced by a [`Clock`].
///
/// `ClockInstant` values are only ever compared to other values from the
/// same clock — there is no notion of wall-clock time here, only "how far
/// past this clock's arbitrary starting point." [`Tasks`](crate::Tasks)
/// uses it to time `extended_cache` entries out and to stamp
/// [`TaskEvent`](crate::TaskEvent) durations; application code that
/// supplies a custom [`Clock`] (for example a manually advanced clock in
/// a test) constructs values with [`from_duration_since_origin`
/// ](Self::from_duration_since_origin).
#[derive(Clone, Copy, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]
pub struct ClockInstant {
	nanos_since_origin: u128,
}

impl ClockInstant {
	/// Create a timestamp from a duration since an arbitrary monotonic origin.
	///
	/// The origin is whatever a particular [`Clock`] implementation treats
	/// as its zero point — [`SystemClock`] uses process-relative
	/// [`Instant::now`] at construction, and a test clock is free to
	/// invent its own. Two `ClockInstant` values are only meaningfully
	/// comparable when they came from the same clock.
	///
	/// ```
	/// use std::time::Duration;
	///
	/// use vorma_tasks::ClockInstant;
	///
	/// let earlier = ClockInstant::from_duration_since_origin(Duration::from_secs(1));
	/// let later = ClockInstant::from_duration_since_origin(Duration::from_secs(2));
	/// assert!(earlier < later);
	/// ```
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

/// Monotonic clock used by [`Tasks`](crate::Tasks) for `extended_cache` TTL
/// expiry and for timing [`TaskEvent`](crate::TaskEvent)s.
///
/// [`TasksOptions::clock`](crate::TasksOptions::clock) defaults to
/// [`SystemClock`], which is what production code wants. Supply a custom
/// implementation to control time deterministically in tests — for example
/// a clock backed by an `AtomicU64` nanosecond counter that only advances
/// when a test calls an explicit `advance` method, so TTL-expiry
/// assertions never depend on real wall-clock timing.
pub trait Clock: Send + Sync + 'static {
	/// Return the current monotonic timestamp.
	fn now(&self) -> ClockInstant;
}

/// Native monotonic clock backed by [`std::time::Instant`].
///
/// The default [`Clock`] for [`TasksOptions`](crate::TasksOptions); reach
/// for a different [`Clock`] implementation only when a test needs
/// deterministic control over TTL expiry.
#[derive(Clone, Debug)]
pub struct SystemClock {
	origin: Instant,
}

impl SystemClock {
	/// Create a system monotonic clock with a fresh origin.
	///
	/// ```
	/// use vorma_tasks::{Clock, SystemClock};
	///
	/// let clock = SystemClock::new();
	/// let first = clock.now();
	/// let second = clock.now();
	/// assert!(second >= first);
	/// ```
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
