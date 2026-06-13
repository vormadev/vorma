//! Domain operations over the store.
/*
READS are `Task`s: the dedupe key is (task identity, typed input), so a
request that needs the same fact twice — middleware preloading the
current user, the story page loading story/comments/author in
parallel — pays for one database read. TTLs are cache POLICY per domain
operation: `Duration::ZERO` (request-scoped dedupe only) is the default
for anything a user can mutate; only deliberately stale-tolerant reads
get a nonzero TTL.

WRITES are plain async fns — mutations must never dedupe.
*/

use std::sync::{Arc, LazyLock};
use std::time::Duration;

use rusqlite::{Connection, OptionalExtension, params};
use serde::Serialize;
use vorma::Task;

use crate::store::{Db, db_err};

/*
FINDING (ledger F-1): task bodies must return Result<O, TaskError<E>>,
so every fallible body hand-wraps its error. This helper exists ONLY to
absorb that friction — with From<E> on TaskError, `?` would just work
and this would be `blocking` alone.
*/
async fn blocking_task<T, F>(db: Arc<Db>, f: F) -> Result<T, vorma::TaskError<vorma::Error>>
where
	T: Send + 'static,
	F: FnOnce(&Connection) -> rusqlite::Result<T> + Send + 'static,
{
	blocking(db, f)
		.await
		.map_err(|error| vorma::TaskError::Failed(std::sync::Arc::new(error)))
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct User {
	pub id: i64,
	pub username: String,
	pub banned: bool,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct Story {
	pub id: i64,
	pub title: String,
	pub url: Option<String>,
	pub body: Option<String>,
	pub author: String,
	pub points: i64,
	pub comment_count: i64,
	pub created_at: i64,
	pub killed: bool,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct Comment {
	pub id: i64,
	pub parent_id: Option<i64>,
	pub author: String,
	pub body: String,
	pub created_at: i64,
	pub killed: bool,
}

/// Input for the front-page task: (db handle is ambient via input).
#[derive(Clone)]
pub struct DbInput<I> {
	pub db: Arc<Db>,
	pub input: I,
}

/*
Task inputs must be the dedupe key. The Db handle rides along but two
requests against the same process share one Db, so keying on the typed
input alone is correct; the wrapper exists because tasks take exactly
one input value.
*/
impl<I: std::hash::Hash> std::hash::Hash for DbInput<I> {
	fn hash<H: std::hash::Hasher>(&self, state: &mut H) {
		self.input.hash(state);
	}
}
impl<I: PartialEq> PartialEq for DbInput<I> {
	fn eq(&self, other: &Self) -> bool {
		self.input == other.input
	}
}
impl<I: Eq> Eq for DbInput<I> {}

async fn blocking<T, F>(db: Arc<Db>, f: F) -> vorma::Result<T>
where
	T: Send + 'static,
	F: FnOnce(&Connection) -> rusqlite::Result<T> + Send + 'static,
{
	tokio::task::spawn_blocking(move || db.with(f).map_err(db_err))
		.await
		.map_err(|source| vorma::Error::new("database task join failed").with_source(source))?
}

const STORY_COLUMNS: &str = "s.id, s.title, s.url, s.body, u.username, \
	(SELECT COUNT(*) FROM votes v WHERE v.story_id = s.id), \
	(SELECT COUNT(*) FROM comments c WHERE c.story_id = s.id AND c.killed = 0), \
	s.created_at, s.killed";

fn story_from_row(row: &rusqlite::Row<'_>) -> rusqlite::Result<Story> {
	Ok(Story {
		id: row.get(0)?,
		title: row.get(1)?,
		url: row.get(2)?,
		body: row.get(3)?,
		author: row.get(4)?,
		points: row.get(5)?,
		comment_count: row.get(6)?,
		created_at: row.get(7)?,
		killed: row.get::<_, i64>(8)? != 0,
	})
}

pub const FRONT_PAGE_SIZE: i64 = 30;

/// Score-ranked front page, one page at a time.
pub static STORIES_PAGE: LazyLock<Task<DbInput<i64>, Vec<Story>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<i64>| async move {
			let page = arg.input.max(1);
			blocking_task(arg.db, move |conn| {
				let mut statement = conn.prepare(&format!(
					"SELECT {STORY_COLUMNS} FROM stories s \
					JOIN users u ON u.id = s.author_id \
					WHERE s.killed = 0 \
					ORDER BY (SELECT COUNT(*) FROM votes v WHERE v.story_id = s.id) DESC, \
						s.created_at DESC \
					LIMIT ?1 OFFSET ?2"
				))?;
				let rows = statement.query_map(
					params![FRONT_PAGE_SIZE, (page - 1) * FRONT_PAGE_SIZE],
					story_from_row,
				)?;
				rows.collect()
			})
			.await
		})
	});

