import { scrollStateManager } from "./scroll_state_manager.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

let beforeUnloadRegistered = false;
let touchDetectionRegistered = false;

function onBeforeUnload(): void {
	scrollStateManager.savePageRefreshState();
}

function onFirstTouch(): void {
	__vormaClientGlobal.set("isTouchDevice", true);
}

export function registerBeforeUnloadScrollStatePersistence(): void {
	if (beforeUnloadRegistered) return;
	window.addEventListener("beforeunload", onBeforeUnload);
	beforeUnloadRegistered = true;
}

export function registerTouchDetection(): void {
	if (touchDetectionRegistered) return;

	window.addEventListener("touchstart", onFirstTouch, { once: true });
	touchDetectionRegistered = true;
}
