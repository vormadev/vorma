import type { historyInstance } from "./npm_history_types.ts";

type HistoryActionValue = "POP" | "PUSH" | "REPLACE";

type HistoryLocationPrelude = Pick<
	historyInstance["location"],
	"key" | "pathname" | "search"
>;

export function analyzeHistoryListenerPrelude(props: {
	action: HistoryActionValue;
	location: HistoryLocationPrelude;
	lastKnownLocation: HistoryLocationPrelude;
}): {
	didLocationKeyChange: boolean;
	popWithinSameDoc: boolean;
	shouldSaveScrollState: boolean;
} {
	const { action, location, lastKnownLocation } = props;
	const didLocationKeyChange = location.key !== lastKnownLocation.key;
	const popWithinSameDoc =
		action === "POP" &&
		location.pathname === lastKnownLocation.pathname &&
		location.search === lastKnownLocation.search;

	return {
		didLocationKeyChange,
		popWithinSameDoc,
		shouldSaveScrollState: !popWithinSameDoc,
	};
}
