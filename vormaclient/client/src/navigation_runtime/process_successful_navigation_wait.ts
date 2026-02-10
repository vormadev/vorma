import { setClientLoadersState } from "../client_loaders.ts";
import { logError } from "../utils/logging.ts";
import type { NavigationOutcome } from "./types.ts";

export async function waitForSuccessfulNavigationAssets(
	outcome: Extract<NavigationOutcome, { type: "success" }>,
): Promise<void> {
	const { waitFnPromise, cssBundlePromises } = outcome;

	const clientLoadersResult = await waitFnPromise;
	setClientLoadersState(clientLoadersResult);

	if (cssBundlePromises.length > 0) {
		try {
			await Promise.all(cssBundlePromises);
		} catch (error) {
			logError("Error preloading CSS bundles:", error);
		}
	}
}
