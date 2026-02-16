import { describe, expect, it } from "vitest";
import { buildRedirectEffectuationCommands } from "../../core/redirect_effectuation_commands.ts";
import type { RedirectData } from "../../core/redirects.ts";

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

describe("redirect effectuation command builder", () => {
	it("builds terminal return-null command for stop plans", () => {
		const commands = buildRedirectEffectuationCommands({
			executionPlan: {
				type: "stop",
				reason: "redirect_effectuation_redirect_data_not_should",
			},
			redirectData: createShouldRedirectData(),
			redirectCount: 0,
		});

		expect(commands).toEqual([
			{
				type: "return_null",
				reason: "redirect_effectuation_redirect_data_not_should",
			},
		]);
	});

	it("builds cleanup and hard-effectuation command sequence", () => {
		const redirectData = createShouldRedirectData({
			shouldRedirectStrategy: "hard",
		});
		const commands = buildRedirectEffectuationCommands({
			executionPlan: {
				type: "cleanup_and_effectuate_hard",
				reason: "redirect_effectuation_strategy_hard",
			},
			redirectData,
			redirectCount: 7,
		});

		expect(commands).toEqual([
			{
				type: "cleanup_redirect_related_navigations",
				reason: "redirect_effectuation_strategy_hard",
			},
			{
				type: "effectuate_hard_redirect",
				redirectData,
				reason: "redirect_effectuation_strategy_hard",
			},
		]);
	});

	it("builds cleanup and soft-effectuation command sequence", () => {
		const redirectData = createShouldRedirectData({
			shouldRedirectStrategy: "soft",
		});
		const originalProps = {
			href: "/origin",
			navigationType: "redirect" as const,
			replace: true,
			scrollToTop: false,
			state: { from: "submit" },
		};
		const commands = buildRedirectEffectuationCommands({
			executionPlan: {
				type: "cleanup_and_effectuate_soft",
				reason: "redirect_effectuation_strategy_soft",
			},
			redirectData,
			redirectCount: 3,
			originalProps,
		});

		expect(commands).toEqual([
			{
				type: "cleanup_redirect_related_navigations",
				reason: "redirect_effectuation_strategy_soft",
			},
			{
				type: "effectuate_soft_redirect",
				redirectData,
				redirectCount: 3,
				originalProps,
				reason: "redirect_effectuation_strategy_soft",
			},
		]);
	});

	it("builds cleanup-and-stop sequence for unknown strategy plans", () => {
		const commands = buildRedirectEffectuationCommands({
			executionPlan: {
				type: "cleanup_and_stop",
				reason: "redirect_effectuation_strategy_unknown",
			},
			redirectData: createShouldRedirectData({
				shouldRedirectStrategy: "unexpected",
			}),
			redirectCount: 0,
		});

		expect(commands).toEqual([
			{
				type: "cleanup_redirect_related_navigations",
				reason: "redirect_effectuation_strategy_unknown",
			},
			{
				type: "return_null",
				reason: "redirect_effectuation_strategy_unknown",
			},
		]);
	});
});
