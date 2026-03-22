/// <reference types="vite/client" />

import { add_window_listener } from "./events.ts";
import { ensure_global } from "./global_state.ts";

function on_pointer_event(event: Event): void {
	const g = ensure_global();
	const pt = (event as PointerEvent).pointerType;
	if (typeof pt !== "string") return;
	const normalized = pt.toLowerCase();
	if (normalized === "touch") {
		g.is_touch_active = true;
	} else if (normalized === "mouse" || normalized === "pen") {
		g.is_touch_active = false;
	}
}

export function register_input_modality_listeners(): void {
	const g = ensure_global();
	if (g.has_registered_input_modality_listeners) return;

	add_window_listener("touchstart", () => {
		ensure_global().is_touch_active = true;
	});
	add_window_listener("pointerdown", on_pointer_event);
	add_window_listener("pointermove", on_pointer_event);
	add_window_listener("pointerenter", on_pointer_event);

	g.has_registered_input_modality_listeners = true;
}
