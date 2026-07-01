//! Loom model checks for the slot wait/notify protocol.
//!
//! Loom executes each model under every meaningfully distinct thread
//! interleaving the memory model allows, so the nanosecond races these
//! pin are constructed deterministically instead of waiting for the
//! scheduler to stumble into them. Run via `make loom-tasks`
//! (RUSTFLAGS="--cfg loom").
//!
//! The models drive the real [`crate::store`] types; the only modeled
//! stand-in is the slot signal, whose loom variant reproduces tokio
//! Notify's wake semantics (notification reaches only waiters that
//! registered beforehand, and no permit is stored).

use std::time::Duration;

use loom::sync::Arc;

use crate::clock::ClockInstant;
use crate::key::{TaskId, fingerprint_for};
use crate::store::{Slot, SlotClaim, SlotLookup, Store, StoredOutcome};

fn fresh_slot() -> (Arc<Store<()>>, std::sync::Arc<Slot<()>>) {
	let store = Arc::new(Store::<()>::new(None));
	let lookup = store.slot_for(fingerprint_for(TaskId(1), &7u32), &7u32, None, || {
		ClockInstant::from_duration_since_origin(Duration::ZERO)
	});
	let slot = match lookup {
		SlotLookup::Found { slot, .. } => slot,
		SlotLookup::CapacityBypass { .. } => unreachable!("store is unbounded"),
	};
	(store, slot)
}

