import type { FormEvent } from "react";
import { useState } from "react";
import { Link, apiClient, app, defineView, useViewData } from "../app.tsx";
import { keyboardShortcuts } from "../vorma.gen.ts";

export default defineView({
	pattern: "/_index",
	component: (props) => {
		const data = useViewData(props);
		const [body, set_body] = useState(data.draft ?? "");
		/*
		Keep the draft in the URL (debounced) so a refresh or shared link
		restores the composer exactly.
		*/
		app.useRouteSync({
			pattern: "/_index",
			search: body.trim() === "" ? {} : { draft: body },
			debounceMs: 350,
		});
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
					pattern: "/api/notes",
					input: { body: trimmed },
				});
				set_body("");
			} catch (cause) {
				/*
				The message is the server's explicit client text, carried by
				the JSON error envelope (e.g. "note body is required").
				*/
				set_error(
					cause instanceof Error ? cause.message : "Could not save note.",
				);
			} finally {
				set_saving(false);
			}
		};

		const import_notes = async (file: File) => {
			const form = new FormData();
			form.append("file", file);
			set_error(null);
			try {
				await apiClient.mutateOrThrow({
					method: "POST",
					pattern: "/api/notes/import",
					input: form,
				});
			} catch (cause) {
				set_error(
					cause instanceof Error ? cause.message : "Could not import notes.",
				);
			}
		};

		return (
			<main>
				<form className="composer" onSubmit={submit_note}>
					<label htmlFor="note-body">
						Note <kbd>{keyboardShortcuts[0]?.keys}</kbd>
					</label>
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
					<label className="import">
						Import file
						<input
							onChange={(event) => {
								const file = event.currentTarget.files?.[0];
								if (file) {
									void import_notes(file);
								}
							}}
							type="file"
						/>
					</label>
					{error ? <p className="error">{error}</p> : null}
				</form>

				<section aria-label="Notes" className="notes">
					{data.notes.map((note) => {
						return (
							<article className="note" key={note.id}>
								<Link href={`/notes/${note.id}`}>#{note.id}</Link>
								<p>{note.body}</p>
								<p className="tags">
									{note.tags.map((tag) => {
										return (
											<Link href={`/tags/${tag}`} key={tag}>
												#{tag}
											</Link>
										);
									})}
								</p>
							</article>
						);
					})}
				</section>
			</main>
		);
	},
});
