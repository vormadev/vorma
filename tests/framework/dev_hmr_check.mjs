import fs from "node:fs/promises";
import path from "node:path";
import { chromium } from "playwright";

const hmr_panel_selector = "[data-bmb-hmr-panel]";
const hmr_marker_selector = "[data-bmb-hmr-marker]";
const hmr_state_selector = "[data-bmb-hmr-state]";
const hmr_action_selector = "[data-bmb-hmr-action]";
const critical_css_probe_selector = "[data-bmb-public-url-probe]";
const rebuilding_overlay_selector = "#vorma-rebuilding-overlay";
const hmr_panel_count_tracker_key = "__bombadil_hmr_panel_count_tracker";
const hmr_panel_observer_key = "__bombadil_hmr_panel_observer";
const rebuilding_overlay_tracker_key = "__bombadil_rebuilding_overlay_tracker";
const rebuilding_overlay_observer_key = "__bombadil_rebuilding_overlay_observer";
const hmr_debug_key = "__bombadil_hmr_debug";
const state_modified = "modified";
const critical_css_probe_value = "1";
const critical_css_timeout_ms = 20_000;
const hmr_timeout_ms = 60_000;
const browser_event_limit = 80;
const diagnostic_text_limit = 4_000;
const known_variants = new Set(["preact", "react", "solid"]);

const [
	,
	,
	base_url,
	variant,
	source_path,
	marker_before,
	marker_after,
	critical_css_path,
	critical_css_marker,
] = process.argv;

if (
	!base_url ||
	!variant ||
	!source_path ||
	!marker_before ||
	!marker_after ||
	!critical_css_path ||
	!critical_css_marker
) {
	throw new Error(
		"usage: node dev_hmr_check.mjs <base-url> <variant> <source-path> <before> <after> <critical-css-path> <critical-css-marker>",
	);
}

if (!known_variants.has(variant)) {
	throw new Error(`unknown variant ${JSON.stringify(variant)}`);
}

