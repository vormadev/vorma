import { defaultErrorBoundary } from "./error_boundary.ts";
import {
	__vormaClientGlobal,
	type RouteErrorComponent,
} from "./vorma_ctx/vorma_ctx.ts";

export type InitClientOptions = {
	defaultErrorBoundary?: RouteErrorComponent;
	useViewTransitions?: boolean;
};

export function applyInitClientOptions(options: InitClientOptions): void {
	if (options.defaultErrorBoundary) {
		__vormaClientGlobal.set(
			"defaultErrorBoundary",
			options.defaultErrorBoundary,
		);
	} else {
		__vormaClientGlobal.set("defaultErrorBoundary", defaultErrorBoundary);
	}

	if (options.useViewTransitions) {
		__vormaClientGlobal.set("useViewTransitions", true);
	}
}
