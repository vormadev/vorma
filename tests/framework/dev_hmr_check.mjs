import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "playwright";

const hmr_panel_selector = "[data-bmb-hmr-panel]";
const hmr_marker_selector = "[data-bmb-hmr-marker]";
const hmr_state_selector = "[data-bmb-hmr-state]";
const hmr_action_selector = "[data-bmb-hmr-action]";
const hmr_panel_count_tracker_key = "__bombadil_hmr_panel_count_tracker";
const hmr_panel_observer_key = "__bombadil_hmr_panel_observer";
const state_modified = "modified";
const hmr_timeout_ms = 60_000;
const known_variants = new Set(["preact", "react", "solid"]);

const [, , base_url, variant, source_path, marker_before, marker_after] = process.argv;

if (!base_url || !variant || !source_path || !marker_before || !marker_after) {
	throw new Error(
		"usage: node dev_hmr_check.mjs <base-url> <variant> <source-path> <before> <after>",
	);
}

if (!known_variants.has(variant)) {
	throw new Error(`unknown variant ${JSON.stringify(variant)}`);
}

const absolute_source_path = path.resolve(source_path);
const original_source = await fs.readFile(absolute_source_path, "utf8");
if (!original_source.includes(marker_before)) {
	throw new Error(`${source_path} does not contain ${marker_before}`);
}
if (original_source.includes(marker_after)) {
	throw new Error(`${source_path} already contains ${marker_after}`);
}

let browser;
try {
	browser = await chromium.launch({ headless: true });
} catch (error) {
	const error_message = String(error?.message ?? error);
	if (!error_message.includes("Executable doesn't exist")) {
		throw error;
	}
	browser = await chromium.launch({ channel: "chrome", headless: true });
}
let main_frame_navigations = 0;

try {
	const page = await browser.newPage();
	await page.goto(base_url);
	await assert_single_hmr_panel(page, "initial render");
	await install_hmr_panel_count_tracker(page);
	await page.waitForFunction(
		([selector, marker]) => {
			return document.querySelector(selector)?.textContent === marker;
		},
		[hmr_marker_selector, marker_before],
		{ timeout: hmr_timeout_ms },
	);

	page.on("framenavigated", (frame) => {
		if (frame === page.mainFrame()) {
			main_frame_navigations++;
		}
	});

	await page.click(hmr_action_selector);
	await page.waitForFunction(
		([selector, state]) => {
			return document.querySelector(selector)?.textContent === state;
		},
		[hmr_state_selector, state_modified],
		{ timeout: hmr_timeout_ms },
	);

	await fs.writeFile(
		absolute_source_path,
		original_source.replace(marker_before, marker_after),
	);

	await page.waitForFunction(
		([selector, marker]) => {
			return document.querySelector(selector)?.textContent === marker;
		},
		[hmr_marker_selector, marker_after],
		{ timeout: hmr_timeout_ms },
	);

	if (main_frame_navigations !== 0) {
		throw new Error(
			`expected HMR without main-frame navigation; observed ${main_frame_navigations}`,
		);
	}

	const actual_state = await page.textContent(hmr_state_selector);
	if (actual_state !== state_modified) {
		throw new Error(
			`expected ${variant} HMR state ${state_modified}, got ${actual_state}`,
		);
	}
	await assert_single_hmr_panel(page, "after HMR");
	const max_hmr_panel_count = await page.evaluate((tracker_key) => {
		return globalThis[tracker_key]?.max_count;
	}, hmr_panel_count_tracker_key);
	if (max_hmr_panel_count !== 1) {
		throw new Error(
			`expected HMR to keep exactly one active view panel; observed max ${max_hmr_panel_count}`,
		);
	}
} finally {
	if (browser) {
		await browser.close();
	}
	await fs.writeFile(absolute_source_path, original_source);
}

async function assert_single_hmr_panel(page, phase) {
	await page.waitForFunction(
		([selector]) => {
			return document.querySelectorAll(selector).length === 1;
		},
		[hmr_panel_selector],
		{ timeout: hmr_timeout_ms },
	);
	const count = await page.locator(hmr_panel_selector).count();
	if (count !== 1) {
		throw new Error(`expected one HMR panel during ${phase}; got ${count}`);
	}
}

async function install_hmr_panel_count_tracker(page) {
	await page.evaluate(
		([selector, tracker_key, observer_key]) => {
			const count_panels = () => {
				return document.querySelectorAll(selector).length;
			};
			const tracker = {
				current_count: count_panels(),
				max_count: count_panels(),
			};
			const observer = new MutationObserver(() => {
				tracker.current_count = count_panels();
				tracker.max_count = Math.max(tracker.max_count, tracker.current_count);
			});
			observer.observe(document.documentElement, {
				childList: true,
				subtree: true,
			});
			globalThis[tracker_key] = tracker;
			globalThis[observer_key] = observer;
		},
		[hmr_panel_selector, hmr_panel_count_tracker_key, hmr_panel_observer_key],
	);
}
