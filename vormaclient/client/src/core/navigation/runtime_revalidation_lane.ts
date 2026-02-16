import { resolveAbsoluteHref } from "vorma/kit/url";
import { hasSameNavigationTarget } from "../../platform/url.ts";
import type { NavigateProps } from "./types.ts";

type RevalidationNavigateResult = Promise<{ didNavigate: boolean }>;

type RevalidationLaneState = {
	inFlightPromise: RevalidationNavigateResult | null;
	inFlightTargetUrl: string | null;
	isTrailingEligible: boolean;
	shouldRunTrailingPass: boolean;
	trailingPromise: RevalidationNavigateResult | null;
	resolveTrailingPromise: ((result: { didNavigate: boolean }) => void) | null;
};

export type DeterministicRevalidationLane = {
	runRevalidation: (props: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}) => RevalidationNavigateResult;
	clearQueuedTrailingRequest: () => void;
	reset: () => void;
};

export function createDeterministicRevalidationLane(props: {
	getCurrentHref: () => string;
	onInFlightTargetMismatch: () => void;
}): DeterministicRevalidationLane {
	const state: RevalidationLaneState = {
		inFlightPromise: null,
		inFlightTargetUrl: null,
		isTrailingEligible: false,
		shouldRunTrailingPass: false,
		trailingPromise: null,
		resolveTrailingPromise: null,
	};

	function resolveAndClearTrailingPromise(props?: {
		result?: { didNavigate: boolean };
	}): void {
		const result = props?.result || { didNavigate: false };
		state.resolveTrailingPromise?.(result);
		state.trailingPromise = null;
		state.resolveTrailingPromise = null;
	}

	function clearQueuedTrailingRequest(): void {
		state.shouldRunTrailingPass = false;
		resolveAndClearTrailingPromise();
	}

	function startPass(startNavigateProps: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}): RevalidationNavigateResult {
		const { navigateSinglePass } = startNavigateProps;
		const revalidationHref = props.getCurrentHref();
		const revalidationProps: NavigateProps = {
			href: revalidationHref,
			navigationType: "revalidation",
		};
		const passPromise = navigateSinglePass(revalidationProps);
		state.inFlightPromise = passPromise;
		state.inFlightTargetUrl = resolveAbsoluteHref({
			href: revalidationHref,
		});
		state.isTrailingEligible = false;

		queueMicrotask(() => {
			if (state.inFlightPromise === passPromise) {
				state.isTrailingEligible = true;
			}
		});

		void passPromise.finally(() => {
			if (state.inFlightPromise !== passPromise) {
				return;
			}

			state.inFlightPromise = null;
			state.inFlightTargetUrl = null;
			state.isTrailingEligible = false;

			if (!state.shouldRunTrailingPass) {
				clearQueuedTrailingRequest();
				return;
			}

			state.shouldRunTrailingPass = false;
			const resolveTrailingPromise = state.resolveTrailingPromise;
			state.trailingPromise = null;
			state.resolveTrailingPromise = null;

			void startPass({ navigateSinglePass }).then(
				(result) => resolveTrailingPromise?.(result),
				() => resolveTrailingPromise?.({ didNavigate: false }),
			);
		});

		return passPromise;
	}

	function scheduleTrailingPass(): RevalidationNavigateResult {
		state.shouldRunTrailingPass = true;

		if (state.trailingPromise) {
			return state.trailingPromise;
		}

		state.trailingPromise = new Promise((resolve) => {
			state.resolveTrailingPromise = resolve;
		});
		return state.trailingPromise;
	}

	function hasInFlightTargetMismatch(): boolean {
		if (!state.inFlightTargetUrl) {
			return false;
		}

		return !hasSameNavigationTarget({
			firstHref: state.inFlightTargetUrl,
			secondHref: props.getCurrentHref(),
		});
	}

	function runRevalidation(runProps: {
		navigateSinglePass: (
			props: NavigateProps,
		) => RevalidationNavigateResult;
	}): RevalidationNavigateResult {
		const { navigateSinglePass } = runProps;
		if (!state.inFlightPromise) {
			return startPass({ navigateSinglePass });
		}

		if (hasInFlightTargetMismatch()) {
			props.onInFlightTargetMismatch();
			clearQueuedTrailingRequest();
			return startPass({ navigateSinglePass });
		}

		if (!state.isTrailingEligible) {
			return state.inFlightPromise;
		}

		return scheduleTrailingPass();
	}

	function reset(): void {
		state.inFlightPromise = null;
		state.inFlightTargetUrl = null;
		state.isTrailingEligible = false;
		clearQueuedTrailingRequest();
	}

	return {
		runRevalidation,
		clearQueuedTrailingRequest: clearQueuedTrailingRequest,
		reset,
	};
}
