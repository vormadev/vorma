import { defineView, useViewData } from "../app.tsx";

export default defineView({
	pattern: "/docs/*",
	component: (props) => {
		const data = useViewData(props);

		if (!data.page) {
			return (
				<article>
					<h2>Doc not found</h2>
					<p className="meta">That page is not in the seeded docs.</p>
				</article>
			);
		}

		return (
			<article className="doc">
				<h2>{data.page.title}</h2>
				{data.page.body.split(/\n\n+/).map((paragraph) => {
					return <p key={paragraph}>{paragraph}</p>;
				})}
			</article>
		);
	},
});