pub static STORY_BY_ID: LazyLock<Task<DbInput<i64>, Option<Story>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<i64>| async move {
			let id = arg.input;
			blocking_task(arg.db, move |conn| {
				conn.query_row(
					&format!(
						"SELECT {STORY_COLUMNS} FROM stories s \
						JOIN users u ON u.id = s.author_id WHERE s.id = ?1"
					),
					params![id],
					story_from_row,
				)
				.optional()
			})
			.await
		})
	});

pub static COMMENTS_FOR_STORY: LazyLock<Task<DbInput<i64>, Vec<Comment>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<i64>| async move {
			let story_id = arg.input;
			blocking_task(arg.db, move |conn| {
				let mut statement = conn.prepare(
					"SELECT c.id, c.parent_id, u.username, c.body, c.created_at, c.killed \
					FROM comments c JOIN users u ON u.id = c.author_id \
					WHERE c.story_id = ?1 ORDER BY c.created_at ASC",
				)?;
				let rows = statement.query_map(params![story_id], |row| {
					Ok(Comment {
						id: row.get(0)?,
						parent_id: row.get(1)?,
						author: row.get(2)?,
						body: row.get(3)?,
						created_at: row.get(4)?,
						killed: row.get::<_, i64>(5)? != 0,
					})
				})?;
				rows.collect()
			})
			.await
		})
	});

/// Session token -> user, the per-request preload shared by middleware
/// and every handler that asks again.
pub static USER_FOR_SESSION: LazyLock<Task<DbInput<String>, Option<User>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<String>| async move {
			let token = arg.input;
			blocking_task(arg.db, move |conn| {
				conn.query_row(
					"SELECT u.id, u.username, u.banned FROM sessions s \
					JOIN users u ON u.id = s.user_id WHERE s.token = ?1",
					params![token],
					|row| {
						Ok(User {
							id: row.get(0)?,
							username: row.get(1)?,
							banned: row.get::<_, i64>(2)? != 0,
						})
					},
				)
				.optional()
			})
			.await
		})
	});

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct UserProfile {
	pub username: String,
	pub karma: i64,
	pub story_count: i64,
	pub comment_count: i64,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct DocsPage {
	pub slug: String,
	pub title: String,
	pub body: String,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct ModLogEntry {
	pub id: i64,
	pub moderator: String,
	pub story_id: i64,
	pub action: String,
	pub created_at: i64,
}

pub static USER_PROFILE: LazyLock<Task<DbInput<String>, Option<UserProfile>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<String>| async move {
			let username = arg.input;
			blocking_task(arg.db, move |conn| {
				conn.query_row(
					"SELECT u.username, 					(SELECT COUNT(*) FROM votes v JOIN stories s ON s.id = v.story_id 						WHERE s.author_id = u.id), 					(SELECT COUNT(*) FROM stories s WHERE s.author_id = u.id AND s.killed = 0), 					(SELECT COUNT(*) FROM comments c WHERE c.author_id = u.id AND c.killed = 0) 					FROM users u WHERE u.username = ?1",
					params![username],
					|row| {
						Ok(UserProfile {
							username: row.get(0)?,
							karma: row.get(1)?,
							story_count: row.get(2)?,
							comment_count: row.get(3)?,
						})
					},
				)
				.optional()
			})
			.await
		})
	});

