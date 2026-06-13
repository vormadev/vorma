import { Link, defineView, useViewData } from "../app.tsx";
import { domainOf, timeAgo } from "../format.ts";

export default defineView({
	pattern: "/s/:story_id",
	component: (props) => {
		const data = useViewData(props);

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

		const domain = data.story.url ? domainOf(data.story.url) : null;
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
						by {data.story.author} {timeAgo(data.story.created_at)}
					</span>
				</article>
				{data.story.body ? <p>{data.story.body}</p> : null}
				<section aria-label="Comments">
					{data.comments.map((comment) => {
						return (
							<article className="comment" key={comment.id}>
								<p className="meta">
									{comment.author} {timeAgo(comment.created_at)}
								</p>
								<p>{comment.body}</p>
							</article>
						);
					})}
					{data.comments.length === 0 ? (
						<p className="meta">No comments yet.</p>
					) : null}
				</section>
			</main>
		);
	},
});
