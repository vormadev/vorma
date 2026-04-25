import { createVormaClient } from "#vorma-client";
import { vormaAppConfig } from "#vorma-gen";

export const app = createVormaClient(vormaAppConfig, {
	linkDefaultProps: { prefetch: "intent", visitOnPointerDown: true },
});

export const {
	Link,
	RootOutlet,
	apiClient,
	cancelPrefetch,
	defineRoute,
	navigate,
	prefetch,
	revalidate,
	useLoaderData,
	useClientLoaderData,
	useRouteState,
	useWorkState,
} = app;
