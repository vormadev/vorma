import { useState } from "react";
import { useApiMutation } from "../api.ts";
import { Link, defineView, useViewData } from "../app.tsx";
import { time_ago } from "../format.ts";
import type { ModAction } from "../vorma.gen.ts";

export default defineView({
	pattern: "/mod",
	component: (props) => {
		const data = useViewData(props);
		const [trigger_default_error, set_trigger_default_error] = useState(false);
		const kill_story = useApiMutation({
			method: "POST",
			pattern: "/api/mod/stories/:story_id/kill",
		});
		const restore_story = useApiMutation({
			method: "POST",
			pattern: "/api/mod/stories/:story_id/restore",
		});
		const busy = kill_story.isPending || restore_story.isPending;

		const run_action = (action: ModAction, story_id: number) => {
			const params = { story_id: String(story_id) };
			if (action === "kill") {
				kill_story.mutate({ params });
				return;
			}
			restore_story.mutate({ params });
		};

		if (trigger_default_error) {
			throw new Error("moderation default boundary probe");
		}

		return (
			<main>
				<header className="section-header">
					<div>
						<h2>Moderation</h2>
						<p className="meta">{data.killed.length} killed stories</p>
					</div>
					<div className="actions">
						<Link pattern="/mod/diagnostics">Diagnostics</Link>
						<button
							onClick={() => {
								set_trigger_default_error(true);
							}}
							type="button"
						>
							Trigger default boundary
						</button>
					</div>
				</header>
				{kill_story.error ? (
					<p className="error">{kill_story.error.message}</p>
				) : null}
				{restore_story.error ? (
					<p className="error">{restore_story.error.message}</p>
				) : null}
				<section>
					<h3>Queue</h3>
					{data.killed.length > 0 ? (
						<ol>
							{data.killed.map((story) => {
								return (
									<li className="story" key={story.id}>
										<span className="points">{story.points}</span>
										<span>
											<Link href={`/s/${story.id}`}>
												{story.title}
											</Link>
										</span>
										<span className="meta">
											by {story.author} {time_ago(story.created_at)}
										</span>
										<button
											disabled={busy}
											onClick={() => {
												run_action("restore", story.id);
											}}
											type="button"
										>
											Restore
										</button>
									</li>
								);
							})}
						</ol>
					) : (
						<p className="meta">No killed stories.</p>
					)}
				</section>
				<section>
					<h3>Log</h3>
					{data.log.length > 0 ? (
						<ol>
							{data.log.map((entry) => {
								return (
									<li key={entry.id}>
										{entry.moderator} {entry.action} story{" "}
										{entry.story_id}{" "}
										<span className="meta">
											{time_ago(entry.created_at)}
										</span>
									</li>
								);
							})}
						</ol>
					) : (
						<p className="meta">No moderation events.</p>
					)}
				</section>
				<props.Outlet />
			</main>
		);
	},
});
