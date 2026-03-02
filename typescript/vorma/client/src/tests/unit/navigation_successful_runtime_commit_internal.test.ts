import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	__vormaClientGlobal,
	addBuildIDListener,
	getBuildIDFromResponse,
	syncRuntimeBuildIDIfChanged,
	VORMA_SYMBOL,
} from "../../runtime.ts";

type TestGlobalState = {
	runtimeRouteSnapshot: {
		buildID: string;
	};
};

function installTestGlobalState(
	overrides: Partial<TestGlobalState> = {},
): void {
	(globalThis as any)[VORMA_SYMBOL] = {
		runtimeRouteSnapshot: {
			buildID: "1",
		},
		...overrides,
	} satisfies TestGlobalState;
}

function getInstalledTestGlobalState(): TestGlobalState {
	return (globalThis as any)[VORMA_SYMBOL] as TestGlobalState;
}

function createResponseWithBuildID(props: { buildID?: string }): Response {
	const headers = new Headers({
		"Content-Type": "application/json",
	});
	if (props.buildID !== undefined) {
		headers.set("X-Wave-Framework-Build-Id", props.buildID);
	}
	return new Response(JSON.stringify({ ok: true }), {
		status: 200,
		headers,
	});
}

beforeEach(() => {
	installTestGlobalState();
});

afterEach(() => {
	vi.restoreAllMocks();
	delete (globalThis as any)[VORMA_SYMBOL];
});

describe("successful navigation build-id sync", () => {
	it("syncs build ID from response only when it changes", () => {
		const events: Array<{ oldID: string; newID: string }> = [];
		const removeBuildIDListener = addBuildIDListener((event) => {
			events.push(event.detail);
		});

		syncRuntimeBuildIDIfChanged({
			nextBuildID: getBuildIDFromResponse(
				createResponseWithBuildID({
					buildID: "1",
				}),
			),
		});

		expect(__vormaClientGlobal.get("runtimeRouteSnapshot").buildID).toBe(
			"1",
		);
		expect(events).toEqual([]);

		syncRuntimeBuildIDIfChanged({
			nextBuildID: getBuildIDFromResponse(
				createResponseWithBuildID({
					buildID: "2",
				}),
			),
		});
		removeBuildIDListener();

		expect(__vormaClientGlobal.get("runtimeRouteSnapshot").buildID).toBe(
			"2",
		);
		expect(events).toEqual([{ newID: "2", oldID: "1" }]);
		expect(
			getInstalledTestGlobalState().runtimeRouteSnapshot?.buildID,
		).toBe("2");
	});
});
