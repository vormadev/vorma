//! Typed resource declarations.
//!
//! Resources are Board's HTTP API. Most return JSON envelopes through generated
//! TypeScript types; `STORY_ATTACHMENT` shows the non-JSON raw-body path.

use serde::{Deserialize, Serialize};
use vorma::HttpExit;

use crate::repo::{self, SearchKey, Story, User};
use crate::views::db_input;
use crate::{app, session};

/// Fixed tag vocabulary offered on submit, exported to TypeScript so the checkbox group
/// and the server validation read from one source instead of two hand-kept lists.
pub const STORY_TAGS: &[&str] = &["tech", "ask", "show", "meta"];
/// Highest number of files one submission may attach under the repeated `attachment`
/// field. `<input type="file" multiple>` places no browser-side cap on its own, so the
/// server enforces one and the client reads the same constant to fail fast before upload.
pub const MAX_STORY_ATTACHMENTS: usize = 4;
/// Combined byte ceiling across every attached file in one submission, well under
/// [`crate::REQUEST_BODY_LIMIT`] to leave room for the text fields and multipart framing
/// around them.
pub const MAX_STORY_ATTACHMENT_BYTES: usize = 128 * 1024;
/// Per-field byte ceiling used by the blanket sweep over every submitted text field,
/// named or not (see the `fields()` validation in `SUBMIT_STORY`). Generous enough for a
/// full story body, tight enough to catch a field stuffed far past anything a real form
/// control on this page could produce.
const MAX_FORM_FIELD_VALUE_BYTES: usize = 4_000;

#[derive(Clone, Debug, Deserialize, vorma::TsGen)]
pub struct LoginInput {
	username: String,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct LoginOutput {
	user: User,
}

#[derive(Clone, Debug, Deserialize, vorma::TsGen)]
pub struct CreateCommentInput {
	body: String,
	#[serde(default)]
	parent_id: Option<i64>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct CreateCommentOutput {
	comment_id: i64,
}

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct SearchInput {
	#[serde(default)]
	q: Option<String>,
	#[serde(default)]
	page: Option<i64>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct SearchOutput {
	stories: Vec<Story>,
	has_more: bool,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct VoteOutput {
	points: i64,
}

pub const LOGIN: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/session";
	input: LoginInput;
	output: LoginOutput;

	handler: |ctx| {
		let username = ctx.input().username.trim().to_ascii_lowercase();
		if username.is_empty() || username.len() > 32 {
			/*
			`HttpExit::err` records the server-side diagnostic. `with_client_msg`
			is the explicit text allowed to cross the wire to the browser.
			*/
			return Err(HttpExit::err(format!(
				"login rejected: bad username {username:?}"
			))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg("usernames are 1-32 characters"));
		}

		let token = session::mint_token()?;
		let user = repo::login(
			std::sync::Arc::clone(&ctx.state().db),
			username,
			token.clone(),
		)
		.await?;

		ctx.response()
			.set_status(vorma::HttpStatusCode::CREATED)
			.set_cookie(session::session_cookie(token));
		Ok(LoginOutput { user })
	};
};

pub const LOGOUT: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::DELETE;
	pattern: "/api/session";
	input: ();
	output: ();

	handler: |ctx| {
		if let Some(token) = session::token_of(ctx.request().headers()) {
			repo::logout(std::sync::Arc::clone(&ctx.state().db), token).await?;
		}
		ctx.response().set_cookie(session::removal_cookie());
		Ok(())
	};
};

pub const SUBMIT_STORY: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/stories";
	input: vorma::FormData;
	output: ();

