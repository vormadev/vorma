/**
 * Serialize a plain JSON-compatible value into `URLSearchParams`, the
 * inverse direction of {@link parseSearchParams}: nested objects flatten to
 * `.`-joined dotted keys, arrays repeat the same key once per element (an
 * empty array/object serializes as one empty-string entry so its presence
 * survives a round trip), `null`/`undefined` become an empty string, and
 * every key at every nesting level sorts alphabetically for a
 * deterministic, cache-friendly query string. Vorma's typed navigation and
 * `apiClient` GET/HEAD calls use this internally to build query strings
 * from typed `search`/`input`; reach for it directly only outside that
 * typed surface.
 */
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
