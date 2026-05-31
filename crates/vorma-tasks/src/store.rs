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
	slots: Mutex<HashMap<KeyFingerprint, Vec<Arc<Slot<E>>>>>,
}

impl<E> Store<E> {
	pub(crate) fn new() -> Self {
		Self {
			lookups: AtomicUsize::new(0),
			slots: Mutex::new(HashMap::new()),
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
		let mut slots = self.slots.lock().expect("task store lock poisoned");
		if cleanup_due {
			stale_slots_removed += cleanup_expired_slots(&mut slots, now);
		}
		let bucket = slots.entry(key.fingerprint()).or_default();
		if ttl.is_some() && !cleanup_due {
			stale_slots_removed += retain_live_slots(bucket, now);
		}

		if let Some(slot) = bucket.iter().find(|slot| slot.matches(key)).cloned() {
			return SlotLookup {
				slot,
				stale_slots_removed,
			};
		}

		let slot = Arc::new(Slot {
			key: Arc::new(key.clone()),
			state: Mutex::new(SlotState::Empty),
			notify: Notify::new(),
		});
		bucket.push(slot.clone());
		SlotLookup {
			slot,
			stale_slots_removed,
		}
	}
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

pub(crate) struct SlotLookup<E> {
	pub(crate) slot: Arc<Slot<E>>,
	pub(crate) stale_slots_removed: usize,
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