	handler: |ctx| {
		/*
		`FormData` is the typed input for multipart/browser form posts. It keeps text
		fields and uploaded files in one generated-client endpoint without pretending
		the request body is JSON.
		*/
		let user = require_user(
			ctx.state(),
			ctx.request().headers(),
			ctx.exec_ctx(),
		)
		.await?;

		/*
		`fields()` is the aggregate view over every text field the client sent, named or
		not — the natural shape for a sweep that does not care which field it is looking
		at. Board's own fields are all covered by their own targeted checks below, but a
		form handler should never assume a client sends exactly the fields it expects;
		this closes that gap for any field name at all, not just the ones this handler
		happens to read by name.
		*/
		if let Some(oversized) = ctx
			.input()
			.fields()
			.iter()
			.find(|field| field.value().len() > MAX_FORM_FIELD_VALUE_BYTES)
		{
			return Err(HttpExit::err(format!(
				"submit rejected: field {:?} exceeds {MAX_FORM_FIELD_VALUE_BYTES} bytes",
				oversized.name()
			))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg("one of the submitted fields is too long"));
		}

		let title = ctx.input().text("title").unwrap_or_default().trim().to_owned();
		let url = normalized_optional(ctx.input().text("url"));
		let body = normalized_optional(ctx.input().text("body"));
		if title.is_empty() || title.len() > 120 {
			return Err(HttpExit::err("submit rejected: bad title length")
				.with_status(vorma::HttpStatusCode::BAD_REQUEST)
				.with_client_msg("titles are 1-120 characters"));
		}
		match (&url, &body) {
			(Some(url), None) if !url.starts_with("https://") && !url.starts_with("http://") => {
				return Err(HttpExit::err(format!("submit rejected: bad url {url:?}"))
					.with_status(vorma::HttpStatusCode::BAD_REQUEST)
					.with_client_msg("links must start with http:// or https://"));
			}
			(Some(_), None) | (None, Some(_)) => {}
			_ => {
				return Err(HttpExit::err("submit rejected: need url XOR body")
					.with_status(vorma::HttpStatusCode::BAD_REQUEST)
					.with_client_msg("provide a link or text, not both"));
			}
		}

		/*
		`fields_named(name)` yields the matching `FormField`s themselves rather than
		just their values, which is the right accessor for a check about the group's
		shape rather than its content: a real client can check at most one box per
		tag Board renders, so more entries than the whole vocabulary is a sure sign
		of a tampered request, independent of which values they carry.
		*/
		if ctx.input().fields_named("tag").count() > STORY_TAGS.len() {
			return Err(HttpExit::err("submit rejected: too many tag fields submitted")
				.with_status(vorma::HttpStatusCode::BAD_REQUEST)
				.with_client_msg("that tag selection is not valid"));
		}
		/*
		`texts(name)` is the repeated-value counterpart to `text(name)`: a checkbox
		group posts one `tag` field per checked box under the same name, and `texts`
		yields every value in request order instead of only the first — the same
		`tag` group the shape check above just validated, now read for its content.
		The vocabulary filter guards against a tampered request naming a tag outside
		the checkboxes Board actually renders.
		*/
		let tags: Vec<String> = ctx
			.input()
			.texts("tag")
			.filter(|tag| STORY_TAGS.contains(tag))
			.map(str::to_owned)
			.collect();

		/*
		`files()` is the aggregate view over every uploaded file, mirroring `fields()`
		above. The count/size caps below are a property of the whole submission, not of
		any one field name, so this is the accessor that reads naturally here: it would
		still catch an oversized batch even if a future field added a second upload
		control under a different name.
		*/
		let files = ctx.input().files();
		if files.len() > MAX_STORY_ATTACHMENTS {
			return Err(HttpExit::err(format!(
				"submit rejected: {} attachments exceeds the {MAX_STORY_ATTACHMENTS} limit",
				files.len()
			))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg(format!("attach at most {MAX_STORY_ATTACHMENTS} files")));
		}
		let total_attachment_bytes: usize = files.iter().map(|file| file.body().len()).sum();
		if total_attachment_bytes > MAX_STORY_ATTACHMENT_BYTES {
			return Err(HttpExit::err(format!(
				"submit rejected: {total_attachment_bytes} attachment bytes exceeds \
				{MAX_STORY_ATTACHMENT_BYTES}"
			))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg("attachments are too large altogether"));
		}

		let story_id = repo::create_story(
			std::sync::Arc::clone(&ctx.state().db),
			user.id,
			title,
			url,
			body,
		)
		.await?;
		if !tags.is_empty() {
			repo::save_story_tags(std::sync::Arc::clone(&ctx.state().db), story_id, tags).await?;
		}
		/*
		`files_named(name)` is the repeated-value counterpart to `file(name)`: a
		`<input type="file" multiple name="attachment">` posts one `attachment` field per
		selected file under that one name, and `files_named` yields every match in
		request order instead of only the first — the multipart model this whole
		accessor family is built on.
		*/
		for file in ctx.input().files_named("attachment") {
			if file.body().is_empty() {
				continue;
			}
			/*
			`into_body()` consumes a `FormFile` and hands back its bytes by value
			instead of borrowing them through `.body()`. `ctx.input()` only ever
			lends a `&FormData`, so reaching `into_body()` means cloning first — worth
			it here because the clone is cheap (the file body is reference-counted
			`Bytes`; only the small name/content-type strings are actually
			duplicated) and it means the storage write below takes ownership
			directly instead of borrowing across the `.await`.
			*/
			let owned_file = file.clone().into_body();
			repo::save_attachment(
				std::sync::Arc::clone(&ctx.state().db),
				story_id,
				file.file_name().unwrap_or("attachment").to_owned(),
				file.content_type()
					.unwrap_or("application/octet-stream")
					.to_owned(),
				owned_file.to_vec(),
			)
			.await?;
		}
		ctx.redirect_with_status(
			format!("/s/{story_id}"),
			vorma::HttpStatusCode::SEE_OTHER,
		)
	};
};

pub const VOTE_STORY: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/stories/:story_id/vote";
	input: ();
	output: VoteOutput;

