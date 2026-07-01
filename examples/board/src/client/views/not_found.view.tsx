import { Link, defineView, useViewData } from "../app.tsx";

export default defineView({
	pattern: "/*",
	component: (props) => {
		const data = useViewData(props);

		return (
			<main>
				<h2>Page not found</h2>
				{/*
				A catch-all view is an app-level fallback. Because it matched,
				the HTTP response is still a successful page render; use
				resource or middleware errors for real HTTP error statuses.
				*/}
				<p>
					Board does not have a page for <code>{data.requested_path}</code>.
				</p>
				{data.primary_source ? (
					<p className="meta">Primary source: {data.primary_source}.</p>
				) : null}
				{data.source_tags.length > 0 ? (
					<p className="meta">Arrived from {data.source_tags.join(", ")}.</p>
				) : null}
				{data.source_pair_count > 0 ? (
					<p className="meta">Source markers: {data.source_pair_count}.</p>
				) : null}
				<p>
					<Link href="/">Back to the front page.</Link>
				</p>
			</main>
		);
	},
});
