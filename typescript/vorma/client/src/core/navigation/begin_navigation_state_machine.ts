import { resolveAbsoluteHref } from "vorma/kit/url";
import {
	findMapEntryByNavigationTarget,
	hasSameNavigationTarget,
} from "../../platform/url.ts";
import type {
	NavigateProps,
	NavigationEntry,
	NavigationIntent,
} from "./types.ts";

export type BeginNavigationAbortInstruction =
	| {
			slot: "active";
			entry: NavigationEntry;
	  }
	| {
			slot: "revalidation";
			entry: NavigationEntry;
	  }
	| {
			slot: "prefetch";
			key: string;
			entry: NavigationEntry;
	  };

export type BeginNavigationPromotion = {
	targetUrl: string;
	type: NavigationEntry["type"];
	intent: NavigationIntent;
	scrollToTop: NavigateProps["scrollToTop"];
	replace: NavigateProps["replace"];
	state: NavigateProps["state"];
};

export type BeginNavigationReuseInstruction = {
	sourceSlot: "active" | "revalidation" | "prefetch";
	sourcePrefetchKey: string | null;
	entry: NavigationEntry;
	promotion: BeginNavigationPromotion | null;
};

export type BeginNavigationCreateInstruction =
	| {
			slot: "active";
			intent: NavigationIntent;
	  }
	| {
			slot: "prefetch";
			targetUrl: string;
	  }
	| {
			slot: "revalidation";
			revalidationHref: string;
	  };

export type BeginNavigationExecutionPlan = {
	targetUrl: string;
	abortInstructions: BeginNavigationAbortInstruction[];
	reuseInstruction: BeginNavigationReuseInstruction | null;
	createInstruction: BeginNavigationCreateInstruction | null;
	shouldReturnImmediatelyAbortedControl: boolean;
};

type BeginNavigationLaneSnapshot = {
	active: NavigationEntry | null;
	revalidation: NavigationEntry | null;
	prefetch: Map<string, NavigationEntry>;
};

function hasEntryWithSameNavigationTarget(props: {
	entry: Pick<NavigationEntry, "targetUrl"> | null;
	targetUrl: string;
}): boolean {
	const { entry, targetUrl } = props;
	return (
		!!entry &&
		hasSameNavigationTarget({
			firstHref: entry.targetUrl,
			secondHref: targetUrl,
		})
	);
}

function findPrefetchMatchByNavigationTarget(props: {
	prefetch: Map<string, NavigationEntry>;
	targetUrl: string;
}):
	| {
			key: string;
			entry: NavigationEntry;
	  }
	| undefined {
	const matchedPrefetch = findMapEntryByNavigationTarget({
		map: props.prefetch,
		targetHref: props.targetUrl,
	});
	if (!matchedPrefetch) {
		return undefined;
	}

	const [key, entry] = matchedPrefetch;
	return { key, entry };
}

function buildActiveLanePromotion(props: {
	navigationProps: NavigateProps;
	targetUrl: string;
}): BeginNavigationPromotion {
	return {
		targetUrl: props.targetUrl,
		type: props.navigationProps.navigationType,
		intent: "navigate",
		scrollToTop: props.navigationProps.scrollToTop,
		replace: props.navigationProps.replace,
		state: props.navigationProps.state,
	};
}

function decideActiveLaneBeginExecutionPlan(props: {
	navigationProps: NavigateProps;
	targetUrl: string;
	lanes: BeginNavigationLaneSnapshot;
}): BeginNavigationExecutionPlan {
	const { navigationProps, targetUrl, lanes } = props;
	const active = lanes.active;
	const revalidation = lanes.revalidation;
	const activeHasSameNavigationTarget = hasEntryWithSameNavigationTarget({
		entry: active,
		targetUrl,
	});
	const revalidationHasSameNavigationTarget =
		hasEntryWithSameNavigationTarget({
			entry: revalidation,
			targetUrl,
		});
	const prefetchMatch = !activeHasSameNavigationTarget
		? findPrefetchMatchByNavigationTarget({
				prefetch: lanes.prefetch,
				targetUrl,
			})
		: undefined;

	const abortInstructions: BeginNavigationAbortInstruction[] = [];
	if (active && !activeHasSameNavigationTarget) {
		abortInstructions.push({
			slot: "active",
			entry: active,
		});
	}

	for (const [key, prefetchEntry] of lanes.prefetch.entries()) {
		if (key !== prefetchMatch?.key) {
			abortInstructions.push({
				slot: "prefetch",
				key,
				entry: prefetchEntry,
			});
		}
	}

	if (revalidation && !revalidationHasSameNavigationTarget) {
		abortInstructions.push({
			slot: "revalidation",
			entry: revalidation,
		});
	}

	const promotion = buildActiveLanePromotion({
		navigationProps,
		targetUrl,
	});

	if (active && activeHasSameNavigationTarget) {
		return {
			targetUrl,
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "active",
				sourcePrefetchKey: null,
				entry: active,
				promotion,
			},
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};
	}

	if (prefetchMatch) {
		return {
			targetUrl,
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: prefetchMatch.key,
				entry: prefetchMatch.entry,
				promotion,
			},
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};
	}

	if (revalidation && revalidationHasSameNavigationTarget) {
		return {
			targetUrl,
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion,
			},
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};
	}

	return {
		targetUrl,
		abortInstructions,
		reuseInstruction: null,
		createInstruction: {
			slot: "active",
			intent: "navigate",
		},
		shouldReturnImmediatelyAbortedControl: false,
	};
}

