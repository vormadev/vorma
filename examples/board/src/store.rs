//! SQLite store: pooled connections, embedded schema, idempotent seed.
/*
rusqlite is synchronous, so every database touch happens inside
`tokio::task::spawn_blocking`, reached through the domain-task layer in
`repo`. The pool is a fixed ring of connections in WAL mode: SQLite
gives many concurrent readers and one writer, which matches this app's
read-heavy shape.
*/

use std::path::Path;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex, MutexGuard};

use rusqlite::Connection;

const SCHEMA: &str = include_str!("schema.sql");
const POOL_SIZE: usize = 4;

/// Application state shared by every handler.
pub struct AppState {
	pub db: Arc<Db>,
}

pub struct Db {
	connections: Vec<Mutex<Connection>>,
	next: AtomicUsize,
}

impl Db {
	/// Open (or create) the database file, apply schema, seed once.
	pub fn open(path: &Path) -> vorma::Result<Arc<Self>> {
		let mut connections = Vec::with_capacity(POOL_SIZE);
		for _ in 0..POOL_SIZE {
			let connection = Connection::open(path).map_err(db_err)?;
			connection
				.pragma_update(None, "journal_mode", "WAL")
				.map_err(db_err)?;
			connection
				.pragma_update(None, "foreign_keys", "ON")
				.map_err(db_err)?;
			connection
				.pragma_update(None, "busy_timeout", 5_000)
				.map_err(db_err)?;
			connections.push(Mutex::new(connection));
		}
		let db = Self {
			connections,
			next: AtomicUsize::new(0),
		};
		db.with(|conn| conn.execute_batch(SCHEMA)).map_err(db_err)?;
		Ok(Arc::new(db))
	}

	/// Run a closure on a pooled connection (callers are inside
	/// spawn_blocking; the mutex wait is a blocking wait by design).
	pub fn with<T>(
		&self,
		f: impl FnOnce(&Connection) -> rusqlite::Result<T>,
	) -> rusqlite::Result<T> {
		f(&self.acquire())
	}

	fn acquire(&self) -> MutexGuard<'_, Connection> {
		let start = self.next.fetch_add(1, Ordering::Relaxed);
		for offset in 0..self.connections.len() {
			let index = (start + offset) % self.connections.len();
			if let Ok(guard) = self.connections[index].try_lock() {
				return guard;
			}
		}
		#[expect(
			clippy::unwrap_used,
			reason = "connection mutexes are never poisoned: closures cannot panic without aborting the blocking task first"
		)]
		self.connections[start % self.connections.len()]
			.lock()
			.unwrap()
	}
}

pub(crate) fn db_err(source: rusqlite::Error) -> vorma::Error {
	vorma::Error::new("database operation failed").with_source(source)
}
