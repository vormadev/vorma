import type { NavigateProps } from "./navigation/types.ts";
import type { RedirectData } from "./redirects.ts";
import type { RedirectEffectuationExecutionPlan } from "./redirect_effectuation_state_machine.ts";

type ShouldRedirectData = Extract<RedirectData, { status: "should" }>;

export type RedirectEffectuationCommand =
	| {
			type: "cleanup_redirect_related_navigations";
			reason:
				| "redirect_effectuation_strategy_hard"
				| "redirect_effectuation_strategy_soft"
				| "redirect_effectuation_strategy_unknown";
	  }
	| {
			type: "effectuate_hard_redirect";
			redirectData: ShouldRedirectData;
			reason: "redirect_effectuation_strategy_hard";
	  }
	| {
			type: "effectuate_soft_redirect";
			redirectData: ShouldRedirectData;
			redirectCount: number;
			originalProps?: NavigateProps;
			reason: "redirect_effectuation_strategy_soft";
	  }
	| {
			type: "return_null";
			reason:
				| "redirect_effectuation_redirect_data_not_should"
				| "redirect_effectuation_strategy_unknown";
	  };

export function buildRedirectEffectuationCommands(props: {
	executionPlan: RedirectEffectuationExecutionPlan;
	redirectData: RedirectData;
	redirectCount: number;
	originalProps?: NavigateProps;
}): RedirectEffectuationCommand[] {
	const { executionPlan, redirectData, redirectCount, originalProps } = props;

	switch (executionPlan.type) {
		case "stop":
			return [
				{
					type: "return_null",
					reason: executionPlan.reason,
				},
			];
		case "cleanup_and_effectuate_hard":
			return [
				{
					type: "cleanup_redirect_related_navigations",
					reason: executionPlan.reason,
				},
				{
					type: "effectuate_hard_redirect",
					redirectData: redirectData as ShouldRedirectData,
					reason: executionPlan.reason,
				},
			];
		case "cleanup_and_effectuate_soft":
			return [
				{
					type: "cleanup_redirect_related_navigations",
					reason: executionPlan.reason,
				},
				{
					type: "effectuate_soft_redirect",
					redirectData: redirectData as ShouldRedirectData,
					redirectCount,
					originalProps,
					reason: executionPlan.reason,
				},
			];
		case "cleanup_and_stop":
			return [
				{
					type: "cleanup_redirect_related_navigations",
					reason: executionPlan.reason,
				},
				{
					type: "return_null",
					reason: executionPlan.reason,
				},
			];
	}
}
