import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { VORMA_SYMBOL } from "../../app/context.ts";
import { syncBuildIDFromResponse } from "../../core/navigation/runtime_navigation_successful_runtime.ts";
import * as eventsModule from "../../platform/events.ts";

type TestGlobalState = {
	buildID: string;
	runtimeRouteSnapshot?: {
		buildID: string;
	};
};

function installTestGlobalState(
	overrides: Partial<TestGlobalState> = {},
): void {
	(globalThis as any)[VORMA_SYMBOL] = {
		buildID: "1",
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
		headers.set("X-Vorma-Build-Id", props.buildID);
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
		const dispatchBuildIDEventSpy = vi
			.spyOn(eventsModule, "dispatchBuildIDEvent")
			.mockImplementation(() => {});

		syncBuildIDFromResponse(
			createResponseWithBuildID({
				buildID: "1",
			}),
		);

		expect(getInstalledTestGlobalState().buildID).toBe("1");
		expect(dispatchBuildIDEventSpy).not.toHaveBeenCalled();

		syncBuildIDFromResponse(
			createResponseWithBuildID({
				buildID: "2",
			}),
		);

		expect(getInstalledTestGlobalState().buildID).toBe("2");
		expect(dispatchBuildIDEventSpy).toHaveBeenCalledTimes(1);
		expect(dispatchBuildIDEventSpy).toHaveBeenCalledWith({
			newID: "2",
			oldID: "1",
		});
		expect(
			getInstalledTestGlobalState().runtimeRouteSnapshot?.buildID,
		).toBe("2");
	});
});
