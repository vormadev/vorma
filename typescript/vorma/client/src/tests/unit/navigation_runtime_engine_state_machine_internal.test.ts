import { describe, expect, it, vi } from "vitest";
import {
	createInitialNavigationRuntimeEngineState,
	executeNavigationRuntimeEngineCommands,
	reduceNavigationRuntimeEngineEvent,
} from "../../runtime.ts";

describe("navigation runtime engine reducer", () => {
	it("ignores stale fetch-resolve ownership outcomes", () => {
		const startTransition = reduceNavigationRuntimeEngineEvent({
			state: createInitialNavigationRuntimeEngineState(),
			event: {
				type: "start",
				lane: "navigate",
				navigationProps: {
					href: "/users",
					navigationType: "userNavigation",
				},
				targetUrl: "http://localhost:3000/users",
				currentHref: "http://localhost:3000/",
				operationID: 101,
			},
		});

		const staleFetchResolveTransition = reduceNavigationRuntimeEngineEvent({
			state: startTransition.state,
			event: {
				type: "fetch-resolve",
				lane: "navigate",
				targetUrl: "http://localhost:3000/users",
				operationID: 102,
				outcomeType: "success",
			},
		});

		expect(staleFetchResolveTransition.commands).toEqual([
			{
				type: "emit-events",
				eventName: "fetch-resolve-stale",
				lane: "navigate",
				targetUrl: "http://localhost:3000/users",
				operationID: 102,
				detail: "ignored_stale_fetch_resolve",
			},
		]);
	});

	it("enforces wait-resolve phase ordering", () => {
		const startTransition = reduceNavigationRuntimeEngineEvent({
			state: createInitialNavigationRuntimeEngineState(),
			event: {
				type: "start",
				lane: "navigate",
				navigationProps: {
					href: "/phase-order",
					navigationType: "userNavigation",
				},
				targetUrl: "http://localhost:3000/phase-order",
				currentHref: "http://localhost:3000/",
				operationID: 10,
			},
		});

		const waitResolveTransition = reduceNavigationRuntimeEngineEvent({
			state: startTransition.state,
			event: {
				type: "wait-resolve",
				lane: "navigate",
				targetUrl: "http://localhost:3000/phase-order",
				operationID: 10,
			},
		});

		expect(waitResolveTransition.commands).toEqual([
			{
				type: "log-error",
				message:
					"Navigation runtime engine wait-resolve event received outside waiting phase.",
				lane: "navigate",
				targetUrl: "http://localhost:3000/phase-order",
				operationID: 10,
			},
		]);
	});

	it("treats start events without operation ownership as planning-only", () => {
		const planningStartTransition = reduceNavigationRuntimeEngineEvent({
			state: createInitialNavigationRuntimeEngineState(),
			event: {
				type: "start",
				lane: "navigate",
				navigationProps: {
					href: "/planning-only",
					navigationType: "userNavigation",
				},
				targetUrl: "http://localhost:3000/planning-only",
				currentHref: "http://localhost:3000/current",
			},
		});

		expect(planningStartTransition.commands).toEqual([
			{
				type: "clear-queued-revalidation-request",
			},
			{
				type: "fetch",
				lane: "navigate",
				targetUrl: "http://localhost:3000/planning-only",
				operationID: null,
			},
		]);
		expect(planningStartTransition.state.lanes.navigate).toEqual({
			targetUrl: null,
			operationID: null,
			phase: "idle",
			ownership: "none",
		});

		const removalTransition = reduceNavigationRuntimeEngineEvent({
			state: planningStartTransition.state,
			event: {
				type: "remove-navigation-requested",
				targetUrl: "http://localhost:3000/planning-only",
				reason: "remove_navigation",
			},
		});
		expect(removalTransition.commands).toEqual([]);
	});

	it("cleans stale revalidation lane on external-location-change", () => {
		const startTransition = reduceNavigationRuntimeEngineEvent({
			state: createInitialNavigationRuntimeEngineState(),
			event: {
				type: "start",
				lane: "revalidate",
				navigationProps: {
					href: "/initial",
					navigationType: "revalidation",
				},
				targetUrl: "http://localhost:3000/initial",
				currentHref: "http://localhost:3000/initial",
				operationID: 7,
			},
		});

		const locationChangedTransition = reduceNavigationRuntimeEngineEvent({
			state: startTransition.state,
			event: {
				type: "external-location-change",
				currentHref: "http://localhost:3000/different",
			},
		});

		expect(locationChangedTransition.commands).toEqual([
			{
				type: "cleanup-entry",
				lane: "revalidate",
				targetUrl: "http://localhost:3000/initial",
				operationID: 7,
				reason: "external_location_change",
			},
			{
				type: "emit-events",
				eventName: "external-location-change",
				lane: "revalidate",
				targetUrl: "http://localhost:3000/initial",
				operationID: 7,
			},
		]);
	});

	it("does nothing when external-location-change keeps same data target", () => {
		const startTransition = reduceNavigationRuntimeEngineEvent({
			state: createInitialNavigationRuntimeEngineState(),
			event: {
				type: "start",
				lane: "revalidate",
				navigationProps: {
					href: "/current#first",
					navigationType: "revalidation",
				},
				targetUrl: "http://localhost:3000/current#first",
				currentHref: "http://localhost:3000/current#first",
				operationID: 9,
			},
		});

		const locationChangedTransition = reduceNavigationRuntimeEngineEvent({
			state: startTransition.state,
			event: {
				type: "external-location-change",
				currentHref: "http://localhost:3000/current#second",
			},
		});

		expect(locationChangedTransition.commands).toEqual([]);
	});

	it("plans cleanup commands for remove-navigation over data-target aliases", () => {
		const navigateTransition = reduceNavigationRuntimeEngineEvent({
			state: createInitialNavigationRuntimeEngineState(),
			event: {
				type: "start",
				lane: "navigate",
				navigationProps: {
					href: "/alias#first",
					navigationType: "userNavigation",
				},
				targetUrl: "http://localhost:3000/alias#first",
				currentHref: "http://localhost:3000/",
				operationID: 1,
			},
		});
		const prefetchTransition = reduceNavigationRuntimeEngineEvent({
			state: navigateTransition.state,
			event: {
				type: "start",
				lane: "prefetch",
				navigationProps: {
					href: "/alias#first",
					navigationType: "prefetch",
				},
				targetUrl: "http://localhost:3000/alias#first",
				currentHref: "http://localhost:3000/",
				operationID: 2,
			},
		});
		const revalidateTransition = reduceNavigationRuntimeEngineEvent({
			state: prefetchTransition.state,
			event: {
				type: "start",
				lane: "revalidate",
				navigationProps: {
					href: "/alias#first",
					navigationType: "revalidation",
				},
				targetUrl: "http://localhost:3000/alias#first",
				currentHref: "http://localhost:3000/alias#first",
				operationID: 3,
			},
		});

		const removalTransition = reduceNavigationRuntimeEngineEvent({
			state: revalidateTransition.state,
			event: {
				type: "remove-navigation-requested",
				targetUrl: "http://localhost:3000/alias#second",
				reason: "remove_navigation",
				causedByOperationID: 99,
			},
		});

		expect(removalTransition.commands).toEqual([
			{
				type: "cleanup-entry",
				lane: "navigate",
				targetUrl: "http://localhost:3000/alias#first",
				operationID: 1,
				reason: "remove_navigation",
				causedByOperationID: 99,
			},
			{
				type: "cleanup-entry",
				lane: "revalidate",
				targetUrl: "http://localhost:3000/alias#first",
				operationID: 3,
				reason: "remove_navigation",
				causedByOperationID: 99,
			},
			{
				type: "cleanup-entry",
				lane: "prefetch",
				targetUrl: "http://localhost:3000/alias#first",
				operationID: 2,
				reason: "remove_navigation",
				causedByOperationID: 99,
			},
		]);
	});

	it("plans ordered clear-all commands", () => {
		const clearAllTransition = reduceNavigationRuntimeEngineEvent({
			state: createInitialNavigationRuntimeEngineState(),
			event: {
				type: "clear-all-requested",
				navigationEntries: [],
				submissionEntries: [],
				targetUrl: "http://localhost:3000/current",
			},
		});

		expect(clearAllTransition.commands).toEqual([
			{
				type: "reset-revalidation-lane",
			},
			{
				type: "clear-runtime-lanes",
			},
			{
				type: "dispatch-clear-all-lifecycle-event",
				navigationEntries: [],
				submissionEntries: [],
				targetUrl: "http://localhost:3000/current",
			},
			{
				type: "emit-events",
				eventName: "clear-all",
				targetUrl: "http://localhost:3000/current",
			},
		]);
	});
});

