import type { historyInstance } from "./npm_history_types.ts";
import { hasSameDataTarget } from "../hash_fragment.ts";

type HistoryLocationPrelude = Pick<
	historyInstance["location"],
	"key" | "pathname" | "search"
>;

function toAbsoluteHref(location: HistoryLocationPrelude): string {
	return new URL(
		`${location.pathname}${location.search}`,
		window.location.origin,
	).href;
}

export function analyzeHistoryListenerPrelude(props: {
	action: historyInstance["action"];
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
		hasSameDataTarget(
			toAbsoluteHref(location),
			toAbsoluteHref(lastKnownLocation),
		);

	return {
		didLocationKeyChange,
		popWithinSameDoc,
		shouldSaveScrollState: !popWithinSameDoc,
	};
}
