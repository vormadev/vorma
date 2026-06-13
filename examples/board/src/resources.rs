//! Resource declarations: the JSON API (login, logout, submit, vote).

use serde::{Deserialize, Serialize};
use vorma::HttpExit;

use crate::repo::{self, SearchKey, Story, User};
use crate::views::db_input;
use crate::{app, session};

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
		let user = require_user(
			ctx.state(),
			ctx.request().headers(),
			ctx.exec_ctx(),
		)
		.await?;

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

		let story_id = repo::create_story(
			std::sync::Arc::clone(&ctx.state().db),
			user.id,
			title,
			url,
			body,
		)
		.await?;
		if let Some(file) = ctx.input().file("attachment")
			&& !file.body().is_empty()
		{
			repo::save_attachment(
				std::sync::Arc::clone(&ctx.state().db),
				story_id,
				file.file_name().unwrap_or("attachment").to_owned(),
				file.content_type()
					.unwrap_or("application/octet-stream")
					.to_owned(),
				file.body().to_vec(),
			)
			.await?;
		}
		ctx.redirect(format!("/s/{story_id}"))
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
			.await
			.map_err(|error| HttpExit::err(error.to_string()))?;
		let points = (*story).as_ref().map_or(1, |story| story.points);
		Ok(VoteOutput { points })
	};
};

/// Session user or a 401 rejection — the resource-side auth gate.
async fn require_user(
	state: &crate::store::AppState,
	headers: &vorma::HttpHeaderMap,
	exec_ctx: &vorma::ExecCtx<vorma::Error>,
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
			.await
			.map_err(|error| HttpExit::err(error.to_string()))?;
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
			.await
			.map_err(|error| HttpExit::err(error.to_string()))?)
		.clone();
		let has_more = stories.len() as i64 == repo::FRONT_PAGE_SIZE;
		Ok(SearchOutput { stories, has_more })
	};
};

/*
Mod actions live under /mod on purpose: the scoped middleware
(patterns: /mod + the /mod splat) gates these RESOURCES exactly like
the mod views — one declaration covers both.
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

/// Attachment download: a non-JSON resource response (typed output is
/// the raw bytes path — content-type comes from the stored file).
pub const STORY_ATTACHMENT: app::Resource = app::resource! {
	kind: vorma::ResourceKind::Query;
	method: vorma::HttpMethod::GET;
	pattern: "/api/stories/:story_id/attachment";
	input: ();
	output: ();

	handler: |ctx| {
		let story_id = parse_story_id(ctx.param("story_id"))?;
		let attachment = (*repo::ATTACHMENT_FOR_STORY
			.run(ctx.exec_ctx(), db_input(ctx.state(), story_id))
			.await
			.map_err(|error| HttpExit::err(error.to_string()))?)
		.clone();
		let Some(attachment) = attachment else {
			return Err(HttpExit::err(format!(
				"attachment missing for story {story_id}"
			))
			.with_status(vorma::HttpStatusCode::NOT_FOUND)
			.with_client_msg("no attachment for that story"));
		};
		ctx.response().set_header(
			vorma::HttpHeaderName::from_static("content-disposition"),
			vorma::HttpHeaderValue::from_str(&format!(
				"inline; filename=\"{}\"",
				attachment.file_name.replace('"', "")
			))
			.map_err(|error| HttpExit::err(error.to_string()))?,
		);
		Ok(())
	};
};

fn parse_story_id(raw: &str) -> Result<i64, HttpExit> {
	raw.parse::<i64>().map_err(|_| {
		HttpExit::err(format!("bad story id {raw:?}"))
			.with_status(vorma::HttpStatusCode::BAD_REQUEST)
			.with_client_msg("story ids are numeric")
	})
}
