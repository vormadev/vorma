// @vitest-environment jsdom

import { Effect } from "effect";
import { beforeEach, describe, expect, it } from "vitest";
import { SCROLL_STORAGE_KEY, SCROLL_STORAGE_RELOAD_KEY } from "./constants.ts";
import {
	WINDOW_EVENT_BEFOREUNLOAD,
	make_runtime_lifecycle,
} from "./effect_runtime/runtime_lifecycle.ts";
import {
	REFRESH_SCROLL_MAX_AGE_MS,
	SCROLL_ENTRY_LIMIT,
	make_scroll_restoration,
} from "./effect_runtime/scroll_restoration.ts";

const FIRST_KEY = "first-key";
const SECOND_KEY = "second-key";
const FIRST_HREF = "http://localhost/first";
const SECOND_HREF = "http://localhost/second";

function run_effect<A, E>(program: Effect.Effect<A, E, never>): Promise<A> {
	return Effect.runPromise(program);
}

function set_scroll_position(x: number, y: number): void {
	Object.defineProperty(window, "scrollX", {
		value: x,
		writable: true,
		configurable: true,
	});
	Object.defineProperty(window, "scrollY", {
		value: y,
		writable: true,
		configurable: true,
	});
}

describe("ccc Effect scroll restoration experiment", () => {
	beforeEach(() => {
		window.history.replaceState({}, "", "/");
		sessionStorage.clear();
		set_scroll_position(0, 0);
	});

	it("saves the current scroll position by history key", async () => {
		const result = await run_effect(
			Effect.gen(function* () {
				const scroll_restoration = yield* make_scroll_restoration();
				set_scroll_position(12, 34);
				yield* scroll_restoration.save_current({
					href: FIRST_HREF,
					key: FIRST_KEY,
					state: undefined,
				});
				return yield* scroll_restoration.popstate_scroll({
					href: FIRST_HREF,
					key: FIRST_KEY,
					state: undefined,
				});
			}),
		);

		expect(result).toEqual({ x: 12, y: 34 });
		expect(sessionStorage.getItem(SCROLL_STORAGE_KEY)).toContain(FIRST_KEY);
	});

	it("keeps only the latest bounded scroll entries", async () => {
		const result = await run_effect(
			Effect.gen(function* () {
				const scroll_restoration = yield* make_scroll_restoration();
				for (let i = 0; i < SCROLL_ENTRY_LIMIT + 1; i++) {
					set_scroll_position(i, i + 1);
					yield* scroll_restoration.save_current({
						href: `http://localhost/${i}`,
						key: `key-${i}`,
						state: undefined,
					});
				}
				const raw = sessionStorage.getItem(SCROLL_STORAGE_KEY);
				return raw ? JSON.parse(raw) : [];
			}),
		);

		expect(result).toHaveLength(SCROLL_ENTRY_LIMIT);
		expect(result[0][0]).toBe("key-1");
	});

	it("prefers fresh reload scroll on boot and consumes the reload entry", async () => {
		sessionStorage.setItem(
			SCROLL_STORAGE_RELOAD_KEY,
			JSON.stringify({
				x: 55,
				y: 77,
				unix: Date.now(),
				href: FIRST_HREF,
			}),
		);

		const result = await run_effect(
			Effect.gen(function* () {
				const scroll_restoration = yield* make_scroll_restoration();
				return yield* scroll_restoration.boot_scroll({
					href: `${FIRST_HREF}#ignored-while-reload-is-fresh`,
					key: FIRST_KEY,
					state: undefined,
				});
			}),
		);

		expect(result).toEqual({ x: 55, y: 77 });
		expect(sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY)).toBeNull();
	});

	it("falls back to hash and top scroll intents", async () => {
		const result = await run_effect(
			Effect.gen(function* () {
				const scroll_restoration = yield* make_scroll_restoration();
				const boot = yield* scroll_restoration.boot_scroll({
					href: `${FIRST_HREF}#section`,
					key: FIRST_KEY,
					state: undefined,
				});
				const navigation = yield* scroll_restoration.navigation_scroll(
					SECOND_HREF,
					undefined,
				);
				const popstate = yield* scroll_restoration.popstate_scroll({
					href: SECOND_HREF,
					key: SECOND_KEY,
					state: undefined,
				});
				return { boot, navigation, popstate };
			}),
		);

		expect(result).toEqual({
			boot: { hash: "#section" },
			navigation: { x: 0, y: 0 },
			popstate: { x: 0, y: 0 },
		});
	});

	it("ignores stale reload scroll", async () => {
		sessionStorage.setItem(
			SCROLL_STORAGE_RELOAD_KEY,
			JSON.stringify({
				x: 55,
				y: 77,
				unix: Date.now() - REFRESH_SCROLL_MAX_AGE_MS - 1,
				href: FIRST_HREF,
			}),
		);

		const result = await run_effect(
			Effect.gen(function* () {
				const scroll_restoration = yield* make_scroll_restoration();
				return yield* scroll_restoration.boot_scroll({
					href: FIRST_HREF,
					key: FIRST_KEY,
					state: undefined,
				});
			}),
		);

		expect(result).toBeUndefined();
	});

	it("owns the reload scroll saver listener through lifecycle shutdown", async () => {
		const lifecycle = Effect.runSync(make_runtime_lifecycle());
		const scroll_restoration = Effect.runSync(make_scroll_restoration());

		await run_effect(
			scroll_restoration.install_reload_scroll_saver(lifecycle),
		);
		set_scroll_position(13, 17);
		window.dispatchEvent(new Event(WINDOW_EVENT_BEFOREUNLOAD));
		expect(sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY)).toContain(
			"13",
		);

		sessionStorage.removeItem(SCROLL_STORAGE_RELOAD_KEY);
		await run_effect(lifecycle.shutdown);
		set_scroll_position(19, 23);
		window.dispatchEvent(new Event(WINDOW_EVENT_BEFOREUNLOAD));
		expect(sessionStorage.getItem(SCROLL_STORAGE_RELOAD_KEY)).toBeNull();
	});
});
