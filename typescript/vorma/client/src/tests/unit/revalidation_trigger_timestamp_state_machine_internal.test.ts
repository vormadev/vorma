import { describe, expect, it } from "vitest";
import {
	createRevalidationTriggerTimestampRuntime,
	reduceRevalidationTriggerTimestampState,
} from "../../runtime.ts";

describe("revalidation trigger timestamp state machine", () => {
	it("reduces committed intent events to the committed timestamp", () => {
		const nextState = reduceRevalidationTriggerTimestampState({
			state: {
				lastTriggeredNavOrRevalidateTimestampMS: 1000,
			},
			event: {
				type: "navigation_or_revalidation_intent_committed",
				committedTimestampMS: 1500,
			},
		});

		expect(nextState).toEqual({
			lastTriggeredNavOrRevalidateTimestampMS: 1500,
		});
	});

	it("initializes runtime timestamp from getNowTimestampMS when initial state is absent", () => {
		const runtime = createRevalidationTriggerTimestampRuntime({
			getNowTimestampMS: () => 1234,
		});

		expect(runtime.getLastTriggeredNavOrRevalidateTimestampMS()).toBe(1234);
	});

	it("records committed intent updates via runtime event dispatch", () => {
		let nowTimestampMS = 500;
		const runtime = createRevalidationTriggerTimestampRuntime({
			getNowTimestampMS: () => nowTimestampMS,
		});

		expect(runtime.getLastTriggeredNavOrRevalidateTimestampMS()).toBe(500);

		nowTimestampMS = 900;
		runtime.recordNavigationOrRevalidationIntentCommitted();

		expect(runtime.getLastTriggeredNavOrRevalidateTimestampMS()).toBe(900);
	});

	it("honors provided initial state before subsequent committed intent events", () => {
		let nowTimestampMS = 3000;
		const runtime = createRevalidationTriggerTimestampRuntime({
			getNowTimestampMS: () => nowTimestampMS,
			initialState: {
				lastTriggeredNavOrRevalidateTimestampMS: 50,
			},
		});

		expect(runtime.getLastTriggeredNavOrRevalidateTimestampMS()).toBe(50);

		nowTimestampMS = 75;
		runtime.recordNavigationOrRevalidationIntentCommitted();

		expect(runtime.getLastTriggeredNavOrRevalidateTimestampMS()).toBe(75);
	});
});
