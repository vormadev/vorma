/*
Formatting helpers are plain app code. Vorma does not try to own display
choices like relative timestamps or hostname cleanup; keep those close to
the product surface that needs them.
*/

const units: Array<[Intl.RelativeTimeFormatUnit, number]> = [
	["year", 365 * 24 * 3600],
	["month", 30 * 24 * 3600],
	["day", 24 * 3600],
	["hour", 3600],
	["minute", 60],
];

const relative_time_format = new Intl.RelativeTimeFormat("en", { numeric: "always" });

export function time_ago(unix_seconds: number): string {
	const delta = Math.floor(Date.now() / 1000) - unix_seconds;
	for (const [unit, seconds] of units) {
		if (delta >= seconds) {
			return relative_time_format.format(-Math.floor(delta / seconds), unit);
		}
	}
	return "just now";
}

export function domain_of(url: string): string | null {
	try {
		return new URL(url).hostname.replace(/^www\./, "");
	} catch {
		return null;
	}
}
