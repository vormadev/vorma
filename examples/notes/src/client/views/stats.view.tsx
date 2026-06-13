import { defineView, useClientLoaderData, useViewData } from "../app.tsx";

export default defineView({
	pattern: "/stats",
	/*
	The client loader runs alongside the server payload: it awaits the
	server data (serverPromise) and derives a client-only fact from it.
	*/
	clientLoader: async ({ serverPromise }) => {
		const stats = (await serverPromise).viewData;
		return {
			derived_density:
				stats.note_count === 0
					? 0
					: Math.round((stats.tag_count / stats.note_count) * 100) / 100,
			loaded_at: new Date().toISOString(),
		};
	},
	/*
	A failing stats handler commits the page with this boundary in the
	failed slot; the message is the server's explicit client text.
	*/
	errorBoundary: ({ error }) => {
		return (
			<main>
				<p className="error">
					{error instanceof Error ? error.message : String(error)}
				</p>
			</main>
		);
	},
	component: (props) => {
		const data = useViewData(props);
		const client_facts = useClientLoaderData(props);

		return (
			<main>
				<h2>Stats</h2>
				<dl className="stats">
					<dt>Notes</dt>
					<dd>{data.note_count}</dd>
					<dt>Tags</dt>
					<dd>{data.tag_count}</dd>
					<dt>Longest note</dt>
					<dd>{data.longest_body_chars} chars</dd>
					<dt>Tag density</dt>
					<dd>{client_facts.derived_density}</dd>
				</dl>
				<p className="hint">Computed {client_facts.loaded_at}</p>
			</main>
		);
	},
});
