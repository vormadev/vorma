import { describe, expect, it, vi } from "vitest";
import type { RuntimeTransitionEvent } from "../../../src/runtime.ts";

function createNavigationFailedTransitionEvent(props: {
	reason: string;
}): RuntimeTransitionEvent {
	return {
		type: "navigation_failed",
		targetUrl: "http://localhost:3000/debug-journal",
		entry: undefined,
		reason: props.reason,
	};
}

describe("navigation runtime state machine debug journal gating", () => {
	it("records and clears entries when build is in dev mode", async () => {
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = true;
		try {
			vi.resetModules();

			const runtimeStateMachineModule = await import("../../runtime.ts");
			const runtimeStateMachine =
				runtimeStateMachineModule.createNavigationRuntimeStateMachine();

			runtimeStateMachine.dispatchTransitionEvent(
				createNavigationFailedTransitionEvent({
					reason: "debug_enabled_probe",
				}),
			);

			const entries = runtimeStateMachine.getDebugJournal();
			expect(entries).toHaveLength(1);
			expect(entries[0]?.reason).toBe("debug_enabled_probe");

			runtimeStateMachine.clearDebugJournal();
			expect(runtimeStateMachine.getDebugJournal()).toEqual([]);
		} finally {
			(import.meta.env as any).DEV = originalDev;
		}
	});

	it("no-ops when build is in production mode", async () => {
		const originalDev = import.meta.env.DEV;
		(import.meta.env as any).DEV = false;
		try {
			vi.resetModules();

			const runtimeStateMachineModule = await import("../../runtime.ts");
			const runtimeStateMachine =
				runtimeStateMachineModule.createNavigationRuntimeStateMachine();

			runtimeStateMachine.dispatchTransitionEvent(
				createNavigationFailedTransitionEvent({
					reason: "debug_disabled_probe",
				}),
			);

			expect(runtimeStateMachine.getDebugJournal()).toEqual([]);
			expect(() => runtimeStateMachine.clearDebugJournal()).not.toThrow();
			expect(runtimeStateMachine.getDebugJournal()).toEqual([]);
		} finally {
			(import.meta.env as any).DEV = originalDev;
		}
	});
});