pub static STORIES_BY_AUTHOR: LazyLock<Task<DbInput<String>, Vec<Story>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<String>| async move {
			let username = arg.input;
			blocking_task(arg.db, move |conn| {
				let mut statement = conn.prepare(&format!(
					"SELECT {STORY_COLUMNS} FROM stories s 					JOIN users u ON u.id = s.author_id 					WHERE u.username = ?1 AND s.killed = 0 					ORDER BY s.created_at DESC LIMIT 50"
				))?;
				let rows = statement.query_map(params![username], story_from_row)?;
				rows.collect()
			})
			.await
		})
	});

pub static COMMENTS_BY_AUTHOR: LazyLock<Task<DbInput<String>, Vec<Comment>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<String>| async move {
			let username = arg.input;
			blocking_task(arg.db, move |conn| {
				let mut statement = conn.prepare(
					"SELECT c.id, c.parent_id, u.username, c.body, c.created_at, c.killed 					FROM comments c JOIN users u ON u.id = c.author_id 					WHERE u.username = ?1 AND c.killed = 0 					ORDER BY c.created_at DESC LIMIT 50",
				)?;
				let rows = statement.query_map(params![username], |row| {
					Ok(Comment {
						id: row.get(0)?,
						parent_id: row.get(1)?,
						author: row.get(2)?,
						body: row.get(3)?,
						created_at: row.get(4)?,
						killed: row.get::<_, i64>(5)? != 0,
					})
				})?;
				rows.collect()
			})
			.await
		})
	});

/*
Docs pages are seeded content that only changes on redeploy or
seed-data edits: a deliberately stale-tolerant nonzero TTL, the one
place this app wants cross-request task caching.
*/
pub static DOCS_PAGE: LazyLock<Task<DbInput<String>, Option<DocsPage>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(
			Duration::from_secs(30),
			|_ctx, arg: DbInput<String>| async move {
				let slug = arg.input;
				blocking_task(arg.db, move |conn| {
					conn.query_row(
						"SELECT slug, title, body FROM docs_pages WHERE slug = ?1",
						params![slug],
						|row| {
							Ok(DocsPage {
								slug: row.get(0)?,
								title: row.get(1)?,
								body: row.get(2)?,
							})
						},
					)
					.optional()
				})
				.await
			},
		)
	});

pub static DOCS_INDEX: LazyLock<Task<DbInput<()>, Vec<DocsPage>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(
			Duration::from_secs(30),
			|_ctx, arg: DbInput<()>| async move {
				blocking_task(arg.db, move |conn| {
					let mut statement =
						conn.prepare("SELECT slug, title, body FROM docs_pages ORDER BY slug")?;
					let rows = statement.query_map([], |row| {
						Ok(DocsPage {
							slug: row.get(0)?,
							title: row.get(1)?,
							body: row.get(2)?,
						})
					})?;
					rows.collect()
				})
				.await
			},
		)
	});

#[derive(Clone, Debug, Default, Eq, Hash, PartialEq)]
pub struct SearchKey {
	pub q: String,
	pub page: i64,
}

pub static SEARCH_STORIES: LazyLock<Task<DbInput<SearchKey>, Vec<Story>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<SearchKey>| async move {
			let SearchKey { q, page } = arg.input;
			let page = page.max(1);
			blocking_task(arg.db, move |conn| {
				let needle = format!("%{q}%");
				let mut statement = conn.prepare(&format!(
					"SELECT {STORY_COLUMNS} FROM stories s 					JOIN users u ON u.id = s.author_id 					WHERE s.killed = 0 AND (s.title LIKE ?1 OR s.body LIKE ?1) 					ORDER BY s.created_at DESC LIMIT ?2 OFFSET ?3"
				))?;
				let rows = statement.query_map(
					params![needle, FRONT_PAGE_SIZE, (page - 1) * FRONT_PAGE_SIZE],
					story_from_row,
				)?;
				rows.collect()
			})
			.await
		})
	});

