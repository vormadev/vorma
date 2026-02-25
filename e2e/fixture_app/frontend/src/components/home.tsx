import type { RouteProps } from "../vorma.gen/index.ts";
import { useLoaderData } from "../vorma.bindings.ts";

export function Home(props: RouteProps<"/_index">) {
	const data = useLoaderData(props);

	return (
		<section id="e2e-home-route">
			<p id="e2e-home-message">{data().message}</p>
			<p id="e2e-home-count">{data().count}</p>
		</section>
	);
}