describe("navigation runtime engine command executor", () => {
	it("enforces cleanup ownership guard before side effects", () => {
		const cleanupNavigationEntry = vi.fn();
		const context = {
			clearQueuedRevalidationRequest: vi.fn(),
			commitSameDocumentHashNavigationWithoutFetch: vi.fn(),
			cleanupNavigationEntry,
			resetRevalidationLane: vi.fn(),
			clearRuntimeLanes: vi.fn(),
			dispatchClearAllLifecycleEvent: vi.fn(),
			isNavigationOperationCurrent: vi.fn(() => false),
		};

		executeNavigationRuntimeEngineCommands({
			commands: [
				{
					type: "cleanup-entry",
					lane: "navigate",
					targetUrl: "http://localhost:3000/owned",
					operationID: 55,
					reason: "test",
				},
			],
			context,
		});
		expect(cleanupNavigationEntry).not.toHaveBeenCalled();

		context.isNavigationOperationCurrent.mockReturnValueOnce(true);
		executeNavigationRuntimeEngineCommands({
			commands: [
				{
					type: "cleanup-entry",
					lane: "navigate",
					targetUrl: "http://localhost:3000/owned",
					operationID: 55,
					reason: "test",
				},
			],
			context,
		});
		expect(cleanupNavigationEntry).toHaveBeenCalledTimes(1);
	});

	it("runs only declared commands in-order", () => {
		const orderedEvents: string[] = [];
		executeNavigationRuntimeEngineCommands({
			commands: [
				{ type: "clear-queued-revalidation-request" },
				{
					type: "emit-events",
					eventName: "step-2",
					lane: "navigate",
					targetUrl: "http://localhost:3000/target",
					operationID: 1,
				},
				{
					type: "cleanup-entry",
					lane: "navigate",
					targetUrl: "http://localhost:3000/target",
					operationID: null,
					reason: "step-3",
				},
			],
			context: {
				clearQueuedRevalidationRequest: () => {
					orderedEvents.push("step-1");
				},
				commitSameDocumentHashNavigationWithoutFetch: () => {
					orderedEvents.push("unexpected");
				},
				cleanupNavigationEntry: () => {
					orderedEvents.push("step-3");
				},
				resetRevalidationLane: () => {
					orderedEvents.push("unexpected");
				},
				clearRuntimeLanes: () => {
					orderedEvents.push("unexpected");
				},
				dispatchClearAllLifecycleEvent: () => {
					orderedEvents.push("unexpected");
				},
				emitEvent: ({ eventName }) => {
					orderedEvents.push(eventName);
				},
			},
		});

		expect(orderedEvents).toEqual(["step-1", "step-2", "step-3"]);
	});

	it("runs clear-all executor commands in reducer order", () => {
		const orderedEvents: string[] = [];
		executeNavigationRuntimeEngineCommands({
			commands: [
				{
					type: "reset-revalidation-lane",
				},
				{
					type: "clear-runtime-lanes",
				},
				{
					type: "dispatch-clear-all-lifecycle-event",
					navigationEntries: [],
					submissionEntries: [],
					targetUrl: "http://localhost:3000/current",
				},
			],
			context: {
				clearQueuedRevalidationRequest: () => {
					orderedEvents.push("unexpected");
				},
				commitSameDocumentHashNavigationWithoutFetch: () => {
					orderedEvents.push("unexpected");
				},
				cleanupNavigationEntry: () => {
					orderedEvents.push("unexpected");
				},
				resetRevalidationLane: () => {
					orderedEvents.push("reset");
				},
				clearRuntimeLanes: () => {
					orderedEvents.push("clear");
				},
				dispatchClearAllLifecycleEvent: () => {
					orderedEvents.push("dispatch");
				},
			},
		});

		expect(orderedEvents).toEqual(["reset", "clear", "dispatch"]);
	});
});
