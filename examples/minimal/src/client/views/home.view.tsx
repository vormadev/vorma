import type { FormEvent } from "react";
import { useState } from "react";
import { Link, apiClient, defineView, useLoaderData } from "../app.tsx";

export default defineView({
	pattern: "/",
	component: (props) => {
		const data = useLoaderData(props);
		const [body, set_body] = useState(data.draft ?? "");
		const [error, set_error] = useState<string | null>(null);
		const [saving, set_saving] = useState(false);

		const submit_note = async (event: FormEvent<HTMLFormElement>) => {
			event.preventDefault();
			const trimmed = body.trim();
			if (trimmed === "") {
				set_error("Write a note first.");
				return;
			}

			set_error(null);
			set_saving(true);
			try {
				await apiClient.mutateOrThrow({
					method: "POST",
					pattern: "/notes",
					input: { body: trimmed },
				});
				set_body("");
			} catch (cause) {
				set_error(
					cause instanceof Error ? cause.message : "Could not save note.",
				);
			} finally {
				set_saving(false);
			}
		};

		return (
			<main className="shell">
				<header className="topbar">
					<img alt="" className="mark" src={data.mark_url} />
					<h1>{data.app_name}</h1>
				</header>

				<form className="composer" onSubmit={submit_note}>
					<label htmlFor="note-body">Note</label>
					<div className="composer_row">
						<input
							id="note-body"
							onChange={(event) => {
								set_body(event.currentTarget.value);
							}}
							placeholder="Write a short note"
							value={body}
						/>
						<button disabled={saving} type="submit">
							{saving ? "Saving" : "Add"}
						</button>
					</div>
					{error ? <p className="error">{error}</p> : null}
				</form>

				<section aria-label="Notes" className="notes">
					{data.notes.map((note) => {
						return (
							<article className="note" key={note.id}>
								<Link href={`/notes/${note.id}`}>#{note.id}</Link>
								<p>{note.body}</p>
							</article>
						);
					})}
				</section>
			</main>
		);
	},
});
