import { debounce } from "vorma/kit/debounce";

/**
 * Run `callback` when the window regains focus AND the page is visible —
 * both the `focus` event and a `visibilitychange` transition to visible are
 * covered (a window can become visible without a `focus` event firing, and
 * vice versa across some browsers/platforms), and the callback is debounced
 * 30ms so a rapid focus/visibility flicker only fires it once. Returns a
 * cleanup function that removes both listeners and cancels any pending
 * debounced call.
 *
 * This is the platform-level primitive `ClientOptions.revalidateOnWindowFocus`
 * builds on internally; reach for it directly only when window-focus
 * behavior outside of Vorma's own revalidation is needed.
 */
export function addOnWindowFocusListener(callback: () => void): () => void {
	const debounced_callback = debounce(callback, 30);
	const if_visible_callback = (): void => {
		if (document.visibilityState === "visible") {
			void debounced_callback();
		}
	};
	window.addEventListener("focus", debounced_callback);
	window.addEventListener("visibilitychange", if_visible_callback);
	return () => {
		window.removeEventListener("focus", debounced_callback);
		window.removeEventListener("visibilitychange", if_visible_callback);
		debounced_callback.cancel();
	};
}
