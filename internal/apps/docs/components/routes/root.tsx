import { app, Link } from "../../vorma.app.ts";

export default app.defineRoute({
	pattern: "/",
	component: (props) => {
		return (
			<>
				<nav>
					<Link pattern="/">Home</Link>
				</nav>
				<props.Outlet />
			</>
		);
	},
});
