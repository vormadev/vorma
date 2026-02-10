import { findNestedMatches } from "vorma/kit/matcher/find-nested";

type PartialMatchLookupInput = {
	pathname: string;
	patternRegistry: any;
	patternToWaitFnMap: Record<string, unknown>;
};

export async function findClientLoaderPartialMatches(
	input: PartialMatchLookupInput,
) {
	const { pathname, patternRegistry, patternToWaitFnMap } = input;

	if (Object.keys(patternToWaitFnMap).length === 0) {
		return null;
	}

	// First try the full path
	const fullResult = findNestedMatches(patternRegistry, pathname);
	if (fullResult) {
		// If we get a full match, we have everything we need
		return fullResult;
	}

	// If no full match, try progressively shorter paths to find partial matches
	const segments = pathname.split("/").filter(Boolean);

	// Try from longest to shortest
	for (let i = segments.length; i >= 0; i--) {
		const partialPath =
			i === 0 ? "/" : "/" + segments.slice(0, i).join("/");
		const result = findNestedMatches(patternRegistry, partialPath);
		if (result) {
			return result; // First match is the longest
		}
	}

	return null;
}