	handler: |ctx| {
		let user = require_user(
			ctx.state(),
			ctx.request().headers(),
			ctx.exec_ctx(),
		)
		.await?;
		let story_id = ctx.param("story_id").parse::<i64>().map_err(|_| {
			HttpExit::err(format!(
				"vote rejected: bad story id {:?}",
				ctx.param("story_id")
			))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg("story ids are numeric")
		})?;

		let inserted =
			repo::vote_story(std::sync::Arc::clone(&ctx.state().db), user.id, story_id).await?;
		if !inserted {
			return Err(HttpExit::err(format!(
				"vote rejected: duplicate vote user={} story={story_id}",
				user.id
			))
			.with_status(vorma::HttpStatusCode::CONFLICT)
			.with_client_msg("you already voted on this story"));
		}

		let story = repo::STORY_BY_ID
			.run(ctx.exec_ctx(), db_input(ctx.state(), story_id))
			.await?;
		let points = (*story).as_ref().map_or(1, |story| story.points);
		Ok(VoteOutput { points })
	};
};

/// Session user or a 401 rejection.
///
/// Helpers like this keep auth checks identical across resources. Middleware handles
/// route-wide gates; resources still validate their own mutation/query boundary.
async fn require_user(
	state: &crate::store::AppState,
	headers: &vorma::HttpHeaderMap,
	exec_ctx: &vorma::tasks::ExecCtx<vorma::Error>,
) -> Result<User, HttpExit> {
	match session::current_user(state, headers, exec_ctx).await {
		Ok(Some(user)) => Ok(user),
		Ok(None) => Err(HttpExit::err("auth required: no session")
			.with_status(vorma::HttpStatusCode::UNAUTHORIZED)
			.with_client_msg("sign in first")),
		Err(error) => Err(HttpExit::err(error.to_string())),
	}
}

fn normalized_optional(value: Option<&str>) -> Option<String> {
	let value = value?.trim();
	if value.is_empty() {
		return None;
	}
	Some(value.to_owned())
}

