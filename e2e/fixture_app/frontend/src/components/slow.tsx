import type { RouteProps } from "../vorma.gen/index.ts";
import { useLoaderData } from "../vorma.bindings.ts";

export function Slow(props: RouteProps<"/slow/:bucket">) {
	const data = useLoaderData(props);

	return (
		<section id="e2e-slow-route">
			<p id="e2e-slow-bucket">{data().bucket}</p>
			<p id="e2e-slow-token">{data().token}</p>
			<p id="e2e-slow-delay">{data().delayMs}</p>
			<p id="e2e-slow-sequence">{data().sequence}</p>
		</section>
	);
}
