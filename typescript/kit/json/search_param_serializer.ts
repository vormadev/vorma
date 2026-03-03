export function serializeToSearchParams(obj: unknown): URLSearchParams {
	const params = new URLSearchParams();

	function appendValue(key: string, value: unknown) {
		if (value === null || value === undefined) {
			params.append(key, "");
			return;
		}

		if (Array.isArray(value)) {
			if (value.length === 0) {
				params.append(key, "");
			} else {
				for (const item of value) {
					appendValue(key, item);
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
				entries.sort(([keyA], [keyB]) => keyA.localeCompare(keyB));
				for (const [subKey, subValue] of entries) {
					const newKey = key ? `${key}.${subKey}` : subKey;
					appendValue(newKey, subValue);
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
		entries.sort(([keyA], [keyB]) => keyA.localeCompare(keyB));
		for (const [key, value] of entries) {
			appendValue(key, value);
		}
	}

	return params;
}
