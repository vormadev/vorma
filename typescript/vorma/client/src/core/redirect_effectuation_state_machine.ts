import type { RedirectData } from "./redirects.ts";

type ShouldRedirectData = Extract<RedirectData, { status: "should" }>;

export type RedirectEffectuationExecutionPlan =
	| {
			type: "stop";
			reason: "redirect_effectuation_redirect_data_not_should";
	  }
	| {
			type: "cleanup_and_effectuate_hard";
			reason: "redirect_effectuation_strategy_hard";
	  }
	| {
			type: "cleanup_and_effectuate_soft";
			reason: "redirect_effectuation_strategy_soft";
	  }
	| {
			type: "cleanup_and_stop";
			reason: "redirect_effectuation_strategy_unknown";
	  };

function resolveRedirectStrategyValue(
	redirectData: ShouldRedirectData,
): string {
	return (
		redirectData as ShouldRedirectData & { shouldRedirectStrategy: string }
	).shouldRedirectStrategy;
}

export function decideRedirectEffectuationExecutionPlan(props: {
	redirectData: RedirectData;
}): RedirectEffectuationExecutionPlan {
	const { redirectData } = props;
	if (redirectData.status !== "should") {
		return {
			type: "stop",
			reason: "redirect_effectuation_redirect_data_not_should",
		};
	}

	switch (resolveRedirectStrategyValue(redirectData)) {
		case "hard":
			return {
				type: "cleanup_and_effectuate_hard",
				reason: "redirect_effectuation_strategy_hard",
			};
		case "soft":
			return {
				type: "cleanup_and_effectuate_soft",
				reason: "redirect_effectuation_strategy_soft",
			};
		default:
			return {
				type: "cleanup_and_stop",
				reason: "redirect_effectuation_strategy_unknown",
			};
	}
}
