import { render_vorma } from "#variant-runtime";
import { createVormaClient } from "#vorma-client";
import { vormaClientSeed } from "#vorma-gen";
import {
	on_vorma_build_skew_detected,
	on_vorma_route_update,
	on_vorma_work_update,
} from "./shared/instrumentation.ts";

export const app = createVormaClient(vormaClientSeed, {
	linkDefaultProps: { prefetch: "intent", visitOnPointerDown: true },
	onBuildSkewDetected: on_vorma_build_skew_detected,
	onRouteUpdate: on_vorma_route_update,
	onWorkUpdate: on_vorma_work_update,
	render: render_vorma,
});

export const {
	Link,
	RootOutlet,
	apiClient,
	cancelPrefetch,
	defineView,
	navigate,
	prefetch,
	revalidate,
	useLoaderData,
	useClientLoaderData,
	useRouteState,
	useWorkState,
} = app;
