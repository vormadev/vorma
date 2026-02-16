/**
 * Deterministically serializes a JSON-compatible value to a stable string.
 * Throws if it detects a circular reference. Does not support Maps, Sets,
 * Functions, or other non-JSON types.
 */
export function jsonStringifyStable(input: unknown): string {
	// First stabilize the structure, then JSON.stringify
	const stabilized = stabilizeStructure(input, new WeakSet());

	return JSON.stringify(stabilized);
}

function stabilizeStructure(value: unknown, visited: WeakSet<object>): unknown {
	// Handle primitives and null
	if (value === null || typeof value !== "object") {
		return value;
	}

	// Prevent circular references
	if (visited.has(value)) {
		throw new Error("Circular reference detected during stable JSON stringification");
	}
	visited.add(value);

	// Handle arrays - recursively stabilize each element
	if (Array.isArray(value)) {
		const result = value.map((item) => stabilizeStructure(item, visited));
		visited.delete(value); // Clean up after processing
		return result;
	}

	// Handle objects - sort keys and recursively stabilize values
	const keys = Object.keys(value).sort();
	const stable: Record<string, unknown> = {};

	// Add sorted keys with stabilized values
	for (const key of keys) {
		stable[key] = stabilizeStructure((value as Record<string, unknown>)[key], visited);
	}

	visited.delete(value); // Clean up after processing
	return stable;
}
