//! Standalone task runtime and cache primitive.
//!
//! A [`Task`] always memoizes by task/input inside one [`ExecCtx`].
//! A zero task TTL disables cross-execution-context caching, and a nonzero task TTL
//! retains successful results for later execution contexts created from the same
//! [`Tasks`] runtime.

#![deny(missing_docs)]
#![forbid(unsafe_code)]
// Loom builds compile only the store protocol; the async layer above it
// is gated out, leaving its helpers intentionally unreferenced.
#![cfg_attr(loom, allow(dead_code))]

#[cfg(not(loom))]
mod cancel;
mod clock;
mod error;
mod key;
#[cfg(not(loom))]
mod observer;
#[cfg(not(loom))]
mod overrides;
mod store;
mod sync;
#[cfg(not(loom))]
mod task;

#[cfg(all(loom, test))]
mod loom_tests;

#[cfg(not(loom))]
pub use cancel::CancelToken;
pub use clock::{Clock, ClockInstant, SystemClock};
pub use error::{Error, Result};
pub use key::TaskId;
#[cfg(not(loom))]
pub use observer::{TaskEvent, TaskEventKind, TaskEventOutcome, TaskObserver, TaskRunSource};
#[cfg(not(loom))]
pub use overrides::{TaskOverrideMode, TaskOverrides};
#[cfg(not(loom))]
pub use task::{ExecCtx, PreparedTask, Task, Tasks, TasksOptions};
