import { describe, expect, it, vi } from "vitest";
import type {
	RedirectData,
	SubmissionEntry,
	SubmitExecutionContext,
} from "../../../src/runtime.ts";
import {
	beginSubmissionLifecycle,
	buildBeginSubmissionLifecycleCommandPlan,
	buildFinishSubmissionLifecycleCommandPlan,
	buildSubmitPostClassificationRuntimeCommandPlan,
	createInitialSubmitRuntimeReducerState,
	decideSubmitPostClassificationExecutionPlan,
	finishSubmissionLifecycle,
	reduceSubmitRuntimeEvent,
} from "../../runtime.ts";

let nextSubmissionOperationID = 1;

function createSubmissionEntry(): SubmissionEntry {
	return {
		operationID: nextSubmissionOperationID++,
		control: {
			abortController: new AbortController(),
			promise: Promise.resolve(undefined),
		},
		startTime: Date.now(),
	};
}

function createSubmitExecutionContext(): {
	context: SubmitExecutionContext;
	transitionEvents: Array<{
		submissionEntry: SubmissionEntry;
		fromState: string;
		toState: string;
		reason: string;
		causedByOperationID?: number | null;
	}>;
	scheduleStatusUpdate: ReturnType<typeof vi.fn>;
} {
	const transitionEvents: Array<{
		submissionEntry: SubmissionEntry;
		fromState: string;
		toState: string;
		reason: string;
		causedByOperationID?: number | null;
	}> = [];
	const scheduleStatusUpdate = vi.fn();
	const context: SubmitExecutionContext = {
		submissions: new Map(),
		scheduleStatusUpdate,
		allocateSubmissionOperationID: () => 1,
		onSubmissionStateTransition: (props) => {
			transitionEvents.push(props);
		},
		navigate: async () => ({ didNavigate: false }),
	};

	return {
		context,
		transitionEvents,
		scheduleStatusUpdate,
	};
}

