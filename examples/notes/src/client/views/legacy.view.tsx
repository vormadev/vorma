import { defineView } from "../app.tsx";

/*
The server handler always redirects; this component exists only to keep
the route's module contract complete.
*/
export default defineView({
	pattern: "/legacy",
	component: () => {
		return <p>Redirecting…</p>;
	},
});
