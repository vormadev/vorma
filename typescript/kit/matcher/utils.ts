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

	if (start < path.length) {
		segments.push(path.substring(start));
	}

	if (path.endsWith("/")) {
		segments.push("");
	}

	return segments;
}

export function stripTrailingSlash(pattern: string): string {
	return pattern.length > 0 && pattern[pattern.length - 1] === "/"
		? pattern.substring(0, pattern.length - 1)
		: pattern;
}
