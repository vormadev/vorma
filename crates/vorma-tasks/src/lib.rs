//! Standalone task runtime and cache primitive.
//!
//! A [`Task`] always memoizes by task/input inside one [`ExecCtx`].
//! A zero task TTL disables cross-execution-context caching, and a nonzero task TTL
//! retains successful results for later execution contexts created from the same
//! [`Tasks`] runtime.

#![forbid(unsafe_code)]

mod cancel;
mod clock;
mod error;
mod key;
mod observer;
mod overrides;
mod store;
mod task;

pub use cancel::CancelToken;
pub use clock::{Clock, ClockInstant, SystemClock};
pub use error::{Error, Result};
pub use key::TaskId;
pub use observer::{TaskEvent, TaskEventKind, TaskEventOutcome, TaskObserver, TaskRunSource};
pub use overrides::{TaskOverrideMode, TaskOverrides};
pub use task::{ExecCtx, PreparedTask, Task, Tasks, TasksOptions};
