import { Link, defineView, useRouteState, useViewData } from "../app.tsx";

export default defineView({
	pattern: "/tags/*",
	component: (props) => {
		const data = useViewData(props);
		const pathname = useRouteState((route) => {
			return new URL(route.href).pathname;
		});

		return (
			<main>
				<nav aria-label="All tags" className="tags">
					{data.all_tags.map((tag) => {
						const href = `/tags/${tag}`;
						return (
							<Link
								data-current={pathname === href ? "1" : undefined}
								href={href}
								key={tag}
							>
								#{tag}
							</Link>
						);
					})}
				</nav>
				<h2>
					{data.selected.length === 0
						? "All notes"
						: `Tagged ${data.selected.map((tag) => `#${tag}`).join(" + ")}`}
				</h2>
				<section className="notes">
					{data.notes.map((note) => {
						return (
							<article className="note" key={note.id}>
								<Link href={`/notes/${note.id}`}>#{note.id}</Link>
								<p>{note.body}</p>
							</article>
						);
					})}
				</section>
			</main>
		);
	},
});
