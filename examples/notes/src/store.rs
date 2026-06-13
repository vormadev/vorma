//! In-memory note store shared by handlers.

use std::sync::{Mutex, MutexGuard};

use serde::Serialize;

pub struct AppState {
	pub(crate) notes: Mutex<NoteStore>,
}

pub(crate) struct NoteStore {
	pub(crate) next_id: u64,
	pub(crate) notes: Vec<Note>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct Note {
	pub(crate) id: u64,
	pub(crate) body: String,
	pub(crate) tags: Vec<String>,
}

impl Note {
	pub(crate) fn new(id: u64, body: impl Into<String>) -> Self {
		let body = body.into();
		let tags = tags_in(&body);
		Self { id, body, tags }
	}
}

/// Hashtag-style tags parsed from a note body.
pub(crate) fn tags_in(body: &str) -> Vec<String> {
	let mut tags = body
		.split_whitespace()
		.filter_map(|word| {
			let tag = word.strip_prefix('#')?;
			let tag = tag.trim_matches(|c: char| !c.is_alphanumeric());
			if tag.is_empty() {
				return None;
			}
			Some(tag.to_ascii_lowercase())
		})
		.collect::<Vec<_>>();
	tags.sort();
	tags.dedup();
	tags
}

pub(crate) fn seed_state() -> AppState {
	AppState {
		notes: Mutex::new(NoteStore {
			next_id: 3,
			notes: vec![
				Note::new(1, "Keep the Rust API honest. #api"),
				Note::new(
					2,
					"Make the example prove the whole surface. #api #coverage",
				),
			],
		}),
	}
}

pub(crate) fn note_store(state: &AppState) -> vorma::Result<MutexGuard<'_, NoteStore>> {
	state
		.notes
		.lock()
		.map_err(|_| vorma::Error::new("note store lock poisoned"))
}
