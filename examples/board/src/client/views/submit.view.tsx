import { type ChangeEvent, type FormEvent, useState } from "react";
import { useApiMutation } from "../api.ts";
import { Link, defineView, useViewData, workIndicator } from "../app.tsx";

export default defineView({
	pattern: "/submit",
	component: (props) => {
		const data = useViewData(props);
		const [attachment_summary, set_attachment_summary] = useState<string | null>(
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
			uploads out of JSON while preserving endpoint typing.
			*/
			const input = new FormData(form);
			submit_story.mutate(
				{ input },
				{
					onSuccess: () => {
						form.reset();
					},
				},
			);
		};

		const preview_attachment = async (event: ChangeEvent<HTMLInputElement>) => {
			const file = event.currentTarget.files?.[0];
			if (!file) {
				set_attachment_summary(null);
				return;
			}
			const active_before = workIndicator.isActive();
			/*
			`workIndicator.track` is for app-owned async work Vorma cannot see.
			API calls and navigations are tracked automatically by the client.
			*/
			const text = await workIndicator.track(file.text());
			const active_after = workIndicator.isActive();
			set_attachment_summary(
				`${file.name}: ${text.length} characters parsed; indicator ` +
					`${active_before ? "was active" : "was idle"} before parse and ` +
					`${active_after ? "is active" : "is idle"} after parse`,
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
					<label>
						Attachment
						<input
							name="attachment"
							onChange={(event) => {
								void preview_attachment(event);
							}}
							type="file"
						/>
					</label>
					{attachment_summary ? (
						<p className="meta">{attachment_summary}</p>
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
