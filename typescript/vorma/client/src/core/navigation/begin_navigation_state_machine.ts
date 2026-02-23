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
	  }
	| {
			slot: "prefetch";
			targetUrl: string;
	  }
	| {
			slot: "revalidation";
			revalidationHref: string;
	  };

type BeginNavigationExecutionPlanBase = {
	abortInstructions: BeginNavigationAbortInstruction[];
};

export type BeginNavigationExecutionPlan =
	| (BeginNavigationExecutionPlanBase & {
			type: "reuse";
			reuseInstruction: BeginNavigationReuseInstruction;
	  })
	| (BeginNavigationExecutionPlanBase & {
			type: "create";
			createInstruction: BeginNavigationCreateInstruction;
	  })
	| (BeginNavigationExecutionPlanBase & {
			type: "immediateAbort";
	  });

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
			type: "reuse",
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "active",
				sourcePrefetchKey: null,
				entry: active,
				promotion,
			},
		};
	}

	if (prefetchMatch) {
		return {
			type: "reuse",
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: prefetchMatch.key,
				entry: prefetchMatch.entry,
				promotion,
			},
		};
	}

	if (revalidation && revalidationHasSameNavigationTarget) {
		return {
			type: "reuse",
			abortInstructions,
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion,
			},
		};
	}

	return {
		type: "create",
		abortInstructions,
		createInstruction: {
			slot: "active",
		},
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
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "active",
				sourcePrefetchKey: null,
				entry: active,
				promotion: null,
			},
		};
	}

	if (prefetchMatch) {
		return {
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "prefetch",
				sourcePrefetchKey: prefetchMatch.key,
				entry: prefetchMatch.entry,
				promotion: null,
			},
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
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion: null,
			},
		};
	}

	if (
		hasSameNavigationTarget({
			firstHref: currentHref,
			secondHref: targetUrl,
		})
	) {
		return {
			type: "immediateAbort",
			abortInstructions: [],
		};
	}

	return {
		type: "create",
		abortInstructions: [],
		createInstruction: {
			slot: "prefetch",
			targetUrl,
		},
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
			type: "reuse",
			abortInstructions: [],
			reuseInstruction: {
				sourceSlot: "revalidation",
				sourcePrefetchKey: null,
				entry: revalidation,
				promotion: null,
			},
		};
	}

	return {
		type: "create",
		abortInstructions: revalidation
			? [
					{
						slot: "revalidation",
						entry: revalidation,
					},
				]
			: [],
		createInstruction: {
			slot: "revalidation",
			revalidationHref: props.currentHref,
		},
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
