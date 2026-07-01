import { Link, defineView, useRouteState, useViewData } from "../app.tsx";

export default defineView({
	pattern: "/docs",
	component: (props) => {
		const data = useViewData(props);
		/*
		`useRouteState` is for route metadata, not view data. Here the parent
		docs layout checks whether its splat child is active so it can show an
		empty-state prompt only at `/docs`.
		*/
		const has_page = useRouteState((route) => {
			return route.matches.some((match) => {
				return match.pattern === "/docs/*";
			});
		});

		return (
			<main className="split">
				<aside>
					<h2>Docs</h2>
					<nav className="stack">
						{data.index.map((page) => {
							return (
								<Link
									attributeMatchRules={{
										includeHash: false,
										includeSearch: false,
									}}
									key={page.slug}
									pattern="/docs/*"
									prefetchDelayMs={100}
									splatValues={page.slug.split("/")}
								>
									{page.title}
								</Link>
							);
						})}
					</nav>
				</aside>
				<section>
					<props.Outlet />
					{has_page ? null : (
						<p className="meta">Choose a doc from the list.</p>
					)}
				</section>
			</main>
		);
	},
});
