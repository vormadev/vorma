use std::any::{Any, TypeId};
use std::hash::{BuildHasher, Hash, Hasher};
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

/*
HashMap bucket placement for KeyFingerprint uses the fingerprint's own
blake3-derived hash field directly instead of re-hashing the struct.
The fingerprint is already collision-resistant, and slot equality is
always verified through the full typed DynKey comparison — the outer
hash only routes lookups to buckets.
*/
pub(crate) type FingerprintHashMap<V> =
	std::collections::HashMap<KeyFingerprint, V, FingerprintHasherBuilder>;

#[derive(Clone, Copy, Default)]
pub(crate) struct FingerprintHasherBuilder;

impl BuildHasher for FingerprintHasherBuilder {
	type Hasher = FingerprintHasher;

	fn build_hasher(&self) -> Self::Hasher {
		FingerprintHasher { hash: 0 }
	}
}

pub(crate) struct FingerprintHasher {
	hash: u64,
}

impl Hasher for FingerprintHasher {
	fn finish(&self) -> u64 {
		self.hash
	}

	fn write_u64(&mut self, value: u64) {
		self.hash = value;
	}

	fn write(&mut self, _bytes: &[u8]) {
		/*
		KeyFingerprint derives Hash, which writes each field in order:
		TaskId.0 (u64), TypeId (u64), hash (u64). The last write_u64
		call is the blake3-derived hash field, which is the value we
		want. Any preceding writes are overwritten.
		*/
	}
}

// Compute a key fingerprint from a borrowed input — no clone. Equality
// of stored keys is always verified on the actual input value, so the
// hash only routes lookups.
pub(crate) fn fingerprint_for<I>(task_id: TaskId, value: &I) -> KeyFingerprint
where
	I: Eq + Hash + 'static,
{
	let mut hasher = Blake3Hasher::new();
	value.hash(&mut hasher);
	let hash = hasher.finish();

	KeyFingerprint {
		task_id,
		input_type: TypeId::of::<I>(),
		hash,
	}
}

struct Blake3Hasher {
	inner: blake3::Hasher,
}

impl Blake3Hasher {
	fn new() -> Self {
		Self {
			inner: blake3::Hasher::new(),
		}
	}
}

impl Hasher for Blake3Hasher {
	fn finish(&self) -> u64 {
		let hash = self.inner.finalize();
		let mut bytes = [0u8; 8];
		bytes.copy_from_slice(&hash.as_bytes()[..8]);
		u64::from_le_bytes(bytes)
	}

	fn write(&mut self, bytes: &[u8]) {
		self.inner.update(bytes);
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