fn finish_slot(slot: &Slot<()>) {
	slot.finish(StoredOutcome::Ok(std::sync::Arc::new(1u32)), None);
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
enum WaitOutcome {
	Ready,
	Reopened,
}

// Production waiter shape, exactly as wait_for_local runs it: register
// as a waiter, then per attempt capture the signal BEFORE re-checking
// state, and only then wait.
fn wait_register_first(slot: &Slot<()>) -> WaitOutcome {
	slot.register_waiter();
	loop {
		let seen = slot.signal.register();
		match slot.claim() {
			SlotClaim::Ready(_) => {
				slot.unregister_waiter();
				return WaitOutcome::Ready;
			}
			SlotClaim::Run => {
				// The runner abandoned; production waiters report
				// cancellation and leave future lookup to rerun.
				slot.abandon();
				slot.unregister_waiter();
				return WaitOutcome::Reopened;
			}
			SlotClaim::Wait => slot.signal.wait_past(seen),
		}
	}
}

/*
The permanent red pin for the lost-wakeup bug this review found: the
pre-fix ordering checked slot state BEFORE registering with the signal.
A runner finishing in that gap notifies nobody — the notification
carries no permit — and the waiter sleeps forever. Loom constructs that
schedule deterministically, detects that every thread is blocked, and
panics; this test passing-by-panicking proves the old ordering loses
wakeups in at least one schedule.
*/
#[test]
#[should_panic]
fn old_claim_first_ordering_loses_wakeups() {
	loom::model(|| {
		let (_store, slot) = fresh_slot();
		assert!(matches!(slot.claim(), SlotClaim::Run));

		let runner = {
			let slot = std::sync::Arc::clone(&slot);
			loom::thread::spawn(move || finish_slot(&slot))
		};

		// Old, buggy waiter shape: state check first, registration after.
		loop {
			match slot.claim() {
				SlotClaim::Ready(_) => break,
				SlotClaim::Run => {
					finish_slot(&slot);
					break;
				}
				SlotClaim::Wait => {
					slot.register_waiter();
					let seen = slot.signal.register();
					slot.signal.wait_past(seen);
					slot.unregister_waiter();
				}
			}
		}

		runner.join().expect("runner thread completes");
	});
}

// The fixed ordering: under every schedule, a waiter that registers
// before re-checking state always observes the finish.
#[test]
fn register_first_ordering_never_loses_wakeups() {
	loom::model(|| {
		let (_store, slot) = fresh_slot();
		assert!(matches!(slot.claim(), SlotClaim::Run));

		let runner = {
			let slot = std::sync::Arc::clone(&slot);
			loom::thread::spawn(move || finish_slot(&slot))
		};

		assert_eq!(wait_register_first(&slot), WaitOutcome::Ready);
		assert!(matches!(slot.claim(), SlotClaim::Ready(_)));

		runner.join().expect("runner thread completes");
	});
}

// Two concurrent resolvers race the first claim: one runs, the other
// waits; every schedule ends with both observing the completed slot.
#[test]
fn claim_race_coalesces_to_one_runner() {
	loom::model(|| {
		let (_store, slot) = fresh_slot();

		let contender = {
			let slot = std::sync::Arc::clone(&slot);
			loom::thread::spawn(move || {
				match slot.claim() {
					SlotClaim::Run => finish_slot(&slot),
					SlotClaim::Ready(_) => {}
					SlotClaim::Wait => assert_eq!(wait_register_first(&slot), WaitOutcome::Ready),
				}
				assert!(matches!(slot.claim(), SlotClaim::Ready(_)));
			})
		};

		match slot.claim() {
			SlotClaim::Run => finish_slot(&slot),
			SlotClaim::Ready(_) => {}
			SlotClaim::Wait => assert_eq!(wait_register_first(&slot), WaitOutcome::Ready),
		}
		assert!(matches!(slot.claim(), SlotClaim::Ready(_)));

		contender.join().expect("contender thread completes");
	});
}

// A runner that abandons wakes a waiter, which reopens the slot and
// reports cancellation — no schedule strands the waiter.
#[test]
fn abandon_wakes_waiter_and_reopens_slot() {
	loom::model(|| {
		let (_store, slot) = fresh_slot();
		assert!(matches!(slot.claim(), SlotClaim::Run));

		let runner = {
			let slot = std::sync::Arc::clone(&slot);
			loom::thread::spawn(move || slot.abandon())
		};

		assert_eq!(wait_register_first(&slot), WaitOutcome::Reopened);
		assert!(matches!(slot.claim(), SlotClaim::Run));

		runner.join().expect("runner thread completes");
	});
}

// Concurrent lookups for one task/input pair always converge on the
// same slot, so coalescing can never split across duplicates.
#[test]
fn concurrent_lookups_share_one_slot() {
	loom::model(|| {
		let store = Arc::new(Store::<()>::new(None));
		let lookup = |store: &Store<()>| match store.slot_for(
			fingerprint_for(TaskId(1), &7u32),
			&7u32,
			None,
			|| ClockInstant::from_duration_since_origin(Duration::ZERO),
		) {
			SlotLookup::Found { slot, .. } => slot,
			SlotLookup::CapacityBypass { .. } => unreachable!("store is unbounded"),
		};

		let other = {
			let store = Arc::clone(&store);
			loom::thread::spawn(move || lookup(&store))
		};
		let mine = lookup(&store);
		let theirs = other.join().expect("lookup thread completes");
		assert!(
			std::sync::Arc::ptr_eq(&mine, &theirs),
			"one task/input pair must resolve to one slot"
		);
	});
}

// Single-flight removes a running slot before publishing its result:
// existing waiters still observe the finish through their slot Arc, but
// later lookups do not reuse the completed value.
#[test]
fn remove_then_finish_wakes_waiters_and_reopens_future_lookup() {
	loom::model(|| {
		let (store, slot) = fresh_slot();
		assert!(matches!(slot.claim(), SlotClaim::Run));

		let waiter = {
			let slot = std::sync::Arc::clone(&slot);
			loom::thread::spawn(move || assert_eq!(wait_register_first(&slot), WaitOutcome::Ready))
		};

		assert!(store.remove_slot(&slot));
		finish_slot(&slot);
		waiter.join().expect("waiter thread completes");

		let lookup = store.slot_for(fingerprint_for(TaskId(1), &7u32), &7u32, None, || {
			ClockInstant::from_duration_since_origin(Duration::ZERO)
		});
		let next_slot = match lookup {
			SlotLookup::Found { slot, .. } => slot,
			SlotLookup::CapacityBypass { .. } => unreachable!("store is unbounded"),
		};
		assert!(!std::sync::Arc::ptr_eq(&slot, &next_slot));
		assert!(matches!(next_slot.claim(), SlotClaim::Run));
	});
}

// Shared-cache cancellation and panic cleanup removes the running slot
// before abandoning it: existing waiters still wake through their slot
// Arc, while future lookups must allocate a fresh slot.
#[test]
fn remove_then_abandon_wakes_waiters_and_reopens_future_lookup() {
	loom::model(|| {
		let (store, slot) = fresh_slot();
		assert!(matches!(slot.claim(), SlotClaim::Run));

		let waiter = {
			let slot = std::sync::Arc::clone(&slot);
			loom::thread::spawn(move || {
				assert_eq!(wait_register_first(&slot), WaitOutcome::Reopened)
			})
		};

		assert!(store.remove_slot(&slot));
		slot.abandon();
		waiter.join().expect("waiter thread completes");

		let lookup = store.slot_for(fingerprint_for(TaskId(1), &7u32), &7u32, None, || {
			ClockInstant::from_duration_since_origin(Duration::ZERO)
		});
		let next_slot = match lookup {
			SlotLookup::Found { slot, .. } => slot,
			SlotLookup::CapacityBypass { .. } => unreachable!("store is unbounded"),
		};
		assert!(!std::sync::Arc::ptr_eq(&slot, &next_slot));
		assert!(matches!(next_slot.claim(), SlotClaim::Run));
	});
}
