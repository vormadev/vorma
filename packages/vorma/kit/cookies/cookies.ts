/**
 * Checks the client cookie for a specific name. Returns the value if
 * found, otherwise undefined. Does not do any encoding or decoding.
 */
export function getClientCookie(name: string) {
	const expected_prefix = `${name}=`;
	const cookie_pairs = document.cookie.split(";");
	for (const cookie_pair of cookie_pairs) {
		const trimmed_pair = cookie_pair.trim();
		if (trimmed_pair.startsWith(expected_prefix)) {
			return trimmed_pair.slice(expected_prefix.length);
		}
	}
	return undefined;
}

/**
 * Sets a client cookie with the specified name and value. The cookie
 * is set to expire in one year and is accessible to all paths on the
 * domain. The SameSite attribute is set to Lax. Does not do any
 * encoding or decoding.
 */
export function setClientCookie(name: string, value: string) {
	document.cookie = `${name}=${value}; path=/; max-age=31536000; SameSite=Lax`;
}
