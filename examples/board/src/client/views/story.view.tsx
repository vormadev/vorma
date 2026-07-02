import { type FormEvent, useEffect, useRef, useState } from "react";
import { QueryError } from "vorma/react";
import { useApiMutation } from "../api.ts";
import {
	Link,
	apiClient,
	defineView,
	useClientLoaderData,
	useViewData,
} from "../app.tsx";
import { domain_of, time_ago } from "../format.ts";

const story_comment_drafts = new Map<string, string>();
const last_story_commit_key = "board:last-story-route-commit";

export type StoryReadState = {
	first_seen_at: number | null;
	seen_before: boolean;
};

type LoadedAttachment = {
	url: string;
	error: string | null;
};

export default defineView({
	pattern: "/s/:story_id",
	/*
	Client loaders run in the browser after the server view data arrives.
	They are for browser-only state or APIs: here, localStorage records
	whether this story has been seen before. `runClientLoaderOnHmr` opts
	this loader into dev-time reruns when the view module changes.
	*/
	runClientLoaderOnHmr: true,
	clientLoader: async ({ params, serverPromise }): Promise<StoryReadState> => {
		const server = await serverPromise;
		const story = server.viewData.story;
		if (!story) {
			return { first_seen_at: null, seen_before: false };
		}

		const key = `board:story:${params.story_id}:seen`;
		const previous = window.localStorage.getItem(key);
		const now = Date.now();
		if (previous === null) {
			window.localStorage.setItem(key, String(now));
		}
		return {
			first_seen_at: Number(previous ?? now),
			seen_before: previous !== null,
		};
	},
	beforeRouteYield: ({ current }) => {
		/*
		Route-yield hooks guard leaving the current route. Use them for real
		client-side state that would be lost on navigation, such as an
		unsent editor draft.
		*/
		const story_id = current.params.story_id;
		const draft = story_id ? story_comment_drafts.get(story_id) : undefined;
		if (draft?.trim() && !window.confirm("Discard your unsent comment?")) {
			throw new Error("navigation cancelled because the comment draft is unsaved");
		}
	},
	beforeRouteCommit: ({ next }) => {
		/*
		Commit hooks run after the next route is ready but before Vorma
		publishes it. They are for last-moment app side effects that should
		only happen for a route that will actually commit.
		*/
		window.sessionStorage.setItem(last_story_commit_key, next.href);
	},
	component: (props) => {
		const data = useViewData(props);
		const loader_data = useClientLoaderData(props);
		const [comment_body, set_comment_body] = useState("");
		const [loaded_attachments, set_loaded_attachments] = useState<
			Record<number, LoadedAttachment>
		>({});
		const comment = useApiMutation({
			method: "POST",
			pattern: "/api/stories/:story_id/comments",
		});
		/*
		A ref mirrors `loaded_attachments` so the unmount cleanup below always
		revokes whatever was loaded most recently without needing the state
		value itself in its dependency array, which would otherwise tear the
		effect down and rebuild it on every attachment load.
		*/
		const loaded_attachments_ref = useRef(loaded_attachments);
		loaded_attachments_ref.current = loaded_attachments;

		useEffect(() => {
			return () => {
				for (const attachment of Object.values(loaded_attachments_ref.current)) {
					URL.revokeObjectURL(attachment.url);
				}
			};
		}, []);

		if (!data.story) {
			return (
				<main>
					<h2>Story not found</h2>
					<p>
						It may have been removed.{" "}
						<Link href="/">Back to the front page.</Link>
					</p>
				</main>
			);
		}

		const story_id = String(data.story.id);
		const domain = data.story.url ? domain_of(data.story.url) : null;

		const submit_comment = (event: FormEvent<HTMLFormElement>) => {
			event.preventDefault();
			const body = comment_body.trim();
			if (body === "") {
				return;
			}
			comment.mutate(
				{
					params: { story_id },
					input: { body },
				},
				{
					onSuccess: () => {
						story_comment_drafts.delete(story_id);
						set_comment_body("");
					},
				},
			);
		};

		const load_attachment = async (attachment_id: number) => {
			try {
				/*
				`queryOrThrow` is the ergonomic path when the UI wants normal
				try/catch flow. This endpoint returns a `Blob`, showing that
				the generated client is typed, not JSON-only. Each attachment
				downloads independently by its own id, since a story can now
				carry several.
				*/
				const blob = await apiClient.queryOrThrow({
					pattern: "/api/stories/:story_id/attachments/:attachment_id",
					params: { story_id, attachment_id: String(attachment_id) },
				});
				set_loaded_attachments((current) => {
					const previous = current[attachment_id];
					if (previous) {
						URL.revokeObjectURL(previous.url);
					}
					return {
						...current,
						[attachment_id]: { url: URL.createObjectURL(blob), error: null },
					};
				});
			} catch (error) {
				set_loaded_attachments((current) => {
					return {
						...current,
						[attachment_id]: {
							url: "",
							error:
								/*
								Typed Vorma errors preserve the original envelope so the UI
								can render the client-visible message without parsing strings.
								*/
								error instanceof QueryError
									? error.result.error
									: error instanceof Error
										? error.message
										: "Attachment unavailable.",
						},
					};
				});
			}
		};

		return (
			<main>
				<article className="story">
					<span className="points">{data.story.points}</span>
					<span>
						{data.story.url ? (
							<a href={data.story.url}>{data.story.title}</a>
						) : (
							<strong>{data.story.title}</strong>
						)}
						{domain ? <span className="meta"> ({domain})</span> : null}
					</span>
					<span className="meta">
						by {data.story.author} {time_ago(data.story.created_at)}
					</span>
				</article>
				{loader_data.first_seen_at ? (
					<p className="meta">
						{loader_data.seen_before ? "First read" : "Marked read"}{" "}
						{new Date(loader_data.first_seen_at).toLocaleString()}
					</p>
				) : null}
				{data.story.body ? <p>{data.story.body}</p> : null}
				{data.tags.length > 0 ? (
					<p className="meta">Tags: {data.tags.join(", ")}</p>
				) : null}
				{data.attachments.length > 0 ? (
					<ul className="attachments">
						{data.attachments.map((attachment) => {
							const loaded = loaded_attachments[attachment.id];
							return (
								<li key={attachment.id}>
									{attachment.file_name}{" "}
									{loaded?.url ? (
										<a
											download={attachment.file_name}
											href={loaded.url}
										>
											Download
										</a>
									) : (
										<button
											onClick={() => {
												void load_attachment(attachment.id);
											}}
											type="button"
										>
											Load
										</button>
									)}
									{loaded?.error ? (
										<span className="error"> {loaded.error}</span>
									) : null}
								</li>
							);
						})}
					</ul>
				) : null}
				<section aria-label="Comments">
					{data.comments.map((comment) => {
						return (
							<article className="comment" key={comment.id}>
								<p className="meta">
									{comment.author} {time_ago(comment.created_at)}
								</p>
								<p>{comment.body}</p>
							</article>
						);
					})}
					{data.comments.length === 0 ? (
						<p className="meta">No comments yet.</p>
					) : null}
					<form className="stack" onSubmit={submit_comment}>
						<label>
							Comment
							<textarea
								onChange={(event) => {
									const next = event.currentTarget.value;
									set_comment_body(next);
									story_comment_drafts.set(story_id, next);
								}}
								required
								rows={4}
								value={comment_body}
							/>
						</label>
						<button disabled={comment.isPending} type="submit">
							Add comment
						</button>
					</form>
					{comment.error ? (
						<p className="error">{comment.error.message}</p>
					) : null}
				</section>
			</main>
		);
	},
});
