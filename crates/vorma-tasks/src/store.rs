use std::any::Any;
use std::hash::Hash;
use std::sync::Arc;
use std::time::Duration;

use rustc_hash::FxHashMap;
use smallvec::SmallVec;

use crate::sync::{AtomicUsize, Mutex, Ordering};

use crate::clock::ClockInstant;
use crate::error::{Error, Result};
use crate::key::{DynKey, KeyData, KeyFingerprint};

const STORE_CLEANUP_INTERVAL: usize = 64;

// Distinct inputs rarely collide on a fingerprint, so buckets hold
// their single slot inline.
type SlotBucket<E> = SmallVec<[Arc<Slot<E>>; 1]>;

pub(crate) struct Store<E> {
	lookups: AtomicUsize,
	state: Mutex<StoreState<E>>,
	max_entries: Option<usize>,
}

impl<E> Store<E> {
	pub(crate) fn new(max_entries: Option<usize>) -> Self {
		Self {
			lookups: AtomicUsize::new(0),
			state: Mutex::new(StoreState {
				slots: FxHashMap::default(),
				entry_count: 0,
			}),
			max_entries,
		}
	}

	// Look up or create the slot for one task/input pair. The input is
	// borrowed for the lookup and cloned only when a new slot must be
	// created; `now` is consulted only for TTL stores.
	pub(crate) fn slot_for<I>(
		&self,
		fingerprint: KeyFingerprint,
		value: &I,
		ttl: Option<Duration>,
		now: impl Fn() -> ClockInstant,
	) -> SlotLookup<E>
	where
		I: Clone + Eq + Hash + Send + Sync + 'static,
	{
		let mut stale_slots_removed = 0usize;
		let cleanup_due = ttl.is_some()
			&& self
				.lookups
				.fetch_add(1, Ordering::Relaxed)
				.is_multiple_of(STORE_CLEANUP_INTERVAL);
		let mut state = self.state.lock().expect("task store lock poisoned");
		if cleanup_due {
			let removed = cleanup_expired_slots(&mut state.slots, now());
			state.entry_count = state.entry_count.saturating_sub(removed);
			stale_slots_removed += removed;
		}

		let mut bucket_removed = 0usize;
		let mut remove_bucket = false;
		let mut found_slot = None;
		if let Some(bucket) = state.slots.get_mut(&fingerprint) {
			if ttl.is_some() && !cleanup_due {
				bucket_removed = retain_live_slots(bucket, now());
			}
			found_slot = bucket
				.iter()
				.find(|slot| slot.matches(fingerprint, value))
				.cloned();
			remove_bucket = bucket.is_empty();
		}
		state.entry_count = state.entry_count.saturating_sub(bucket_removed);
		stale_slots_removed += bucket_removed;
		if remove_bucket {
			state.slots.remove(&fingerprint);
		}

		if let Some(slot) = found_slot {
			return SlotLookup::Found {
				slot,
				stale_slots_removed,
			};
		}

		if let Some(max_entries) = self.max_entries
			&& state.entry_count >= max_entries
		{
			let removed = cleanup_expired_slots(&mut state.slots, now());
			state.entry_count = state.entry_count.saturating_sub(removed);
			stale_slots_removed += removed;
			if state.entry_count >= max_entries {
				return SlotLookup::CapacityBypass {
					stale_slots_removed,
					max_entries,
				};
			}
		}

		let slot = Arc::new(Slot {
			key: Arc::new(KeyData::from_parts(value.clone(), fingerprint)),
			state: Mutex::new(SlotState::Empty),
			waiters: AtomicUsize::new(0),
			signal: SlotSignal::new(),
		});
		state
			.slots
			.entry(fingerprint)
			.or_default()
			.push(slot.clone());
		state.entry_count += 1;
		SlotLookup::Found {
			slot,
			stale_slots_removed,
		}
	}
}

struct StoreState<E> {
	slots: FxHashMap<KeyFingerprint, SlotBucket<E>>,
	entry_count: usize,
}

fn cleanup_expired_slots<E>(
	slots: &mut FxHashMap<KeyFingerprint, SlotBucket<E>>,
	now: ClockInstant,
) -> usize {
	let mut stale_slots_removed = 0usize;
	slots.retain(|_, bucket| {
		stale_slots_removed += retain_live_slots(bucket, now);
		!bucket.is_empty()
	});
	stale_slots_removed
}

fn retain_live_slots<E>(bucket: &mut SlotBucket<E>, now: ClockInstant) -> usize {
	let before = bucket.len();
	bucket.retain(|slot| !slot.is_expired(now));
	before - bucket.len()
}

pub(crate) enum SlotLookup<E> {
	Found {
		slot: Arc<Slot<E>>,
		stale_slots_removed: usize,
	},
	CapacityBypass {
		stale_slots_removed: usize,
		max_entries: usize,
	},
}

pub(crate) struct Slot<E> {
	key: Arc<dyn DynKey>,
	state: Mutex<SlotState<E>>,
	// Number of registered waiters; finish and abandon skip the notify
	// handshake entirely when nobody is waiting.
	waiters: AtomicUsize,
	pub(crate) signal: SlotSignal,
}

impl<E> Slot<E> {
	fn matches<I>(&self, fingerprint: KeyFingerprint, value: &I) -> bool
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

	// The slot's own key allocation, shared into execution paths for
	// cycle detection.
	pub(crate) fn shared_key(&self) -> Arc<dyn DynKey> {
		self.key.clone()
	}

