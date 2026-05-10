import { describe, expect, it } from "vitest";
import {
	derive_work,
	work_activity_kind,
	type WorkModelFacts,
} from "./work.ts";

const base_model = {
	active_route_operation_id: null,
	operations: {},
	prefetch_operation_id: null,
	refresh: { kind: "idle" },
	submissions: {},
} satisfies WorkModelFacts;

describe("derive_work", () => {
	it("projects visible navigation without leaking indicator flags", () => {
		const derived = derive_work({
			...base_model,
			active_route_operation_id: "nav-1",
			operations: {
				"nav-1": {
					browser_key: "browser-1",
					href: "https://example.com/next",
					id: "nav-1",
					kind: "navigation",
					public_call_ids: ["call-1"],
					redirect_count: 0,
					replace: false,
					rights: ["publish_route"],
					source: "navigate",
					state: { from: "test" },
					status: "started",
					skip_work_indicator: true,
				},
			},
		});

		expect(derived.projection).toEqual({
			apiRequests: [],
			navigation: {
				href: "https://example.com/next",
				replace: false,
				source: "navigate",
			},
			prefetch: null,
			revalidation: null,
		});
		expect(derived.activity).toEqual([
			{
				kind: work_activity_kind.navigation,
				skip_work_indicator: true,
			},
		]);
	});

	it("projects refresh work only for visible refresh phases", () => {
		const pending = derive_work({
			...base_model,
			refresh: {
				attempt: 0,
				demand: {
					operation_id: "reval-1",
					public_call_ids: ["call-1"],
					reason: "manual",
					skip_work_indicator: false,
				},
				kind: "pending",
			},
		});
		const retrying = derive_work({
			...base_model,
			refresh: {
				attempt: 3,
				demand: {
					operation_id: "reval-1",
					public_call_ids: ["call-1"],
					reason: "manual",
					skip_work_indicator: true,
				},
				kind: "retrying",
				timer_id: "timer-1",
			},
		});

		expect(pending.projection.revalidation).toBeNull();
		expect(pending.activity).toEqual([]);
		expect(retrying.projection.revalidation).toEqual({
			attempt: 3,
			status: "retrying",
		});
		expect(retrying.activity).toEqual([
			{
				kind: work_activity_kind.revalidation,
				skip_work_indicator: true,
			},
		]);
	});

	it("projects prefetch work while resources are still pending", () => {
		const started = derive_work({
			...base_model,
			operations: {
				"prefetch-1": {
					href: "https://example.com/warm",
					id: "prefetch-1",
					kind: "route_prefetch",
					public_call_ids: [],
					rights: ["abort_effects"],
					status: "started",
				},
			},
			prefetch_operation_id: "prefetch-1",
		});
		const prepared = derive_work({
			...base_model,
			operations: {
				"prefetch-1": {
					href: "https://example.com/warm",
					id: "prefetch-1",
					kind: "route_prefetch",
					prepared_resource_key: "resource-1",
					public_call_ids: [],
					rights: [],
					status: "prepared",
				},
			},
			prefetch_operation_id: "prefetch-1",
		});

		expect(started.projection.prefetch).toEqual({
			href: "https://example.com/warm",
		});
		expect(started.activity).toEqual([
			{
				kind: work_activity_kind.prefetch,
			},
		]);
		expect(prepared.projection.prefetch).toBeNull();
		expect(prepared.activity).toEqual([]);
	});

	it("projects API submissions as public requests and private activity", () => {
		const derived = derive_work({
			...base_model,
			submissions: {
				"submit-1": {
					href: "https://example.com/api/save",
					method: "POST",
					operation_id: "api-1",
					public_call_id: "call-1",
					revalidate: true,
					revalidation_operation_id: "reval-1",
					skip_work_indicator: false,
					submission_key: "submit-1",
				},
			},
		});

		expect(derived.projection.apiRequests).toEqual([
			{
				href: "https://example.com/api/save",
				key: "submit-1",
				method: "POST",
			},
		]);
		expect(derived.activity).toEqual([
			{
				kind: work_activity_kind.api_request,
				skip_work_indicator: false,
			},
		]);
	});
});
