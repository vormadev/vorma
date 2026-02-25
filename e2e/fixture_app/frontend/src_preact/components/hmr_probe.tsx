import type { RouteProps } from "../vorma.gen/index.ts";

export const hmrProbeToken = "hmr-probe-baseline";

export function HMRProbe(_props: RouteProps<"/hmr-probe">) {
	return (
		<section id="e2e-hmr-probe-route">
			<p id="e2e-hmr-probe-token">{hmrProbeToken}</p>
		</section>
	);
}
