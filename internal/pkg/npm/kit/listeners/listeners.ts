import { debounce } from "vorma/kit/debounce";

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
