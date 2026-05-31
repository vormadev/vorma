import { Link, defineView, useLoaderData } from "../app.tsx";

export default defineView({
	pattern: "/notes/:note_id",
	component: (props) => {
		const data = useLoaderData(props);

		return (
			<main className="shell">
				<header className="topbar">
					<Link href="/">Back</Link>
					<h1>{data.app_name}</h1>
				</header>
				<section className="notes">
					{data.note ? (
						<article className="note">
							<h2>Note #{data.note.id}</h2>
							<p>{data.note.body}</p>
						</article>
					) : (
						<article className="note">
							<h2>Note not found</h2>
							<p>No note exists for {data.note_id}.</p>
						</article>
					)}
				</section>
			</main>
		);
	},
});