	fn is_expired(&self, now: ClockInstant) -> bool {
		let state = self.state.lock().expect("slot state lock poisoned");
		match &*state {
			SlotState::Done {
				expires_at: Some(expires_at),
				..
			} => *expires_at <= now,
			_ => false,
		}
	}

	pub(crate) fn claim(&self) -> SlotClaim<E> {
		let mut state = self.state.lock().expect("slot state lock poisoned");
		match &*state {
			SlotState::Done { outcome, .. } => SlotClaim::Ready(outcome.clone()),
			SlotState::Running => SlotClaim::Wait,
			SlotState::Empty => {
				*state = SlotState::Running;
				SlotClaim::Run
			}
		}
	}

	// Register intent to wait before re-checking state; finish and
	// abandon only notify registered waiters.
	pub(crate) fn register_waiter(&self) {
		self.waiters.fetch_add(1, Ordering::AcqRel);
	}

	pub(crate) fn unregister_waiter(&self) {
		self.waiters.fetch_sub(1, Ordering::AcqRel);
	}

	pub(crate) fn finish(&self, outcome: StoredOutcome<E>, expires_at: Option<ClockInstant>) {
		let mut state = self.state.lock().expect("slot state lock poisoned");
		*state = SlotState::Done {
			outcome,
			expires_at,
		};
		drop(state);
		if self.waiters.load(Ordering::Acquire) > 0 {
			self.signal.notify_registered();
		}
	}

	pub(crate) fn abandon(&self) {
		let mut state = self.state.lock().expect("slot state lock poisoned");
		if matches!(*state, SlotState::Running) {
			*state = SlotState::Empty;
		}
		drop(state);
		if self.waiters.load(Ordering::Acquire) > 0 {
			self.signal.notify_registered();
		}
	}
}

enum SlotState<E> {
	Empty,
	Running,
	Done {
		outcome: StoredOutcome<E>,
		expires_at: Option<ClockInstant>,
	},
}

pub(crate) enum SlotClaim<E> {
	Ready(StoredOutcome<E>),
	Wait,
	Run,
}

pub(crate) enum StoredOutcome<E> {
	Ok(Arc<dyn Any + Send + Sync>),
	Err(Error<E>),
}

impl<E> StoredOutcome<E> {
	pub(crate) fn is_ok(&self) -> bool {
		matches!(self, Self::Ok(_))
	}

	pub(crate) fn is_cancelled(&self) -> bool {
		matches!(self, Self::Err(error) if error.is_cancelled())
	}
}

impl<E> Clone for StoredOutcome<E> {
	fn clone(&self) -> Self {
		match self {
			Self::Ok(value) => Self::Ok(value.clone()),
			Self::Err(error) => Self::Err(error.clone()),
		}
	}
}

pub(crate) struct RunningGuard<E> {
	slot: Arc<Slot<E>>,
	active: bool,
}

impl<E> RunningGuard<E> {
	pub(crate) fn new(slot: Arc<Slot<E>>) -> Self {
		Self { slot, active: true }
	}

	pub(crate) fn disarm(mut self) {
		self.active = false;
	}
}

impl<E> Drop for RunningGuard<E> {
	fn drop(&mut self) {
		if self.active {
			self.slot.abandon();
		}
	}
}

/*
The slot wake signal. In production it is tokio's Notify, used with the
register-then-recheck pattern (Notified::enable before re-checking slot
state). Under loom it is an epoch/condvar pair with the same wake
semantics — notify wakes only waiters registered before the bump, and
no permit is stored — so the loom models exercise the exact protocol
the async code runs.
*/
#[cfg(not(loom))]
pub(crate) struct SlotSignal {
	notify: tokio::sync::Notify,
}

#[cfg(not(loom))]
impl SlotSignal {
	pub(crate) fn new() -> Self {
		Self {
			notify: tokio::sync::Notify::new(),
		}
	}

	pub(crate) fn notified(&self) -> tokio::sync::futures::Notified<'_> {
		self.notify.notified()
	}

	pub(crate) fn notify_registered(&self) {
		self.notify.notify_waiters();
	}
}

#[cfg(loom)]
pub(crate) struct SlotSignal {
	epoch: loom::sync::Mutex<u64>,
	wake: loom::sync::Condvar,
}

#[cfg(loom)]
impl SlotSignal {
	pub(crate) fn new() -> Self {
		Self {
			epoch: loom::sync::Mutex::new(0),
			wake: loom::sync::Condvar::new(),
		}
	}

	// Mirror of Notified::enable: capture the current epoch; a later
	// wait observes only notifications that bump past it.
	pub(crate) fn register(&self) -> u64 {
		*self.epoch.lock().expect("signal lock poisoned")
	}

	pub(crate) fn wait_past(&self, seen: u64) {
		let mut epoch = self.epoch.lock().expect("signal lock poisoned");
		while *epoch == seen {
			epoch = self.wake.wait(epoch).expect("signal lock poisoned");
		}
	}

	pub(crate) fn notify_registered(&self) {
		let mut epoch = self.epoch.lock().expect("signal lock poisoned");
		*epoch += 1;
		self.wake.notify_all();
	}
}

pub(crate) fn decode_outcome<O, E>(
	outcome: StoredOutcome<E>,
	task_name: &'static str,
) -> Result<Arc<O>, E>
where
	O: Send + Sync + 'static,
{
	match outcome {
		StoredOutcome::Ok(value) => value
			.downcast::<O>()
			.map_err(|_| Error::TypeMismatch { task: task_name }),
		StoredOutcome::Err(error) => Err(error),
	}
}
