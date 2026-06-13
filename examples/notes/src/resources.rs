//! Resource declarations: the app's JSON API.

use serde::{Deserialize, Serialize};
use vorma::{HttpCookie, HttpExit};

use crate::store::{Note, note_store};
use crate::{app, session};

#[derive(Clone, Debug, Deserialize, vorma::TsGen)]
pub struct CreateNoteInput {
	body: String,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct CreateNoteOutput {
	note: Note,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct DeleteNoteOutput {
	deleted: Note,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct ImportNotesOutput {
	imported: usize,
}

#[derive(Clone, Debug, Deserialize, vorma::TsGen)]
pub struct LoginInput {
	user: String,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct LoginOutput {
	session: String,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct PingOutput {
	ok: bool,
}

pub const CREATE_NOTE: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/notes";
	input: CreateNoteInput;
	output: CreateNoteOutput;

	handler: |ctx| {
		let body = ctx.input().body.trim();
		/*
		A rejection is one returned value: `with_client_msg` is the ONLY
		client-visible text (the error envelope the browser receives);
		the `err` record and any source stay server-side.
		*/
		if body.is_empty() {
			return Err(HttpExit::err("create note rejected: blank body")
				.with_status(vorma::HttpStatusCode::BAD_REQUEST)
				.with_client_msg("note body is required"));
		}
		if body.len() > 500 {
			return Err(HttpExit::err(format!(
				"create note rejected: body length {} exceeds 500",
				body.len()
			))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg("note body must be at most 500 characters"));
		}

		let mut store = note_store(ctx.state())?;
		let id = store.next_id;
		store.next_id += 1;
		let note = Note::new(id, body);
		store.notes.push(note.clone());

		ctx.response().set_status(vorma::HttpStatusCode::CREATED);

		Ok(CreateNoteOutput { note })
	};
};

pub const DELETE_NOTE: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::DELETE;
	pattern: "/api/notes/:note_id";
	input: ();
	output: DeleteNoteOutput;

	handler: |ctx| {
		let id = ctx.param("note_id").parse::<u64>().map_err(|_| {
			HttpExit::err(format!("delete rejected: bad id {:?}", ctx.param("note_id")))
				.with_status(vorma::HttpStatusCode::BAD_REQUEST)
				.with_client_msg("note ids are numeric")
		})?;
		let mut store = note_store(ctx.state())?;
		let Some(position) = store.notes.iter().position(|note| note.id == id) else {
			return Err(HttpExit::err(format!("delete rejected: no note {id}"))
				.with_status(vorma::HttpStatusCode::NOT_FOUND)
				.with_client_msg("that note no longer exists"));
		};
		let deleted = store.notes.remove(position);
		Ok(DeleteNoteOutput { deleted })
	};
};

/// One-request multi-note import from a multipart text file.
pub const IMPORT_NOTES: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/notes/import";
	input: vorma::FormData;
	output: ImportNotesOutput;

	handler: |ctx| {
		let Some(file) = ctx.input().files().first() else {
			return Err(HttpExit::err("import rejected: no file part")
				.with_status(vorma::HttpStatusCode::BAD_REQUEST)
				.with_client_msg("attach a notes file"));
		};
		let text = String::from_utf8_lossy(file.body());
		let mut store = note_store(ctx.state())?;
		let mut imported = 0;
		for line in text.lines().map(str::trim).filter(|line| !line.is_empty()) {
			let id = store.next_id;
			store.next_id += 1;
			store.notes.push(Note::new(id, line));
			imported += 1;
		}
		ctx.response().set_status(vorma::HttpStatusCode::CREATED);
		Ok(ImportNotesOutput { imported })
	};
};

/// Duplicate a note, then redirect the client at the fresh copy.
pub const DUPLICATE_NOTE: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/notes/:note_id/duplicate";
	input: ();
	output: ();

	handler: |ctx| {
		let id = ctx.param("note_id").parse::<u64>().ok();
		let mut store = note_store(ctx.state())?;
		let Some(original) = id.and_then(|id| {
			store.notes.iter().find(|note| note.id == id).cloned()
		}) else {
			return Err(HttpExit::err("duplicate rejected: missing source note")
				.with_status(vorma::HttpStatusCode::NOT_FOUND)
				.with_client_msg("that note no longer exists"));
		};
		let new_id = store.next_id;
		store.next_id += 1;
		store.notes.push(Note::new(new_id, original.body));
		drop(store);
		ctx.redirect(format!("/notes/{new_id}"))
	};
};

pub const LOGIN: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/session";
	input: LoginInput;
	output: LoginOutput;

	handler: |ctx| {
		let user = ctx.input().user.trim().to_ascii_lowercase();
		if user.is_empty() {
			return Err(HttpExit::err("login rejected: blank user")
				.with_status(vorma::HttpStatusCode::BAD_REQUEST)
				.with_client_msg("a user name is required"));
		}
		ctx.response()
			.set_status(vorma::HttpStatusCode::CREATED)
			.set_cookie(
				HttpCookie::build((session::SESSION_COOKIE, user.clone()))
					.path("/")
					.http_only(true)
					.build(),
			);
		Ok(LoginOutput { session: user })
	};
};

pub const LOGOUT: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::DELETE;
	pattern: "/api/session";
	input: ();
	output: ();

	handler: |ctx| {
		ctx.response().set_cookie(
			HttpCookie::build((session::SESSION_COOKIE, ""))
				.path("/")
				.removal()
				.build(),
		);
		Ok(())
	};
};

pub const PING: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Query;
	method: vorma::HttpMethod::GET;
	pattern: "/api/ping";
	input: ();
	output: PingOutput;
	handler: |ctx| {
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("x-notes-api"),
			vorma::HttpHeaderValue::from_static("1"),
		);
		Ok(PingOutput { ok: true })
	};
};
