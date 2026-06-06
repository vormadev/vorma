use std::any::Any;
use std::collections::HashMap;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::time::Duration;

use tokio::sync::Notify;

use crate::clock::ClockInstant;
use crate::error::{Error, Result};
use crate::key::{DynKey, KeyData, KeyFingerprint};

const STORE_CLEANUP_INTERVAL: usize = 64;

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
				slots: HashMap::new(),
				entry_count: 0,
			}),
			max_entries,
		}
	}

	pub(crate) fn slot_for<I>(
		&self,
		key: &KeyData<I>,
		ttl: Option<Duration>,
		now: ClockInstant,
	) -> SlotLookup<E>
	where
		I: Clone + Eq + std::hash::Hash + Send + Sync + 'static,
	{
		let mut stale_slots_removed = 0usize;
		let cleanup_due = ttl.is_some()
			&& self
				.lookups
				.fetch_add(1, Ordering::Relaxed)
				.is_multiple_of(STORE_CLEANUP_INTERVAL);
		let mut state = self.state.lock().expect("task store lock poisoned");
		if cleanup_due {
			let removed = cleanup_expired_slots(&mut state.slots, now);
			state.entry_count = state.entry_count.saturating_sub(removed);
			stale_slots_removed += removed;
		}

		let fingerprint = key.fingerprint();
		let mut bucket_removed = 0usize;
		let mut remove_bucket = false;
		let mut found_slot = None;
		if let Some(bucket) = state.slots.get_mut(&fingerprint) {
			if ttl.is_some() && !cleanup_due {
				bucket_removed = retain_live_slots(bucket, now);
			}
			found_slot = bucket.iter().find(|slot| slot.matches(key)).cloned();
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
			let removed = cleanup_expired_slots(&mut state.slots, now);
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
			key: Arc::new(key.clone()),
			state: Mutex::new(SlotState::Empty),
			notify: Notify::new(),
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
	slots: HashMap<KeyFingerprint, Vec<Arc<Slot<E>>>>,
	entry_count: usize,
}

fn cleanup_expired_slots<E>(
	slots: &mut HashMap<KeyFingerprint, Vec<Arc<Slot<E>>>>,
	now: ClockInstant,
) -> usize {
	let mut stale_slots_removed = 0usize;
	slots.retain(|_, bucket| {
		stale_slots_removed += retain_live_slots(bucket, now);
		!bucket.is_empty()
	});
	stale_slots_removed
}

fn retain_live_slots<E>(bucket: &mut Vec<Arc<Slot<E>>>, now: ClockInstant) -> usize {
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
	pub(crate) notify: Notify,
}

impl<E> Slot<E> {
	fn matches<I>(&self, key: &KeyData<I>) -> bool
	where
		I: Eq + std::hash::Hash + Send + Sync + 'static,
	{
		self.key.fingerprint() == key.fingerprint()
			&& self
				.key
				.as_any()
				.downcast_ref::<I>()
				.is_some_and(|existing| existing == key.value())
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

	pub(crate) fn finish(&self, outcome: StoredOutcome<E>, expires_at: Option<ClockInstant>) {
		let mut state = self.state.lock().expect("slot state lock poisoned");
		*state = SlotState::Done {
			outcome,
			expires_at,
		};
		drop(state);
		self.notify.notify_waiters();
	}

	pub(crate) fn abandon(&self) {
		let mut state = self.state.lock().expect("slot state lock poisoned");
		if matches!(*state, SlotState::Running) {
			*state = SlotState::Empty;
		}
		drop(state);
		self.notify.notify_waiters();
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
