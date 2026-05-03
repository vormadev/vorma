import { app, Link } from "../../vorma.app.ts";

export default app.defineView({
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
