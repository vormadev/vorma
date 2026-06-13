//! View declarations: the nested rendering tree.
/*
Pattern shapes exercised here: a root layout ("/"), an explicit index
("/_index"), a dynamic param ("/notes/:note_id"), a splat ("/tags" + star), a
client-loader/error-boundary host ("/stats"), a middleware-gated section
("/admin"), and a pure redirect ("/legacy").
*/

use std::sync::LazyLock;
use std::time::Duration;

use serde::{Deserialize, Serialize};
use vorma::Task;

use crate::store::{Note, note_store};
use crate::{APP_DESCRIPTION, APP_NAME, app};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct LayoutData {
	app_name: String,
	mark_url: String,
	signed_in: bool,
}

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct HomeInput {
	#[serde(default)]
	draft: Option<String>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct HomePage {
	draft: Option<String>,
	notes: Vec<Note>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct NotePage {
	note_id: String,
	note: Option<Note>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct TagsPage {
	/// Tag path segments the splat matched (empty = the tag index).
	selected: Vec<String>,
	notes: Vec<Note>,
	all_tags: Vec<String>,
}

#[derive(Clone, Debug, Default, Deserialize, vorma::TsGen)]
pub struct StatsInput {
	#[serde(default)]
	fail: Option<bool>,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct StatsPage {
	note_count: usize,
	tag_count: usize,
	longest_body_chars: usize,
}

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct AdminPage {
	session: String,
}

/// Root layout: shell facts every nested view renders under.
pub const LAYOUT: app::View = app::view! {
	client_file: "src/client/views/layout.view.tsx";
	pattern: "/";
	input: ();
	output: LayoutData;
	handler: |ctx| {
		let mark_url = ctx.public_url("mark.svg")?;
		ctx.head()
			.title(APP_NAME)
			.description(APP_DESCRIPTION)
			.icon(mark_url.clone());
		Ok(LayoutData {
			app_name: APP_NAME.to_owned(),
			mark_url,
			signed_in: crate::session::session_of(ctx.request().headers()).is_some(),
		})
	};
};

/// Home index under the layout.
pub const HOME: app::View = app::view! {
	client_file: "src/client/views/home.view.tsx";
	pattern: "/_index";
	input: HomeInput;
	output: HomePage;
	handler: |ctx| {
		Ok(HomePage {
			draft: ctx.input().draft.clone(),
			notes: note_store(ctx.state())?.notes.clone(),
		})
	};
};

pub const NOTE: app::View = app::view! {
	client_file: "src/client/views/note.view.tsx";
	pattern: "/notes/:note_id";
	input: ();
	output: NotePage;

	handler: |ctx| {
		let note_id = ctx.param("note_id").to_owned();
		let store = note_store(ctx.state())?;
		let note = note_id.parse::<u64>().ok().and_then(|id| {
			store.notes.iter().find(|note| note.id == id).cloned()
		});

		if let Some(note) = &note {
			ctx.head()
				.title(format!("Note #{}", note.id))
				.description("Read a tiny note.");
		} else {
			/*
			A missing note is a rendered UI state, not an HTTP error: views
			are a rendering protocol, so the page commits normally and the
			component shows the not-found treatment.
			*/
			ctx.head()
				.title("Note not found")
				.description("The requested note was not found.");
		}

		Ok(NotePage { note_id, note })
	};
};

/// Splat view: `/tags`, `/tags/api`, `/tags/api/coverage`, ...
pub const TAGS: app::View = app::view! {
	client_file: "src/client/views/tags.view.tsx";
	pattern: "/tags/*";
	input: ();
	output: TagsPage;
	handler: |ctx| {
		let selected = ctx
			.splat_values()
			.iter()
			.map(|segment| segment.to_ascii_lowercase())
			.collect::<Vec<_>>();
		let store = note_store(ctx.state())?;
		let notes = store
			.notes
			.iter()
			.filter(|note| selected.iter().all(|tag| note.tags.contains(tag)))
			.cloned()
			.collect::<Vec<_>>();
		let mut all_tags = store
			.notes
			.iter()
			.flat_map(|note| note.tags.iter().cloned())
			.collect::<Vec<_>>();
		all_tags.sort();
		all_tags.dedup();

		ctx.head().title(if selected.is_empty() {
			"Tags".to_owned()
		} else {
			format!("Tag: {}", selected.join("/"))
		});

		Ok(TagsPage {
			selected,
			notes,
			all_tags,
		})
	};
};

/*
A cached subtask: same-input runs within the TTL are deduplicated across
concurrent requests, which is the task runtime's whole point. The handler
awaits it through the request's ExecCtx so cancellation propagates.
*/
static STATS_TASK: LazyLock<Task<usize, (usize, usize), vorma::Error>> = LazyLock::new(|| {
	Task::new(
		Duration::from_millis(250),
		|_ctx, longest: usize| async move { Ok((longest, longest.div_euclid(2))) },
	)
});

pub const STATS: app::View = app::view! {
	client_file: "src/client/views/stats.view.tsx";
	pattern: "/stats";
	input: StatsInput;
	output: StatsPage;
	handler: |ctx| {
		if ctx.input().fail == Some(true) {
			/*
			Segment error: the layout above still renders, HTTP stays 200,
			and ONLY the explicit client message reaches the error boundary.
			*/
			return Err(vorma::ViewExit::err("stats failed: fail=true requested")
				.with_client_msg("Stats are unavailable right now."));
		}

		let (note_count, tag_count, longest) = {
			let store = note_store(ctx.state())?;
			let mut tags = store
				.notes
				.iter()
				.flat_map(|note| note.tags.iter().cloned())
				.collect::<Vec<_>>();
			tags.sort();
			tags.dedup();
			let longest = store
				.notes
				.iter()
				.map(|note| note.body.chars().count())
				.max()
				.unwrap_or(0);
			(store.notes.len(), tags.len(), longest)
		};
		let (longest_body_chars, _half) = *STATS_TASK
			.run(ctx.exec_ctx(), longest)
			.await
			.map_err(|source| vorma::ViewExit::err(source.to_string()))?;

		ctx.head().title("Stats");
		Ok(StatsPage {
			note_count,
			tag_count,
			longest_body_chars,
		})
	};
};

/// Gated by the session middleware (redirect / 401 live there).
pub const ADMIN: app::View = app::view! {
	client_file: "src/client/views/admin.view.tsx";
	pattern: "/admin";
	input: ();
	output: AdminPage;
	handler: |ctx| {
		let session = crate::session::session_of(ctx.request().headers())
			.unwrap_or_else(|| "anonymous".to_owned());
		ctx.head().title("Admin");
		Ok(AdminPage { session })
	};
};

#[derive(Clone, Debug, Serialize, vorma::TsGen)]
pub struct LegacyPage {}

/// Permanently moved: the handler IS the redirect.
pub const LEGACY: app::View = app::view! {
	client_file: "src/client/views/legacy.view.tsx";
	pattern: "/legacy";
	input: ();
	output: LegacyPage;
	handler: |ctx| ctx.redirect("/");
};