const absolute_source_path = path.resolve(source_path);
const original_source = await fs.readFile(absolute_source_path, "utf8");
const absolute_critical_css_path = path.resolve(critical_css_path);
const original_critical_css = await fs.readFile(absolute_critical_css_path, "utf8");
if (!original_source.includes(marker_before)) {
	throw new Error(`${source_path} does not contain ${marker_before}`);
}
if (original_source.includes(marker_after)) {
	throw new Error(`${source_path} already contains ${marker_after}`);
}
if (original_critical_css.includes(critical_css_marker)) {
	throw new Error(`${critical_css_path} already contains ${critical_css_marker}`);
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
const main_frame_navigation_counter = {
	active: false,
	count: 0,
};
const browser_events = [];

try {
	const page = await browser.newPage();
	install_browser_event_recorders(page, browser_events);
	await install_hmr_debug_hook(page);
	await page.goto(base_url);
	await assert_single_hmr_panel(page, browser_events, "initial render");
	await install_hmr_panel_count_tracker(page);
	await wait_for_page_condition(
		page,
		browser_events,
		"initial HMR marker",
		([selector, marker]) => {
			return document.querySelector(selector)?.textContent === marker;
		},
		[hmr_marker_selector, marker_before],
	);
	await wait_for_no_rebuilding_overlay(page, browser_events);
	await install_rebuilding_overlay_tracker(page);

	page.on("framenavigated", (frame) => {
		if (frame === page.mainFrame()) {
			if (main_frame_navigation_counter.active) {
				main_frame_navigation_counter.count++;
			}
		}
	});

	main_frame_navigation_counter.active = true;
	await fs.writeFile(
		absolute_critical_css_path,
		`${original_critical_css}\n${critical_css_probe_selector} { --${critical_css_marker}: ${critical_css_probe_value}; outline-color: rgb(1, 2, 3); outline-style: solid; outline-width: 3px; }\n`,
	);
	await wait_for_page_condition(
		page,
		browser_events,
		"critical CSS hot apply",
		([selector, marker, value]) => {
			const element = document.querySelector(selector);
			if (!element) {
				return false;
			}
			return (
				getComputedStyle(element).getPropertyValue(`--${marker}`).trim() === value
			);
		},
		[critical_css_probe_selector, critical_css_marker, critical_css_probe_value],
		critical_css_timeout_ms,
	);
	await assert_no_rebuilding_overlay_seen(page, browser_events, "critical CSS update");
	if (main_frame_navigation_counter.count !== 0) {
		throw new Error(
			"expected critical CSS update without main-frame navigation; observed " +
				main_frame_navigation_counter.count +
				"\n" +
				(await capture_page_diagnostics(page, browser_events)),
		);
	}

	try {
		await page.click(hmr_action_selector, { timeout: hmr_timeout_ms });
	} catch (error) {
		throw new Error(
			`click HMR action failed: ${format_error_value(error)}\n${await capture_page_diagnostics(page, browser_events)}`,
		);
	}
	await wait_for_page_condition(
		page,
		browser_events,
		"HMR state mutation",
		([selector, state]) => {
			return document.querySelector(selector)?.textContent === state;
		},
		[hmr_state_selector, state_modified],
	);

	await fs.writeFile(
		absolute_source_path,
		original_source.replace(marker_before, marker_after),
	);

	await wait_for_page_condition(
		page,
		browser_events,
		"updated HMR marker",
		([selector, marker]) => {
			return document.querySelector(selector)?.textContent === marker;
		},
		[hmr_marker_selector, marker_after],
	);

	if (main_frame_navigation_counter.count !== 0) {
		throw new Error(
			"expected HMR without main-frame navigation; observed " +
				main_frame_navigation_counter.count +
				"\n" +
				(await capture_page_diagnostics(page, browser_events)),
		);
	}

	const actual_state = await page.textContent(hmr_state_selector);
	if (actual_state !== state_modified) {
		throw new Error(
			`expected ${variant} HMR state ${state_modified}, got ${actual_state}\n${await capture_page_diagnostics(page, browser_events)}`,
		);
	}
	await assert_single_hmr_panel(page, browser_events, "after HMR");
	const max_hmr_panel_count = await page.evaluate((tracker_key) => {
		return globalThis[tracker_key]?.max_count;
	}, hmr_panel_count_tracker_key);
	if (max_hmr_panel_count !== 1) {
		throw new Error(
			`expected HMR to keep exactly one active view panel; observed max ${max_hmr_panel_count}\n${await capture_page_diagnostics(page, browser_events)}`,
		);
	}
} finally {
	if (browser) {
		await browser.close();
	}
	await fs.writeFile(absolute_source_path, original_source);
	await fs.writeFile(absolute_critical_css_path, original_critical_css);
}

function install_browser_event_recorders(page, browser_events) {
	page.on("console", (message) => {
		record_browser_event(browser_events, {
			kind: "console",
			type: message.type(),
			text: truncate_text(message.text(), diagnostic_text_limit),
			location: message.location(),
		});
	});
	page.on("pageerror", (error) => {
		record_browser_event(browser_events, {
			kind: "pageerror",
			error: format_error_value(error),
		});
	});
	page.on("requestfailed", (request) => {
		record_browser_event(browser_events, {
			kind: "requestfailed",
			method: request.method(),
			resource_type: request.resourceType(),
			url: request.url(),
			failure: request.failure()?.errorText ?? null,
		});
	});
	page.on("response", (response) => {
		if (response.status() < 400) {
			return;
		}
		record_browser_event(browser_events, {
			kind: "response",
			status: response.status(),
			url: response.url(),
		});
	});
	page.on("framenavigated", (frame) => {
		record_browser_event(browser_events, {
			kind: "framenavigated",
			is_main_frame: frame === page.mainFrame(),
			url: frame.url(),
		});
	});
}

async function install_hmr_debug_hook(page) {
	await page.addInitScript(
		([debug_key, marker_selector]) => {
			const debug = {
				assignments: 0,
				assigned_type: null,
				hook_calls: [],
			};
			Object.defineProperty(globalThis, debug_key, {
				value: debug,
				configurable: true,
			});
			let current_value;
			Object.defineProperty(globalThis, "__vorma_hmr_view_update", {
				configurable: true,
				get() {
					return current_value;
				},
				set(next_value) {
					debug.assignments++;
					debug.assigned_type = typeof next_value;
					if (typeof next_value !== "function") {
						current_value = next_value;
						return;
					}
					current_value = async (raw_url, mod) => {
						const call = {
							raw_url,
							module_keys:
								mod && typeof mod === "object"
									? Object.keys(mod).sort()
									: [],
							marker_before:
								document.querySelector(marker_selector)?.textContent ??
								null,
							completed: false,
							error: null,
							marker_after: null,
							stack: new Error().stack ?? null,
						};
						debug.hook_calls.push(call);
						try {
							const result = await next_value(raw_url, mod);
							call.completed = true;
							call.marker_after =
								document.querySelector(marker_selector)?.textContent ??
								null;
							return result;
						} catch (error) {
							call.error =
								error instanceof Error
									? (error.stack ?? error.message)
									: String(error);
							throw error;
						}
					};
				},
			});
		},
		[hmr_debug_key, hmr_marker_selector],
	);
}

async function assert_single_hmr_panel(page, browser_events, phase) {
	await wait_for_page_condition(
		page,
		browser_events,
		`one HMR panel during ${phase}`,
		([selector]) => {
			return document.querySelectorAll(selector).length === 1;
		},
		[hmr_panel_selector],
	);
	const count = await page.locator(hmr_panel_selector).count();
	if (count !== 1) {
		throw new Error(
			`expected one HMR panel during ${phase}; got ${count}\n${await capture_page_diagnostics(page, browser_events)}`,
		);
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

async function wait_for_no_rebuilding_overlay(page, browser_events) {
	await wait_for_page_condition(
		page,
		browser_events,
		"rebuilding overlay removal",
		([selector]) => {
			return !document.querySelector(selector);
		},
		[rebuilding_overlay_selector],
	);
}

async function install_rebuilding_overlay_tracker(page) {
	await page.evaluate(
		([selector, tracker_key, observer_key]) => {
			const overlay_visible = () => {
				return document.querySelector(selector) !== null;
			};
			const tracker = {
				currently_visible: overlay_visible(),
				observed: overlay_visible(),
			};
			const observer = new MutationObserver(() => {
				tracker.currently_visible = overlay_visible();
				tracker.observed = tracker.observed || tracker.currently_visible;
			});
			observer.observe(document.documentElement, {
				childList: true,
				subtree: true,
			});
			globalThis[tracker_key] = tracker;
			globalThis[observer_key] = observer;
		},
		[
			rebuilding_overlay_selector,
			rebuilding_overlay_tracker_key,
			rebuilding_overlay_observer_key,
		],
	);
}

async function assert_no_rebuilding_overlay_seen(page, browser_events, phase) {
	const observed = await page.evaluate((tracker_key) => {
		return globalThis[tracker_key]?.observed ?? null;
	}, rebuilding_overlay_tracker_key);
	if (observed !== false) {
		throw new Error(
			`expected no rebuilding overlay during ${phase}; observed ${observed}\n${await capture_page_diagnostics(page, browser_events)}`,
		);
	}
}

async function wait_for_page_condition(
	page,
	browser_events,
	label,
	predicate,
	arg,
	timeout_ms = hmr_timeout_ms,
) {
	try {
		await page.waitForFunction(predicate, arg, { timeout: timeout_ms });
	} catch (error) {
		throw new Error(
			`${label} timed out after ${timeout_ms}ms: ${format_error_value(error)}\n${await capture_page_diagnostics(page, browser_events)}`,
		);
	}
}

async function capture_page_diagnostics(page, browser_events) {
	const page_state = await page
		.evaluate(
			([
				panel_selector,
				marker_selector,
				state_selector,
				action_selector,
				critical_css_selector,
				critical_css_marker_name,
				overlay_selector,
				panel_tracker_key,
				overlay_tracker_key,
				debug_key,
				text_limit,
			]) => {
				const text_content = (selector) => {
					return document.querySelector(selector)?.textContent ?? null;
				};
				const overlay = document.querySelector(overlay_selector);
				const critical_css_probe = document.querySelector(critical_css_selector);
				return {
					location_href: window.location.href,
					ready_state: document.readyState,
					hmr_panel_count: document.querySelectorAll(panel_selector).length,
					hmr_marker_text: text_content(marker_selector),
					hmr_state_text: text_content(state_selector),
					hmr_action_count: document.querySelectorAll(action_selector).length,
					critical_css_probe_property:
						critical_css_probe === null
							? null
							: getComputedStyle(critical_css_probe)
									.getPropertyValue(`--${critical_css_marker_name}`)
									.trim(),
					rebuilding_overlay_text:
						overlay?.textContent?.slice(0, text_limit) ?? null,
					panel_count_tracker: globalThis[panel_tracker_key] ?? null,
					rebuilding_overlay_tracker: globalThis[overlay_tracker_key] ?? null,
					hmr_debug: globalThis[debug_key] ?? null,
					body_text: document.body?.innerText?.slice(0, text_limit) ?? "",
				};
			},
			[
				hmr_panel_selector,
				hmr_marker_selector,
				hmr_state_selector,
				hmr_action_selector,
				critical_css_probe_selector,
				critical_css_marker,
				rebuilding_overlay_selector,
				hmr_panel_count_tracker_key,
				rebuilding_overlay_tracker_key,
				hmr_debug_key,
				diagnostic_text_limit,
			],
		)
		.catch((error) => {
			return {
				diagnostic_error: format_error_value(error),
			};
		});
	return JSON.stringify(
		{
			page: page_state,
			main_frame_navigations: main_frame_navigation_counter.count,
			browser_events,
		},
		null,
		"\t",
	);
}

function record_browser_event(browser_events, event) {
	browser_events.push({
		time: new Date().toISOString(),
		...event,
	});
	while (browser_events.length > browser_event_limit) {
		browser_events.shift();
	}
}

function truncate_text(value, max_length) {
	const text = String(value);
	if (text.length <= max_length) {
		return text;
	}
	return `${text.slice(0, max_length)}...`;
}

function format_error_value(error) {
	if (error instanceof Error) {
		return error.stack ?? error.message;
	}
	return String(error);
}
