import type { RouteErrorComponent } from "./vorma_ctx/vorma_ctx.ts";

export const defaultErrorBoundary: RouteErrorComponent = (props: {
	error: string;
}) => {
	return "Route Error: " + props.error;
};
