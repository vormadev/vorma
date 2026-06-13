import { describe, expect, it } from "vitest";
import {
	derive_work_projection,
	derive_work_state,
	type WorkSources,
} from "./work_projection.ts";

function empty_sources(): WorkSources {
	return {
		nav: null,
		active_revalidation: null,
		refresh: { kind: "idle" },
		refresh_pending_visible: false,
		submissions: [],
		prefetch: null,
	};
}

describe("derive_work_state", () => {
	it("projects idle sources to an empty work state", () => {
		expect(derive_work_state(empty_sources())).toEqual({
			navigation: null,
			revalidation: null,
			prefetch: null,
			apiRequests: [],
		});
	});

	it("projects a navigation without leaking skip_work_indicator", () => {
		const sources = empty_sources();
		sources.nav = {
			href: "https://x.example/a",
			replace: true,
			source: "navigate",
			skip_work_indicator: true,
		};
		expect(derive_work_state(sources).navigation).toEqual({
			href: "https://x.example/a",
			replace: true,
			source: "navigate",
		});
	});

	it("prefers the running revalidation over scheduler states", () => {
		const sources = empty_sources();
		sources.active_revalidation = { attempt: 3 };
		sources.refresh = { kind: "retrying", attempt: 5 };
		expect(derive_work_state(sources).revalidation).toEqual({
			status: "running",
			attempt: 3,
		});
	});

	it("projects debouncing and retrying scheduler states", () => {
		const sources = empty_sources();
		sources.refresh = { kind: "debouncing" };
		expect(derive_work_state(sources).revalidation).toEqual({
			status: "debouncing",
			attempt: 0,
		});

		sources.refresh = { kind: "retrying", attempt: 2 };
		expect(derive_work_state(sources).revalidation).toEqual({
			status: "retrying",
			attempt: 2,
		});
	});

	it("lists submissions and pending prefetch", () => {
		const sources = empty_sources();
		sources.submissions = [
			{ key: "k1", method: "POST", href: "https://x.example/api" },
		];
		sources.prefetch = { href: "https://x.example/next" };
		const work = derive_work_state(sources);
		expect(work.apiRequests).toEqual([
			{ key: "k1", method: "POST", href: "https://x.example/api" },
		]);
		expect(work.prefetch).toEqual({ href: "https://x.example/next" });
	});
});

describe("derive_work_projection", () => {
	it("a pending demand counts only when visible", () => {
		const sources = empty_sources();
		sources.refresh = { kind: "pending", attempt: 0 };
		sources.refresh_pending_visible = false;
		expect(derive_work_projection(sources)).toEqual([]);

		sources.refresh_pending_visible = true;
		expect(derive_work_projection(sources)).toEqual([
			{ kind: "revalidation", skip_work_indicator: undefined },
		]);
	});

	it("debouncing and retrying count regardless of visibility", () => {
		const sources = empty_sources();
		sources.refresh = { kind: "debouncing" };
		expect(derive_work_projection(sources)).toEqual([
			{ kind: "revalidation", skip_work_indicator: undefined },
		]);
	});

	it("running revalidation carries its own skip flag over the demand's", () => {
		const sources = empty_sources();
		sources.active_revalidation = { attempt: 1, skip_work_indicator: true };
		sources.refresh = { kind: "pending", attempt: 1 };
		sources.refresh_demand_skip_work_indicator = false;
		expect(derive_work_projection(sources)).toEqual([
			{ kind: "revalidation", skip_work_indicator: true },
		]);
	});

	it("projects every concurrent work kind in stable order", () => {
		const sources: WorkSources = {
			nav: {
				href: "https://x.example/a",
				replace: false,
				source: "navigate",
				skip_work_indicator: false,
			},
			active_revalidation: null,
			refresh: { kind: "retrying", attempt: 1 },
			refresh_demand_skip_work_indicator: true,
			refresh_pending_visible: false,
			submissions: [
				{ key: "k1", method: "POST", href: "h1", skip_work_indicator: true },
				{ key: "k2", method: "GET", href: "h2" },
			],
			prefetch: { href: "https://x.example/p" },
		};
		expect(derive_work_projection(sources)).toEqual([
			{ kind: "navigation", skip_work_indicator: false },
			{ kind: "revalidation", skip_work_indicator: true },
			{ kind: "apiRequest", skip_work_indicator: true },
			{ kind: "apiRequest", skip_work_indicator: undefined },
			{ kind: "prefetch" },
		]);
	});
});
