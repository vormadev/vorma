export function serializeToSearchParams(obj: unknown): URLSearchParams {
	const params = new URLSearchParams();

	function append_value(key: string, value: unknown) {
		if (value === null || value === undefined) {
			params.append(key, "");
			return;
		}

		if (Array.isArray(value)) {
			if (value.length === 0) {
				params.append(key, "");
			} else {
				for (const item of value) {
					append_value(key, item);
				}
			}
			return;
		}

		if (typeof value === "object") {
			const entries = Object.entries(value as Record<string, unknown>);
			if (entries.length === 0) {
				params.append(key, "");
			} else {
				// Sort nested keys alphabetically
				entries.sort(([key_a], [key_b]) => key_a.localeCompare(key_b));
				for (const [sub_key, sub_value] of entries) {
					const new_key = key ? `${key}.${sub_key}` : sub_key;
					append_value(new_key, sub_value);
				}
			}
			return;
		}

		// oxlint-disable-next-line no-base-to-string
		params.append(key, String(value));
	}

	if (typeof obj === "object" && obj !== null) {
		// Sort top-level keys alphabetically
		const entries = Object.entries(obj);
		entries.sort(([key_a], [key_b]) => key_a.localeCompare(key_b));
		for (const [key, value] of entries) {
			append_value(key, value);
		}
	}

	return params;
}
