import { useEffect, useState } from "react";
import { useApiQuery } from "../api.ts";
import {
	Link,
	apiClient,
	cancelPrefetch,
	defineView,
	navigate,
	prefetch,
	toHref,
	useRouteSync,
	useViewData,
} from "../app.tsx";
import { domain_of, time_ago } from "../format.ts";

function search_target(query: string) {
	return {
		pattern: "/search" as const,
		search: { q: query || undefined },
	};
}

export default defineView({
	pattern: "/search",
	component: (props) => {
		const data = useViewData(props);
		const [query, set_query] = useState(data.q);
		const [envelope_message, set_envelope_message] = useState<string | null>(null);
		const results = useApiQuery({
			pattern: "/api/search",
			input: { q: data.q },
		});
		/*
		`toHref` is useful when the app needs the canonical URL as data rather
		than as a rendered Link, for example copy/share UI or analytics labels.
		*/
		const canonical_href = toHref(search_target(query));

		useEffect(() => {
			set_query(data.q);
		}, [data.q]);

		useRouteSync({
			/*
			Route sync keeps local input state and the URL together without
			imperative navigation on every keystroke. `replace` avoids filling
			browser history with intermediate search text.
			*/
			pattern: "/search",
			search: { q: query || undefined },
			debounceMs: 250,
			replace: true,
		});

		const probe_search_envelope = async () => {
			/*
			Use non-throwing `query` when the success/error envelope itself is
			part of the UI. Most screens prefer `queryOrThrow` through
			`useApiQuery`.
			*/
			const result = await apiClient.query({
				pattern: "/api/search",
				input: { q: query || undefined },
				skipWorkIndicator: true,
			});
			set_envelope_message(
				result.success
					? `${result.data.stories.length} stories from result envelope`
					: result.error,
			);
		};

		return (
			<main>
				<h2>Search</h2>
				<label className="stack">
					Query
					<input
						onChange={(event) => {
							set_query(event.currentTarget.value);
						}}
						type="search"
						value={query}
					/>
				</label>
				<div className="actions">
					<button
						onClick={() => {
							/*
							Imperative `navigate` is for command-style controls.
							Links should still use `<Link>` so the browser gets a
							real anchor whenever possible.
							*/
							void navigate({
								...search_target(query),
								replace: true,
								state: { source: "search-button" },
							});
						}}
						onFocus={() => {
							prefetch(search_target(query));
						}}
						onMouseEnter={() => {
							prefetch(search_target(query));
						}}
						onMouseLeave={() => {
							/*
							Cancel explicit prefetches when intent ends. Link
							intent prefetch handles this automatically for anchors.
							*/
							cancelPrefetch(search_target(query));
						}}
						type="button"
					>
						Go
					</button>
					<button
						onClick={() => {
							void probe_search_envelope();
						}}
						type="button"
					>
						Check envelope
					</button>
				</div>
				<p className="meta">Canonical: {canonical_href}</p>
				{envelope_message ? <p className="meta">{envelope_message}</p> : null}
				{results.error ? <p className="error">{results.error.message}</p> : null}
				{results.isFetching ? <p className="meta">Searching...</p> : null}
				<ol>
					{results.data?.stories.map((story) => {
						const domain = story.url ? domain_of(story.url) : null;
						return (
							<li className="story" key={story.id}>
								<span className="points">{story.points}</span>
								<span>
									<Link href={`/s/${story.id}`}>{story.title}</Link>
									{domain ? (
										<span className="meta"> ({domain})</span>
									) : null}
								</span>
								<span className="meta">
									by {story.author} {time_ago(story.created_at)}
								</span>
							</li>
						);
					})}
				</ol>
				{results.data && results.data.stories.length === 0 ? (
					<p className="meta">No stories found.</p>
				) : null}
			</main>
		);
	},
});
