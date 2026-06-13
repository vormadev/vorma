use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{SystemTime, UNIX_EPOCH};

use crate::contracts::{RouteTypeContract, TypeRefContract};

static TEST_TEMP_DIR_COUNTER: AtomicU64 = AtomicU64::new(0);

pub(crate) fn route_type_contract() -> RouteTypeContract {
	RouteTypeContract::new(TypeRefContract::Unit, TypeRefContract::Unknown)
}

pub(crate) fn unique_temp_root(prefix: &str) -> PathBuf {
	let nanos = SystemTime::now()
		.duration_since(UNIX_EPOCH)
		.unwrap()
		.as_nanos();
	let sequence = TEST_TEMP_DIR_COUNTER.fetch_add(1, Ordering::Relaxed);
	std::env::temp_dir().join(format!("{prefix}-{nanos}-{sequence}"))
}
