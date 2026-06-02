/**
 * Performs a deep equality comparison of two JSON-compatible values.
 * Handles null, undefined, primitives, arrays, and plain objects, recursively.
 * Does not support Maps, Sets, Functions, or other non-JSON types.
 */
export function jsonDeepEquals(a: unknown, b: unknown): boolean {
	// Tautology
	if (a === b) {
		return true;
	}

	// If both were null or both were undefined, we would have early returned above.
	// So if either at this point is loosely null, we know they're not equal.
	if (a == null || b == null) {
		return false;
	}

	// If types are different, we know they're not equal.
	if (typeof a !== typeof b) {
		return false;
	}

	const a_is_array = Array.isArray(a);
	const b_is_array = Array.isArray(b);

	// If one is an array and the other is not, we know they're not equal.
	if (a_is_array !== b_is_array) {
		return false;
	}

	// Handle arrays
	if (a_is_array && b_is_array) {
		const left_array = a as unknown[];
		const right_array = b as unknown[];
		if (left_array.length !== right_array.length) {
			return false;
		}
		return left_array.every((item, index) =>
			jsonDeepEquals(item, right_array[index]),
		);
	}

	// Handle objects
	if (typeof a === "object" && typeof b === "object") {
		const left_object = a as Record<string, unknown>;
		const right_object = b as Record<string, unknown>;
		const a_keys = Object.keys(left_object);
		const b_keys = Object.keys(right_object);

		if (a_keys.length !== b_keys.length) {
			return false;
		}

		return a_keys.every((key) => {
			return (
				Object.hasOwn(right_object, key) &&
				jsonDeepEquals(left_object[key], right_object[key])
			);
		});
	}

	return false;
}
