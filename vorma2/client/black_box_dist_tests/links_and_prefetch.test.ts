// Assertions covered: 30, 31, 32

import { h, render as render_preact } from "preact";
import { act } from "preact/test-utils";
import React from "react";
import { flushSync } from "react-dom";
import { createRoot } from "react-dom/client";
import { createComponent } from "solid-js";
import { render as render_solid } from "solid-js/web";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	create_isolated_client_test_runtime,
	read_scroll_state_for_testing,
	reset_client_runtime_for_testing,
} from "vorma/testing";
import {
	create_abort_aware_fetch_recorder,
	create_deferred,
	create_deferred_fetch_call,
	create_route_data_response,
	expect_status_idle,
	load_client,
	request_input_to_url,
	wait_for_request_count,
	with_unhandled_rejection_capture,
} from "./setup.ts";

// ─── Config ──────────────────────────────────────────────────────

const TEST_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
} as const;

// ─── Solid Disposer Tracking ─────────────────────────────────────

const active_solid_disposers = new Set<() => void>();

function render_solid_tracked(
	renderer: () => unknown,
	container: Element,
): () => void {
	const dispose = render_solid(renderer as any, container as any);
	active_solid_disposers.add(dispose);
	return () => {
		dispose();
		active_solid_disposers.delete(dispose);
	};
}

// ─── Adapter Runtime ─────────────────────────────────────────────

async function init_adapter_runtime() {
	vi.resetModules();
	create_isolated_client_test_runtime();
	const client = await import("vorma/client");
	await client.initClient({ vormaAppConfig: TEST_APP_CONFIG as any });
	return client;
}

// ─── React Link Rendering ────────────────────────────────────────

function render_react_link(props: {
	typed_link: unknown;
	link_props: Record<string, unknown>;
}): { anchor: HTMLAnchorElement; cleanup: () => void } {
	const container = document.createElement("div");
	document.body.appendChild(container);
	const root = createRoot(container);
	flushSync(() => {
		root.render(
			React.createElement(props.typed_link as any, props.link_props),
		);
	});
	const anchor = container.querySelector("a");
	if (!(anchor instanceof HTMLAnchorElement)) {
		root.unmount();
		container.remove();
		throw new Error("Expected react link to render an anchor element.");
	}
	return {
		anchor,
		cleanup: () => {
			root.unmount();
			container.remove();
		},
	};
}

// ─── Setup / Teardown ────────────────────────────────────────────

beforeEach(() => {
	reset_client_runtime_for_testing();
});

afterEach(async () => {
	active_solid_disposers.forEach((dispose) => dispose());
	active_solid_disposers.clear();
});

// ─────────────────────────────────────────────────────────────────
// Assertion 30: Link Click Interception
// ─────────────────────────────────────────────────────────────────

