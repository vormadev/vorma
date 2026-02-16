export function parseSegments(path: string): string[] {
	if (path === "" || path === "/") {
		return path === "/" ? [""] : [];
	}

	const startIdx = path.startsWith("/") ? 1 : 0;
	const segments: string[] = [];
	let start = startIdx;

	for (let i = startIdx; i < path.length; i++) {
		if (path[i] === "/") {
			if (i > start) {
				segments.push(path.substring(start, i));
			}
			start = i + 1;
		}
	}

	// Add the last segment if it exists
	if (start < path.length) {
		segments.push(path.substring(start));
	}

	// Add empty segment for trailing slash
	if (path.endsWith("/")) {
		segments.push("");
	}

	return segments;
}
