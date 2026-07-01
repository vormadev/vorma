import { defineView, useViewData } from "../app.tsx";
import { time_ago } from "../format.ts";

export default defineView({
	pattern: "/u/:username/comments",
	component: (props) => {
		const data = useViewData(props);

		return (
			<section aria-label="User comments">
				<h3>Comments</h3>
				{data.comments.length > 0 ? (
					data.comments.map((comment) => {
						return (
							<article className="comment" key={comment.id}>
								<p className="meta">
									{comment.author} · {time_ago(comment.created_at)}
								</p>
								<p>{comment.body}</p>
							</article>
						);
					})
				) : (
					<p className="meta">No comments yet.</p>
				)}
			</section>
		);
	},
});
