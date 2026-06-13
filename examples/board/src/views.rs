//! View declarations: layout, front page, story page.

use serde::{Deserialize, Serialize};

use crate::repo::{self, Comment, DbInput, DocsPage, ModLogEntry, Story, User, UserProfile};
use crate::store::AppState;
use crate::{APP_NAME, app};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct LayoutData {
	app_name: String,
	current_user: Option<User>,
}

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct FrontInput {
	#[serde(default)]
	page: Option<i64>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct FrontPage {
	stories: Vec<Story>,
	page: i64,
	has_more: bool,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct StoryPage {
	story: Option<Story>,
	comments: Vec<Comment>,
}

/// Root layout: shell facts plus the session user.
/*
USER_FOR_SESSION here re-runs the task the preload middleware already
started — same task, same input, one database read per request.
*/
pub const LAYOUT: app::View = app::view! {
	client_file: "src/client/views/layout.view.tsx";
	pattern: "/";
	input: ();
	output: LayoutData;
	handler: |ctx| {
		ctx.head().title(APP_NAME);
		let current_user =
			crate::session::current_user(ctx.state(), ctx.request().headers(), ctx.exec_ctx())
				.await
				.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		Ok(LayoutData {
			app_name: APP_NAME.to_owned(),
			current_user,
		})
	};
};

pub const FRONT: app::View = app::view! {
	client_file: "src/client/views/front.view.tsx";
	pattern: "/_index";
	input: FrontInput;
	output: FrontPage;
	handler: |ctx| {
		let page = ctx.input().page.unwrap_or(1).max(1);
		let stories = repo::STORIES_PAGE
			.run(ctx.exec_ctx(), db_input(ctx.state(), page))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		let stories = (*stories).clone();
		let has_more = stories.len() as i64 == repo::FRONT_PAGE_SIZE;
		Ok(FrontPage {
			stories,
			page,
			has_more,
		})
	};
};

pub const STORY: app::View = app::view! {
	client_file: "src/client/views/story.view.tsx";
	pattern: "/s/:story_id";
	input: ();
	output: StoryPage;
	handler: |ctx| {
		let Ok(story_id) = ctx.param("story_id").parse::<i64>() else {
			/*
			Bad id is a rendered not-found state, same as a missing row:
			views are a rendering protocol.
			*/
			ctx.head().title("Story not found");
			return Ok(StoryPage {
				story: None,
				comments: Vec::new(),
			});
		};

		/*
		Prewarm both reads in parallel through the task runtime's own
		combinator, then take the typed results from the request-scope
		dedupe — the follow-up runs are cache hits, not re-executions.
		*/
		ctx.exec_ctx()
			.run_parallel([
				repo::STORY_BY_ID.bind_input(db_input(ctx.state(), story_id)),
				repo::COMMENTS_FOR_STORY.bind_input(db_input(ctx.state(), story_id)),
			])
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		let story = (*repo::STORY_BY_ID
			.run(ctx.exec_ctx(), db_input(ctx.state(), story_id))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();
		let comments = (*repo::COMMENTS_FOR_STORY
			.run(ctx.exec_ctx(), db_input(ctx.state(), story_id))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();

		match &story {
			Some(story) if !story.killed => {
				ctx.head()
					.title(format!("{} | {APP_NAME}", story.title))
					.description(format!("{} points", story.points))
					.meta_property_content("og:title", story.title.clone())
					.meta_property_content("og:type", "article")
					.meta_name_content("twitter:title", story.title.clone());
			}
			_ => {
				ctx.head().title("Story not found");
			}
		}
		// A killed story renders the not-found treatment but keeps its
		// comment count private: clear both.
		if story.as_ref().is_some_and(|story| story.killed) {
			return Ok(StoryPage {
				story: None,
				comments: Vec::new(),
			});
		}

		Ok(StoryPage { story, comments })
	};
};

pub(crate) fn db_input<I>(state: &AppState, input: I) -> DbInput<I> {
	DbInput {
		db: std::sync::Arc::clone(&state.db),
		input,
	}
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct SubmitPage {
	signed_in: bool,
}

/// Submit form host; the gate is UX-side here (the POST itself 401s).
pub const SUBMIT: app::View = app::view! {
	client_file: "src/client/views/submit.view.tsx";
	pattern: "/submit";
	input: ();
	output: SubmitPage;
	handler: |ctx| {
		ctx.head().title(format!("Submit | {APP_NAME}"));
		let signed_in =
			crate::session::current_user(ctx.state(), ctx.request().headers(), ctx.exec_ctx())
				.await
				.map_err(|error| vorma::ViewExit::err(error.to_string()))?
				.is_some();
		Ok(SubmitPage { signed_in })
	};
};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct UserPage {
	profile: Option<UserProfile>,
	stories: Vec<Story>,
}

/// Profile parent: owns the profile facts; tabs nest beneath it.
pub const USER: app::View = app::view! {
	client_file: "src/client/views/user.view.tsx";
	pattern: "/u/:username";
	input: ();
	output: UserPage;
	handler: |ctx| {
		let username = ctx.param("username").to_ascii_lowercase();
		ctx.exec_ctx()
			.run_parallel([
				repo::USER_PROFILE.bind_input(db_input(ctx.state(), username.clone())),
				repo::STORIES_BY_AUTHOR.bind_input(db_input(ctx.state(), username.clone())),
			])
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		let profile = (*repo::USER_PROFILE
			.run(ctx.exec_ctx(), db_input(ctx.state(), username.clone()))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();
		let stories = (*repo::STORIES_BY_AUTHOR
			.run(ctx.exec_ctx(), db_input(ctx.state(), username.clone()))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();

		match &profile {
			Some(profile) => {
				ctx.head().title(format!("{} | {APP_NAME}", profile.username));
			}
			None => {
				ctx.head().title("User not found");
			}
		}
		Ok(UserPage { profile, stories })
	};
};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct UserCommentsPage {
	comments: Vec<Comment>,
}

/// Nested tab under the profile parent.
pub const USER_COMMENTS: app::View = app::view! {
	client_file: "src/client/views/user_comments.view.tsx";
	pattern: "/u/:username/comments";
	input: ();
	output: UserCommentsPage;
	handler: |ctx| {
		let username = ctx.param("username").to_ascii_lowercase();
		let comments = (*repo::COMMENTS_BY_AUTHOR
			.run(ctx.exec_ctx(), db_input(ctx.state(), username))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();
		Ok(UserCommentsPage { comments })
	};
};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct DocsIndexPage {
	index: Vec<DocsPage>,
}

/*
Splat patterns match one-or-more segments, never their bare root, so
the docs section is a nested pair: "/docs" is the index/sidebar parent
and the splat child renders individual pages inside its outlet.
*/
pub const DOCS_INDEX_VIEW: app::View = app::view! {
	client_file: "src/client/views/docs.view.tsx";
	pattern: "/docs";
	input: ();
	output: DocsIndexPage;
	handler: |ctx| {
		ctx.head().title(format!("Docs | {APP_NAME}"));
		let index = (*repo::DOCS_INDEX
			.run(ctx.exec_ctx(), db_input(ctx.state(), ()))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();
		Ok(DocsIndexPage { index })
	};
};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct DocsPageView {
	page: Option<DocsPage>,
}

pub const DOCS_PAGE_VIEW: app::View = app::view! {
	client_file: "src/client/views/docs_page.view.tsx";
	pattern: "/docs/*";
	input: ();
	output: DocsPageView;
	handler: |ctx| {
		let slug = ctx.splat_values().join("/");
		let page = (*repo::DOCS_PAGE
			.run(ctx.exec_ctx(), db_input(ctx.state(), slug))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();
		if let Some(page) = &page {
			ctx.head().title(format!("{} | {APP_NAME}", page.title));
		}
		Ok(DocsPageView { page })
	};
};

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct SearchPageInput {
	#[serde(default)]
	q: Option<String>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct SearchPage {
	q: String,
}

/// Search shell: the QUERY itself runs client-side via the search
/// resource (useApiQuery's home); the view carries the URL state.
pub const SEARCH: app::View = app::view! {
	client_file: "src/client/views/search.view.tsx";
	pattern: "/search";
	input: SearchPageInput;
	output: SearchPage;
	handler: |ctx| {
		ctx.head().title(format!("Search | {APP_NAME}"));
		Ok(SearchPage {
			q: ctx.input().q.clone().unwrap_or_default(),
		})
	};
};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct ModPage {
	killed: Vec<Story>,
	log: Vec<ModLogEntry>,
}

/// Mod queue parent (the scoped middleware gates everything under /mod).
pub const MOD: app::View = app::view! {
	client_file: "src/client/views/mod.view.tsx";
	pattern: "/mod";
	input: ();
	output: ModPage;
	handler: |ctx| {
		ctx.head().title(format!("Moderation | {APP_NAME}"));
		ctx.exec_ctx()
			.run_parallel([
				repo::KILLED_STORIES.bind_input(db_input(ctx.state(), ())),
				repo::MOD_LOG.bind_input(db_input(ctx.state(), ())),
			])
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		let killed = (*repo::KILLED_STORIES
			.run(ctx.exec_ctx(), db_input(ctx.state(), ()))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();
		let log = (*repo::MOD_LOG
			.run(ctx.exec_ctx(), db_input(ctx.state(), ()))
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?)
		.clone();
		Ok(ModPage { killed, log })
	};
};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct ModDiagnosticsPage {}

/// Deliberately failing segment: the mod queue (parent) still renders;
/// only this slot shows its error boundary with the explicit message.
pub const MOD_DIAGNOSTICS: app::View = app::view! {
	client_file: "src/client/views/mod_diagnostics.view.tsx";
	pattern: "/mod/diagnostics";
	input: ();
	output: ModDiagnosticsPage;
	handler: |_ctx| {
		Err(vorma::ViewExit::err(
			"diagnostics backend intentionally unwired (census F12 segment-error demo)",
		)
		.with_client_msg("Diagnostics are not available in the demo."))
	};
};
