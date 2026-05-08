import { Effect, TestContext } from "effect";
import { describe, expect, it } from "vitest";
import type {
	ClientCommit,
	WorkState,
} from "./effect_runtime/client_contract.ts";
import {
	WORK_NAVIGATION_SOURCE_NAVIGATE,
	WORK_NAVIGATION_SOURCE_REDIRECT,
	WORK_REVALIDATION_STATUS_DEBOUNCING,
	WORK_REVALIDATION_STATUS_RETRYING,
	type WorkIndicatorActivity,
	make_work_state_actor,
} from "./effect_runtime/work_state_actor.ts";

const FIRST_HREF = "http://localhost/first";
const SECOND_HREF = "http://localhost/second";
const PREFETCH_HREF = "http://localhost/prefetch";
const API_KEY_SAVE = "save";
const API_KEY_SEARCH = "search";
const API_METHOD_GET = "GET";
const API_METHOD_POST = "POST";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(
		program.pipe(Effect.provide(TestContext.TestContext)),
	);
}

function emitted_work(commits: ClientCommit[]): WorkState[] {
	return commits
		.map((commit) => {
			return commit.work;
		})
		.filter((work): work is WorkState => {
			return work !== undefined;
		});
}

describe("ccc Effect work state actor experiment", () => {
	it("emits work commits only when the projected state changes", async () => {
		const commits: ClientCommit[] = [];

		const result = await run_effect(
			Effect.gen(function* () {
				const actor = yield* make_work_state_actor({
					commit: (commit) => {
						commits.push(commit);
					},
				});
				yield* actor.set_navigation({
					href: FIRST_HREF,
					replace: false,
					source: WORK_NAVIGATION_SOURCE_NAVIGATE,
				});
				yield* actor.set_navigation({
					href: FIRST_HREF,
					replace: false,
					source: WORK_NAVIGATION_SOURCE_NAVIGATE,
				});
				yield* actor.set_navigation(null);
				return yield* actor.snapshot;
			}),
		);

		expect(result).toEqual({
			navigation: null,
			revalidation: null,
			prefetch: null,
			apiRequests: [],
		});
		expect(emitted_work(commits)).toEqual([
			{
				navigation: {
					href: FIRST_HREF,
					replace: false,
					source: WORK_NAVIGATION_SOURCE_NAVIGATE,
				},
				revalidation: null,
				prefetch: null,
				apiRequests: [],
			},
			{
				navigation: null,
				revalidation: null,
				prefetch: null,
				apiRequests: [],
			},
		]);
	});

	it("keeps skip metadata out of public work while updating indicator activity", async () => {
		const activities: WorkIndicatorActivity[] = [];

		const result = await run_effect(
			Effect.gen(function* () {
				const actor = yield* make_work_state_actor({
					on_indicator_update: (activity) => {
						return Effect.sync(() => {
							activities.push(activity);
						});
					},
				});
				yield* actor.set_navigation({
					href: FIRST_HREF,
					replace: false,
					skipworkIndicator: true,
					source: WORK_NAVIGATION_SOURCE_NAVIGATE,
				});
				const skipped_navigation = yield* actor.indicator_activity;
				yield* actor.set_navigation({
					href: FIRST_HREF,
					replace: false,
					source: WORK_NAVIGATION_SOURCE_NAVIGATE,
				});
				const visible_navigation = yield* actor.indicator_activity;
				yield* actor.set_api_requests([
					{
						key: API_KEY_SAVE,
						method: API_METHOD_POST,
						href: `${FIRST_HREF}/api/save`,
						skipworkIndicator: true,
					},
				]);
				const skipped_api = yield* actor.indicator_activity;
				yield* actor.set_api_requests([
					{
						key: API_KEY_SAVE,
						method: API_METHOD_POST,
						href: `${FIRST_HREF}/api/save`,
					},
				]);
				const visible_api = yield* actor.indicator_activity;
				const snapshot = yield* actor.snapshot;
				return {
					skipped_navigation,
					visible_navigation,
					skipped_api,
					visible_api,
					snapshot,
				};
			}),
		);

		expect(result.skipped_navigation.navigation).toBe(false);
		expect(result.visible_navigation.navigation).toBe(true);
		expect(result.skipped_api.apiRequests).toBe(false);
		expect(result.visible_api.apiRequests).toBe(true);
		expect(result.snapshot).toEqual({
			navigation: {
				href: FIRST_HREF,
				replace: false,
				source: WORK_NAVIGATION_SOURCE_NAVIGATE,
			},
			revalidation: null,
			prefetch: null,
			apiRequests: [
				{
					key: API_KEY_SAVE,
					method: API_METHOD_POST,
					href: `${FIRST_HREF}/api/save`,
				},
			],
		});
		expect(activities).toEqual([
			{
				navigation: false,
				revalidation: false,
				apiRequests: false,
			},
			{
				navigation: true,
				revalidation: false,
				apiRequests: false,
			},
			{
				navigation: true,
				revalidation: false,
				apiRequests: true,
			},
		]);
	});

	it("composes navigation, revalidation, prefetch, and API request work", async () => {
		const updates: WorkState[] = [];

		const result = await run_effect(
			Effect.gen(function* () {
				const actor = yield* make_work_state_actor({
					on_update: (work) => {
						return Effect.sync(() => {
							updates.push(work);
						});
					},
				});
				yield* actor.set_navigation({
					href: SECOND_HREF,
					replace: true,
					source: WORK_NAVIGATION_SOURCE_REDIRECT,
				});
				yield* actor.set_revalidation({
					status: WORK_REVALIDATION_STATUS_DEBOUNCING,
					attempt: 0,
				});
				yield* actor.set_prefetch({ href: PREFETCH_HREF });
				yield* actor.begin_api_request({
					key: API_KEY_SAVE,
					method: API_METHOD_POST,
					href: `${SECOND_HREF}/api/save`,
				});
				yield* actor.begin_api_request({
					key: API_KEY_SEARCH,
					method: API_METHOD_GET,
					href: `${SECOND_HREF}/api/search`,
				});
				return yield* actor.snapshot;
			}),
		);

		expect(result).toEqual({
			navigation: {
				href: SECOND_HREF,
				replace: true,
				source: WORK_NAVIGATION_SOURCE_REDIRECT,
			},
			revalidation: {
				status: WORK_REVALIDATION_STATUS_DEBOUNCING,
				attempt: 0,
			},
			prefetch: { href: PREFETCH_HREF },
			apiRequests: [
				{
					key: API_KEY_SAVE,
					method: API_METHOD_POST,
					href: `${SECOND_HREF}/api/save`,
				},
				{
					key: API_KEY_SEARCH,
					method: API_METHOD_GET,
					href: `${SECOND_HREF}/api/search`,
				},
			],
		});
		expect(updates).toHaveLength(5);
		expect(updates[updates.length - 1]).toEqual(result);
	});

	it("replaces keyed API request work without duplicating entries", async () => {
		const commits: ClientCommit[] = [];

		const result = await run_effect(
			Effect.gen(function* () {
				const actor = yield* make_work_state_actor({
					commit: (commit) => {
						commits.push(commit);
					},
				});
				yield* actor.begin_api_request({
					key: API_KEY_SAVE,
					method: API_METHOD_POST,
					href: `${FIRST_HREF}/api/slow`,
				});
				yield* actor.begin_api_request({
					key: API_KEY_SAVE,
					method: API_METHOD_POST,
					href: `${FIRST_HREF}/api/fast`,
				});
				yield* actor.end_api_request(API_KEY_SEARCH);
				const before_end = yield* actor.snapshot;
				yield* actor.end_api_request(API_KEY_SAVE);
				const after_end = yield* actor.snapshot;
				return { before_end, after_end };
			}),
		);

		expect(result.before_end.apiRequests).toEqual([
			{
				key: API_KEY_SAVE,
				method: API_METHOD_POST,
				href: `${FIRST_HREF}/api/fast`,
			},
		]);
		expect(result.after_end.apiRequests).toEqual([]);
		expect(emitted_work(commits)).toEqual([
			{
				navigation: null,
				revalidation: null,
				prefetch: null,
				apiRequests: [
					{
						key: API_KEY_SAVE,
						method: API_METHOD_POST,
						href: `${FIRST_HREF}/api/slow`,
					},
				],
			},
			{
				navigation: null,
				revalidation: null,
				prefetch: null,
				apiRequests: [
					{
						key: API_KEY_SAVE,
						method: API_METHOD_POST,
						href: `${FIRST_HREF}/api/fast`,
					},
				],
			},
			{
				navigation: null,
				revalidation: null,
				prefetch: null,
				apiRequests: [],
			},
		]);
	});

	it("does not duplicate unchanged work when current state is flushed", async () => {
		const commits: ClientCommit[] = [];

		await run_effect(
			Effect.gen(function* () {
				const actor = yield* make_work_state_actor({
					commit: (commit) => {
						commits.push(commit);
					},
				});
				yield* actor.set_revalidation({
					status: WORK_REVALIDATION_STATUS_RETRYING,
					attempt: 3,
				});
				yield* actor.emit_current;
				yield* actor.set_revalidation(null);
			}),
		);

		expect(emitted_work(commits)).toEqual([
			{
				navigation: null,
				revalidation: {
					status: WORK_REVALIDATION_STATUS_RETRYING,
					attempt: 3,
				},
				prefetch: null,
				apiRequests: [],
			},
			{
				navigation: null,
				revalidation: null,
				prefetch: null,
				apiRequests: [],
			},
		]);
	});
});