pub const CREATE_COMMENT: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/stories/:story_id/comments";
	input: CreateCommentInput;
	output: CreateCommentOutput;

	handler: |ctx| {
		let user = require_user(
			ctx.state(),
			ctx.request().headers(),
			ctx.exec_ctx(),
		)
		.await?;
		let story_id = parse_story_id(ctx.param("story_id"))?;
		let body = ctx.input().body.trim().to_owned();
		if body.is_empty() || body.len() > 2000 {
			return Err(HttpExit::err("comment rejected: bad body length")
				.with_status(vorma::HttpStatusCode::BAD_REQUEST)
				.with_client_msg("comments are 1-2000 characters"));
		}
		let story = repo::STORY_BY_ID
			.run(ctx.exec_ctx(), db_input(ctx.state(), story_id))
			.await?;
		if (*story).as_ref().is_none_or(|story| story.killed) {
			return Err(HttpExit::err(format!(
				"comment rejected: story {story_id} missing or killed"
			))
			.with_status(vorma::HttpStatusCode::NOT_FOUND)
			.with_client_msg("that story no longer exists"));
		}

		let comment_id = repo::create_comment(
			std::sync::Arc::clone(&ctx.state().db),
			story_id,
			ctx.input().parent_id,
			user.id,
			body,
		)
		.await?;
		ctx.response().set_status(vorma::HttpStatusCode::CREATED);
		Ok(CreateCommentOutput { comment_id })
	};
};

pub const SEARCH: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Query;
	method: vorma::HttpMethod::GET;
	pattern: "/api/search";
	input: SearchInput;
	output: SearchOutput;

	handler: |ctx| {
		let q = ctx.input().q.clone().unwrap_or_default().trim().to_owned();
		let page = ctx.input().page.unwrap_or(1).max(1);
		if q.is_empty() {
			return Ok(SearchOutput {
				stories: Vec::new(),
				has_more: false,
			});
		}
		let stories = (*repo::SEARCH_STORIES
			.run(
				ctx.exec_ctx(),
				db_input(ctx.state(), SearchKey { q, page }),
			)
			.await?)
		.clone();
		let has_more = stories.len() as i64 == repo::FRONT_PAGE_SIZE;
		Ok(SearchOutput { stories, has_more })
	};
};

/*
Mod browser views live under `/mod`, while these mutation endpoints live under
the matching API prefix. Middleware scopes are URL patterns, so the middleware declares
both URL spaces explicitly instead of assuming a browser route implies an API route.
*/
pub const KILL_STORY: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/mod/stories/:story_id/kill";
	input: ();
	output: ();

	handler: |ctx| {
		let user = require_user(
			ctx.state(),
			ctx.request().headers(),
			ctx.exec_ctx(),
		)
		.await?;
		let story_id = parse_story_id(ctx.param("story_id"))?;
		let changed = repo::set_story_killed(
			std::sync::Arc::clone(&ctx.state().db),
			user.id,
			story_id,
			true,
		)
		.await?;
		if !changed {
			return Err(HttpExit::err(format!("kill rejected: no story {story_id}"))
				.with_status(vorma::HttpStatusCode::NOT_FOUND)
				.with_client_msg("that story does not exist"));
		}
		Ok(())
	};
};

pub const RESTORE_STORY: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/mod/stories/:story_id/restore";
	input: ();
	output: ();

	handler: |ctx| {
		let user = require_user(
			ctx.state(),
			ctx.request().headers(),
			ctx.exec_ctx(),
		)
		.await?;
		let story_id = parse_story_id(ctx.param("story_id"))?;
		let changed = repo::set_story_killed(
			std::sync::Arc::clone(&ctx.state().db),
			user.id,
			story_id,
			false,
		)
		.await?;
		if !changed {
			return Err(HttpExit::err(format!(
				"restore rejected: no story {story_id}"
			))
			.with_status(vorma::HttpStatusCode::NOT_FOUND)
			.with_client_msg("that story does not exist"));
		}
		Ok(())
	};
};

