import type { RouteProps } from "../vorma.gen/index.ts";
import { useLoaderData } from "../vorma.bindings.ts";

export function User(props: RouteProps<"/users/:id">) {
	const data = useLoaderData(props);

	return (
		<section id="e2e-user-route">
			<p id="e2e-user-id">{data().id}</p>
			<p id="e2e-user-query">{data().query}</p>
			<p id="e2e-user-mode">{data().buildTag}</p>
		</section>
	);
}
