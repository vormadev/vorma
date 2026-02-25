import type { RouteProps } from "../vorma.gen/index.ts";

export function Explode(_props: RouteProps<"/explode">) {
	return <div id="e2e-explode-unreachable">Explode route body</div>;
}

export function ErrorBoundary(props: { error: unknown }) {
	return (
		<div id="e2e-explode-boundary">
			{`explode-boundary:${String(props.error)}`}
		</div>
	);
}
