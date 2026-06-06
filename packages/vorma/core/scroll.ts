import { SCROLL_STORAGE_KEY } from "./constants.ts";

export const MAX_SCROLL_ENTRIES = 50;

export type ScrollState = { x: number; y: number } | { hash: string };

export type ScrollIntent = {
	scroll: ScrollState;
	target_entry_id: string;
};

type ScrollTestOptions = {
	scroll_to?: (x: number, y: number) => void;
};

type StoredScrollEntry = [string, { x: number; y: number }];

export function apply_scroll(
	scroll: ScrollState | undefined,
	test_options?: ScrollTestOptions,
): void {
	if (!scroll) {
		return;
	}
	if ("hash" in scroll) {
		const raw = scroll.hash.startsWith("#") ? scroll.hash.slice(1) : scroll.hash;
		let id: string;
		try {
			id = decodeURIComponent(raw);
		} catch {
			id = raw;
		}
		document.getElementById(id)?.scrollIntoView();
		return;
	}
	const scroll_to =
		test_options?.scroll_to ??
		((x: number, y: number) => {
			window.scrollTo(x, y);
		});
	scroll_to(scroll.x, scroll.y);
}

export function get_scroll_pos(): { x: number; y: number } {
	return { x: window.scrollX, y: window.scrollY };
}

function read_scroll_entries(): StoredScrollEntry[] {
	try {
		const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
		if (!raw) {
			return [];
		}
		const parsed = JSON.parse(raw);
		if (!Array.isArray(parsed)) {
			return [];
		}
		return parsed.filter((e: unknown): e is StoredScrollEntry => {
			return (
				Array.isArray(e) &&
				e.length === 2 &&
				typeof e[0] === "string" &&
				!!e[1] &&
				typeof e[1] === "object" &&
				Number.isFinite((e[1] as any).x) &&
				Number.isFinite((e[1] as any).y)
			);
		});
	} catch {
		return [];
	}
}

export function save_scroll_for_key(key: string, pos: { x: number; y: number }): void {
	const entries = read_scroll_entries().filter((e) => e[0] !== key);
	entries.push([key, pos]);
	if (entries.length > MAX_SCROLL_ENTRIES) {
		entries.splice(0, entries.length - MAX_SCROLL_ENTRIES);
	}
	try {
		sessionStorage.setItem(SCROLL_STORAGE_KEY, JSON.stringify(entries));
	} catch {}
}

export function get_scroll_for_key(key: string): { x: number; y: number } | undefined {
	for (const [k, v] of read_scroll_entries()) {
		if (k === key) {
			return v;
		}
	}
	return undefined;
}
