pub(crate) const STATUS_NO_MATCH: u32 = AbiStatus::NoMatch.code();
pub(crate) const STATUS_MATCH: u32 = AbiStatus::Match.code();
pub(crate) const STATUS_ERROR: u32 = AbiStatus::Error.code();

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
	let mut buffer = Vec::<u8>::with_capacity(len);
	let ptr = buffer.as_mut_ptr();
	std::mem::forget(buffer);
	ptr
}

#[allow(clippy::not_unsafe_ptr_arg_deref)]
pub(crate) fn dealloc(ptr: *mut u8, len: usize) {
	if ptr.is_null() {
		return;
	}

	unsafe {
		drop(Vec::from_raw_parts(ptr, 0, len));
	}
}

pub(crate) fn read_str<'a>(ptr: *const u8, len: usize) -> Option<&'a str> {
	if len == 0 {
		return Some("");
	}
	if ptr.is_null() {
		return None;
	}

	let bytes = unsafe { std::slice::from_raw_parts(ptr, len) };
	std::str::from_utf8(bytes).ok()
}
