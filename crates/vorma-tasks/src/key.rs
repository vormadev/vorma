use std::any::{Any, TypeId};
use std::hash::{Hash, Hasher};
use std::sync::Arc;

use rustc_hash::FxHasher;

/// Opaque identity for one task definition inside the current process.
#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq)]
pub struct TaskId(pub(crate) u64);

#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq)]
pub(crate) struct KeyFingerprint {
	task_id: TaskId,
	input_type: TypeId,
	hash: u64,
}

// Compute a key fingerprint from a borrowed input — no clone. Equality
// of stored keys is always verified on the actual input value, so the
// hash only routes lookups.
pub(crate) fn fingerprint_for<I>(task_id: TaskId, value: &I) -> KeyFingerprint
where
	I: Eq + Hash + 'static,
{
	let mut hasher = FxHasher::default();
	value.hash(&mut hasher);
	KeyFingerprint {
		task_id,
		input_type: TypeId::of::<I>(),
		hash: hasher.finish(),
	}
}

pub(crate) trait DynKey: Send + Sync {
	fn as_any(&self) -> &dyn Any;
	fn fingerprint(&self) -> KeyFingerprint;
}

pub(crate) struct KeyData<I> {
	value: I,
	fingerprint: KeyFingerprint,
}

impl<I> KeyData<I>
where
	I: Eq + Hash + Send + Sync + 'static,
{
	// Take ownership of an already-cloned input; the fingerprint was
	// computed from the borrowed original.
	pub(crate) fn from_parts(value: I, fingerprint: KeyFingerprint) -> Self {
		Self { value, fingerprint }
	}
}

impl<I> DynKey for KeyData<I>
where
	I: Eq + Hash + Send + Sync + 'static,
{
	fn as_any(&self) -> &dyn Any {
		&self.value
	}

	fn fingerprint(&self) -> KeyFingerprint {
		self.fingerprint
	}
}

// One ancestor task/input pair on an execution path, used for cycle
// detection. Shares the slot's own key allocation.
#[derive(Clone)]
pub(crate) struct PathKey {
	key: Arc<dyn DynKey>,
}

impl PathKey {
	pub(crate) fn from_dyn(key: Arc<dyn DynKey>) -> Self {
		Self { key }
	}

	pub(crate) fn new<I>(task_id: TaskId, input: &I) -> Self
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
	{
		let fingerprint = fingerprint_for(task_id, input);
		Self {
			key: Arc::new(KeyData::from_parts(input.clone(), fingerprint)),
		}
	}

	pub(crate) fn matches<I>(&self, fingerprint: KeyFingerprint, value: &I) -> bool
	where
		I: Eq + Hash + Send + Sync + 'static,
	{
		self.key.fingerprint() == fingerprint
			&& self
				.key
				.as_any()
				.downcast_ref::<I>()
				.is_some_and(|existing| existing == value)
	}
}
