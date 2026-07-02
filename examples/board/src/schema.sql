CREATE TABLE IF NOT EXISTS users (
	id INTEGER PRIMARY KEY,
	username TEXT NOT NULL UNIQUE,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	banned INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS sessions (
	token TEXT PRIMARY KEY,
	user_id INTEGER NOT NULL REFERENCES users(id),
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS stories (
	id INTEGER PRIMARY KEY,
	author_id INTEGER NOT NULL REFERENCES users(id),
	title TEXT NOT NULL,
	url TEXT,
	body TEXT,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	killed INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS comments (
	id INTEGER PRIMARY KEY,
	story_id INTEGER NOT NULL REFERENCES stories(id),
	parent_id INTEGER REFERENCES comments(id),
	author_id INTEGER NOT NULL REFERENCES users(id),
	body TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	killed INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS votes (
	user_id INTEGER NOT NULL REFERENCES users(id),
	story_id INTEGER NOT NULL REFERENCES stories(id),
	created_at INTEGER NOT NULL DEFAULT (unixepoch()),
	PRIMARY KEY (user_id, story_id)
);

CREATE INDEX IF NOT EXISTS idx_stories_created ON stories(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_comments_story ON comments(story_id);

CREATE TABLE IF NOT EXISTS attachments (
	id INTEGER PRIMARY KEY,
	story_id INTEGER NOT NULL REFERENCES stories(id),
	file_name TEXT NOT NULL,
	content_type TEXT NOT NULL,
	body BLOB NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_attachments_story ON attachments(story_id);

CREATE TABLE IF NOT EXISTS story_tags (
	story_id INTEGER NOT NULL REFERENCES stories(id),
	tag TEXT NOT NULL,
	PRIMARY KEY (story_id, tag)
);

CREATE TABLE IF NOT EXISTS docs_pages (
	slug TEXT PRIMARY KEY,
	title TEXT NOT NULL,
	body TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS mod_log (
	id INTEGER PRIMARY KEY,
	moderator_id INTEGER NOT NULL REFERENCES users(id),
	story_id INTEGER NOT NULL REFERENCES stories(id),
	action TEXT NOT NULL,
	created_at INTEGER NOT NULL DEFAULT (unixepoch())
);
