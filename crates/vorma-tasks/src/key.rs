use std::any::{Any, TypeId};
use std::collections::hash_map::RandomState;
use std::hash::{BuildHasher, Hash, Hasher};
use std::sync::Arc;
use std::sync::LazyLock;

/// Opaque identity for one task definition inside the current process.
///
/// Returned by [`Task::id`](crate::Task::id) and carried on every
/// [`TaskEvent`](crate::TaskEvent). There is no public constructor —
/// `TaskId` values only ever come from a real declared task, and comparing
/// two `TaskId`s is the only operation application code performs on them
/// directly (for example, filtering an observer's events down to one task
/// of interest).
#[derive(Clone, Copy, Debug, Eq, Hash, PartialEq)]
pub struct TaskId(pub(crate) u64);

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) struct KeyFingerprint {
	task_id: TaskId,
	input_type: TypeId,
	hash: u64,
}

/*
KeyFingerprint hashes as its precomputed `hash` field alone: that field
is a keyed SipHash already mixing the task id and the input, and slot
equality is always verified through the full typed DynKey comparison, so
the map's outer hash only routes lookups to buckets. Writing just the
one u64 is what lets FingerprintHasher be a bare passthrough.
*/
impl Hash for KeyFingerprint {
	fn hash<H: Hasher>(&self, state: &mut H) {
		state.write_u64(self.hash);
	}
}

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
		// KeyFingerprint only ever routes its precomputed hash through
		// write_u64; no byte writes reach this passthrough.
	}
}

/*
Process-global keyed hasher builder for fingerprints. std's RandomState
is keyed SipHash-1-3 seeded once per process, so task inputs — which can
be attacker-influenced in server contexts — cannot be pushed into
predictable bucket collisions from outside the process, and the per-
resolve cost stays in the ~10-15ns range on small inputs.
*/
static FINGERPRINT_HASH_STATE: LazyLock<RandomState> = LazyLock::new(RandomState::new);

// Compute a key fingerprint from a borrowed input — no clone. Equality
// of stored keys is always verified on the actual input value, so the
// hash only routes lookups.
pub(crate) fn fingerprint_for<I>(task_id: TaskId, value: &I) -> KeyFingerprint
where
	I: Eq + Hash + 'static,
{
	let mut hasher = FINGERPRINT_HASH_STATE.build_hasher();
	hasher.write_u64(task_id.0);
	value.hash(&mut hasher);
	let hash = hasher.finish();

	KeyFingerprint {
		task_id,
		input_type: TypeId::of::<I>(),
		hash,
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
