import { useAtomValue } from "jotai";
import type { FormEvent } from "react";
import { useEffect, useState } from "react";
import { THEMES } from "vorma/kit/theme";
import { useApiMutation } from "../api.ts";
import {
	Link,
	defineView,
	getRouteState,
	getWorkState,
	navigate,
	revalidate,
	usePatternClientLoaderData,
	usePatternViewData,
	useViewData,
	useWorkState,
} from "../app.tsx";
import {
	read_runtime_observer_snapshot,
	subscribe_runtime_observer,
} from "../runtime_observer.ts";
import { theme_atom, toggle_theme } from "../theme.ts";
import { keyboard_shortcuts } from "../vorma.gen.ts";
import type { StoryReadState } from "./story.view.tsx";

type RuntimeSnapshot = {
	href: string;
	matches: string;
	work: string;
};

function read_runtime_snapshot(): RuntimeSnapshot {
	/*
	The imperative state readers are useful in event handlers or integration
	points that are not React renders. Inside render, prefer selector hooks
	like `useWorkState` so React can update the component automatically.
	*/
	const route = getRouteState();
	const work = getWorkState();
	return {
		href: route.href,
		matches: route.matches.map((match) => match.pattern).join(" > "),
		work:
			[
				work.navigation ? "navigation" : null,
				work.revalidation ? "revalidation" : null,
				work.prefetch ? "prefetch" : null,
				work.apiRequests.length > 0 ? `${work.apiRequests.length} api` : null,
			]
				.filter((part) => {
					return part !== null;
				})
				.join(", ") || "idle",
	};
}

const search_shortcut = keyboard_shortcuts.find((shortcut) => {
	return shortcut.action === "focus-search";
});

const client_mark_url = vormaPublicUrl("mark.svg");