describe("links and prefetch", () => {
	describe("click interception (assertion 30)", () => {
		describe("eligible internal clicks", () => {
			it("prevents default and navigates for eligible internal primary clicks", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						matched_patterns: ["/eligible/:id"],
						loaders_data: [{ value: "ok" }],
					}),
				);
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/eligible/:id",
						params: { id: "42" },
						children: "Eligible",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(true);
					expect(fetch_spy).toHaveBeenCalledTimes(1);
					expect(window.location.pathname).toBe("/eligible/42");
				} finally {
					cleanup();
				}
			});

			it("handles text-node click targets inside anchors", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						matched_patterns: ["/text-node/:id"],
						loaders_data: [{ value: "ok" }],
					}),
				);
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/text-node/:id",
						params: { id: "42" },
						children: "Text Node",
					},
				});

				try {
					const text_node = anchor.firstChild;
					expect(text_node).not.toBeNull();
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					text_node!.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(true);
					expect(window.location.pathname).toBe("/text-node/42");
				} finally {
					cleanup();
				}
			});

			it("runs beforeNavigate callback on internal clicks (react)", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						matched_patterns: ["/before-begin/:id"],
						loaders_data: [{ value: "ok" }],
					}),
				);
				const before_begin = vi.fn();
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/before-begin/:id",
						params: { id: "42" },
						beforeNavigate: before_begin,
						children: "Click",
					},
				});

				try {
					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(before_begin).toHaveBeenCalledTimes(1);
					expect(window.location.pathname).toBe("/before-begin/42");
				} finally {
					cleanup();
				}
			});

			it("runs beforeNavigate callback on internal clicks (preact)", async () => {
				await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						matched_patterns: ["/preact-begin/:id"],
						loaders_data: [{ value: "ok" }],
					}),
				);
				const before_begin = vi.fn();
				const typed_link = preact_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const container = document.createElement("div");
				document.body.appendChild(container);

				try {
					await act(async () => {
						render_preact(
							h(typed_link as any, {
								pattern: "/preact-begin/:id",
								params: { id: "42" },
								beforeNavigate: before_begin,
								children: "Click",
							}),
							container,
						);
					});
					const anchor = container.querySelector("a");
					expect(anchor).not.toBeNull();
					anchor!.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(before_begin).toHaveBeenCalledTimes(1);
					expect(window.location.pathname).toBe("/preact-begin/42");
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("runs beforeNavigate callback on internal clicks (solid)", async () => {
				await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						matched_patterns: ["/solid-begin/:id"],
						loaders_data: [{ value: "ok" }],
					}),
				);
				const before_begin = vi.fn();
				const typed_link = solid_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return (typed_link as any)({
						pattern: "/solid-begin/:id",
						params: { id: "42" },
						beforeNavigate: before_begin,
						children: "Click",
					});
				}, container);

				try {
					const anchor = container.querySelector("a");
					expect(anchor).not.toBeNull();
					anchor!.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(before_begin).toHaveBeenCalledTimes(1);
					expect(window.location.pathname).toBe("/solid-begin/42");
				} finally {
					dispose();
					container.remove();
				}
			});
		});

		describe("fallthrough clicks", () => {
			it("does not intercept modified clicks (react)", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/modified/:id",
						params: { id: "42" },
						children: "Modified",
					},
				});

				try {
					anchor.addEventListener(
						"click",
						(e) => e.preventDefault(),
						{
							once: true,
						},
					);
					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
							metaKey: true,
						}),
					);
					await vi.runAllTimersAsync();

					expect(fetch_spy).not.toHaveBeenCalled();
					expect(window.location.pathname).toBe("/");
				} finally {
					cleanup();
				}
			});

			it("does not intercept modified clicks (preact)", async () => {
				await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = preact_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const container = document.createElement("div");
				document.body.appendChild(container);

				try {
					await act(async () => {
						render_preact(
							h(typed_link as any, {
								pattern: "/preact-modified/:id",
								params: { id: "42" },
								children: "Modified",
							}),
							container,
						);
					});
					const anchor = container.querySelector("a");
					anchor?.addEventListener(
						"click",
						(e) => e.preventDefault(),
						{
							once: true,
						},
					);
					anchor?.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							metaKey: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("does not intercept modified clicks (solid)", async () => {
				await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = solid_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return (typed_link as any)({
						pattern: "/solid-modified/:id",
						params: { id: "42" },
						children: "Modified",
					});
				}, container);

				try {
					const anchor = container.querySelector("a");
					anchor?.addEventListener(
						"click",
						(e) => e.preventDefault(),
						{
							once: true,
						},
					);
					anchor?.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							metaKey: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					dispose();
					container.remove();
				}
			});

			it("does not intercept non-primary button clicks", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/secondary/:id",
						params: { id: "42" },
						children: "Secondary",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 1,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(false);
					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});

			it("does not intercept target=_blank clicks (react)", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/new-tab/:id",
						params: { id: "42" },
						target: "_blank",
						children: "New Tab",
					},
				});

				try {
					anchor.addEventListener(
						"click",
						(e) => e.preventDefault(),
						{
							once: true,
						},
					);
					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});

			it("does not intercept target=_blank clicks (preact)", async () => {
				await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = preact_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const container = document.createElement("div");
				document.body.appendChild(container);

				try {
					await act(async () => {
						render_preact(
							h(typed_link as any, {
								pattern: "/preact-tab/:id",
								params: { id: "42" },
								target: "_blank",
								children: "New Tab",
							}),
							container,
						);
					});
					const anchor = container.querySelector("a");
					anchor?.addEventListener(
						"click",
						(e) => e.preventDefault(),
						{
							once: true,
						},
					);
					anchor?.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("does not intercept target=_blank clicks (solid)", async () => {
				await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = solid_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return (typed_link as any)({
						pattern: "/solid-tab/:id",
						params: { id: "42" },
						target: "_blank",
						children: "New Tab",
					});
				}, container);

				try {
					const anchor = container.querySelector("a");
					anchor?.addEventListener(
						"click",
						(e) => e.preventDefault(),
						{
							once: true,
						},
					);
					anchor?.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					dispose();
					container.remove();
				}
			});

			it("honors consumer onClick preventDefault and skips navigation", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const consumer_click = vi.fn((event: MouseEvent) => {
					event.preventDefault();
				});
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/prevent/:id",
						params: { id: "42" },
						onClick: consumer_click,
						children: "Prevent",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(consumer_click).toHaveBeenCalledTimes(1);
					expect(click.defaultPrevented).toBe(true);
					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});
		});

		describe("same-document hash and no-op clicks", () => {
			it("handles hash-only same-document clicks without fetch", async () => {
				await init_adapter_runtime();
				window.history.replaceState({}, "", "/hash-only/42");
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const before_begin = vi.fn();
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/hash-only/:id",
						params: { id: "42" },
						hash: "#details",
						beforeNavigate: before_begin,
						children: "Hash Only",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(true);
					expect(fetch_spy).not.toHaveBeenCalled();
					expect(window.location.pathname).toBe("/hash-only/42");
					expect(window.location.hash).toBe("#details");
					expect(before_begin).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});

			it("handles hash-removal clicks without fetch", async () => {
				await init_adapter_runtime();
				window.history.replaceState({}, "", "/hash-remove/42#details");
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/hash-remove/:id",
						params: { id: "42" },
						children: "Remove Hash",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(true);
					expect(fetch_spy).not.toHaveBeenCalled();
					expect(window.location.hash).toBe("");
				} finally {
					cleanup();
				}
			});

			it("does not fetch for same-document no-op hash clicks", async () => {
				await init_adapter_runtime();
				window.history.replaceState({}, "", "/noop-hash/42#details");
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const hash_target = document.createElement("section");
				hash_target.id = "details";
				const scroll_spy = vi.fn();
				Object.defineProperty(hash_target, "scrollIntoView", {
					value: scroll_spy,
					writable: true,
					configurable: true,
				});
				document.body.appendChild(hash_target);
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/noop-hash/:id",
						params: { id: "42" },
						hash: "#details",
						children: "Noop Hash",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(true);
					expect(fetch_spy).not.toHaveBeenCalled();
					expect(scroll_spy).toHaveBeenCalledTimes(1);
				} finally {
					hash_target.remove();
					cleanup();
				}
			});

			it("prevents default for same-document no-op without hash", async () => {
				await init_adapter_runtime();
				window.history.replaceState({}, "", "/noop-no-hash/42");
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const scroll_spy = vi.spyOn(window, "scrollTo");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/noop-no-hash/:id",
						params: { id: "42" },
						children: "No-op",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(true);
					expect(fetch_spy).not.toHaveBeenCalled();
					expect(scroll_spy).toHaveBeenCalledWith(0, 0);
				} finally {
					cleanup();
				}
			});

			it("keeps scroll position for no-op without hash when scrollToTop=false", async () => {
				await init_adapter_runtime();
				window.history.replaceState({}, "", "/noop-no-scroll/42");
				const react_adapter = await import("vorma/react");
				const scroll_spy = vi.spyOn(window, "scrollTo");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/noop-no-scroll/:id",
						params: { id: "42" },
						scrollToTop: false,
						children: "No Scroll",
					},
				});

				try {
					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(scroll_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});

			it("does not fetch for encoding-equivalent hash clicks", async () => {
				await init_adapter_runtime();
				window.history.replaceState({}, "", "/encoding-hash/42#~");
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/encoding-hash/:id",
						params: { id: "42" },
						hash: "#%7E",
						children: "Equivalent Hash",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(true);
					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});
		});

		describe("external links", () => {
			it("marks external hrefs and does not prevent default", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "https://external.example/page#x",
						children: "External",
					},
				});

				try {
					expect(anchor.getAttribute("data-external")).toBe("true");
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(false);
					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});

			it("does not save scroll state for modifier-key same-document hash clicks", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				window.history.replaceState({}, "", "/current-page");
				const before = read_scroll_state_for_testing();
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/current-page#section",
						children: "Hash Link",
					},
				});

				try {
					anchor.addEventListener(
						"click",
						(e) => e.preventDefault(),
						{ once: true },
					);
					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
							ctrlKey: true,
						}),
					);
					await vi.runAllTimersAsync();

					expect(read_scroll_state_for_testing()).toEqual(before);
				} finally {
					cleanup();
				}
			});

			it("does not treat cross-origin hash links as same-document", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch");
				const before = read_scroll_state_for_testing();
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "https://external.example/page#x",
						children: "Cross Origin Hash",
					},
				});

				try {
					const click = new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					});
					anchor.dispatchEvent(click);
					await vi.runAllTimersAsync();

					expect(click.defaultPrevented).toBe(false);
					expect(fetch_spy).not.toHaveBeenCalled();
					expect(read_scroll_state_for_testing()).toEqual(before);
				} finally {
					cleanup();
				}
			});
		});

		describe("prop stripping", () => {
			it("strips navigation-only props from rendered anchors (react)", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/strip-test",
						replace: true,
						state: { a: 1 },
						prefetch: "intent",
						prefetchDelayMs: 25,
						beforeNavigate: () => {},
						children: "Strip",
					},
				});

				try {
					expect(anchor.getAttribute("href")).toBe("/strip-test");
					expect(anchor.getAttribute("replace")).toBeNull();
					expect(anchor.getAttribute("state")).toBeNull();
					expect(anchor.getAttribute("prefetch")).toBeNull();
					expect(anchor.getAttribute("prefetchdelayms")).toBeNull();
					expect(anchor.getAttribute("beforeNavigate")).toBeNull();
				} finally {
					cleanup();
				}
			});

			it("strips navigation-only props from rendered anchors (preact)", async () => {
				await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				const container = document.createElement("div");
				document.body.appendChild(container);

				try {
					await act(async () => {
						render_preact(
							h(preact_adapter.VormaLink as any, {
								href: "/preact-strip",
								replace: true,
								state: { a: 1 },
								prefetch: "intent",
								prefetchDelayMs: 25,
								beforeNavigate: () => {},
								children: "Strip",
							}),
							container,
						);
					});
					const anchor = container.querySelector("a");
					expect(anchor).not.toBeNull();
					expect(anchor?.getAttribute("replace")).toBeNull();
					expect(anchor?.getAttribute("state")).toBeNull();
					expect(anchor?.getAttribute("prefetch")).toBeNull();
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("strips navigation-only props from rendered anchors (solid)", async () => {
				await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return (solid_adapter.VormaLink as any)({
						href: "/solid-strip",
						replace: true,
						state: { a: 1 },
						prefetch: "intent",
						prefetchDelayMs: 25,
						beforeNavigate: () => {},
						children: "Strip",
					});
				}, container);

				try {
					const anchor = container.querySelector("a");
					expect(anchor).not.toBeNull();
					expect(anchor?.getAttribute("replace")).toBeNull();
					expect(anchor?.getAttribute("state")).toBeNull();
					expect(anchor?.getAttribute("prefetch")).toBeNull();
				} finally {
					dispose();
					container.remove();
				}
			});
		});
	});

	// ─────────────────────────────────────────────────────────
	// Assertion 31: Intent Prefetch
	// ─────────────────────────────────────────────────────────

	describe("intent prefetch (assertion 31)", () => {
		it("runs prefetch after delay without committing navigation (react)", async () => {
			const client = await init_adapter_runtime();
			const react_adapter = await import("vorma/react");
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/prefetch/:id"],
					loaders_data: [{ value: "prefetched" }],
				}),
			);
			const typed_link = react_adapter.makeTypedLink(
				TEST_APP_CONFIG as any,
				{ prefetch: "intent", prefetchDelayMs: 50 },
			);
			const { anchor, cleanup } = render_react_link({
				typed_link,
				link_props: {
					pattern: "/prefetch/:id",
					params: { id: "42" },
					children: "Prefetch",
				},
			});

			try {
				anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.advanceTimersByTimeAsync(49);
				expect(fetch_spy).not.toHaveBeenCalled();

				await vi.advanceTimersByTimeAsync(1);
				await vi.runAllTimersAsync();

				expect(fetch_spy).toHaveBeenCalledTimes(1);
				expect(window.location.pathname).toBe("/");
				expect_status_idle(client.getStatus());
			} finally {
				cleanup();
			}
		});

		it("runs prefetch after delay without committing navigation (preact)", async () => {
			const client = await init_adapter_runtime();
			const preact_adapter = await import("vorma/preact");
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/preact-prefetch/:id"],
					loaders_data: [{ value: "prefetched" }],
				}),
			);
			const typed_link = preact_adapter.makeTypedLink(
				TEST_APP_CONFIG as any,
				{ prefetch: "intent", prefetchDelayMs: 50 },
			);
			const container = document.createElement("div");
			document.body.appendChild(container);

			try {
				await act(async () => {
					render_preact(
						h(typed_link as any, {
							pattern: "/preact-prefetch/:id",
							params: { id: "42" },
							children: "Prefetch",
						}),
						container,
					);
				});
				const anchor = container.querySelector("a");
				anchor?.dispatchEvent(new Event("focus"));
				await vi.advanceTimersByTimeAsync(49);
				expect(fetch_spy).not.toHaveBeenCalled();

				await vi.advanceTimersByTimeAsync(1);
				await vi.runAllTimersAsync();

				expect(fetch_spy).toHaveBeenCalledTimes(1);
				expect(window.location.pathname).toBe("/");
				expect_status_idle(client.getStatus());
			} finally {
				render_preact(null, container);
				container.remove();
			}
		});

		it("runs prefetch after delay without committing navigation (solid)", async () => {
			const client = await init_adapter_runtime();
			const solid_adapter = await import("vorma/solid");
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					matched_patterns: ["/solid-prefetch/:id"],
					loaders_data: [{ value: "prefetched" }],
				}),
			);
			const typed_link = solid_adapter.makeTypedLink(
				TEST_APP_CONFIG as any,
				{ prefetch: "intent", prefetchDelayMs: 50 },
			);
			const container = document.createElement("div");
			document.body.appendChild(container);
			const dispose = render_solid_tracked(() => {
				return (typed_link as any)({
					pattern: "/solid-prefetch/:id",
					params: { id: "42" },
					children: "Prefetch",
				});
			}, container);

			try {
				const anchor = container.querySelector("a");
				anchor?.dispatchEvent(new Event("focus"));
				await vi.advanceTimersByTimeAsync(49);
				expect(fetch_spy).not.toHaveBeenCalled();

				await vi.advanceTimersByTimeAsync(1);
				await vi.runAllTimersAsync();

				expect(fetch_spy).toHaveBeenCalledTimes(1);
				expect(window.location.pathname).toBe("/");
				expect_status_idle(client.getStatus());
			} finally {
				dispose();
				container.remove();
			}
		});

		it("only prefetches eligible internal http targets", async () => {
			await init_adapter_runtime();
			const react_adapter = await import("vorma/react");
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());

			const external = render_react_link({
				typed_link: react_adapter.VormaLink,
				link_props: {
					href: "https://external.example/path",
					prefetch: "intent",
					prefetchDelayMs: 0,
					children: "External",
				},
			});
			try {
				external.anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.runAllTimersAsync();
				expect(fetch_spy).not.toHaveBeenCalled();
			} finally {
				external.cleanup();
			}

			const mailto = render_react_link({
				typed_link: react_adapter.VormaLink,
				link_props: {
					href: "mailto:test@example.com",
					prefetch: "intent",
					prefetchDelayMs: 0,
					children: "Mailto",
				},
			});
			try {
				mailto.anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.runAllTimersAsync();
				expect(fetch_spy).not.toHaveBeenCalled();
			} finally {
				mailto.cleanup();
			}

			const internal = render_react_link({
				typed_link: react_adapter.VormaLink,
				link_props: {
					href: "/internal-prefetch",
					prefetch: "intent",
					prefetchDelayMs: 0,
					children: "Internal",
				},
			});
			try {
				internal.anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.runAllTimersAsync();
				expect(fetch_spy).toHaveBeenCalledTimes(1);
			} finally {
				internal.cleanup();
			}
		});

		it("skips prefetch when href is the current page", async () => {
			await init_adapter_runtime();
			const react_adapter = await import("vorma/react");
			window.history.replaceState({}, "", "/current-page");
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());
			const { anchor, cleanup } = render_react_link({
				typed_link: react_adapter.VormaLink,
				link_props: {
					href: "/current-page",
					prefetch: "intent",
					prefetchDelayMs: 0,
					children: "Current",
				},
			});

			try {
				anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.runAllTimersAsync();
				expect(fetch_spy).not.toHaveBeenCalled();
			} finally {
				cleanup();
			}
		});

		it("retries prefetch after current-page no-op once location changes", async () => {
			await init_adapter_runtime();
			const react_adapter = await import("vorma/react");
			window.history.replaceState({}, "", "/current-page");
			const fetch_spy = vi
				.spyOn(window, "fetch")
				.mockResolvedValue(create_route_data_response());
			const { anchor, cleanup } = render_react_link({
				typed_link: react_adapter.VormaLink,
				link_props: {
					href: "/current-page",
					prefetch: "intent",
					prefetchDelayMs: 50,
					children: "Retry",
				},
			});

			try {
				anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.advanceTimersByTimeAsync(50);
				expect(fetch_spy).not.toHaveBeenCalled();

				window.history.replaceState({}, "", "/other-page");
				anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.advanceTimersByTimeAsync(50);
				await vi.runAllTimersAsync();

				expect(fetch_spy).toHaveBeenCalledTimes(1);
			} finally {
				cleanup();
			}
		});

		it("does not start work when prefetch=none (react)", async () => {
			await init_adapter_runtime();
			const react_adapter = await import("vorma/react");
			const fetch_spy = vi.spyOn(window, "fetch");
			const typed_link = react_adapter.makeTypedLink(
				TEST_APP_CONFIG as any,
				{ prefetch: "none" },
			);
			const { anchor, cleanup } = render_react_link({
				typed_link,
				link_props: {
					pattern: "/no-prefetch/:id",
					params: { id: "42" },
					children: "None",
				},
			});

			try {
				anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.advanceTimersByTimeAsync(200);
				await vi.runAllTimersAsync();
				expect(fetch_spy).not.toHaveBeenCalled();
			} finally {
				cleanup();
			}
		});

		it("does not start work when prefetch=none (preact)", async () => {
			await init_adapter_runtime();
			const preact_adapter = await import("vorma/preact");
			const fetch_spy = vi.spyOn(window, "fetch");
			const typed_link = preact_adapter.makeTypedLink(
				TEST_APP_CONFIG as any,
				{ prefetch: "none" },
			);
			const container = document.createElement("div");
			document.body.appendChild(container);

			try {
				await act(async () => {
					render_preact(
						h(typed_link as any, {
							pattern: "/preact-no-prefetch/:id",
							params: { id: "42" },
							children: "None",
						}),
						container,
					);
				});
				container.querySelector("a")?.dispatchEvent(new Event("focus"));
				await vi.advanceTimersByTimeAsync(200);
				await vi.runAllTimersAsync();
				expect(fetch_spy).not.toHaveBeenCalled();
			} finally {
				render_preact(null, container);
				container.remove();
			}
		});

		it("does not start work when prefetch=none (solid)", async () => {
			await init_adapter_runtime();
			const solid_adapter = await import("vorma/solid");
			const fetch_spy = vi.spyOn(window, "fetch");
			const typed_link = solid_adapter.makeTypedLink(
				TEST_APP_CONFIG as any,
				{ prefetch: "none" },
			);
			const container = document.createElement("div");
			document.body.appendChild(container);
			const dispose = render_solid_tracked(() => {
				return (typed_link as any)({
					pattern: "/solid-no-prefetch/:id",
					params: { id: "42" },
					children: "None",
				});
			}, container);

			try {
				container
					.querySelector("a")
					?.dispatchEvent(
						new Event("pointerenter", { bubbles: true }),
					);
				await vi.advanceTimersByTimeAsync(200);
				await vi.runAllTimersAsync();
				expect(fetch_spy).not.toHaveBeenCalled();
			} finally {
				dispose();
				container.remove();
			}
		});

		it("does not emit loading status during prefetch", async () => {
			const client = await init_adapter_runtime();
			const react_adapter = await import("vorma/react");
			const deferred = create_deferred<Response>();
			vi.spyOn(window, "fetch").mockImplementation(
				() => deferred.promise,
			);
			let is_running = false;
			const start = vi.fn(() => {
				is_running = true;
			});
			const stop = vi.fn(() => {
				is_running = false;
			});
			const cleanup_indicator = client.setupGlobalLoadingIndicator({
				start,
				stop,
				isRunning: () => is_running,
				startDelayMS: 0,
				stopDelayMS: 0,
			});
			const statuses: {
				isNavigating: boolean;
				isSubmitting: boolean;
				isRevalidating: boolean;
			}[] = [];
			const remove_listener = client.addStatusListener((event) => {
				statuses.push(event.detail);
			});
			const typed_link = react_adapter.makeTypedLink(
				TEST_APP_CONFIG as any,
				{ prefetch: "intent", prefetchDelayMs: 50 },
			);
			const { anchor, cleanup } = render_react_link({
				typed_link,
				link_props: {
					pattern: "/prefetch-status/:id",
					params: { id: "42" },
					children: "Status",
				},
			});

			try {
				anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.advanceTimersByTimeAsync(50);
				expect(start).not.toHaveBeenCalled();
				expect(is_running).toBe(false);
				expect(
					statuses.every(
						(s) =>
							!s.isNavigating &&
							!s.isSubmitting &&
							!s.isRevalidating,
					),
				).toBe(true);

				deferred.resolve(create_route_data_response());
				await vi.runAllTimersAsync();

				expect_status_idle(client.getStatus());
				expect(start).not.toHaveBeenCalled();
			} finally {
				cleanup_indicator();
				remove_listener();
				cleanup();
			}
		});

		it("warms artifacts without committing page mutations", async () => {
			const client = await init_adapter_runtime();
			const react_adapter = await import("vorma/react");
			vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					title: { dangerousInnerHTML: "Should Not Commit" },
				}),
			);
			const typed_link = react_adapter.makeTypedLink(
				TEST_APP_CONFIG as any,
				{ prefetch: "intent", prefetchDelayMs: 0 },
			);
			const { anchor, cleanup } = render_react_link({
				typed_link,
				link_props: {
					pattern: "/warm-only",
					children: "Warm",
				},
			});

			try {
				const initial_title = document.title;
				const initial_pathname = window.location.pathname;
				anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.runAllTimersAsync();

				expect(window.location.pathname).toBe(initial_pathname);
				expect(document.title).toBe(initial_title);
				expect_status_idle(client.getStatus());
			} finally {
				cleanup();
			}
		});

		it("applies prefetched css only after navigation commit", async () => {
			await init_adapter_runtime();
			const react_adapter = await import("vorma/react");
			const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
				create_route_data_response({
					css_bundles: ["/prefetch-commit.css"],
					title: { dangerousInnerHTML: "Prefetch Commit" },
				}),
			);
			const append_child = document.head.appendChild.bind(document.head);
			vi.spyOn(document.head, "appendChild").mockImplementation(
				(node) => {
					if (
						node instanceof HTMLLinkElement &&
						node.rel === "preload" &&
						node.getAttribute("as") === "style"
					) {
						void Promise.resolve().then(() =>
							node.dispatchEvent(new Event("load")),
						);
					}
					return append_child(node);
				},
			);
			const { anchor, cleanup } = render_react_link({
				typed_link: react_adapter.VormaLink,
				link_props: {
					href: "/prefetch-commit",
					prefetch: "intent",
					prefetchDelayMs: 0,
					children: "Commit",
				},
			});

			try {
				anchor.dispatchEvent(
					new FocusEvent("focusin", { bubbles: true }),
				);
				await vi.runAllTimersAsync();
				expect(
					document.head.querySelector(
						'link[rel="stylesheet"][data-vorma-css-bundle="/prefetch-commit.css"]',
					),
				).toBeNull();

				fetch_spy.mockClear();
				anchor.dispatchEvent(
					new MouseEvent("click", {
						bubbles: true,
						cancelable: true,
						button: 0,
					}),
				);
				await vi.runAllTimersAsync();

				expect(fetch_spy).not.toHaveBeenCalled();
				expect(
					document.querySelectorAll(
						'link[rel="stylesheet"][data-vorma-css-bundle="/prefetch-commit.css"]',
					),
				).toHaveLength(1);
				expect(document.title).toBe("Prefetch Commit");
			} finally {
				cleanup();
			}
		});
	});

	// ─────────────────────────────────────────────────────────
	// Assertion 32: Prefetch Dedup, Cancel, Upgrade, Stale Drop
	// ─────────────────────────────────────────────────────────

	describe("prefetch mechanics (assertion 32)", () => {
		describe("deduplication", () => {
			it("deduplicates same-data prefetches when only hash differs", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const first = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-dedupe#first",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "First",
					},
				});
				const second = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-dedupe#second",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "Second",
					},
				});

				try {
					first.anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					second.anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					expect(fetch_spy).toHaveBeenCalledTimes(1);
				} finally {
					first.cleanup();
					second.cleanup();
				}
			});

			it("deduplicates identical prefetches for the same href", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const first = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-identical",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "First",
					},
				});
				const second = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-identical",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "Second",
					},
				});

				try {
					first.anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					second.anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					expect(fetch_spy).toHaveBeenCalledTimes(1);
				} finally {
					first.cleanup();
					second.cleanup();
				}
			});

			it("aborts shared prefetch when stop is called from alias handler", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const signals: AbortSignal[] = [];
				vi.spyOn(window, "fetch").mockImplementation((_input, init) => {
					const signal = (init as RequestInit | undefined)?.signal;
					if (signal) signals.push(signal);
					return new Promise(() => {});
				});
				const first = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-stop#first",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "First",
					},
				});
				const second = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-stop#second",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "Second",
					},
				});

				try {
					first.anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					second.anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					expect(signals).toHaveLength(1);
					expect(signals[0]?.aborted).toBe(false);

					second.anchor.dispatchEvent(
						new FocusEvent("focusout", { bubbles: true }),
					);
					expect(signals[0]?.aborted).toBe(true);
				} finally {
					first.cleanup();
					second.cleanup();
				}
			});
		});

		describe("cancellation", () => {
			it("cancels pending timer on blur (react)", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ prefetch: "intent", prefetchDelayMs: 50 },
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/cancel/:id",
						params: { id: "42" },
						children: "Cancel",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(25);
					anchor.dispatchEvent(
						new FocusEvent("focusout", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(50);
					await vi.runAllTimersAsync();
					expect(fetch_spy).not.toHaveBeenCalled();

					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(50);
					await vi.runAllTimersAsync();
					expect(fetch_spy).toHaveBeenCalledTimes(1);
				} finally {
					cleanup();
				}
			});

			it("cancels pending timer on blur (preact)", async () => {
				await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const typed_link = preact_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ prefetch: "intent", prefetchDelayMs: 50 },
				);
				const container = document.createElement("div");
				document.body.appendChild(container);

				try {
					await act(async () => {
						render_preact(
							h(typed_link as any, {
								pattern: "/preact-cancel/:id",
								params: { id: "42" },
								children: "Cancel",
							}),
							container,
						);
					});
					const anchor = container.querySelector("a");
					anchor?.dispatchEvent(new Event("focus"));
					await vi.advanceTimersByTimeAsync(25);
					anchor?.dispatchEvent(new Event("blur"));
					await vi.advanceTimersByTimeAsync(50);
					await vi.runAllTimersAsync();
					expect(fetch_spy).not.toHaveBeenCalled();

					anchor?.dispatchEvent(new Event("focus"));
					await vi.advanceTimersByTimeAsync(50);
					await vi.runAllTimersAsync();
					expect(fetch_spy).toHaveBeenCalledTimes(1);
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("cancels pending timer on blur (solid)", async () => {
				await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const typed_link = solid_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ prefetch: "intent", prefetchDelayMs: 50 },
				);
				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return (typed_link as any)({
						pattern: "/solid-cancel/:id",
						params: { id: "42" },
						children: "Cancel",
					});
				}, container);

				try {
					const anchor = container.querySelector("a");
					anchor?.dispatchEvent(new Event("focus"));
					await vi.advanceTimersByTimeAsync(25);
					anchor?.dispatchEvent(new Event("blur"));
					await vi.advanceTimersByTimeAsync(50);
					await vi.runAllTimersAsync();
					expect(fetch_spy).not.toHaveBeenCalled();

					anchor?.dispatchEvent(new Event("focus"));
					await vi.advanceTimersByTimeAsync(50);
					await vi.runAllTimersAsync();
					expect(fetch_spy).toHaveBeenCalledTimes(1);
				} finally {
					dispose();
					container.remove();
				}
			});

			it("aborts in-flight prefetch when stop is called", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const signals: AbortSignal[] = [];
				vi.spyOn(window, "fetch").mockImplementation((_input, init) => {
					const signal = (init as RequestInit | undefined)?.signal;
					if (signal) signals.push(signal);
					return new Promise(() => {});
				});
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/abort-inflight",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "Abort",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					expect(signals).toHaveLength(1);
					expect(signals[0]?.aborted).toBe(false);

					anchor.dispatchEvent(
						new FocusEvent("focusout", { bubbles: true }),
					);
					expect(signals[0]?.aborted).toBe(true);
				} finally {
					cleanup();
				}
			});

			it("cancels pending prefetch timer on hash-only click", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				window.history.replaceState({}, "", "/hash-page");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const before_begin = vi.fn();
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/hash-page#section-a",
						prefetch: "intent",
						prefetchDelayMs: 200,
						beforeNavigate: before_begin,
						children: "Hash Click",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(100);
					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.advanceTimersByTimeAsync(200);

					expect(before_begin).not.toHaveBeenCalled();
					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});

			it("does not leak orphan timers from multiple starts before stop", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/orphan-timer",
						prefetch: "intent",
						prefetchDelayMs: 200,
						children: "Orphan",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					anchor.dispatchEvent(
						new FocusEvent("focusout", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(250);
					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
				}
			});

			it("clears pending timer when timer id is zero", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const set_timeout_spy = vi
					.spyOn(window, "setTimeout")
					.mockImplementation(() => 0 as any);
				const clear_timeout_spy = vi.spyOn(window, "clearTimeout");
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/zero-timer",
						prefetch: "intent",
						children: "Zero",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					anchor.dispatchEvent(
						new FocusEvent("focusout", { bubbles: true }),
					);
					expect(clear_timeout_spy).toHaveBeenCalledWith(0);
					await vi.advanceTimersByTimeAsync(200);
					expect(fetch_spy).not.toHaveBeenCalled();
				} finally {
					cleanup();
					set_timeout_spy.mockRestore();
					clear_timeout_spy.mockRestore();
				}
			});

			it("drops completed cache on stop so later prefetches refetch", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/completed-cache",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "Cache",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					expect(fetch_spy).toHaveBeenCalledTimes(1);

					anchor.dispatchEvent(
						new FocusEvent("focusout", { bubbles: true }),
					);
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					expect(fetch_spy).toHaveBeenCalledTimes(2);
				} finally {
					cleanup();
				}
			});
		});

		describe("touch modality", () => {
			it("keeps pending timer active on pointerleave during touch modality", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ prefetch: "intent", prefetchDelayMs: 50 },
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/touch-pending/:id",
						params: { id: "42" },
						children: "Touch Pending",
					},
				});

				try {
					const touch_down = new Event("pointerdown", {
						bubbles: true,
					});
					Object.defineProperty(touch_down, "pointerType", {
						value: "touch",
					});
					anchor.dispatchEvent(touch_down);
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(25);
					anchor.dispatchEvent(
						new Event("pointerout", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(25);
					await vi.runAllTimersAsync();

					expect(fetch_spy).toHaveBeenCalledTimes(1);
				} finally {
					cleanup();
				}
			});

			it("does not abort in-flight prefetch on pointerleave during touch modality", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const { requests } = create_abort_aware_fetch_recorder();
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ prefetch: "intent", prefetchDelayMs: 0 },
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/touch-inflight/:id",
						params: { id: "42" },
						children: "Touch In-flight",
					},
				});

				try {
					const touch_down = new Event("pointerdown", {
						bubbles: true,
					});
					Object.defineProperty(touch_down, "pointerType", {
						value: "touch",
					});
					anchor.dispatchEvent(touch_down);
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await wait_for_request_count({ requests, count: 1 });

					const signal = requests[0]?.init?.signal as
						| AbortSignal
						| undefined;
					expect(signal?.aborted).toBe(false);

					anchor.dispatchEvent(
						new Event("pointerout", { bubbles: true }),
					);
					expect(signal?.aborted).toBe(false);

					requests[0]?.deferred.resolve(create_route_data_response());
					await vi.runAllTimersAsync();
				} finally {
					cleanup();
				}
			});

			it("aborts in-flight prefetch on pointerleave after switching to fine pointer", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const { requests } = create_abort_aware_fetch_recorder();
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ prefetch: "intent", prefetchDelayMs: 0 },
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/touch-fine/:id",
						params: { id: "42" },
						children: "Touch Fine",
					},
				});

				try {
					const touch_down = new Event("pointerdown", {
						bubbles: true,
					});
					Object.defineProperty(touch_down, "pointerType", {
						value: "touch",
					});
					anchor.dispatchEvent(touch_down);
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await wait_for_request_count({ requests, count: 1 });

					const signal = requests[0]?.init?.signal as
						| AbortSignal
						| undefined;
					expect(signal?.aborted).toBe(false);

					const mouse_move = new Event("pointermove");
					Object.defineProperty(mouse_move, "pointerType", {
						value: "mouse",
					});
					window.dispatchEvent(mouse_move);

					anchor.dispatchEvent(
						new Event("pointerout", { bubbles: true }),
					);
					expect(signal?.aborted).toBe(true);
					await vi.runAllTimersAsync();
				} finally {
					cleanup();
				}
			});
		});

		describe("navigation upgrade", () => {
			it("reuses completed prefetch on click without refetching", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const fetch_spy = vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						title: { dangerousInnerHTML: "Prefetch Reuse" },
					}),
				);
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-reuse",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "Reuse",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.runAllTimersAsync();
					expect(fetch_spy).toHaveBeenCalledTimes(1);

					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(fetch_spy).toHaveBeenCalledTimes(1);
					expect(window.location.pathname).toBe("/prefetch-reuse");
					expect(document.title).toBe("Prefetch Reuse");
					expect_status_idle(client.getStatus());
				} finally {
					cleanup();
				}
			});

			it("does not abort upgraded navigation when prefetch handlers stop", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const deferred = create_deferred<Response>();
				let signal: AbortSignal | undefined;
				vi.spyOn(window, "fetch").mockImplementation((_input, init) => {
					signal =
						(init as RequestInit | undefined)?.signal ?? undefined;
					return deferred.promise;
				});
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/upgrade-test",
						prefetch: "intent",
						children: "Upgrade",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(100);
					expect(signal?.aborted).toBe(false);

					const nav = client.vormaNavigate("/upgrade-test");
					anchor.dispatchEvent(
						new FocusEvent("focusout", { bubbles: true }),
					);
					expect(signal?.aborted).toBe(false);
					expect(client.getStatus().isNavigating).toBe(true);

					deferred.resolve(create_route_data_response());
					await nav;
					await vi.runAllTimersAsync();
				} finally {
					cleanup();
				}
			});

			it("upgrades same-data prefetch to navigation when hash differs", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const deferred = create_deferred<Response>();
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockImplementation(() => deferred.promise);
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/hash-upgrade#prefetch",
						prefetch: "intent",
						children: "Hash Upgrade",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(100);
					expect(fetch_spy).toHaveBeenCalledTimes(1);
					expect(client.getStatus().isNavigating).toBe(false);

					const nav = client.vormaNavigate("/hash-upgrade#final");
					await vi.advanceTimersByTimeAsync(8);
					expect(fetch_spy).toHaveBeenCalledTimes(1);
					expect(client.getStatus().isNavigating).toBe(true);

					deferred.resolve(
						create_route_data_response({
							title: { dangerousInnerHTML: "Hash Upgrade" },
						}),
					);
					await nav;
					await vi.runAllTimersAsync();

					expect(window.location.pathname).toBe("/hash-upgrade");
					expect(window.location.hash).toBe("#final");
					expect(document.title).toBe("Hash Upgrade");
				} finally {
					cleanup();
				}
			});

			it("supports click while prefetch is in-flight and runs render callbacks", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => deferred.promise,
				);
				const before_begin = vi.fn();
				const before_render = vi.fn();
				const after_render = vi.fn();
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/click-during",
						prefetch: "intent",
						beforeNavigate: before_begin,
						beforeRender: before_render,
						afterRender: after_render,
						children: "Click During",
					},
				});

				try {
					anchor.dispatchEvent(
						new FocusEvent("focusin", { bubbles: true }),
					);
					await vi.advanceTimersByTimeAsync(100);

					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					deferred.resolve(
						create_route_data_response({
							title: { dangerousInnerHTML: "Eventual" },
						}),
					);
					await vi.runAllTimersAsync();

					expect(before_begin).toHaveBeenCalledTimes(1);
					expect(before_render).toHaveBeenCalledTimes(1);
					expect(after_render).toHaveBeenCalledTimes(1);
					expect(document.title).toBe("Eventual");
				} finally {
					cleanup();
				}
			});

			it("runs beforeNavigate on direct click without prior prefetch", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response({
						title: { dangerousInnerHTML: "Direct Click" },
					}),
				);
				const before_begin = vi.fn();
				const before_render = vi.fn();
				const after_render = vi.fn();
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/direct-click",
						prefetch: "intent",
						beforeNavigate: before_begin,
						beforeRender: before_render,
						afterRender: after_render,
						children: "Direct",
					},
				});

				try {
					anchor.dispatchEvent(
						new MouseEvent("click", {
							bubbles: true,
							cancelable: true,
							button: 0,
						}),
					);
					await vi.runAllTimersAsync();

					expect(before_begin).toHaveBeenCalledTimes(1);
					expect(before_render).toHaveBeenCalledTimes(1);
					expect(after_render).toHaveBeenCalledTimes(1);
					expect(document.title).toBe("Direct Click");
				} finally {
					cleanup();
				}
			});
		});

		describe("stale and failed prefetch drop", () => {
			it("drops stale settled prefetch without unhandled rejections", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const deferred = create_deferred<Response>();
				vi.spyOn(window, "fetch").mockImplementation(
					() => deferred.promise,
				);
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ prefetch: "intent", prefetchDelayMs: 50 },
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/stale-drop/:id",
						params: { id: "42" },
						children: "Stale",
					},
				});

				try {
					const { unhandled_rejections } =
						await with_unhandled_rejection_capture({
							run: async () => {
								anchor.dispatchEvent(
									new FocusEvent("focusin", {
										bubbles: true,
									}),
								);
								await vi.advanceTimersByTimeAsync(50);

								anchor.dispatchEvent(
									new FocusEvent("focusout", {
										bubbles: true,
									}),
								);
								deferred.resolve(create_route_data_response());
								await vi.runAllTimersAsync();
							},
						});

					expect(unhandled_rejections).toEqual([]);
					expect(window.location.pathname).toBe("/");
					expect_status_idle(client.getStatus());
				} finally {
					cleanup();
				}
			});

			it("does not leak unhandled rejections when prefetch resolves to redirect", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const add_loader = react_adapter.makeTypedAddClientLoader(
					TEST_APP_CONFIG as any,
				);
				const server_errors: unknown[] = [];
				add_loader({
					pattern: "/prefetch-redirect",
					clientLoader: async ({ serverDataPromise }) => {
						try {
							await serverDataPromise;
						} catch (error) {
							server_errors.push(error);
							throw error;
						}
						return null;
					},
				});
				vi.spyOn(window, "fetch").mockResolvedValue(
					create_route_data_response(
						{},
						{
							headers: {
								"X-Client-Redirect": "/redirect-target",
							},
						},
					),
				);
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-redirect",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "Redirect",
					},
				});

				try {
					const { unhandled_rejections } =
						await with_unhandled_rejection_capture({
							run: async () => {
								anchor.dispatchEvent(
									new FocusEvent("focusin", {
										bubbles: true,
									}),
								);
								await vi.runAllTimersAsync();
							},
						});

					expect(unhandled_rejections).toEqual([]);
					expect(server_errors).toHaveLength(1);
					expect(server_errors[0]).toBeInstanceOf(Error);
					expect((server_errors[0] as Error).name).toBe("AbortError");
				} finally {
					cleanup();
				}
			});

			it("rejects serverDataPromise with AbortError for failed prefetch responses", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const add_loader = react_adapter.makeTypedAddClientLoader(
					TEST_APP_CONFIG as any,
				);
				const server_errors: unknown[] = [];
				add_loader({
					pattern: "/prefetch-failed",
					clientLoader: async ({ serverDataPromise }) => {
						try {
							await serverDataPromise;
						} catch (error) {
							server_errors.push(error);
							throw error;
						}
						return null;
					},
				});
				vi.spyOn(window, "fetch").mockResolvedValue(
					new Response("Server error", { status: 500 }),
				);
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/prefetch-failed",
						prefetch: "intent",
						prefetchDelayMs: 0,
						children: "Failed",
					},
				});

				try {
					const { unhandled_rejections } =
						await with_unhandled_rejection_capture({
							run: async () => {
								anchor.dispatchEvent(
									new FocusEvent("focusin", {
										bubbles: true,
									}),
								);
								await vi.runAllTimersAsync();
							},
						});

					expect(unhandled_rejections).toEqual([]);
					expect(server_errors).toHaveLength(1);
					expect((server_errors[0] as Error).name).toBe("AbortError");
				} finally {
					cleanup();
				}
			});

			it("recovers from beforeNavigate prefetch callback failures", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const before_begin = vi
					.fn()
					.mockImplementationOnce(() => {
						throw new Error("beforeNavigate failed");
					})
					.mockImplementation(() => {});
				const fetch_spy = vi
					.spyOn(window, "fetch")
					.mockResolvedValue(create_route_data_response());
				const { anchor, cleanup } = render_react_link({
					typed_link: react_adapter.VormaLink,
					link_props: {
						href: "/before-begin-retry",
						prefetch: "intent",
						prefetchDelayMs: 0,
						beforeNavigate: before_begin,
						children: "Retry",
					},
				});

				try {
					const { unhandled_rejections } =
						await with_unhandled_rejection_capture({
							run: async () => {
								anchor.dispatchEvent(
									new FocusEvent("focusin", {
										bubbles: true,
									}),
								);
								await vi.runAllTimersAsync();
								anchor.dispatchEvent(
									new FocusEvent("focusin", {
										bubbles: true,
									}),
								);
								await vi.runAllTimersAsync();
							},
						});

					expect(unhandled_rejections).toEqual([]);
					expect(before_begin).toHaveBeenCalledTimes(2);
					expect(fetch_spy).toHaveBeenCalledTimes(1);
				} finally {
					cleanup();
				}
			});

			it("does not leak unhandled rejections when link fetch fails", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.spyOn(console, "error").mockImplementation(() => {});
				vi.spyOn(window, "fetch").mockRejectedValue(
					new Error("network failed"),
				);
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{},
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/failing-click",
						children: "Failing",
					},
				});

				try {
					const { unhandled_rejections } =
						await with_unhandled_rejection_capture({
							run: async () => {
								anchor.dispatchEvent(
									new MouseEvent("click", {
										bubbles: true,
										cancelable: true,
										button: 0,
									}),
								);
								await vi.runAllTimersAsync();
							},
						});

					expect(unhandled_rejections).toEqual([]);
					expect(client.getStatus().isNavigating).toBe(false);
				} finally {
					cleanup();
				}
			});
		});

		describe("typed link factory defaults", () => {
			it("lets per-link className override factory defaults (react)", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ className: "default-class" },
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/products/:id",
						params: { id: "42" },
						search: "?q=abc",
						hash: "#panel",
						className: "custom-class",
					},
				});

				try {
					expect(anchor.getAttribute("href")).toBe(
						`${window.location.origin}/products/42?q=abc#panel`,
					);
					expect(anchor.className).toBe("custom-class");
				} finally {
					cleanup();
				}
			});

			it("applies factory defaults when per-link props omit optional fields (react)", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const typed_link = react_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ className: "default-class" },
				);
				const { anchor, cleanup } = render_react_link({
					typed_link,
					link_props: {
						pattern: "/products/:id",
						params: { id: "42" },
					},
				});

				try {
					expect(anchor.className).toBe("default-class");
				} finally {
					cleanup();
				}
			});

			it("lets per-link class override factory defaults (preact)", async () => {
				await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				const typed_link = preact_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ className: "default-class" },
				);
				const container = document.createElement("div");
				document.body.appendChild(container);

				try {
					await act(async () => {
						render_preact(
							h(typed_link as any, {
								pattern: "/products/:id",
								params: { id: "42" },
								className: "custom-class",
							}),
							container,
						);
					});
					const anchor = container.querySelector("a");
					expect(anchor?.className).toBe("custom-class");
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("lets per-link class override factory defaults (solid)", async () => {
				await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const typed_link = solid_adapter.makeTypedLink(
					TEST_APP_CONFIG as any,
					{ class: "default-class" },
				);
				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return (typed_link as any)({
						pattern: "/products/:id",
						params: { id: "42" },
						class: "custom-class",
						children: "Products",
					});
				}, container);

				try {
					const anchor = container.querySelector("a");
					expect(anchor?.getAttribute("class")).toBe("custom-class");
				} finally {
					dispose();
					container.remove();
				}
			});
		});
	});
});
