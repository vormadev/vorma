import { describe, expect, it } from "vitest";
import { decideRedirectEffectuationExecutionPlan } from "../../core/redirect_effectuation_state_machine.ts";
import type { RedirectData } from "../../core/redirects.ts";

function createDidRedirectData(): RedirectData {
	return {
		status: "did",
		href: "/already-redirected",
		hrefDetails: {
			isHTTP: true,
		},
	} as unknown as RedirectData;
}

function createShouldRedirectData(props?: {
	shouldRedirectStrategy?: string;
}): RedirectData {
	return {
		status: "should",
		href: "/target",
		latestBuildID: "build-2",
		shouldRedirectStrategy: props?.shouldRedirectStrategy ?? "hard",
		hrefDetails: {
			isHTTP: true,
			isExternal: false,
			isInternal: true,
			absoluteURL: "http://localhost:3000/target",
			relativeURL: "/target",
		},
	} as unknown as RedirectData;
}

describe("redirect effectuation state machine", () => {
	it("stops when redirect data is not should", () => {
		const plan = decideRedirectEffectuationExecutionPlan({
			redirectData: createDidRedirectData(),
		});

		expect(plan).toEqual({
			type: "stop",
			reason: "redirect_effectuation_redirect_data_not_should",
		});
	});

	it("decides hard strategy effectuation for should redirects", () => {
		const plan = decideRedirectEffectuationExecutionPlan({
			redirectData: createShouldRedirectData({
				shouldRedirectStrategy: "hard",
			}),
		});

		expect(plan).toEqual({
			type: "cleanup_and_effectuate_hard",
			reason: "redirect_effectuation_strategy_hard",
		});
	});

	it("decides soft strategy effectuation for should redirects", () => {
		const plan = decideRedirectEffectuationExecutionPlan({
			redirectData: createShouldRedirectData({
				shouldRedirectStrategy: "soft",
			}),
		});

		expect(plan).toEqual({
			type: "cleanup_and_effectuate_soft",
			reason: "redirect_effectuation_strategy_soft",
		});
	});

	it("falls back to cleanup-and-stop for unknown should strategy values", () => {
		const plan = decideRedirectEffectuationExecutionPlan({
			redirectData: createShouldRedirectData({
				shouldRedirectStrategy: "unexpected",
			}),
		});

		expect(plan).toEqual({
			type: "cleanup_and_stop",
			reason: "redirect_effectuation_strategy_unknown",
		});
	});
});