export default defineView({
	pattern: "/",
	component: (props) => {
		const data = useViewData(props);
		const busy = useWorkState((work) => {
			return (
				work.navigation !== null ||
				work.revalidation !== null ||
				work.prefetch !== null ||
				work.apiRequests.length > 0
			);
		});
		/*
		Pattern hooks let a layout read a specific matched child route's data
		without prop-drilling through every outlet. They return undefined when
		that pattern is not currently active.
		*/
		const story_data = usePatternViewData("/s/:story_id");
		const story_read = usePatternClientLoaderData<StoryReadState>("/s/:story_id");
		const [username, set_username] = useState("");
		const [runtime_snapshot, set_runtime_snapshot] = useState(read_runtime_snapshot);
		const [observer_snapshot, set_observer_snapshot] = useState(
			read_runtime_observer_snapshot,
		);
		const theme_state = useAtomValue(theme_atom);
		const sign_in = useApiMutation({ method: "POST", pattern: "/api/session" });
		const sign_out = useApiMutation({ method: "DELETE", pattern: "/api/session" });

		useEffect(() => {
			/*
			App-level route/work/build-skew callbacks are not React hooks, so
			Board adapts them through a normal event subscription.
			*/
			return subscribe_runtime_observer(() => {
				set_observer_snapshot(read_runtime_observer_snapshot());
			});
		}, []);

		useEffect(() => {
			if (!search_shortcut) {
				return;
			}
			/*
			`keyboard_shortcuts` is generated from Rust-owned app data. This is
			useful for configuration the server and browser should share without
			duplicating constants by hand.
			*/
			const handle_keydown = (event: KeyboardEvent) => {
				if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
					event.preventDefault();
					void navigate({ pattern: "/search" });
				}
			};
			window.addEventListener("keydown", handle_keydown);
			return () => {
				window.removeEventListener("keydown", handle_keydown);
			};
		}, []);

		const submit_sign_in = (event: FormEvent<HTMLFormElement>) => {
			event.preventDefault();
			sign_in.mutate(
				{ input: { username } },
				{
					onSuccess: () => {
						set_username("");
					},
				},
			);
		};
		const mark_url_mismatch = data.mark_url !== client_mark_url;

		return (
			<div className="shell" data-busy={busy ? "1" : undefined}>
				<header className="topbar">
					<Link className="brand" href="/" state={{ source: "topbar-home" }}>
						{/*
						Browser modules use `vormaPublicUrl()` when the client owns
						the asset reference. Server handlers use `ctx.public_url()`
						when the resolved URL belongs in typed view data. Both resolve
						through the same public asset map.
						*/}
						<img alt="" className="mark" src={client_mark_url} />
						<h1>{data.app_name}</h1>
					</Link>
					<nav>
						<Link pattern="/submit" prefetchDelayMs={80}>
							Submit
						</Link>
						<Link
							pattern="/search"
							prefetchDelayMs={120}
							state={{ source: "topbar-search" }}
						>
							Search
							{search_shortcut ? <kbd>{search_shortcut.keys}</kbd> : null}
						</Link>
						<Link
							attributeMatchRules={{
								includeHash: false,
								includeSearch: false,
							}}
							pattern="/docs"
						>
							Docs
						</Link>
						{data.current_user ? (
							<Link pattern="/mod" visitOnPointerDown>
								Mod
							</Link>
						) : null}
					</nav>
					{data.current_user ? (
						<>
							<Link
								pattern="/u/:username"
								params={{ username: data.current_user.username }}
								skipWorkIndicator
							>
								{data.current_user.username}
							</Link>
							<button
								disabled={sign_out.isPending}
								onClick={() => {
									sign_out.mutate({});
								}}
								type="button"
							>
								Sign out
							</button>
						</>
					) : (
						<form onSubmit={submit_sign_in}>
							<input
								aria-label="Username"
								onChange={(event) => {
									set_username(event.currentTarget.value);
								}}
								placeholder="username"
								value={username}
							/>
							<button disabled={sign_in.isPending} type="submit">
								Sign in
							</button>
						</form>
					)}
					<button onClick={toggle_theme} type="button">
						Theme: {theme_state.theme}
						{theme_state.theme === THEMES.System
							? ` (${theme_state.resolved})`
							: null}
					</button>
				</header>
				{sign_in.error ? <p className="error">{sign_in.error.message}</p> : null}
				{mark_url_mismatch ? (
					<p className="error">Public asset manifest mismatch.</p>
				) : null}
				<section className="runtime-panel" aria-label="Runtime status">
					<div>
						<strong>Route</strong>{" "}
						<span className="meta">{runtime_snapshot.href}</span>
					</div>
					<div>
						<strong>Matches</strong>{" "}
						<span className="meta">{runtime_snapshot.matches || "none"}</span>
					</div>
					<div>
						<strong>Work</strong>{" "}
						<span className="meta">{runtime_snapshot.work}</span>
					</div>
					{story_data?.story ? (
						<div>
							<strong>Reading</strong>{" "}
							<span className="meta">
								{story_data.story.title}
								{story_read?.first_seen_at
									? ` since ${new Date(
											story_read.first_seen_at,
										).toLocaleTimeString()}`
									: null}
							</span>
						</div>
					) : null}
					{observer_snapshot.route_href ? (
						<div>
							<strong>Last route update</strong>{" "}
							<span className="meta">
								{observer_snapshot.route_reason} ·{" "}
								{observer_snapshot.route_href}
							</span>
						</div>
					) : null}
					{observer_snapshot.build_skew ? (
						<p className="error">
							Build skew: {observer_snapshot.build_skew}
						</p>
					) : null}
					<div className="actions">
						<button
							onClick={() => {
								set_runtime_snapshot(read_runtime_snapshot());
							}}
							type="button"
						>
							Snapshot
						</button>
						<button
							onClick={() => {
								void revalidate();
							}}
							type="button"
						>
							Refresh route data
						</button>
					</div>
				</section>
				<props.Outlet />
				<footer className="site-stats meta">
					{/*
					These totals come from the server's `single_flight` task, so a
					burst of concurrent requests shares one aggregate scan while
					each fresh page still reflects the current database.
					*/}
					{data.stats.stories} stories · {data.stats.comments} comments ·{" "}
					{data.stats.votes} votes
				</footer>
			</div>
		);
	},
});
