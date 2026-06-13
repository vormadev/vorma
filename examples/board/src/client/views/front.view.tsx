import { useApiMutation } from "../api.ts";
import { Link, defineView, useViewData } from "../app.tsx";
import { domainOf, timeAgo } from "../format.ts";

export default defineView({
	pattern: "/_index",
	component: (props) => {
		const data = useViewData(props);
		/*
		One hook serves the whole list: the endpoint is fixed; which
		story varies per click, so params ride mutate(). Route data
		(points, ordering) refreshes through vorma auto-revalidation.
		*/
		const vote = useApiMutation({
			method: "POST",
			pattern: "/api/stories/:story_id/vote",
		});

		return (
			<main>
				{vote.error ? <p className="error">{vote.error.message}</p> : null}
				<ol>
					{data.stories.map((story) => {
						const domain = story.url ? domainOf(story.url) : null;
						return (
							<li className="story" key={story.id}>
								<span className="points">{story.points}</span>
								<span>
									<button
										aria-label={`Vote for ${story.title}`}
										disabled={vote.isPending}
										onClick={() => {
											vote.mutate({
												params: { story_id: String(story.id) },
											});
										}}
										type="button"
									>
										▲
									</button>{" "}
									{story.url ? (
										<a href={story.url}>{story.title}</a>
									) : (
										<Link href={`/s/${story.id}`}>{story.title}</Link>
									)}
									{domain ? (
										<span className="meta"> ({domain})</span>
									) : null}
								</span>
								<span className="meta">
									by {story.author} {timeAgo(story.created_at)} ·{" "}
									<Link href={`/s/${story.id}`}>
										{story.comment_count} comments
									</Link>
								</span>
							</li>
						);
					})}
				</ol>
				<nav aria-label="Pages">
					{data.page > 1 ? (
						<Link pattern="/_index" search={{ page: data.page - 1 }}>
							Newer
						</Link>
					) : null}{" "}
					{data.has_more ? (
						<Link pattern="/_index" search={{ page: data.page + 1 }}>
							Older
						</Link>
					) : null}
				</nav>
			</main>
		);
	},
});
