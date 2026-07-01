//! Typed view declarations.
//!
//! Views are route segments. Each view returns serializable data for its client module
//! and can also contribute document head metadata. Vorma runs matching sibling/parent
//! view handlers in parallel and sends the ordered view-data array to the browser.

use serde::{Deserialize, Serialize};

use crate::repo::{self, Comment, DbInput, DocsPage, ModLogEntry, Story, User, UserProfile};
use crate::store::AppState;
use crate::{APP_NAME, MARK_ASSET, app};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct LayoutData {
	app_name: String,
	mark_url: String,
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

/*
The root layout is the persistent shell. It asks for the current user even though
middleware already preloads that task: same task plus same input inside one request means
one database read, shared by middleware, layout, and resources.
*/
pub const LAYOUT: app::View = app::view! {
	client_file: "src/client/views/layout.view.tsx";
	pattern: "/";
	input: ();
	output: LayoutData;
	handler: |ctx| {
		ctx.head().title(APP_NAME);
		/*
		View handlers can also resolve public assets. Use this when the
		browser component needs the final URL as part of its typed view data.
		*/
		let mark_url = ctx.public_url(MARK_ASSET)?;
		let current_user =
			crate::session::current_user(ctx.state(), ctx.request().headers(), ctx.exec_ctx())
				.await
				.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		Ok(LayoutData {
			app_name: APP_NAME.to_owned(),
			mark_url,
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
		/*
		Typed input comes from query/search decoding. The Rust type is the source of
		the generated TypeScript input type for this view.
		*/
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
			A malformed route param is a rendered state here, not an HTTP error:
			views describe UI segments. Reserve HTTP status semantics for
			resources and middleware.
			*/
			ctx.head().title("Story not found");
			return Ok(StoryPage {
				story: None,
				comments: Vec::new(),
			});
		};

		let mut batch = vorma::tasks::ParallelBatch::new();
		let story = batch.add(repo::STORY_BY_ID, db_input(ctx.state(), story_id));
		let comments = batch.add(repo::COMMENTS_FOR_STORY, db_input(ctx.state(), story_id));
		let outputs = batch
			.run(ctx.exec_ctx())
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		let story = (*outputs.take(story)).clone();
		let comments = (*outputs.take(comments)).clone();

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
		// A killed story renders the not-found treatment and hides its comment count.
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

/// Submit form host.
///
/// This view only decides what to render. The POST resource still performs the real
/// authorization check, because resources are the HTTP boundary.
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

/// Profile parent view.
///
/// Parent views own data shared by nested children. The comments tab below renders in the
/// parent's outlet without refetching the profile facts.
pub const USER: app::View = app::view! {
	client_file: "src/client/views/user.view.tsx";
	pattern: "/u/:username";
	input: ();
	output: UserPage;
	handler: |ctx| {
		let raw_username = ctx.param("username");
		let username = raw_username.to_ascii_lowercase();
		if raw_username != username {
			/*
			View redirects are for route canonicalization before rendering.
			They differ from resource redirects only in the exit type: views
			do not own HTTP error statuses, but they can still redirect.
			*/
			let raw_prefix = format!("/u/{raw_username}");
			let canonical_prefix = format!("/u/{username}");
			let location = ctx
				.request()
				.path()
				.replacen(&raw_prefix, &canonical_prefix, 1);
			return ctx.redirect(location);
		}
		let mut batch = vorma::tasks::ParallelBatch::new();
		let profile = batch.add(repo::USER_PROFILE, db_input(ctx.state(), username.clone()));
		let stories =
			batch.add(repo::STORIES_BY_AUTHOR, db_input(ctx.state(), username.clone()));
		let outputs = batch
			.run(ctx.exec_ctx())
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		let profile = (*outputs.take(profile)).clone();
		let stories = (*outputs.take(stories)).clone();

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
Splat patterns match one-or-more segments, never their bare root. Use a parent route for
the section root and a splat child route for arbitrary nested pages.
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

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct NotFoundPage {
	requested_path: String,
	primary_source: Option<String>,
	source_tags: Vec<String>,
	source_pair_count: usize,
}

/*
The root catch-all is a normal view, not an HTTP error. If it matches, Vorma found a
route and renders the app with status 200; the UI can still explain that Board has no
page for the requested URL. This fallback also reads the raw search params as
unstructured route-adjacent metadata; ordinary route state should still prefer typed view
input.
*/
pub const NOT_FOUND: app::View = app::view! {
	client_file: "src/client/views/not_found.view.tsx";
	pattern: "/*";
	input: ();
	output: NotFoundPage;
	handler: |ctx| {
		ctx.head().title(format!("Page not found | {APP_NAME}"));
		let request = ctx.request();
		let search_params = request.search_params();
		let primary_source = search_params.get("from");
		let source_tags = search_params.get_all("from").collect();
		let source_pair_count = search_params
			.iter()
			.filter(|(name, _value)| name == "from")
			.count();
		Ok(NotFoundPage {
			requested_path: request.path().to_owned(),
			primary_source,
			source_tags,
			source_pair_count,
		})
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

/// Search shell.
///
/// The server view owns the shareable URL state. The client view calls the typed search
/// resource with `useApiQuery`, which is the usual shape for interactive search results
/// that should refetch as the user edits the query.
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

/// Mod queue parent.
///
/// The scoped middleware protects both this browser route and the matching `/api/mod/*`
/// resources. The view can focus on data loading because the gate is centralized.
pub const MOD: app::View = app::view! {
	client_file: "src/client/views/mod.view.tsx";
	pattern: "/mod";
	input: ();
	output: ModPage;
	handler: |ctx| {
		ctx.head().title(format!("Moderation | {APP_NAME}"));
		let mut batch = vorma::tasks::ParallelBatch::new();
		let killed = batch.add(repo::KILLED_STORIES, db_input(ctx.state(), ()));
		let log = batch.add(repo::MOD_LOG, db_input(ctx.state(), ()));
		let outputs = batch
			.run(ctx.exec_ctx())
			.await
			.map_err(|error| vorma::ViewExit::err(error.to_string()))?;
		let killed = (*outputs.take(killed)).clone();
		let log = (*outputs.take(log)).clone();
		Ok(ModPage { killed, log })
	};
};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct ModDiagnosticsPage {}

/// Deliberately failing child segment.
///
/// This gives the example app a visible route-error-boundary path: the `/mod` parent still
/// renders, while only the diagnostics outlet shows its client-visible message.
pub const MOD_DIAGNOSTICS: app::View = app::view! {
	client_file: "src/client/views/mod_diagnostics.view.tsx";
	pattern: "/mod/diagnostics";
	input: ();
	output: ModDiagnosticsPage;
	handler: |_ctx| {
		Err(vorma::ViewExit::err(
			"diagnostics backend intentionally unwired",
		)
		.with_client_msg("Diagnostics are not available in the demo."))
	};
};