pub static KILLED_STORIES: LazyLock<Task<DbInput<()>, Vec<Story>, vorma::Error>> = LazyLock::new(
	|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<()>| async move {
			blocking_task(arg.db, move |conn| {
				let mut statement = conn.prepare(&format!(
					"SELECT {STORY_COLUMNS} FROM stories s 					JOIN users u ON u.id = s.author_id 					WHERE s.killed = 1 ORDER BY s.created_at DESC LIMIT 100"
				))?;
				let rows = statement.query_map([], story_from_row)?;
				rows.collect()
			})
			.await
		})
	},
);

pub static MOD_LOG: LazyLock<Task<DbInput<()>, Vec<ModLogEntry>, vorma::Error>> = LazyLock::new(
	|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<()>| async move {
			blocking_task(arg.db, move |conn| {
				let mut statement = conn.prepare(
					"SELECT m.id, u.username, m.story_id, m.action, m.created_at 					FROM mod_log m JOIN users u ON u.id = m.moderator_id 					ORDER BY m.created_at DESC, m.id DESC LIMIT 100",
				)?;
				let rows = statement.query_map([], |row| {
					Ok(ModLogEntry {
						id: row.get(0)?,
						moderator: row.get(1)?,
						story_id: row.get(2)?,
						action: row.get(3)?,
						created_at: row.get(4)?,
					})
				})?;
				rows.collect()
			})
			.await
		})
	},
);

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct Attachment {
	pub file_name: String,
	pub content_type: String,
	#[serde(skip)]
	pub body: Vec<u8>,
}

pub static ATTACHMENT_FOR_STORY: LazyLock<Task<DbInput<i64>, Option<Attachment>, vorma::Error>> =
	LazyLock::new(|| {
		Task::new(Duration::ZERO, |_ctx, arg: DbInput<i64>| async move {
			let story_id = arg.input;
			blocking_task(arg.db, move |conn| {
				conn.query_row(
					"SELECT file_name, content_type, body FROM attachments WHERE story_id = ?1",
					params![story_id],
					|row| {
						Ok(Attachment {
							file_name: row.get(0)?,
							content_type: row.get(1)?,
							body: row.get(2)?,
						})
					},
				)
				.optional()
			})
			.await
		})
	});

// ---- writes: plain async fns, never deduped ----

pub async fn login(db: Arc<Db>, username: String, token: String) -> vorma::Result<User> {
	blocking(db, move |conn| {
		conn.execute(
			"INSERT INTO users (username) VALUES (?1) ON CONFLICT(username) DO NOTHING",
			params![username],
		)?;
		let user = conn.query_row(
			"SELECT id, username, banned FROM users WHERE username = ?1",
			params![username],
			|row| {
				Ok(User {
					id: row.get(0)?,
					username: row.get(1)?,
					banned: row.get::<_, i64>(2)? != 0,
				})
			},
		)?;
		conn.execute(
			"INSERT INTO sessions (token, user_id) VALUES (?1, ?2)",
			params![token, user.id],
		)?;
		Ok(user)
	})
	.await
}

pub async fn logout(db: Arc<Db>, token: String) -> vorma::Result<()> {
	blocking(db, move |conn| {
		conn.execute("DELETE FROM sessions WHERE token = ?1", params![token])?;
		Ok(())
	})
	.await
}

pub async fn create_story(
	db: Arc<Db>,
	author_id: i64,
	title: String,
	url: Option<String>,
	body: Option<String>,
) -> vorma::Result<i64> {
	blocking(db, move |conn| {
		conn.execute(
			"INSERT INTO stories (author_id, title, url, body) VALUES (?1, ?2, ?3, ?4)",
			params![author_id, title, url, body],
		)?;
		Ok(conn.last_insert_rowid())
	})
	.await
}

