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

	const aIsArray = Array.isArray(a);
	const bIsArray = Array.isArray(b);

	// If one is an array and the other is not, we know they're not equal.
	if (aIsArray !== bIsArray) {
		return false;
	}

	// Handle arrays
	if (aIsArray && bIsArray) {
		const leftArray = a as unknown[];
		const rightArray = b as unknown[];
		if (leftArray.length !== rightArray.length) {
			return false;
		}
		return leftArray.every((item, index) => jsonDeepEquals(item, rightArray[index]));
	}

	// Handle objects
	if (typeof a === "object" && typeof b === "object") {
		const leftObject = a as Record<string, unknown>;
		const rightObject = b as Record<string, unknown>;
		const aKeys = Object.keys(leftObject);
		const bKeys = Object.keys(rightObject);

		if (aKeys.length !== bKeys.length) {
			return false;
		}

		return aKeys.every((key) => {
			return (
				Object.hasOwn(rightObject, key) &&
				jsonDeepEquals(leftObject[key], rightObject[key])
			);
		});
	}

	return false;
}
