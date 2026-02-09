import { addOnWindowFocusListener } from "vorma/kit/listeners";
import {
	getLastTriggeredNavOrRevalidateTimestampMS,
	getStatus,
	revalidate,
} from "../client.ts";

/**
 * If called, will setup listeners to revalidate the current route when
 * the window regains focus and at least `staleTimeMS` has passed since
 * the last revalidation. The `staleTimeMS` option defaults to 5,000
 * (5 seconds). Returns a cleanup function.
 */
export function revalidateOnWindowFocus(options?: { staleTimeMS?: number }) {
	const staleTimeMS = options?.staleTimeMS ?? 5_000;
	return addOnWindowFocusListener(() => {
		const status = getStatus();
		if (
			!status.isNavigating &&
			!status.isSubmitting &&
			!status.isRevalidating
		) {
			if (
				Date.now() - getLastTriggeredNavOrRevalidateTimestampMS() <
				staleTimeMS
			) {
				return;
			}
			revalidate();
		}
	});
}
