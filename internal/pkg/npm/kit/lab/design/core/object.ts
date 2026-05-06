export function compact_object<T extends object>(input: T): T {
	return Object.fromEntries(
		Object.entries(input).filter((entry): entry is [string, unknown] => {
			return entry[1] !== undefined;
		}),
	) as T;
}

function is_mergeable_token_shape(
	value: unknown,
): value is Record<string, unknown> {
	return typeof value === "object" && value !== null && !Array.isArray(value);
}

export function merge_token_shapes(left: unknown, right: unknown): unknown {
	if (left === undefined) {
		return right;
	}

	if (right === undefined) {
		return left;
	}

	if (!is_mergeable_token_shape(left) || !is_mergeable_token_shape(right)) {
		return left;
	}

	return Object.fromEntries(
		Array.from(new Set([...Object.keys(left), ...Object.keys(right)])).map(
			(key) => {
				return [key, merge_token_shapes(left[key], right[key])];
			},
		),
	);
}
