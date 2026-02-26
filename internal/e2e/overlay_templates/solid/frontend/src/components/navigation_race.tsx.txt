import { createSignal } from "solid-js";
import type { RouteProps } from "../vorma.gen/index.ts";
import { navigate } from "../vorma.bindings.ts";

export function NavigationRace(_props: RouteProps<"/navigation-race">) {
	const [localStatus, setLocalStatus] = createSignal("idle");

	async function runSlowThenFastNavigationRace() {
		setLocalStatus("running");

		const slowNavigation = navigate({
			pattern: "/slow/:bucket",
			params: { bucket: "alpha" },
			search: "?delay=1200&token=slow",
		});
		await sleepMS(25);
		const fastNavigation = navigate({
			pattern: "/slow/:bucket",
			params: { bucket: "beta" },
			search: "?delay=35&token=fast",
		});

		await Promise.allSettled([slowNavigation, fastNavigation]);
		setLocalStatus("completed");
	}

	async function runFastThenSlowNavigationRace() {
		setLocalStatus("running");

		const fastNavigation = navigate({
			pattern: "/slow/:bucket",
			params: { bucket: "beta" },
			search: "?delay=35&token=fast-first",
		});
		await sleepMS(25);
		const slowNavigation = navigate({
			pattern: "/slow/:bucket",
			params: { bucket: "alpha" },
			search: "?delay=900&token=slow-last",
		});

		await Promise.allSettled([fastNavigation, slowNavigation]);
		setLocalStatus("completed");
	}

	return (
		<section id="e2e-navigation-race-route">
			<button
				id="e2e-run-nav-race-slow-then-fast"
				type="button"
				onClick={runSlowThenFastNavigationRace}
			>
				Run Slow Then Fast Navigation Race
			</button>
			<button
				id="e2e-run-nav-race-fast-then-slow"
				type="button"
				onClick={runFastThenSlowNavigationRace}
			>
				Run Fast Then Slow Navigation Race
			</button>
			<p id="e2e-nav-race-local-status">{localStatus()}</p>
		</section>
	);
}

async function sleepMS(milliseconds: number): Promise<void> {
	await new Promise((resolve) => {
		setTimeout(resolve, milliseconds);
	});
}
