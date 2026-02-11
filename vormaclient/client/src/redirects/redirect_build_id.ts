import { dispatchBuildIDEvent } from "../events.ts";
import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";
import type { RedirectData } from "./redirects.ts";

export function syncBuildIDFromRedirectData(redirectData: RedirectData): void {
	if (redirectData.status !== "should") {
		return;
	}

	const oldID = __vormaClientGlobal.get("buildID");
	const newID = redirectData.latestBuildID;
	if (newID && newID !== oldID) {
		__vormaClientGlobal.set("buildID", newID);
		dispatchBuildIDEvent({ newID, oldID });
	}
}
