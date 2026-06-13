//! Runtime environment helpers.

use std::cell::Cell;
use std::thread::LocalKey;

use crate::constants::{ENV_KEY_IS_BUILD, ENV_KEY_IS_DEV};

thread_local! {
	static SCOPED_BUILD_MODE_DEPTH: Cell<usize> = const { Cell::new(0) };
	static SCOPED_DEV_MODE_DEPTH: Cell<usize> = const { Cell::new(0) };
}

/// Whether the current process is running as a Vorma build/live-state entry.
pub fn is_build() -> bool {
	std::env::var_os(ENV_KEY_IS_BUILD).is_some() || scoped_mode_is_active(&SCOPED_BUILD_MODE_DEPTH)
}

/// Whether the current process is running an app server under Vorma dev mode.
pub fn is_dev() -> bool {
	std::env::var_os(ENV_KEY_IS_DEV).is_some() || scoped_mode_is_active(&SCOPED_DEV_MODE_DEPTH)
}

/// Run a closure under scoped build mode for in-process build entrypoints.
pub fn with_build_mode<R>(body: impl FnOnce() -> R) -> R {
	with_scoped_mode(&SCOPED_BUILD_MODE_DEPTH, body)
}

/// Run a closure under scoped dev mode for in-process runtime tests.
pub fn with_dev_mode<R>(body: impl FnOnce() -> R) -> R {
	with_scoped_mode(&SCOPED_DEV_MODE_DEPTH, body)
}

fn scoped_mode_is_active(depth: &'static LocalKey<Cell<usize>>) -> bool {
	depth.with(|depth| depth.get() > 0)
}

fn with_scoped_mode<R>(depth: &'static LocalKey<Cell<usize>>, body: impl FnOnce() -> R) -> R {
	struct ScopedModeGuard(&'static LocalKey<Cell<usize>>);

	impl Drop for ScopedModeGuard {
		fn drop(&mut self) {
			self.0.with(|depth| {
				let current = depth.get();
				depth.set(current.saturating_sub(1));
			});
		}
	}

	depth.with(|depth| depth.set(depth.get() + 1));
	let _guard = ScopedModeGuard(depth);
	body()
}

#[cfg(test)]
mod tests {
	use super::*;

	#[test]
	fn scoped_build_mode_sets_and_restores_build_mode() {
		assert!(!is_build());
		let nested = with_build_mode(|| {
			assert!(is_build());
			with_build_mode(is_build)
		});

		assert!(nested);
		assert!(!is_build());
	}

	#[test]
	fn scoped_dev_mode_sets_and_restores_dev_mode() {
		assert!(!is_dev());
		let nested = with_dev_mode(|| {
			assert!(is_dev());
			with_dev_mode(is_dev)
		});

		assert!(nested);
		assert!(!is_dev());
	}
}