/*
A bulk audit export over every story and its comments: the realistic case where a task
body is worth writing to cooperate with cancellation instead of running to completion no
matter what. The client's `handler_timeout` (server main) or a mod simply navigating away
mid-export are both ordinary ways for this request's execution context to end before
`MOD_EXPORT_SCAN` finishes; `repo::MOD_EXPORT_SCAN` checks `ctx.is_cancelled()` between
chunks and returns whatever it already gathered instead of continuing to scan a table
nobody is waiting on anymore. `digest.complete` tells the caller which case happened.
*/
pub const MOD_EXPORT: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Mutation;
	method: vorma::HttpMethod::POST;
	pattern: "/api/mod/export";
	input: ();
	output: repo::ModExportDigest;

	handler: |ctx| {
		require_user(
			ctx.state(),
			ctx.request().headers(),
			ctx.exec_ctx(),
		)
		.await?;
		let digest = repo::MOD_EXPORT_SCAN
			.run(ctx.exec_ctx(), db_input(ctx.state(), ()))
			.await?;
		Ok((*digest).clone())
	};
};

/// Attachment download.
///
/// `ResourceBody` is the raw-response output type. Use it when the endpoint should return
/// bytes with a real content type, while still keeping the endpoint in the generated
/// client contract as a typed `Blob` result.
pub const STORY_ATTACHMENT: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Query;
	method: vorma::HttpMethod::GET;
	pattern: "/api/stories/:story_id/attachments/:attachment_id";
	input: ();
	output: vorma::ResourceBody;

	handler: |ctx| {
		let story_id = parse_story_id(ctx.param("story_id"))?;
		let attachment_id = ctx.param("attachment_id").parse::<i64>().map_err(|_| {
			HttpExit::err(format!(
				"attachment rejected: bad attachment id {:?}",
				ctx.param("attachment_id")
			))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg("attachment ids are numeric")
		})?;
		let attachment = (*repo::ATTACHMENT_BY_ID
			.run(ctx.exec_ctx(), db_input(ctx.state(), attachment_id))
			.await?)
		.clone();
		/*
		Attachment ids are one global sequence, not scoped per story, so a valid id for
		a different story must still 404 here rather than serve cross-story content —
		the route's `:story_id` segment is part of the resource's identity, not just a
		display label.
		*/
		let Some(attachment) = attachment.filter(|attachment| attachment.story_id == story_id)
		else {
			return Err(HttpExit::err(format!(
				"attachment {attachment_id} missing for story {story_id}"
			))
			.with_status(vorma::HttpStatusCode::NOT_FOUND)
			.with_client_msg("no such attachment for that story"));
		};
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("content-disposition"),
			vorma::HttpHeaderValue::from_str(&format!(
				"inline; filename=\"{}\"",
				attachment.file_name.replace('"', "")
			))
			.map_err(|error| HttpExit::err(error.to_string()))?,
		);
		/*
		`append_header` adds a value without replacing existing ones, which is the
		correct choice for headers that legitimately carry a list. `Vary` is the
		classic case: append each header the response depends on instead of
		overwriting a prior `Vary` value.
		*/
		ctx.response().append_header(
			vorma::HttpHeaderName::from_static("vary"),
			vorma::HttpHeaderValue::from_static("accept-encoding"),
		);
		let content_type = vorma::HttpHeaderValue::from_str(&attachment.content_type)
			.map_err(|error| HttpExit::err(error.to_string()))?;
		Ok(vorma::ResourceBody::new(content_type, attachment.body))
	};
};

fn parse_story_id(raw: &str) -> Result<i64, HttpExit> {
	raw.parse::<i64>().map_err(|_| {
		HttpExit::err(format!("bad story id {raw:?}"))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg("story ids are numeric")
	})
}
