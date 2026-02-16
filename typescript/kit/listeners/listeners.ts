import { debounce } from "vorma/kit/debounce";

export function addOnWindowFocusListener(callback: () => void): () => void {
	const debouncedCallback = debounce(callback, 30);
	const ifVisibleCallback = () => {
		if (document.visibilityState === "visible") {
			debouncedCallback();
		}
	};
	window.addEventListener("focus", debouncedCallback);
	window.addEventListener("visibilitychange", ifVisibleCallback);
	return () => {
		window.removeEventListener("focus", debouncedCallback);
		window.removeEventListener("visibilitychange", ifVisibleCallback);
	};
}
