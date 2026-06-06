use std::alloc::Layout;
use std::sync::Mutex;

pub(crate) const STATUS_NO_MATCH: u32 = AbiStatus::NoMatch.code();
pub(crate) const STATUS_MATCH: u32 = AbiStatus::Match.code();
pub(crate) const STATUS_ERROR: u32 = AbiStatus::Error.code();
pub(crate) const MAX_INPUT_LEN: usize = 64 * 1024;
pub(crate) const MAX_LIVE_ALLOCATIONS: usize = 128;

static ALLOCATIONS: Mutex<Vec<Allocation>> = Mutex::new(Vec::new());

struct Allocation {
	ptr: usize,
	len: usize,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub(crate) enum AbiStatus {
	NoMatch,
	Match,
	Error,
}

impl AbiStatus {
	const fn code(self) -> u32 {
		match self {
			Self::NoMatch => 0,
			Self::Match => 1,
			Self::Error => 2,
		}
	}
}

impl From<AbiStatus> for u32 {
	fn from(status: AbiStatus) -> Self {
		status.code()
	}
}

pub(crate) fn alloc(len: usize) -> *mut u8 {
	if len > MAX_INPUT_LEN {
		return std::ptr::null_mut();
	}
	if len == 0 {
		return std::ptr::NonNull::<u8>::dangling().as_ptr();
	}

	let Ok(layout) = Layout::array::<u8>(len) else {
		return std::ptr::null_mut();
	};
	let mut allocations = ALLOCATIONS
		.lock()
		.expect("client matcher allocation registry should not be poisoned");
	if allocations.len() >= MAX_LIVE_ALLOCATIONS {
		return std::ptr::null_mut();
	}

	let ptr = unsafe { std::alloc::alloc(layout) };
	if ptr.is_null() {
		return std::ptr::null_mut();
	}
	allocations.push(Allocation {
		ptr: ptr as usize,
		len,
	});
	ptr
}

pub(crate) fn dealloc(ptr: *mut u8, len: usize) {
	if ptr.is_null() || len == 0 {
		return;
	}

	let mut allocations = ALLOCATIONS
		.lock()
		.expect("client matcher allocation registry should not be poisoned");
	let Some(index) = allocations
		.iter()
		.position(|allocation| allocation.ptr == ptr as usize && allocation.len == len)
	else {
		return;
	};
	allocations.swap_remove(index);
	drop(allocations);

	if let Ok(layout) = Layout::array::<u8>(len) {
		unsafe {
			std::alloc::dealloc(ptr, layout);
		}
	}
}

pub(crate) fn read_str<'a>(ptr: *const u8, len: usize) -> Option<&'a str> {
	if len > MAX_INPUT_LEN {
		return None;
	}
	if len == 0 {
		return Some("");
	}
	if ptr.is_null() {
		return None;
	}

	let bytes = unsafe { std::slice::from_raw_parts(ptr, len) };
	std::str::from_utf8(bytes).ok()
}