describe("submission lifecycle execution", () => {
	it("classifies submit response outcomes into redirect/error/success execution plans", () => {
		const redirectData: RedirectData = {
			status: "should",
			shouldRedirectStrategy: "soft",
			latestBuildID: "2",
			href: "/next",
			hrefDetails: {
				url: new URL("http://localhost:3000/next"),
				isHTTP: true,
				isInternal: true,
				isExternal: false,
				absoluteURL: "http://localhost:3000/next",
				relativeURL: "/next",
			},
		};

		expect(
			decideSubmitPostClassificationExecutionPlan({
				response: new Response("{}", {
					status: 500,
				}),
				redirectData,
			}),
		).toEqual({
			type: "redirect",
			redirectData,
		});

		expect(
			decideSubmitPostClassificationExecutionPlan({
				response: new Response("{}", {
					status: 422,
				}),
				redirectData: null,
			}),
		).toEqual({
			type: "error",
			error: "422",
		});

		expect(
			decideSubmitPostClassificationExecutionPlan({
				response: new Response("{}", {
					status: 200,
				}),
				redirectData: null,
				requestInit: {
					method: "POST",
				},
			}),
		).toEqual({
			type: "success",
			shouldAutoRevalidate: true,
		});

		expect(
			decideSubmitPostClassificationExecutionPlan({
				response: new Response("{}", {
					status: 200,
				}),
				redirectData: null,
				requestInit: {
					method: "GET",
				},
			}),
		).toEqual({
			type: "success",
			shouldAutoRevalidate: false,
		});
	});

	it("builds post-classification command plans from execution plans", () => {
		const redirectData: RedirectData = {
			status: "should",
			shouldRedirectStrategy: "soft",
			latestBuildID: "2",
			href: "/next",
			hrefDetails: {
				url: new URL("http://localhost:3000/next"),
				isHTTP: true,
				isInternal: true,
				isExternal: false,
				absoluteURL: "http://localhost:3000/next",
				relativeURL: "/next",
			},
		};

		expect(
			buildSubmitPostClassificationRuntimeCommandPlan({
				executionPlan: {
					type: "redirect",
					redirectData,
				},
				currentHref: "http://localhost:3000/current",
			}),
		).toEqual({
			terminalResult: {
				type: "redirect",
			},
			commands: [
				{
					type: "effectuate_redirect",
					redirectData,
				},
			],
		});

		expect(
			buildSubmitPostClassificationRuntimeCommandPlan({
				executionPlan: {
					type: "error",
					error: "422",
				},
				currentHref: "http://localhost:3000/current",
			}),
		).toEqual({
			terminalResult: {
				type: "error",
				error: "422",
			},
			commands: [],
		});

		expect(
			buildSubmitPostClassificationRuntimeCommandPlan({
				executionPlan: {
					type: "success",
					shouldAutoRevalidate: true,
				},
				currentHref: "http://localhost:3000/current",
			}),
		).toEqual({
			terminalResult: {
				type: "success",
			},
			commands: [
				{
					type: "auto_revalidate_navigation",
					href: "http://localhost:3000/current",
				},
			],
		});

		expect(
			buildSubmitPostClassificationRuntimeCommandPlan({
				executionPlan: {
					type: "success",
					shouldAutoRevalidate: false,
				},
				currentHref: "http://localhost:3000/current",
			}),
		).toEqual({
			terminalResult: {
				type: "success",
			},
			commands: [],
		});
	});

	it("reduces request/response/payload checkpoints through explicit submit runtime phases", () => {
		const initialState = createInitialSubmitRuntimeReducerState();
		const response = new Response("{}", {
			status: 200,
		});

		const requestResolvedTransition = reduceSubmitRuntimeEvent({
			state: initialState,
			event: {
				type: "request_resolved",
				response,
				staleSubmitResult: null,
			},
		});
		expect(requestResolvedTransition.state).toEqual({
			phase: "awaiting_response_classification",
			ownership: "current",
		});
		expect(requestResolvedTransition.commands).toEqual([
			{
				type: "sync_build_id_from_response",
				response,
			},
		]);
		expect(requestResolvedTransition.terminalResult).toEqual({
			type: "continue",
		});

		const responseClassifiedTransition = reduceSubmitRuntimeEvent({
			state: requestResolvedTransition.state,
			event: {
				type: "response_classified",
				response,
				redirectData: null,
				requestInit: {
					method: "POST",
				},
				currentHref: "http://localhost:3000/current",
				staleSubmitResult: null,
			},
		});
		expect(responseClassifiedTransition.state).toEqual({
			phase: "awaiting_success_payload_ownership_checkpoint",
			ownership: "current",
		});
		expect(responseClassifiedTransition.shouldReadSuccessPayload).toBe(
			true,
		);
		expect(
			responseClassifiedTransition.postClassificationCommandPlan
				?.terminalResult,
		).toEqual({
			type: "success",
		});

		const successPayloadParsedTransition = reduceSubmitRuntimeEvent({
			state: responseClassifiedTransition.state,
			event: {
				type: "success_payload_parsed",
				staleSubmitResult: null,
			},
		});
		expect(successPayloadParsedTransition.state).toEqual({
			phase: "awaiting_post_classification_command_execution",
			ownership: "current",
		});
		expect(successPayloadParsedTransition.terminalResult).toEqual({
			type: "continue",
		});
	});

	it("short-circuits submit runtime reducer transitions to stale ownership", () => {
		const staleResult = {
			success: false as const,
			error: "Aborted",
		};

		const requestStaleTransition = reduceSubmitRuntimeEvent({
			state: createInitialSubmitRuntimeReducerState(),
			event: {
				type: "request_resolved",
				response: new Response("{}", { status: 200 }),
				staleSubmitResult: staleResult,
			},
		});
		expect(requestStaleTransition.state).toEqual({
			phase: "completed",
			ownership: "stale",
		});
		expect(requestStaleTransition.terminalResult).toEqual({
			type: "stale",
			result: staleResult,
		});

		const payloadStaleTransition = reduceSubmitRuntimeEvent({
			state: {
				phase: "awaiting_success_payload_ownership_checkpoint",
				ownership: "current",
			},
			event: {
				type: "success_payload_parsed",
				staleSubmitResult: staleResult,
			},
		});
		expect(payloadStaleTransition.state).toEqual({
			phase: "completed",
			ownership: "stale",
		});
		expect(payloadStaleTransition.terminalResult).toEqual({
			type: "stale",
			result: staleResult,
		});
	});

	it("plans redirect and error terminals from response-classification reducer events", () => {
		const redirectData: RedirectData = {
			status: "should",
			shouldRedirectStrategy: "soft",
			latestBuildID: "2",
			href: "/next",
			hrefDetails: {
				url: new URL("http://localhost:3000/next"),
				isHTTP: true,
				isInternal: true,
				isExternal: false,
				absoluteURL: "http://localhost:3000/next",
				relativeURL: "/next",
			},
		};

		const redirectTransition = reduceSubmitRuntimeEvent({
			state: {
				phase: "awaiting_response_classification",
				ownership: "current",
			},
			event: {
				type: "response_classified",
				response: new Response("{}", { status: 500 }),
				redirectData,
				currentHref: "http://localhost:3000/current",
				staleSubmitResult: null,
			},
		});
		expect(redirectTransition.state).toEqual({
			phase: "awaiting_post_classification_command_execution",
			ownership: "current",
		});
		expect(
			redirectTransition.postClassificationCommandPlan?.terminalResult,
		).toEqual({
			type: "redirect",
		});
		expect(redirectTransition.shouldReadSuccessPayload).toBe(false);

		const errorTransition = reduceSubmitRuntimeEvent({
			state: {
				phase: "awaiting_response_classification",
				ownership: "current",
			},
			event: {
				type: "response_classified",
				response: new Response("{}", { status: 422 }),
				redirectData: null,
				currentHref: "http://localhost:3000/current",
				staleSubmitResult: null,
			},
		});
		expect(errorTransition.state).toEqual({
			phase: "completed",
			ownership: "current",
		});
		expect(errorTransition.terminalResult).toEqual({
			type: "error",
			error: "422",
		});
	});

	it("fails loud when submit runtime reducer receives events in the wrong phase", () => {
		expect(() =>
			reduceSubmitRuntimeEvent({
				state: createInitialSubmitRuntimeReducerState(),
				event: {
					type: "success_payload_parsed",
					staleSubmitResult: null,
				},
			}),
		).toThrow(
			'Submit runtime reducer event "success_payload_parsed" cannot run from phase "awaiting_request_resolution". Expected "awaiting_success_payload_ownership_checkpoint".',
		);
	});

	it("plans begin lifecycle commands with dedupe abort before started transition", () => {
		const existingSubmissionEntry = createSubmissionEntry();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		const commandPlan = buildBeginSubmissionLifecycleCommandPlan({
			submissionKey,
			submissionEntry,
			existingSubmissionEntry,
		});

		expect(commandPlan.commands).toEqual([
			{
				type: "abort_submission",
				submissionEntry: existingSubmissionEntry,
				abortReason: "deduped",
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry: existingSubmissionEntry,
				fromState: "submitting",
				toState: "aborted",
				reason: "submission_deduped_by_newer_submission",
				causedByOperationID: submissionEntry.operationID,
			},
			{
				type: "set_submission",
				submissionKey,
				submissionEntry,
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry,
				fromState: "none",
				toState: "submitting",
				reason: "submission_started",
				causedByOperationID: null,
			},
			{
				type: "schedule_status_update",
			},
		]);
	});

	it("plans finish lifecycle commands with conditional delete/removal transition", () => {
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		const removePlan = buildFinishSubmissionLifecycleCommandPlan({
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: true,
		});
		expect(removePlan.commands).toEqual([
			{
				type: "delete_submission",
				submissionKey,
			},
			{
				type: "emit_submission_state_transition",
				submissionEntry,
				fromState: "submitting",
				toState: "removed",
				reason: "submission_finished",
				causedByOperationID: null,
			},
			{
				type: "schedule_status_update",
			},
		]);

		const stalePlan = buildFinishSubmissionLifecycleCommandPlan({
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: false,
		});
		expect(stalePlan.commands).toEqual([
			{
				type: "schedule_status_update",
			},
		]);
	});

	it("begins with dedupe abort transition when a keyed submission already exists", () => {
		const { context, transitionEvents, scheduleStatusUpdate } =
			createSubmitExecutionContext();
		const existingSubmissionEntry = createSubmissionEntry();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		context.submissions.set(submissionKey, existingSubmissionEntry);
		const existingAbortSpy = vi.spyOn(
			existingSubmissionEntry.control.abortController!,
			"abort",
		);

		beginSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			existingSubmissionEntry,
		});

		expect(existingAbortSpy).toHaveBeenCalledWith("deduped");
		expect(context.submissions.get(submissionKey)).toBe(submissionEntry);
		expect(transitionEvents).toEqual([
			{
				submissionEntry: existingSubmissionEntry,
				fromState: "submitting",
				toState: "aborted",
				reason: "submission_deduped_by_newer_submission",
				causedByOperationID: submissionEntry.operationID,
			},
			{
				submissionEntry,
				fromState: "none",
				toState: "submitting",
				reason: "submission_started",
				causedByOperationID: null,
			},
		]);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("begins unique submissions without a dedupe abort transition", () => {
		const { context, transitionEvents, scheduleStatusUpdate } =
			createSubmitExecutionContext();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = Symbol("submission");

		beginSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			existingSubmissionEntry: undefined,
		});

		expect(context.submissions.get(submissionKey)).toBe(submissionEntry);
		expect(transitionEvents).toEqual([
			{
				submissionEntry,
				fromState: "none",
				toState: "submitting",
				reason: "submission_started",
				causedByOperationID: null,
			},
		]);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("finishes by removing current submissions and emitting removal transition", () => {
		const { context, transitionEvents, scheduleStatusUpdate } =
			createSubmitExecutionContext();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		context.submissions.set(submissionKey, submissionEntry);

		finishSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: true,
		});

		expect(context.submissions.has(submissionKey)).toBe(false);
		expect(transitionEvents).toEqual([
			{
				submissionEntry,
				fromState: "submitting",
				toState: "removed",
				reason: "submission_finished",
				causedByOperationID: null,
			},
		]);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});

	it("finishes stale submissions without removing or emitting removal transition", () => {
		const { context, transitionEvents, scheduleStatusUpdate } =
			createSubmitExecutionContext();
		const submissionEntry = createSubmissionEntry();
		const submissionKey = "submission:users:save";
		context.submissions.set(submissionKey, submissionEntry);

		finishSubmissionLifecycle({
			context,
			submissionKey,
			submissionEntry,
			shouldRemoveSubmissionEntry: false,
		});

		expect(context.submissions.get(submissionKey)).toBe(submissionEntry);
		expect(transitionEvents).toEqual([]);
		expect(scheduleStatusUpdate).toHaveBeenCalledTimes(1);
	});
});
