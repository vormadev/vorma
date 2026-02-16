import { describe, expect, it } from "vitest";
import { buildBeginNavigationExecutionCommands } from "../../core/navigation/begin_navigation_commands.ts";
import type { BeginNavigationExecutionPlan } from "../../core/navigation/begin_navigation_state_machine.ts";
import type {
	NavigationEntry,
	NavigationOutcome,
} from "../../core/navigation/types.ts";

let nextOperationID = 1;

function createEntry(props: {
	type: NavigationEntry["type"];
	intent: NavigationEntry["intent"];
	targetUrl?: string;
}): NavigationEntry {
	const targetUrl = props.targetUrl ?? "http://localhost:3000/target";
	const controlPromise = Promise.resolve({
		type: "aborted",
	} as const satisfies NavigationOutcome);

	return {
		operationID: nextOperationID++,
		control: {
			abortController: new AbortController(),
			promise: controlPromise,
		},
		type: props.type,
		intent: props.intent,
		phase: "fetching",
		startTime: Date.now(),
		targetUrl,
		originUrl: "http://localhost:3000/origin",
		scrollToTop: true,
		replace: false,
		state: undefined,
	};
}

describe("begin navigation command builder", () => {
	it("maps abort instructions followed by reuse instruction", () => {
		const activeEntry = createEntry({
			type: "userNavigation",
			intent: "navigate",
			targetUrl: "http://localhost:3000/active",
		});
		const prefetchEntry = createEntry({
			type: "prefetch",
			intent: "none",
			targetUrl: "http://localhost:3000/prefetch",
		});
		const executionPlan: BeginNavigationExecutionPlan = {
			targetUrl: "http://localhost:3000/next",
			abortInstructions: [
				{
					slot: "active",
					entry: activeEntry,
				},
				{
					slot: "prefetch",
					key: prefetchEntry.targetUrl,
					entry: prefetchEntry,
				},
			],
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: prefetchEntry.targetUrl,
				entry: prefetchEntry,
				promotion: {
					targetUrl: "http://localhost:3000/next",
					type: "userNavigation",
					intent: "navigate",
					scrollToTop: false,
					replace: true,
					state: { ok: true },
				},
			},
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};

		const commands = buildBeginNavigationExecutionCommands({
			executionPlan,
		});

		expect(commands).toEqual([
			{
				type: "abort_instruction",
				abortInstruction: {
					slot: "active",
					entry: activeEntry,
				},
			},
			{
				type: "abort_instruction",
				abortInstruction: {
					slot: "prefetch",
					key: prefetchEntry.targetUrl,
					entry: prefetchEntry,
				},
			},
			{
				type: "reuse_instruction",
				reuseInstruction: executionPlan.reuseInstruction,
			},
		]);
	});

	it("maps immediate-abort plan to a terminal immediate-abort command", () => {
		const revalidationEntry = createEntry({
			type: "revalidation",
			intent: "revalidate",
			targetUrl: "http://localhost:3000/revalidation",
		});
		const executionPlan: BeginNavigationExecutionPlan = {
			targetUrl: "http://localhost:3000/prefetch",
			abortInstructions: [
				{
					slot: "revalidation",
					entry: revalidationEntry,
				},
			],
			reuseInstruction: null,
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: true,
		};

		const commands = buildBeginNavigationExecutionCommands({
			executionPlan,
		});

		expect(commands).toEqual([
			{
				type: "abort_instruction",
				abortInstruction: {
					slot: "revalidation",
					entry: revalidationEntry,
				},
			},
			{
				type: "return_immediately_aborted_control",
			},
		]);
	});

	it("maps active create instruction", () => {
		const executionPlan: BeginNavigationExecutionPlan = {
			targetUrl: "http://localhost:3000/active",
			abortInstructions: [],
			reuseInstruction: null,
			createInstruction: {
				slot: "active",
				intent: "navigate",
			},
			shouldReturnImmediatelyAbortedControl: false,
		};

		const commands = buildBeginNavigationExecutionCommands({
			executionPlan,
		});

		expect(commands).toEqual([
			{
				type: "create_active_control",
				intent: "navigate",
			},
		]);
	});

	it("maps prefetch create instruction", () => {
		const executionPlan: BeginNavigationExecutionPlan = {
			targetUrl: "http://localhost:3000/prefetch",
			abortInstructions: [],
			reuseInstruction: null,
			createInstruction: {
				slot: "prefetch",
				targetUrl: "http://localhost:3000/prefetch",
			},
			shouldReturnImmediatelyAbortedControl: false,
		};

		const commands = buildBeginNavigationExecutionCommands({
			executionPlan,
		});

		expect(commands).toEqual([
			{
				type: "create_prefetch_control",
				targetUrl: "http://localhost:3000/prefetch",
			},
		]);
	});

	it("maps revalidation create instruction", () => {
		const executionPlan: BeginNavigationExecutionPlan = {
			targetUrl: "http://localhost:3000/revalidate",
			abortInstructions: [],
			reuseInstruction: null,
			createInstruction: {
				slot: "revalidation",
				revalidationHref:
					"http://localhost:3000/revalidate?vorma_json=1",
			},
			shouldReturnImmediatelyAbortedControl: false,
		};

		const commands = buildBeginNavigationExecutionCommands({
			executionPlan,
		});

		expect(commands).toEqual([
			{
				type: "create_revalidation_control",
				revalidationHref:
					"http://localhost:3000/revalidate?vorma_json=1",
			},
		]);
	});

	it("throws when plan has no terminal reuse, abort-return, or create instruction", () => {
		const executionPlan: BeginNavigationExecutionPlan = {
			targetUrl: "http://localhost:3000/invalid",
			abortInstructions: [],
			reuseInstruction: null,
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};

		expect(() => {
			buildBeginNavigationExecutionCommands({
				executionPlan,
			});
		}).toThrow(
			"Begin navigation execution plan was missing reuse, immediate-abort, and create instructions.",
		);
	});
});
