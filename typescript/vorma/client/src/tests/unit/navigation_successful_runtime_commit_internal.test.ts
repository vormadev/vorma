import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { VORMA_SYMBOL } from "../../app/context.ts";
import { commitSuccessfulNavigationGlobalState } from "../../core/navigation/runtime_navigation_successful_runtime.ts";
import * as eventsModule from "../../platform/events.ts";

type TestGlobalState = {
	buildID: string;
	clientLoadersData: Array<unknown>;
	outermostClientError: string | undefined;
	outermostClientErrorIdx: number | undefined;
};

function installTestGlobalState(
	overrides: Partial<TestGlobalState> = {},
): void {
	(globalThis as any)[VORMA_SYMBOL] = {
		buildID: "1",
		clientLoadersData: [],
		outermostClientError: undefined,
		outermostClientErrorIdx: undefined,
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

describe("successful navigation global commit surface", () => {
	it("syncs build ID from response only when it changes", () => {
		const dispatchBuildIDEventSpy = vi
			.spyOn(eventsModule, "dispatchBuildIDEvent")
			.mockImplementation(() => {});

		commitSuccessfulNavigationGlobalState({
			commit: {
				type: "sync_build_id_from_response",
				response: createResponseWithBuildID({
					buildID: "1",
				}),
			},
		});

		expect(getInstalledTestGlobalState().buildID).toBe("1");
		expect(dispatchBuildIDEventSpy).not.toHaveBeenCalled();

		commitSuccessfulNavigationGlobalState({
			commit: {
				type: "sync_build_id_from_response",
				response: createResponseWithBuildID({
					buildID: "2",
				}),
			},
		});

		expect(getInstalledTestGlobalState().buildID).toBe("2");
		expect(dispatchBuildIDEventSpy).toHaveBeenCalledTimes(1);
		expect(dispatchBuildIDEventSpy).toHaveBeenCalledWith({
			newID: "2",
			oldID: "1",
		});
	});

	it("commits client loader state through one write path", () => {
		commitSuccessfulNavigationGlobalState({
			commit: {
				type: "set_client_loaders_state",
				clientLoadersResult: {
					data: ["client-data"],
					errorMessage: "loader boom",
				},
			},
		});

		expect(getInstalledTestGlobalState().clientLoadersData).toEqual([
			"client-data",
		]);
		expect(getInstalledTestGlobalState().outermostClientError).toBe(
			"loader boom",
		);
		expect(getInstalledTestGlobalState().outermostClientErrorIdx).toBe(0);
	});
});
