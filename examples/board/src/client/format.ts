/*
App-owned formatting on platform APIs — deliberately NOT a framework
concern (and kit/* is out of scope for this app by ruling).
*/

const UNITS: Array<[Intl.RelativeTimeFormatUnit, number]> = [
	["year", 365 * 24 * 3600],
	["month", 30 * 24 * 3600],
	["day", 24 * 3600],
	["hour", 3600],
	["minute", 60],
];

const relative = new Intl.RelativeTimeFormat("en", { numeric: "always" });

export function timeAgo(unixSeconds: number): string {
	const delta = Math.floor(Date.now() / 1000) - unixSeconds;
	for (const [unit, seconds] of UNITS) {
		if (delta >= seconds) {
			return relative.format(-Math.floor(delta / seconds), unit);
		}
	}
	return "just now";
}

export function domainOf(url: string): string | null {
	try {
		return new URL(url).hostname.replace(/^www\./, "");
	} catch {
		return null;
	}
}
