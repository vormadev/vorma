import { isAbortError } from "./utils/errors.ts";
import { logError } from "./utils/logging.ts";

export function processSettledClientLoaderResults(props: {
	results: Array<PromiseSettledResult<any>>;
	matchedPatterns: Array<string>;
}): {
	data: Array<any>;
	errorMessage: string | undefined;
} {
	const { results, matchedPatterns } = props;
	const data: Array<any> = [];
	let errorMessage: string | undefined;

	for (let i = 0; i < results.length; i++) {
		const result = results[i];
		if (!result) {
			data.push(undefined);
			continue;
		}

		if (result.status === "fulfilled") {
			data.push(result.value);
		} else {
			// This is a rejection
			if (!isAbortError(result.reason)) {
				// This is the first true error we've hit
				const pattern = matchedPatterns[i];
				logError(
					`Client loader error for pattern ${pattern}:`,
					result.reason,
				);
				errorMessage =
					result.reason instanceof Error
						? result.reason.message
						: String(result.reason);

				// We found the highest error. Stop processing.
				// The .catch() wrapper already aborted any children.
			}
			data.push(undefined);
			break; // Stop at the first error
		}
	}

	return { data, errorMessage };
}