/// Returns false when the UNIQUE(user, story) constraint rejects a
/// duplicate vote — the caller turns that into a client-visible
/// rejection.
pub async fn vote_story(db: Arc<Db>, user_id: i64, story_id: i64) -> vorma::Result<bool> {
	blocking(db, move |conn| {
		let inserted = conn.execute(
			"INSERT OR IGNORE INTO votes (user_id, story_id) VALUES (?1, ?2)",
			params![user_id, story_id],
		)?;
		Ok(inserted == 1)
	})
	.await
}

pub async fn create_comment(
	db: Arc<Db>,
	story_id: i64,
	parent_id: Option<i64>,
	author_id: i64,
	body: String,
) -> vorma::Result<i64> {
	blocking(db, move |conn| {
		conn.execute(
			"INSERT INTO comments (story_id, parent_id, author_id, body) \
			VALUES (?1, ?2, ?3, ?4)",
			params![story_id, parent_id, author_id, body],
		)?;
		Ok(conn.last_insert_rowid())
	})
	.await
}

pub async fn save_attachment(
	db: Arc<Db>,
	story_id: i64,
	file_name: String,
	content_type: String,
	body: Vec<u8>,
) -> vorma::Result<()> {
	blocking(db, move |conn| {
		conn.execute(
			"INSERT INTO attachments (story_id, file_name, content_type, body) \
			VALUES (?1, ?2, ?3, ?4)",
			params![story_id, file_name, content_type, body],
		)?;
		Ok(())
	})
	.await
}

/// Returns false when the story does not exist.
pub async fn set_story_killed(
	db: Arc<Db>,
	moderator_id: i64,
	story_id: i64,
	killed: bool,
) -> vorma::Result<bool> {
	blocking(db, move |conn| {
		let changed = conn.execute(
			"UPDATE stories SET killed = ?2 WHERE id = ?1",
			params![story_id, killed as i64],
		)?;
		if changed == 1 {
			conn.execute(
				"INSERT INTO mod_log (moderator_id, story_id, action) VALUES (?1, ?2, ?3)",
				params![
					moderator_id,
					story_id,
					if killed { "kill" } else { "restore" }
				],
			)?;
		}
		Ok(changed == 1)
	})
	.await
}

/// Idempotent seed: docs pages from seed-data markdown files. The
/// first heading line becomes the title; slugs mirror the file paths.
pub fn seed_docs(db: &Db, seed_dir: &std::path::Path) -> vorma::Result<()> {
	let mut pages = Vec::new();
	collect_doc_pages(seed_dir, "", &mut pages)?;
	db.with(|conn| {
		for (slug, title, body) in &pages {
			conn.execute(
				"INSERT INTO docs_pages (slug, title, body) VALUES (?1, ?2, ?3) \
				ON CONFLICT(slug) DO UPDATE SET title = excluded.title, body = excluded.body",
				params![slug, title, body],
			)?;
		}
		Ok(())
	})
	.map_err(crate::store::db_err)
}

fn collect_doc_pages(
	dir: &std::path::Path,
	prefix: &str,
	pages: &mut Vec<(String, String, String)>,
) -> vorma::Result<()> {
	let entries = std::fs::read_dir(dir)
		.map_err(|source| vorma::Error::new("read seed-data docs dir").with_source(source))?;
	for entry in entries {
		let entry = entry
			.map_err(|source| vorma::Error::new("read seed-data entry").with_source(source))?;
		let path = entry.path();
		let name = entry.file_name().to_string_lossy().into_owned();
		if path.is_dir() {
			let nested = if prefix.is_empty() {
				name
			} else {
				format!("{prefix}/{name}")
			};
			collect_doc_pages(&path, &nested, pages)?;
			continue;
		}
		let Some(stem) = name.strip_suffix(".md") else {
			continue;
		};
		let body = std::fs::read_to_string(&path)
			.map_err(|source| vorma::Error::new("read seed-data doc").with_source(source))?;
		let title = body
			.lines()
			.find_map(|line| line.strip_prefix("# "))
			.unwrap_or(stem)
			.to_owned();
		let slug = if prefix.is_empty() {
			stem.to_owned()
		} else {
			format!("{prefix}/{stem}")
		};
		pages.push((slug, title, body));
	}
	Ok(())
}
