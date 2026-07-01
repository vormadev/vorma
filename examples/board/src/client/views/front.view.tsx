import { useApiMutation } from "../api.ts";
import { Link, defineView, useViewData } from "../app.tsx";
import { domain_of, time_ago } from "../format.ts";
import { front_page_size } from "../vorma.gen.ts";

/*
`front_page_size` is exported by the Rust app config through `TsDrafter`.
Use generated constants for server-owned values that the browser should
display or calculate with, instead of duplicating magic numbers in TS.
*/
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
				<p className="meta">
					Page {data.page} · {front_page_size} stories per page
				</p>
				{vote.error ? <p className="error">{vote.error.message}</p> : null}
				<ol>
					{data.stories.map((story) => {
						const domain = story.url ? domain_of(story.url) : null;
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
										/*
										Internal story links use delayed intent prefetch
										and pointerdown navigation because they are high
										confidence next clicks in a dense list.
										*/
										<Link
											href={`/s/${story.id}`}
											prefetchDelayMs={60}
											state={{ source: "front-title" }}
											visitOnPointerDown
										>
											{story.title}
										</Link>
									)}
									{domain ? (
										<span className="meta"> ({domain})</span>
									) : null}
								</span>
								<span className="meta">
									by {story.author} {time_ago(story.created_at)} ·{" "}
									<Link
										href={`/s/${story.id}`}
										prefetchDelayMs={60}
										state={{ source: "front-comments" }}
										visitOnPointerDown
									>
										{story.comment_count} comments
									</Link>
								</span>
							</li>
						);
					})}
				</ol>
				<nav aria-label="Pages">
					{/*
					Pagination links opt into scroll-to-top because they replace
					the list contents. In-page tab links elsewhere keep scroll
					position because their layout context stays the same.
					*/}
					{data.page > 1 ? (
						<Link
							pattern="/_index"
							scrollToTop
							search={{ page: data.page - 1 }}
						>
							Newer
						</Link>
					) : null}{" "}
					{data.has_more ? (
						<Link
							pattern="/_index"
							scrollToTop
							search={{ page: data.page + 1 }}
						>
							Older
						</Link>
					) : null}
				</nav>
			</main>
		);
	},
});
