/**
 * Checks the client cookie for a specific name. Returns the value if
 * found, otherwise undefined. Does not do any encoding or decoding.
 */
export function getClientCookie(name: string) {
	const expectedPrefix = `${name}=`;
	const cookiePairs = document.cookie.split(";");
	for (const cookiePair of cookiePairs) {
		const trimmedPair = cookiePair.trim();
		if (trimmedPair.startsWith(expectedPrefix)) {
			return trimmedPair.slice(expectedPrefix.length);
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
