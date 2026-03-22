/// <reference types="vite/client" />

export function default_error_boundary(props: { error: unknown }): string {
	return `Route Error: ${String(props.error)}`;
}

export function format_error_for_rendering(outermost_error: unknown): string {
	if (outermost_error instanceof Error) {
		return "Error: " + (outermost_error.message || "unknown");
	}
	if (typeof outermost_error === "string") {
		return "Error: " + outermost_error;
	}
	if (outermost_error != null) {
		const s = JSON.stringify(outermost_error);
		if (s !== undefined) return "Error: " + s;
	}
	return "Error: unknown";
}
