import { Effect } from "effect";
import { describe, expect, it } from "vitest";
import { make_boot_route_state } from "./effect_runtime/boot_route_state.ts";
import type { RouteRecord } from "./effect_runtime/route_preparer.ts";

const CLIENT_BUILD_ID = "build-1";
const BOOT_HREF = "http://localhost/boot";
const BOOT_PATTERN = "/boot";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

function route_record(): RouteRecord {
	return {
		params: { id: "1" },
		splat_values: [],
		matches: [
			{
				pattern: BOOT_PATTERN,
				input: { id: "1" },
				module_url: "/boot.js",
				module: {},
				loader_data: { server: true },
				client_loader_data: { client: true },
			},
		],
		error: null,
		client_build_id: CLIENT_BUILD_ID,
	};
}

describe("ccc Effect boot route state experiment", () => {
	it("captures provisional route state only for boot preparation", async () => {
		const boot_route_state = Effect.runSync(make_boot_route_state());
		await run_effect(
			boot_route_state.capture({
				route: route_record(),
				prepare_input: {
					raw_payload: {},
					url: new URL(BOOT_HREF),
					trigger: "navigation",
					href: BOOT_HREF,
					history_state: { source: "navigation" },
				},
			}),
		);
		expect(await run_effect(boot_route_state.snapshot)).toBeNull();

		await run_effect(
			boot_route_state.capture({
				route: route_record(),
				prepare_input: {
					raw_payload: {},
					url: new URL(BOOT_HREF),
					trigger: "boot",
					href: BOOT_HREF,
					history_state: { source: "boot" },
				},
			}),
		);
		const snapshot = await run_effect(boot_route_state.snapshot);

		expect(snapshot?.href).toBe(BOOT_HREF);
		expect(snapshot?.clientBuildID).toBe(CLIENT_BUILD_ID);
		expect(snapshot?.matches[0]?.pattern).toBe(BOOT_PATTERN);

		await run_effect(boot_route_state.clear);
		expect(await run_effect(boot_route_state.snapshot)).toBeNull();
	});
});
