use std::any::{Any, TypeId};
use std::collections::hash_map::DefaultHasher;
use std::hash::{Hash, Hasher};
use std::sync::Arc;

/// Opaque identity for one task definition inside the current process.
#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq)]
pub struct TaskId(pub(crate) u64);

#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq)]
pub(crate) struct KeyFingerprint {
	task_id: TaskId,
	input_type: TypeId,
	hash: u64,
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
	pub(crate) fn new(task_id: TaskId, value: &I) -> Self
	where
		I: Clone,
	{
		Self {
			value: value.clone(),
			fingerprint: fingerprint(task_id, value),
		}
	}

	pub(crate) fn value(&self) -> &I {
		&self.value
	}

	pub(crate) fn fingerprint(&self) -> KeyFingerprint {
		self.fingerprint
	}
}

impl<I> Clone for KeyData<I>
where
	I: Clone,
{
	fn clone(&self) -> Self {
		Self {
			value: self.value.clone(),
			fingerprint: self.fingerprint,
		}
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

#[derive(Clone)]
pub(crate) struct PathKey {
	key: Arc<dyn DynKey>,
}

impl PathKey {
	pub(crate) fn new<I>(task_id: TaskId, input: &I) -> Self
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
	{
		Self {
			key: Arc::new(KeyData::new(task_id, input)),
		}
	}

	pub(crate) fn matches<I>(&self, key: &KeyData<I>) -> bool
	where
		I: Eq + Hash + Send + Sync + 'static,
	{
		self.key.fingerprint() == key.fingerprint()
			&& self
				.key
				.as_any()
				.downcast_ref::<I>()
				.is_some_and(|existing| existing == key.value())
	}
}

fn fingerprint<I>(task_id: TaskId, value: &I) -> KeyFingerprint
where
	I: Eq + Hash + 'static,
{
	let mut hasher = DefaultHasher::new();
	value.hash(&mut hasher);
	KeyFingerprint {
		task_id,
		input_type: TypeId::of::<I>(),
		hash: hasher.finish(),
	}
}
