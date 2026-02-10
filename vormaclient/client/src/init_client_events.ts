import { scrollStateManager } from "./scroll_state_manager.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export function registerBeforeUnloadScrollStatePersistence(): void {
	window.addEventListener("beforeunload", () => {
		scrollStateManager.savePageRefreshState();
	});
}

export function registerTouchDetection(): void {
	window.addEventListener(
		"touchstart",
		() => {
			__vormaClientGlobal.set("isTouchDevice", true);
		},
		{ once: true },
	);
}