function decidePrefetchBeginExecutionPlan(props: {
	targetUrl: string;
	currentHref: string;
	lanes: BeginNavigationLaneSnapshot;
}): BeginNavigationExecutionPlan {
	const { targetUrl, currentHref, lanes } = props;
	const active = lanes.active;
	const revalidation = lanes.revalidation;
	const prefetchMatch = findPrefetchMatchByNavigationTarget({
		prefetch: lanes.prefetch,
		targetUrl,
	});

	if (
		active &&
		hasEntryWithSameNavigationTarget({
			entry: active,
			targetUrl,
		})
	) {
		return {
			targetUrl,
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "active",
				sourcePrefetchKey: null,
				entry: active,
				promotion: null,
			},
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};
	}

	if (prefetchMatch) {
		return {
			targetUrl,
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: prefetchMatch.key,
				entry: prefetchMatch.entry,
				promotion: null,
			},
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};
	}

	if (
		revalidation &&
		hasEntryWithSameNavigationTarget({
			entry: revalidation,
			targetUrl,
		})
	) {
		return {
			targetUrl,
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion: null,
			},
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};
	}

	if (
		hasSameNavigationTarget({
			firstHref: currentHref,
			secondHref: targetUrl,
		})
	) {
		return {
			targetUrl,
			abortInstructions: [],
			reuseInstruction: null,
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: true,
		};
	}

	return {
		targetUrl,
		abortInstructions: [],
		reuseInstruction: null,
		createInstruction: {
			slot: "prefetch",
			targetUrl,
		},
		shouldReturnImmediatelyAbortedControl: false,
	};
}

function decideRevalidationBeginExecutionPlan(props: {
	currentHref: string;
	lanes: BeginNavigationLaneSnapshot;
}): BeginNavigationExecutionPlan {
	const targetUrl = resolveAbsoluteHref({ href: props.currentHref });
	const revalidation = props.lanes.revalidation;
	if (
		revalidation &&
		hasEntryWithSameNavigationTarget({
			entry: revalidation,
			targetUrl,
		})
	) {
		return {
			targetUrl,
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion: null,
			},
			createInstruction: null,
			shouldReturnImmediatelyAbortedControl: false,
		};
	}

	return {
		targetUrl,
		abortInstructions: revalidation
			? [
					{
						slot: "revalidation",
						entry: revalidation,
					},
				]
			: [],
		reuseInstruction: null,
		createInstruction: {
			slot: "revalidation",
			revalidationHref: props.currentHref,
		},
		shouldReturnImmediatelyAbortedControl: false,
	};
}

export function resolveBeginNavigationTargetURL(props: {
	navigationProps: NavigateProps;
	currentHref: string;
}): string {
	if (props.navigationProps.navigationType === "revalidation") {
		return resolveAbsoluteHref({ href: props.currentHref });
	}

	return resolveAbsoluteHref({ href: props.navigationProps.href });
}

export function decideBeginNavigationExecutionPlan(props: {
	navigationProps: NavigateProps;
	currentHref: string;
	lanes: BeginNavigationLaneSnapshot;
}): BeginNavigationExecutionPlan {
	const { navigationProps, currentHref, lanes } = props;
	const targetUrl = resolveBeginNavigationTargetURL({
		navigationProps,
		currentHref,
	});

	switch (navigationProps.navigationType) {
		case "userNavigation":
		case "browserHistory":
		case "redirect":
		case "action":
			return decideActiveLaneBeginExecutionPlan({
				navigationProps,
				targetUrl,
				lanes,
			});
		case "prefetch":
			return decidePrefetchBeginExecutionPlan({
				targetUrl,
				currentHref,
				lanes,
			});
		case "revalidation":
			return decideRevalidationBeginExecutionPlan({
				currentHref,
				lanes,
			});
	}
}
