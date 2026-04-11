import { R, type Result } from "vorma/kit/result";

/**
 * Deterministically serializes a JSON-compatible value to a stable string.
 * Returns an error Result if it detects a circular reference. Does not support
 * Maps, Sets, Functions, or other non-JSON types.
 */
export function jsonStringifyStable(input: unknown): Result<string> {
	const stabilized_res = stabilizeStructure(input, new WeakSet());
	if (!stabilized_res.ok) {
		return R.err(stabilized_res.err);
	}
	const stabilized = stabilized_res.val;
	try {
		const jsonString = JSON.stringify(stabilized);
		return R.ok(jsonString);
	} catch (err) {
		return R.err(
			`Error during JSON stringification: ${
				err instanceof Error ? err.message : String(err)
			}`,
		);
	}
}

function stabilizeStructure(
	value: unknown,
	visited: WeakSet<object>,
): Result<unknown> {
	if (value === null || typeof value !== "object") {
		return R.ok(value);
	}

	if (visited.has(value)) {
		return R.err(
			"Circular reference detected during stable JSON stringification",
		);
	}
	visited.add(value);

	if (Array.isArray(value)) {
		const result: unknown[] = [];
		for (const item of value) {
			const item_res = stabilizeStructure(item, visited);
			if (!item_res.ok) {
				return R.err(item_res.err);
			}
			result.push(item_res.val);
		}
		visited.delete(value);
		return R.ok(result);
	}

	const keys = Object.keys(value).sort();
	const stable: Record<string, unknown> = {};
	for (const key of keys) {
		const val_res = stabilizeStructure(
			(value as Record<string, unknown>)[key],
			visited,
		);
		if (!val_res.ok) {
			return R.err(val_res.err);
		}
		stable[key] = val_res.val;
	}
	visited.delete(value);
	return R.ok(stable);
}
