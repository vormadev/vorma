import * as runtime from "#variant-runtime";
import * as client from "./vorma.app.ts";

export const ui = {
	variant: runtime.variant,
	defineView: (input: any): any => {
		return client.defineView(runtime.prepare_view_definition(input));
	},
	Link: client.Link,
	navigate: client.navigate,
	prefetch: client.prefetch,
	cancelPrefetch: client.cancelPrefetch,
	apiClient: client.apiClient,
	revalidate: client.revalidate,
	useViewData: runtime.use_view_data ?? client.useViewData,
	useClientLoaderData: runtime.use_client_loader_data ?? client.useClientLoaderData,
	useRouteState: runtime.use_route_state ?? client.useRouteState,
	useWorkState: runtime.use_work_state ?? client.useWorkState,
	h: runtime.h,
	class_prop: runtime.class_prop,
	input_event: runtime.input_event,
	use_text_state: runtime.use_text_state,
	read_box: runtime.read_box,
	read_text_state: runtime.read_text_state,
	dynamic: runtime.dynamic,
};
