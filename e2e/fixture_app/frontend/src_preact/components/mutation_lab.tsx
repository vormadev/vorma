import { useState } from "preact/hooks";
import type { RouteProps } from "../vorma.gen/index.ts";
import { api } from "../vorma.bindings.ts";

export function MutationLab(_props: RouteProps<"/mutation-lab">) {
	const [incrementCount, setIncrementCount] = useState<number | null>(null);

	async function runIncrement() {
		const response = await api.mutate({ pattern: "/increment-count" });
		if (!response.success) {
			setIncrementCount(null);
			return;
		}
		setIncrementCount(response.data.count);
	}

	async function runReset() {
		const response = await api.mutate({ pattern: "/reset-count" });
		if (!response.success) {
			setIncrementCount(null);
			return;
		}
		setIncrementCount(response.data.count);
	}

	return (
		<section id="e2e-mutation-route">
			<button
				id="e2e-increment-button"
				type="button"
				onClick={runIncrement}
			>
				Increment Counter
			</button>
			<button id="e2e-reset-button" type="button" onClick={runReset}>
				Reset Counter
			</button>
			<p id="e2e-increment-result">{incrementCount ?? "unset"}</p>
		</section>
	);
}
