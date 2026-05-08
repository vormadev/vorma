// @vitest-environment jsdom

import { Effect, Result as EffectResult } from "effect";
import { afterEach, beforeEach, describe, expect, it } from "vitest";
import {
	ModulePreloadFailed,
	preload_modules,
	preload_modules_effect,
} from "./modules.ts";

let original_dev: boolean;

beforeEach(() => {
	document.head.innerHTML = "";
	original_dev = import.meta.env.DEV;
});

afterEach(() => {
	import.meta.env.DEV = original_dev;
});

describe("preload_modules", () => {
	it("creates modulepreload link elements", () => {
		import.meta.env.DEV = false;

		preload_modules(["/chunk-a.js"]);

		const link = document.head.querySelector(
			'link[rel="modulepreload"][href="/chunk-a.js"]',
		) as HTMLLinkElement;
		expect(link).not.toBeNull();
		expect(link.rel).toBe("modulepreload");
	});

	it("creates multiple modulepreload links", () => {
		import.meta.env.DEV = false;

		preload_modules(["/a.js", "/b.js", "/c.js"]);

		const links = document.head.querySelectorAll(
			'link[rel="modulepreload"]',
		);
		expect(links).toHaveLength(3);
	});

	it("deduplicates same path within a single call", () => {
		import.meta.env.DEV = false;

		preload_modules(["/a.js", "/a.js"]);

		const links = document.head.querySelectorAll(
			'link[rel="modulepreload"][href="/a.js"]',
		);
		expect(links).toHaveLength(1);
	});

	it("deduplicates across multiple calls", () => {
		import.meta.env.DEV = false;

		preload_modules(["/a.js"]);
		preload_modules(["/a.js"]);

		const links = document.head.querySelectorAll(
			'link[rel="modulepreload"][href="/a.js"]',
		);
		expect(links).toHaveLength(1);
	});

	it("skips in dev mode", () => {
		import.meta.env.DEV = true;

		preload_modules(["/a.js", "/b.js"]);

		const links = document.head.querySelectorAll(
			'link[rel="modulepreload"]',
		);
		expect(links).toHaveLength(0);
	});

	it("tags module preload DOM failures", () => {
		import.meta.env.DEV = false;

		const result = Effect.runSync(
			Effect.result(preload_modules_effect(['/bad"quote.js'])),
		);

		expect(EffectResult.isFailure(result)).toBe(true);
		if (EffectResult.isFailure(result)) {
			expect(result.failure).toBeInstanceOf(ModulePreloadFailed);
		}
	});
});
