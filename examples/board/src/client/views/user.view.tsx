import { Link, defineView, useViewData } from "../app.tsx";
import { time_ago } from "../format.ts";

export default defineView({
	pattern: "/u/:username",
	component: (props) => {
		const data = useViewData(props);

		if (!data.profile) {
			return (
				<main>
					<h2>User not found</h2>
					<p>
						That profile does not exist.{" "}
						<Link href="/">Back to the front page.</Link>
					</p>
				</main>
			);
		}

		return (
			<main>
				<header className="section-header">
					<div>
						<h2>{data.profile.username}</h2>
						<p className="meta">
							{data.profile.karma} karma · {data.profile.story_count}{" "}
							stories · {data.profile.comment_count} comments
						</p>
					</div>
					<nav className="tabs">
						{/*
						Tabs use `replace` and preserve scroll because switching
						between sibling views is a local state change inside the
						profile section, not a new document-like navigation.
						*/}
						<Link
							pattern="/u/:username"
							params={{ username: data.profile.username }}
							replace
							scrollToTop={false}
							state={{ tab: "stories" }}
						>
							Stories
						</Link>
						<Link
							pattern="/u/:username/comments"
							params={{ username: data.profile.username }}
							replace
							scrollToTop={false}
							state={{ tab: "comments" }}
						>
							Comments
						</Link>
					</nav>
				</header>
				{data.stories.length > 0 ? (
					<ol>
						{data.stories.map((story) => {
							return (
								<li className="story" key={story.id}>
									<span className="points">{story.points}</span>
									<span>
										<Link
											href={`/s/${story.id}`}
											state={{ source: "profile-story" }}
										>
											{story.title}
										</Link>
									</span>
									<span className="meta">
										{time_ago(story.created_at)}
									</span>
								</li>
							);
						})}
					</ol>
				) : (
					<p className="meta">No stories yet.</p>
				)}
				<props.Outlet />
			</main>
		);
	},
});
