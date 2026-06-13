import { Link, apiClient, defineView, navigate, useViewData } from "../app.tsx";

export default defineView({
	pattern: "/notes/:note_id",
	/*
	beforeRouteCommit runs after the payload lands and before the UI
	swaps — the place for view-scoped document side effects.
	*/
	beforeRouteCommit: () => {
		document.documentElement.dataset.section = "note";
	},
	component: (props) => {
		const data = useViewData(props);

		const remove = async (id: number) => {
			await apiClient.mutateOrThrow({
				method: "DELETE",
				pattern: "/api/notes/:note_id",
				params: { note_id: String(id) },
				input: undefined,
			});
			await navigate({ pattern: "/_index" });
		};

		const duplicate = async (id: number) => {
			/*
			The server answers with a redirect at the fresh copy; the api
			client surfaces it and the router follows.
			*/
			await apiClient.mutateOrThrow({
				method: "POST",
				pattern: "/api/notes/:note_id/duplicate",
				params: { note_id: String(id) },
				input: undefined,
			});
		};

		return (
			<main>
				<section className="notes">
					{data.note ? (
						<article className="note">
							<h2>Note #{data.note.id}</h2>
							<p>{data.note.body}</p>
							<div className="note_actions">
								<button
									onClick={() => {
										void duplicate(data.note!.id);
									}}
									type="button"
								>
									Duplicate
								</button>
								<button
									onClick={() => {
										void remove(data.note!.id);
									}}
									type="button"
								>
									Delete
								</button>
							</div>
						</article>
					) : (
						<article className="note">
							<h2>Note not found</h2>
							<p>No note exists for {data.note_id}.</p>
							<Link href="/">Back to notes</Link>
						</article>
					)}
				</section>
			</main>
		);
	},
});
