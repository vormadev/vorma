import type { Action, Location } from "history";

export function analyzeHistoryListenerPrelude(props: {
	action: Action;
	location: Location;
	lastKnownLocation: Location;
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
