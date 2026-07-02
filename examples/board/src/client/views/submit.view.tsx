import { type ChangeEvent, type FormEvent, useState } from "react";
import { useApiMutation } from "../api.ts";
import { Link, defineView, useViewData, workIndicator } from "../app.tsx";
import {
	max_story_attachment_bytes,
	max_story_attachments,
	story_tags,
} from "../vorma.gen.ts";

export default defineView({
	pattern: "/submit",
	component: (props) => {
		const data = useViewData(props);
		const [attachment_summary, set_attachment_summary] = useState<string | null>(
			null,
		);
		const [attachment_warning, set_attachment_warning] = useState<string | null>(
			null,
		);
		const submit_story = useApiMutation({
			method: "POST",
			pattern: "/api/stories",
		});

		const submit = (event: FormEvent<HTMLFormElement>) => {
			event.preventDefault();
			const form = event.currentTarget;
			/*
			Generated clients can accept `FormData` directly when the resource
			input is `vorma::FormData`. That keeps browser-native forms and file
			uploads out of JSON while preserving endpoint typing. A repeated
			`<input type="file" multiple name="attachment">` and a checkbox
			group sharing `name="tag"` both serialize the same way any repeated
			HTML form field does: several entries under one name in the same
			`FormData`. The server reads them back with `files_named("attachment")`
			and `texts("tag")`, the multi-value counterparts to the single-value
			`file()`/`text()` this form already used for its other fields.
			*/
			const input = new FormData(form);
			submit_story.mutate(
				{ input },
				{
					onSuccess: () => {
						form.reset();
						set_attachment_summary(null);
						set_attachment_warning(null);
					},
				},
			);
		};

		const preview_attachments = async (event: ChangeEvent<HTMLInputElement>) => {
			const files = Array.from(event.currentTarget.files ?? []);
			if (files.length === 0) {
				set_attachment_summary(null);
				set_attachment_warning(null);
				return;
			}
			/*
			`workIndicator.track` is for app-owned async work Vorma cannot see.
			API calls and navigations are tracked automatically by the client.
			Parsing every selected file's text is genuine async work the
			indicator should reflect while it runs, same as the single-file
			version of this preview did.
			*/
			const texts = await workIndicator.track(
				Promise.all(files.map((file) => file.text())),
			);
			const total_bytes = files.reduce((sum, file) => sum + file.size, 0);
			const total_chars = texts.reduce((sum, text) => sum + text.length, 0);
			set_attachment_summary(
				`${files.length} file${files.length === 1 ? "" : "s"}, ` +
					`${total_chars} characters parsed, ${total_bytes} bytes total`,
			);
			/*
			The server enforces `max_story_attachments`/`max_story_attachment_bytes`
			for real; this is only an early warning so the submitter finds out
			before uploading instead of after a rejected POST. Both constants come
			from the same generated module the server's own validation reads its
			values from, so the two checks can never drift apart.
			*/
			set_attachment_warning(
				files.length > max_story_attachments
					? `Attach at most ${max_story_attachments} files.`
					: total_bytes > max_story_attachment_bytes
						? "Attachments are too large altogether."
						: null,
			);
		};

		if (!data.signed_in) {
			return (
				<main>
					<h2>Submit</h2>
					<p>
						Sign in from the header before submitting.{" "}
						<Link href="/">Back to the front page.</Link>
					</p>
				</main>
			);
		}

		return (
			<main>
				<h2>Submit</h2>
				<form className="stack" onSubmit={submit}>
					<label>
						Title
						<input maxLength={120} name="title" required />
					</label>
					<label>
						URL
						<input name="url" placeholder="https://example.com" type="url" />
					</label>
					<label>
						Text
						<textarea name="body" rows={6} />
					</label>
					<fieldset>
						<legend>Tags</legend>
						{story_tags.map((tag) => {
							return (
								<label key={tag}>
									<input name="tag" type="checkbox" value={tag} />
									{tag}
								</label>
							);
						})}
					</fieldset>
					<label>
						Attachments
						<input
							multiple
							name="attachment"
							onChange={(event) => {
								void preview_attachments(event);
							}}
							type="file"
						/>
					</label>
					{attachment_summary ? (
						<p className="meta">{attachment_summary}</p>
					) : null}
					{attachment_warning ? (
						<p className="error">{attachment_warning}</p>
					) : null}
					<button disabled={submit_story.isPending} type="submit">
						Submit story
					</button>
				</form>
				{submit_story.error ? (
					<p className="error">{submit_story.error.message}</p>
				) : null}
			</main>
		);
	},
});
