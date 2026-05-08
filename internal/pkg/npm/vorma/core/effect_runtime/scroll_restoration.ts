import { Effect, Ref } from "effect";
import { SCROLL_STORAGE_KEY, SCROLL_STORAGE_RELOAD_KEY } from "../constants.ts";
import type { ScrollState } from "./client_contract.ts";
import type { HistoryPosition } from "./route_publisher.ts";
import {
	type RuntimeLifecycle,
	WINDOW_EVENT_BEFOREUNLOAD,
} from "./runtime_lifecycle.ts";

export const REFRESH_SCROLL_MAX_AGE_MS = 3333;
export const SCROLL_ENTRY_LIMIT = 50;

export type ScrollPosition = {
	x: number;
	y: number;
};

export type ScrollRestoration = {
	set_manual_restoration: Effect.Effect<void>;
	save_current: (position: HistoryPosition) => Effect.Effect<void>;
	save_reload_scroll: Effect.Effect<void>;
	install_reload_scroll_saver: (
		lifecycle: RuntimeLifecycle,
	) => Effect.Effect<void>;
	boot_scroll: (
		position: HistoryPosition,
	) => Effect.Effect<ScrollState | undefined>;
	navigation_scroll: (
		href: string,
		scroll_to_top: boolean | undefined,
	) => Effect.Effect<ScrollState | undefined>;
	popstate_scroll: (position: HistoryPosition) => Effect.Effect<ScrollState>;
};

type StoredScrollEntry = [string, ScrollPosition];

type ReloadScrollEntry = ScrollPosition & {
	href: string;
	unix: number;
};

export function make_scroll_restoration(): Effect.Effect<
	ScrollRestoration,
	never
> {
	return Effect.gen(function* () {
		const entries_ref = yield* Ref.make(new Map(read_scroll_entries()));
		const save_reload_scroll = Effect.sync(save_reload_scroll_now);

		return {
			set_manual_restoration: Effect.sync(() => {
				try {
					window.history.scrollRestoration = "manual";
				} catch {}
			}),
			save_current: (position) => {
				if (position.key.length === 0) {
					return Effect.void;
				}
				return Effect.gen(function* () {
					const scroll = yield* current_scroll;
					const next = new Map(read_scroll_entries());
					next.delete(position.key);
					next.set(position.key, scroll);
					trim_scroll_entries(next);
					yield* Ref.set(entries_ref, next);
					const entries = next;
					yield* write_scroll_entries(entries);
				});
			},
			save_reload_scroll,
			install_reload_scroll_saver: (lifecycle) => {
				return lifecycle.listen_window_effect(
					WINDOW_EVENT_BEFOREUNLOAD,
					() => {
						return save_reload_scroll;
					},
				);
			},
			boot_scroll: (position) => {
				return Effect.gen(function* () {
					const reload_scroll = yield* read_reload_scroll();
					if (
						reload_scroll &&
						Date.now() - reload_scroll.unix <=
							REFRESH_SCROLL_MAX_AGE_MS &&
						same_route_without_hash(
							reload_scroll.href,
							position.href,
						)
					) {
						return {
							x: reload_scroll.x,
							y: reload_scroll.y,
						};
					}
					const hash = normalized_hash(position.href);
					if (hash) {
						return { hash };
					}
					return undefined;
				});
			},
			navigation_scroll: (href, scroll_to_top) => {
				return Effect.sync(() => {
					const hash = normalized_hash(href);
					if (hash) {
						return { hash };
					}
					if (scroll_to_top === false) {
						return undefined;
					}
					return { x: 0, y: 0 };
				});
			},
			popstate_scroll: (position) => {
				return Effect.gen(function* () {
					const hash = normalized_hash(position.href);
					if (hash) {
						return { hash };
					}
					const entries = yield* Ref.get(entries_ref);
					return entries.get(position.key) ?? { x: 0, y: 0 };
				});
			},
		};
	});
}

const current_scroll = Effect.sync((): ScrollPosition => {
	return current_scroll_now();
});

function current_scroll_now(): ScrollPosition {
	return {
		x: window.scrollX,
		y: window.scrollY,
	};
}

function save_reload_scroll_now(): void {
	try {
		const entry: ReloadScrollEntry = {
			...current_scroll_now(),
			unix: Date.now(),
			href: window.location.href,
		};
		sessionStorage.setItem(
			SCROLL_STORAGE_RELOAD_KEY,
			JSON.stringify(entry),
		);
	} catch {}
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
		return parsed.filter((entry): entry is StoredScrollEntry => {
			return (
				Array.isArray(entry) &&
				entry.length === 2 &&
				typeof entry[0] === "string" &&
				is_scroll_position(entry[1])
			);
		});
	} catch {
		return [];
	}
}

function write_scroll_entries(
	entries: Map<string, ScrollPosition>,
): Effect.Effect<void> {
	return Effect.sync(() => {
		try {
			sessionStorage.setItem(
				SCROLL_STORAGE_KEY,
				JSON.stringify([...entries.entries()]),
			);
		} catch {}
	});
}

function read_reload_scroll(): Effect.Effect<ReloadScrollEntry | null> {
	return Effect.sync(() => {
		try {
			const raw = sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY);
			if (!raw) {
				return null;
			}
			sessionStorage.removeItem(SCROLL_STORAGE_RELOAD_KEY);
			const parsed = JSON.parse(raw);
			if (is_reload_scroll_entry(parsed)) {
				return parsed;
			}
			return null;
		} catch {
			return null;
		}
	});
}

function is_scroll_position(value: unknown): value is ScrollPosition {
	if (!value || typeof value !== "object") {
		return false;
	}
	const candidate = value as Record<string, unknown>;
	return (
		typeof candidate.x === "number" &&
		Number.isFinite(candidate.x) &&
		typeof candidate.y === "number" &&
		Number.isFinite(candidate.y)
	);
}

function is_reload_scroll_entry(value: unknown): value is ReloadScrollEntry {
	if (!is_scroll_position(value)) {
		return false;
	}
	const candidate = value as Record<string, unknown>;
	return (
		typeof candidate.href === "string" && typeof candidate.unix === "number"
	);
}

function trim_scroll_entries(entries: Map<string, ScrollPosition>): void {
	while (entries.size > SCROLL_ENTRY_LIMIT) {
		const key = entries.keys().next().value;
		if (typeof key !== "string") {
			return;
		}
		entries.delete(key);
	}
}

function normalized_hash(href: string): string | null {
	let hash = "";
	try {
		hash = new URL(href, window.location.href).hash;
	} catch {
		return null;
	}
	if (hash.length === 0) {
		return null;
	}
	const raw = hash.startsWith("#") ? hash.slice(1) : hash;
	if (raw.length === 0) {
		return null;
	}
	return hash;
}

function same_route_without_hash(a: string, b: string): boolean {
	try {
		const first = new URL(a, window.location.href);
		const second = new URL(b, window.location.href);
		first.hash = "";
		second.hash = "";
		return first.href === second.href;
	} catch {
		return a === b;
	}
}
