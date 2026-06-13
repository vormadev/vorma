// Synchronization primitives, swappable for loom's model-checked
// versions: the protocol the loom models verify is the protocol
// production runs.

#[cfg(not(loom))]
pub(crate) use std::sync::Mutex;
#[cfg(not(loom))]
pub(crate) use std::sync::atomic::{AtomicUsize, Ordering};

#[cfg(loom)]
pub(crate) use loom::sync::Mutex;
#[cfg(loom)]
pub(crate) use loom::sync::atomic::{AtomicUsize, Ordering};
