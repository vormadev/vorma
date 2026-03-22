// Assertions covered: 27, 28, 29

import { h, render as render_preact } from "preact";
import {
	useEffect as use_preact_effect,
	useRef as use_preact_ref,
	useState as use_preact_state,
} from "preact/hooks";
import { act } from "preact/test-utils";
import React from "react";
import { flushSync } from "react-dom";
import { createRoot } from "react-dom/client";
import { createComponent, createEffect, createMemo, onCleanup } from "solid-js";
import { render as render_solid } from "solid-js/web";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	create_isolated_client_test_runtime,
	reset_client_runtime_for_testing,
} from "vorma/testing";
import { create_route_data_response } from "./setup.ts";

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

// ─── Helpers ─────────────────────────────────────────────────────

async function init_adapter_runtime() {
	vi.resetModules();
	create_isolated_client_test_runtime();
	const client = await import("vorma/client");
	await client.initClient({
		vormaAppConfig: TEST_APP_CONFIG as any,
	});
	return client;
}

async function navigate_with_response(
	client: { vormaNavigate: (href: string) => Promise<unknown> },
	href: string,
	overrides: Record<string, unknown> = {},
): Promise<void> {
	vi.spyOn(window, "fetch").mockResolvedValueOnce(
		create_route_data_response(overrides as any),
	);
	await client.vormaNavigate(href);
	await vi.runAllTimersAsync();
}

async function wait_for_dom(
	assertion: () => void,
	max_ticks = 30,
): Promise<void> {
	let last_error: unknown;
	for (let i = 0; i < max_ticks; i += 1) {
		try {
			assertion();
			return;
		} catch (error) {
			last_error = error;
			await Promise.resolve();
			await vi.advanceTimersByTimeAsync(1);
		}
	}
	throw last_error;
}

// ─── Setup / Teardown ────────────────────────────────────────────

beforeEach(() => {
	reset_client_runtime_for_testing();
});

afterEach(async () => {
	active_solid_disposers.forEach((dispose) => dispose());
	active_solid_disposers.clear();
});

// ─── Assertion 27: Root Outlets, Error Boundaries, Listeners ─────

describe("adapters", () => {
	describe("root outlets (assertion 27)", () => {
		// ─── React Root Outlet ───────────────────────────

		describe("react", () => {
			it("renders fallback when root component is missing", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.doMock("/missing-root.js", () => ({
					default: undefined,
				}));
				vi.doMock("/child.js", () => ({
					default: () =>
						React.createElement("div", {}, "child-content"),
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);
				flushSync(() => {
					root.render(
						React.createElement(
							react_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						),
					);
				});

				try {
					await navigate_with_response(client, "/react/fallback", {
						matched_patterns: ["/", "/child"],
						import_urls: ["/missing-root.js", "/child.js"],
						export_keys: ["default", "default"],
						error_export_keys: ["", ""],
						loaders_data: [{}, {}],
					});
					await wait_for_dom(() => {
						expect(container.textContent).toBe("child-content");
					});
				} finally {
					root.unmount();
					container.remove();
				}
			});

			it("renders custom error boundary from export key", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.doMock("/error-custom.js", () => ({
					default: () =>
						React.createElement("div", {}, "should-not-render"),
					ErrorBoundary: (props: { error: unknown }) =>
						React.createElement(
							"div",
							{},
							`handled:${String(props.error)}`,
						),
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);
				flushSync(() => {
					root.render(
						React.createElement(
							react_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						),
					);
				});

				try {
					await navigate_with_response(
						client,
						"/react/error-custom",
						{
							matched_patterns: ["/"],
							import_urls: ["/error-custom.js"],
							export_keys: ["default"],
							error_export_keys: ["ErrorBoundary"],
							loaders_data: [{}],
							outermost_server_error: "boom",
							outermost_server_error_idx: 0,
						},
					);
					await wait_for_dom(() => {
						expect(container.textContent).toContain("handled:boom");
					});
					expect(container.textContent).not.toContain(
						"should-not-render",
					);
				} finally {
					root.unmount();
					container.remove();
				}
			});

			it("renders child error boundary under live parent layout", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.doMock("/parent.js", () => ({
					default: (props: { Outlet: unknown }) =>
						React.createElement(
							"section",
							{},
							React.createElement(
								"div",
								{ "data-parent": true },
								"parent-node",
							),
							React.createElement(props.Outlet as any, {}),
						),
				}));
				vi.doMock("/child-error.js", () => ({
					default: () => React.createElement("div", {}, "child-node"),
					ErrorBoundary: (props: { error: unknown }) =>
						React.createElement(
							"div",
							{},
							`handled:${String(props.error)}`,
						),
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);
				flushSync(() => {
					root.render(
						React.createElement(
							react_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						),
					);
				});

				try {
					await navigate_with_response(client, "/react/child-error", {
						matched_patterns: ["/parent", "/parent/child"],
						import_urls: ["/parent.js", "/child-error.js"],
						export_keys: ["default", "default"],
						error_export_keys: ["", "ErrorBoundary"],
						loaders_data: [{}, {}],
						outermost_server_error: "child-boom",
						outermost_server_error_idx: 1,
					});
					await wait_for_dom(() => {
						expect(
							container.querySelector("[data-parent]")
								?.textContent,
						).toBe("parent-node");
						expect(container.textContent).toContain(
							"handled:child-boom",
						);
					});
					expect(container.textContent).not.toContain("child-node");
				} finally {
					root.unmount();
					container.remove();
				}
			});

			it("preserves active error boundary across hash-only navigations", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.doMock("/hash-error.js", () => ({
					default: () =>
						React.createElement("div", {}, "should-not-render"),
					ErrorBoundary: (props: { error: unknown }) =>
						React.createElement(
							"div",
							{},
							`hash-handled:${String(props.error)}`,
						),
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);
				flushSync(() => {
					root.render(
						React.createElement(
							react_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						),
					);
				});

				try {
					await navigate_with_response(client, "/react/hash-error", {
						matched_patterns: ["/hash-error"],
						import_urls: ["/hash-error.js"],
						export_keys: ["default"],
						error_export_keys: ["ErrorBoundary"],
						loaders_data: [{}],
						outermost_server_error: "boom",
						outermost_server_error_idx: 0,
					});
					await wait_for_dom(() => {
						expect(container.textContent).toContain(
							"hash-handled:boom",
						);
					});

					vi.spyOn(window, "fetch").mockResolvedValue(
						create_route_data_response({
							matched_patterns: ["/hash-error"],
							import_urls: ["/hash-error.js"],
							export_keys: ["default"],
							error_export_keys: ["ErrorBoundary"],
							loaders_data: [{}],
							outermost_server_error: "boom",
							outermost_server_error_idx: 0,
						} as any),
					);
					await client.vormaNavigate("/react/hash-error#next");
					await vi.runAllTimersAsync();

					expect(container.textContent).toContain(
						"hash-handled:boom",
					);
					expect(container.textContent).not.toContain(
						"should-not-render",
					);
				} finally {
					root.unmount();
					container.remove();
				}
			});

			it("defaults idx to 0 when omitted", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				vi.doMock("/default-idx.js", () => ({
					default: (props: { idx: number }) =>
						React.createElement("div", {}, `idx:${props.idx}`),
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);
				flushSync(() => {
					root.render(
						React.createElement(
							react_adapter.VormaRootOutlet as any,
						),
					);
				});

				try {
					await navigate_with_response(client, "/react/default-idx", {
						matched_patterns: ["/"],
						import_urls: ["/default-idx.js"],
						export_keys: ["default"],
						error_export_keys: [""],
						loaders_data: [{}],
					});
					await wait_for_dom(() => {
						expect(container.textContent).toContain("idx:0");
					});
				} finally {
					root.unmount();
					container.remove();
				}
			});

			it("initializes root listeners only once across remounts", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const add_spy = vi.spyOn(window, "addEventListener");
				const container_a = document.createElement("div");
				const container_b = document.createElement("div");
				document.body.appendChild(container_a);
				document.body.appendChild(container_b);
				const root_a = createRoot(container_a);
				const root_b = createRoot(container_b);

				try {
					flushSync(() => {
						root_a.render(
							React.createElement(
								react_adapter.VormaRootOutlet as any,
								{ idx: 0 },
							),
						);
					});
					flushSync(() => {
						root_a.unmount();
					});
					flushSync(() => {
						root_b.render(
							React.createElement(
								react_adapter.VormaRootOutlet as any,
								{ idx: 0 },
							),
						);
					});

					expect(
						add_spy.mock.calls.filter(
							([name]) => name === "vorma:route-change",
						).length,
					).toBe(1);
				} finally {
					root_b.unmount();
					container_a.remove();
					container_b.remove();
				}
			});

			it("idx>0 outlet does not initialize root listeners", async () => {
				await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const add_spy = vi.spyOn(window, "addEventListener");
				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);

				try {
					flushSync(() => {
						root.render(
							React.createElement(
								react_adapter.VormaRootOutlet as any,
								{ idx: 1 },
							),
						);
					});

					expect(
						add_spy.mock.calls.filter(
							([name]) => name === "vorma:route-change",
						).length,
					).toBe(0);
				} finally {
					root.unmount();
					container.remove();
				}
			});
		});

		// ─── Preact Root Outlet ──────────────────────────

		describe("preact", () => {
			it("renders custom error boundary from export key", async () => {
				const client = await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				vi.doMock("/preact-error-custom.js", () => ({
					default: () => h("div", {}, "should-not-render"),
					ErrorBoundary: (props: { error: unknown }) =>
						h("div", {}, `handled:${String(props.error)}`),
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				try {
					await act(async () => {
						render_preact(
							h(preact_adapter.VormaRootOutlet as any, {
								idx: 0,
							}),
							container,
						);
					});

					await navigate_with_response(
						client,
						"/preact/error-custom",
						{
							matched_patterns: ["/"],
							import_urls: ["/preact-error-custom.js"],
							export_keys: ["default"],
							error_export_keys: ["ErrorBoundary"],
							loaders_data: [{}],
							outermost_server_error: "boom",
							outermost_server_error_idx: 0,
						},
					);
					await wait_for_dom(() => {
						expect(container.textContent).toContain("handled:boom");
					});
					expect(container.textContent).not.toContain(
						"should-not-render",
					);
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("defaults idx to 0 when omitted", async () => {
				const client = await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				vi.doMock("/preact-default-idx.js", () => ({
					default: (props: { idx: number }) =>
						h("div", {}, `idx:${props.idx}`),
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				try {
					await act(async () => {
						render_preact(
							h(preact_adapter.VormaRootOutlet as any, {}),
							container,
						);
					});

					await navigate_with_response(
						client,
						"/preact/default-idx",
						{
							matched_patterns: ["/"],
							import_urls: ["/preact-default-idx.js"],
							export_keys: ["default"],
							error_export_keys: [""],
							loaders_data: [{}],
						},
					);
					await wait_for_dom(() => {
						expect(container.textContent).toContain("idx:0");
					});
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("initializes root listeners only once across remounts", async () => {
				await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				const add_spy = vi.spyOn(window, "addEventListener");
				const container_a = document.createElement("div");
				const container_b = document.createElement("div");
				document.body.appendChild(container_a);
				document.body.appendChild(container_b);

				try {
					await act(async () => {
						render_preact(
							h(preact_adapter.VormaRootOutlet as any, {
								idx: 0,
							}),
							container_a,
						);
					});
					await act(async () => {
						render_preact(null, container_a);
					});
					await act(async () => {
						render_preact(
							h(preact_adapter.VormaRootOutlet as any, {
								idx: 0,
							}),
							container_b,
						);
					});

					expect(
						add_spy.mock.calls.filter(
							([name]) => name === "vorma:route-change",
						).length,
					).toBe(1);
				} finally {
					render_preact(null, container_b);
					container_a.remove();
					container_b.remove();
				}
			});

			it("idx>0 outlet does not initialize root listeners", async () => {
				await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				const add_spy = vi.spyOn(window, "addEventListener");
				const container = document.createElement("div");
				document.body.appendChild(container);

				try {
					await act(async () => {
						render_preact(
							h(preact_adapter.VormaRootOutlet as any, {
								idx: 1,
							}),
							container,
						);
					});

					expect(
						add_spy.mock.calls.filter(
							([name]) => name === "vorma:route-change",
						).length,
					).toBe(0);
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});
		});

		// ─── Solid Root Outlet ───────────────────────────

		describe("solid", () => {
			it("renders custom error boundary from export key", async () => {
				const client = await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const error_boundary = vi.fn((props: { error: unknown }) => {
					const node = document.createElement("div");
					node.textContent = `handled:${String(props.error)}`;
					return node;
				});
				const error_default = vi.fn(() => {
					const node = document.createElement("div");
					node.textContent = "should-not-render";
					return node;
				});
				vi.doMock("/solid-error-custom.js", () => ({
					default: error_default,
					ErrorBoundary: error_boundary,
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return createComponent(
						solid_adapter.VormaRootOutlet as any,
						{ idx: 0 },
					);
				}, container);

				try {
					await navigate_with_response(
						client,
						"/solid/error-custom",
						{
							matched_patterns: ["/"],
							import_urls: ["/solid-error-custom.js"],
							export_keys: ["default"],
							error_export_keys: ["ErrorBoundary"],
							loaders_data: [{}],
							outermost_server_error: "boom",
							outermost_server_error_idx: 0,
						},
					);
					await wait_for_dom(() => {
						expect(error_boundary).toHaveBeenCalled();
					});
					expect(error_boundary).toHaveBeenLastCalledWith(
						expect.objectContaining({ error: "boom" }),
					);
					expect(error_default).not.toHaveBeenCalled();
				} finally {
					dispose();
					container.remove();
				}
			});

			it("defaults idx to 0 when omitted", async () => {
				const client = await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const solid_default = vi.fn((props: { idx: number }) => {
					return `idx:${props.idx}`;
				});
				vi.doMock("/solid-default-idx.js", () => ({
					default: solid_default,
				}));

				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return createComponent(
						solid_adapter.VormaRootOutlet as any,
						{},
					);
				}, container);

				try {
					await navigate_with_response(client, "/solid/default-idx", {
						matched_patterns: ["/"],
						import_urls: ["/solid-default-idx.js"],
						export_keys: ["default"],
						error_export_keys: [""],
						loaders_data: [{}],
					});
					await wait_for_dom(() => {
						expect(solid_default).toHaveBeenCalledWith(
							expect.objectContaining({ idx: 0 }),
						);
					});
				} finally {
					dispose();
					container.remove();
				}
			});

			it("initializes root listeners only once across remounts", async () => {
				await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const add_spy = vi.spyOn(window, "addEventListener");
				const container_a = document.createElement("div");
				const container_b = document.createElement("div");
				document.body.appendChild(container_a);
				document.body.appendChild(container_b);

				try {
					const dispose_a = render_solid_tracked(() => {
						return createComponent(
							solid_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						);
					}, container_a);
					dispose_a();

					const dispose_b = render_solid_tracked(() => {
						return createComponent(
							solid_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						);
					}, container_b);

					expect(
						add_spy.mock.calls.filter(
							([name]) => name === "vorma:route-change",
						).length,
					).toBe(1);

					dispose_b();
				} finally {
					container_a.remove();
					container_b.remove();
				}
			});

			it("idx>0 outlet does not initialize root listeners", async () => {
				await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				const add_spy = vi.spyOn(window, "addEventListener");
				const container = document.createElement("div");
				document.body.appendChild(container);

				const dispose = render_solid_tracked(() => {
					return createComponent(
						solid_adapter.VormaRootOutlet as any,
						{ idx: 1 },
					);
				}, container);

				try {
					expect(
						add_spy.mock.calls.filter(
							([name]) => name === "vorma:route-change",
						).length,
					).toBe(0);
				} finally {
					dispose();
					container.remove();
				}
			});
		});
	});

	// ─── Assertion 28: Location, Selectors, Data Stability ───

	describe("selectors and location (assertion 28)", () => {
		// describe("preact location signal", () => {
		// 	it("updates only on location events, not route events", async () => {
		// 		const client = await init_adapter_runtime();
		// 		const preact_adapter = await import("vorma/preact");
		// 		vi.doMock("/preact-loc-root.js", () => ({
		// 			default: () => h("div", {}, "root"),
		// 		}));
		// 		vi.spyOn(window, "fetch").mockImplementation(() =>
		// 			Promise.resolve(
		// 				create_route_data_response({
		// 					matched_patterns: ["/"],
		// 					import_urls: ["/preact-loc-root.js"],
		// 					export_keys: ["default"],
		// 					error_export_keys: [""],
		// 				} as any),
		// 			),
		// 		);

		// 		const container = document.createElement("div");
		// 		document.body.appendChild(container);
		// 		let location_render_count = 0;
		// 		const LocationProbe = () => {
		// 			location_render_count += 1;
		// 			return h(
		// 				"div",
		// 				{ "data-location-probe": true },
		// 				preact_adapter.location.value.pathname,
		// 			);
		// 		};

		// 		try {
		// 			await act(async () => {
		// 				render_preact(
		// 					h(
		// 						"div",
		// 						{},
		// 						h(preact_adapter.VormaRootOutlet as any, {
		// 							idx: 0,
		// 						}),
		// 						h(LocationProbe, {}),
		// 					),
		// 					container,
		// 				);
		// 			});
		// 			await client.vormaNavigate("/preact-location-base");
		// 			await wait_for_dom(() => {
		// 				expect(
		// 					container.querySelector("[data-location-probe]")
		// 						?.textContent,
		// 				).toBe("/preact-location-base");
		// 			});
		// 			const renders_after_nav = location_render_count;

		// 			dispatch_route_change_event();
		// 			await vi.runAllTimersAsync();
		// 			expect(location_render_count).toBe(renders_after_nav);

		// 			window.history.pushState(null, "", "/preact-location-next");
		// 			dispatch_location_event();
		// 			await wait_for_dom(() => {
		// 				expect(
		// 					container.querySelector("[data-location-probe]")
		// 						?.textContent,
		// 				).toBe("/preact-location-next");
		// 			});
		// 			expect(location_render_count).toBe(renders_after_nav + 1);
		// 		} finally {
		// 			render_preact(null, container);
		// 			container.remove();
		// 		}
		// 	});
		// });

		// describe("solid location signal", () => {
		// 	it("updates only on location events, not route events", async () => {
		// 		const client = await init_adapter_runtime();
		// 		const solid_adapter = await import("vorma/solid");
		// 		vi.doMock("/solid-loc-root.js", () => ({
		// 			default: () => "",
		// 		}));
		// 		vi.spyOn(window, "fetch").mockImplementation(() =>
		// 			Promise.resolve(
		// 				create_route_data_response({
		// 					matched_patterns: ["/"],
		// 					import_urls: ["/solid-loc-root.js"],
		// 					export_keys: ["default"],
		// 					error_export_keys: [""],
		// 				} as any),
		// 			),
		// 		);

		// 		const container = document.createElement("div");
		// 		document.body.appendChild(container);
		// 		let location_render_count = 0;
		// 		const LocationProbe = () => {
		// 			const node = document.createElement("div");
		// 			node.setAttribute("data-location-probe", "true");
		// 			createEffect(() => {
		// 				location_render_count += 1;
		// 				node.textContent = solid_adapter.location().pathname;
		// 			});
		// 			return node;
		// 		};
		// 		const dispose = render_solid_tracked(() => {
		// 			return [
		// 				createComponent(solid_adapter.VormaRootOutlet as any, {
		// 					idx: 0,
		// 				}),
		// 				LocationProbe(),
		// 			];
		// 		}, container);

		// 		try {
		// 			await client.vormaNavigate("/solid-location-base");
		// 			await wait_for_dom(() => {
		// 				expect(
		// 					container.querySelector("[data-location-probe]")
		// 						?.textContent,
		// 				).toBe("/solid-location-base");
		// 			});
		// 			const renders_after_nav = location_render_count;

		// 			dispatch_route_change_event();
		// 			await vi.runAllTimersAsync();
		// 			expect(location_render_count).toBe(renders_after_nav);

		// 			window.history.pushState(null, "", "/solid-location-next");
		// 			dispatch_location_event();
		// 			await wait_for_dom(() => {
		// 				expect(
		// 					container.querySelector("[data-location-probe]")
		// 						?.textContent,
		// 				).toBe("/solid-location-next");
		// 			});
		// 			expect(location_render_count).toBe(renders_after_nav + 1);
		// 		} finally {
		// 			dispose();
		// 			container.remove();
		// 		}
		// 	});
		// });

		describe("cross-adapter data-only stability", () => {
			async function capture_data_only_mount_count(
				adapter_name: "react" | "preact" | "solid",
			): Promise<number> {
				const client = await init_adapter_runtime();
				const module_path = `/data-stable-${adapter_name}.js`;
				const href = `/data-stable-${adapter_name}`;

				if (adapter_name === "react") {
					const react_adapter = await import("vorma/react");
					let mount_count = 0;
					const StableRoot = () => {
						const ref = React.useRef<number | null>(null);
						if (ref.current === null) {
							mount_count += 1;
							ref.current = mount_count;
						}
						return React.createElement(
							"div",
							{},
							`root:${ref.current}`,
						);
					};
					vi.doMock(module_path, () => ({
						default: StableRoot,
					}));
					vi.spyOn(window, "fetch")
						.mockResolvedValueOnce(
							create_route_data_response({
								matched_patterns: ["/"],
								import_urls: [module_path],
								export_keys: ["default"],
								error_export_keys: [""],
								loaders_data: [{ value: "a" }],
							} as any),
						)
						.mockResolvedValueOnce(
							create_route_data_response({
								matched_patterns: ["/"],
								import_urls: [module_path],
								export_keys: ["default"],
								error_export_keys: [""],
								loaders_data: [{ value: "b" }],
							} as any),
						);

					const container = document.createElement("div");
					document.body.appendChild(container);
					const root = createRoot(container);
					flushSync(() => {
						root.render(
							React.createElement(
								react_adapter.VormaRootOutlet as any,
								{ idx: 0 },
							),
						);
					});
					try {
						await client.vormaNavigate(href);
						await vi.runAllTimersAsync();
						await client.revalidate();
						await vi.runAllTimersAsync();
						return mount_count;
					} finally {
						flushSync(() => root.unmount());
						container.remove();
					}
				}

				if (adapter_name === "preact") {
					const preact_adapter = await import("vorma/preact");
					let mount_count = 0;
					const StableRoot = () => {
						use_preact_state(() => {
							mount_count += 1;
							return mount_count;
						});
						return h("div", {}, "root");
					};
					vi.doMock(module_path, () => ({
						default: StableRoot,
					}));
					vi.spyOn(window, "fetch")
						.mockResolvedValueOnce(
							create_route_data_response({
								matched_patterns: ["/"],
								import_urls: [module_path],
								export_keys: ["default"],
								error_export_keys: [""],
								loaders_data: [{ value: "a" }],
							} as any),
						)
						.mockResolvedValueOnce(
							create_route_data_response({
								matched_patterns: ["/"],
								import_urls: [module_path],
								export_keys: ["default"],
								error_export_keys: [""],
								loaders_data: [{ value: "b" }],
							} as any),
						);

					const container = document.createElement("div");
					document.body.appendChild(container);
					try {
						await act(async () => {
							render_preact(
								h(preact_adapter.VormaRootOutlet as any, {
									idx: 0,
								}),
								container,
							);
						});
						await client.vormaNavigate(href);
						await vi.runAllTimersAsync();
						await client.revalidate();
						await vi.runAllTimersAsync();
						return mount_count;
					} finally {
						render_preact(null, container);
						container.remove();
					}
				}

				// solid
				const solid_adapter = await import("vorma/solid");
				let mount_count = 0;
				const StableRoot = () => {
					mount_count += 1;
					return "solid-root";
				};
				vi.doMock(module_path, () => ({
					default: StableRoot,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/"],
							import_urls: [module_path],
							export_keys: ["default"],
							error_export_keys: [""],
							loaders_data: [{ value: "a" }],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/"],
							import_urls: [module_path],
							export_keys: ["default"],
							error_export_keys: [""],
							loaders_data: [{ value: "b" }],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return createComponent(
						solid_adapter.VormaRootOutlet as any,
						{ idx: 0 },
					);
				}, container);
				try {
					await client.vormaNavigate(href);
					await vi.runAllTimersAsync();
					await client.revalidate();
					await vi.runAllTimersAsync();
					return mount_count;
				} finally {
					dispose();
					container.remove();
				}
			}

			it("preserves stable component mounts across data-only updates (react)", async () => {
				const count = await capture_data_only_mount_count("react");
				expect(count).toBe(1);
			});

			it("preserves stable component mounts across data-only updates (preact)", async () => {
				const count = await capture_data_only_mount_count("preact");
				expect(count).toBe(1);
			});

			it("preserves stable component mounts across data-only updates (solid)", async () => {
				const count = await capture_data_only_mount_count("solid");
				expect(count).toBe(1);
			});
		});
	});

	// ─── Assertion 29: Component Identity ────────────────────

	describe("component identity (assertion 29)", () => {
		describe("react", () => {
			it("keeps parent instance when only child route changes", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				let parent_mount_count = 0;
				let parent_unmount_count = 0;
				const ChildA = () => React.createElement("div", {}, "child-a");
				const ChildB = () => React.createElement("div", {}, "child-b");
				const Parent = (props: { Outlet: any }) => {
					const ref = React.useRef<number | null>(null);
					if (ref.current === null) {
						parent_mount_count += 1;
						ref.current = parent_mount_count;
					}
					React.useEffect(() => {
						return () => {
							parent_unmount_count += 1;
						};
					}, []);
					const [draft, set_draft] = React.useState("initial");
					return React.createElement(
						"section",
						{},
						React.createElement(
							"div",
							{ "data-draft": true },
							draft,
						),
						React.createElement("input", {
							value: draft,
							onInput: (e: React.FormEvent<HTMLInputElement>) =>
								set_draft(e.currentTarget.value),
						}),
						React.createElement(props.Outlet as any, {}),
					);
				};
				vi.doMock("/react-parent.js", () => ({
					default: Parent,
				}));
				vi.doMock("/react-child-a.js", () => ({
					default: ChildA,
				}));
				vi.doMock("/react-child-b.js", () => ({
					default: ChildB,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/react-parent.js",
								"/react-child-a.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/react-parent.js",
								"/react-child-b.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);
				flushSync(() => {
					root.render(
						React.createElement(
							react_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						),
					);
				});

				try {
					await client.vormaNavigate("/react-parent-a");
					await wait_for_dom(() => {
						expect(container.textContent).toContain("child-a");
					});

					const input = container.querySelector("input");
					if (!(input instanceof HTMLInputElement)) {
						throw new Error("Expected input element.");
					}
					input.value = "typed-value";
					input.dispatchEvent(new Event("input", { bubbles: true }));
					await wait_for_dom(() => {
						expect(
							container.querySelector("[data-draft]")
								?.textContent,
						).toBe("typed-value");
					});

					await client.vormaNavigate("/react-parent-b");
					await wait_for_dom(() => {
						expect(container.textContent).toContain("child-b");
					});

					expect(
						container.querySelector("[data-draft]")?.textContent,
					).toBe("typed-value");
					expect(parent_mount_count).toBe(1);
					expect(parent_unmount_count).toBe(0);
				} finally {
					root.unmount();
					container.remove();
				}
			});

			it("uses freshest module on later commits with same key", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				const ChildA = vi.fn(() =>
					React.createElement("div", {}, "child-a"),
				);
				const ChildB = vi.fn(() =>
					React.createElement("div", {}, "child-b"),
				);
				const Parent = (props: { Outlet: any }) =>
					React.createElement(props.Outlet as any, {});
				vi.doMock("/react-fresh-parent.js", () => ({
					default: Parent,
				}));
				vi.doMock("/react-fresh-child.js", () => ({
					default: ChildA,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/react-fresh-parent.js",
								"/react-fresh-child.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/react-fresh-parent.js",
								"/react-fresh-child.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);
				flushSync(() => {
					root.render(
						React.createElement(
							react_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						),
					);
				});

				try {
					await client.vormaNavigate("/react-fresh-a");
					await wait_for_dom(() => {
						expect(ChildA).toHaveBeenCalled();
					});

					vi.doMock("/react-fresh-child.js", () => ({
						default: ChildB,
					}));

					await client.vormaNavigate("/react-fresh-b");
					await wait_for_dom(() => {
						expect(ChildB).toHaveBeenCalled();
					});
				} finally {
					root.unmount();
					container.remove();
				}
			});

			it("remounts child when export key changes", async () => {
				const client = await init_adapter_runtime();
				const react_adapter = await import("vorma/react");
				let child_mount_count = 0;
				let child_cleanup_count = 0;
				const ChildComp = () => {
					const ref = React.useRef<number | null>(null);
					if (ref.current === null) {
						child_mount_count += 1;
						ref.current = child_mount_count;
					}
					React.useEffect(() => {
						return () => {
							child_cleanup_count += 1;
						};
					}, []);
					return React.createElement(
						"div",
						{},
						`child:${ref.current}`,
					);
				};
				const Parent = (props: { Outlet: any }) =>
					React.createElement(props.Outlet as any, {});
				vi.doMock("/react-export-parent.js", () => ({
					default: Parent,
				}));
				vi.doMock("/react-export-child.js", () => ({
					childA: ChildComp,
					childB: ChildComp,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/react-export-parent.js",
								"/react-export-child.js",
							],
							export_keys: ["default", "childA"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/react-export-parent.js",
								"/react-export-child.js",
							],
							export_keys: ["default", "childB"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				const root = createRoot(container);
				flushSync(() => {
					root.render(
						React.createElement(
							react_adapter.VormaRootOutlet as any,
							{ idx: 0 },
						),
					);
				});

				try {
					await client.vormaNavigate("/react-export-a");
					await wait_for_dom(() => {
						expect(container.textContent).toContain("child:1");
					});

					await client.vormaNavigate("/react-export-b");
					await wait_for_dom(() => {
						expect(container.textContent).toContain("child:2");
					});
					expect(child_cleanup_count).toBeGreaterThanOrEqual(1);
				} finally {
					root.unmount();
					container.remove();
				}
			});
		});

		describe("solid", () => {
			it("remounts child when export key changes", async () => {
				const client = await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				let child_a_mount = 0;
				let child_a_cleanup = 0;
				let child_b_mount = 0;
				const ChildA = () => {
					child_a_mount += 1;
					onCleanup(() => {
						child_a_cleanup += 1;
					});
					return "solid-child-a";
				};
				const ChildB = () => {
					child_b_mount += 1;
					return "solid-child-b";
				};
				const Parent = (props: { Outlet: any }) => props.Outlet({});
				vi.doMock("/solid-export-parent.js", () => ({
					default: Parent,
				}));
				vi.doMock("/solid-export-child.js", () => ({
					childA: ChildA,
					childB: ChildB,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/solid-export-parent.js",
								"/solid-export-child.js",
							],
							export_keys: ["default", "childA"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/solid-export-parent.js",
								"/solid-export-child.js",
							],
							export_keys: ["default", "childB"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return createComponent(
						solid_adapter.VormaRootOutlet as any,
						{ idx: 0 },
					);
				}, container);

				try {
					await client.vormaNavigate("/solid-export-a");
					await wait_for_dom(() => {
						expect(child_a_mount).toBeGreaterThan(0);
					});

					await client.vormaNavigate("/solid-export-b");
					await wait_for_dom(() => {
						expect(child_b_mount).toBeGreaterThan(0);
					});
					expect(child_a_cleanup).toBeGreaterThanOrEqual(1);
				} finally {
					dispose();
					container.remove();
				}
			});

			it("remounts child when matched pattern changes even if component identity is reused", async () => {
				const client = await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				let mount_count = 0;
				let cleanup_count = 0;
				const SharedChild = () => {
					mount_count += 1;
					onCleanup(() => {
						cleanup_count += 1;
					});
					return "solid-shared-child";
				};
				const Parent = (props: { Outlet: any }) => props.Outlet({});
				vi.doMock("/solid-pattern-parent.js", () => ({
					default: Parent,
				}));
				vi.doMock("/solid-pattern-child.js", () => ({
					default: SharedChild,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child-a"],
							import_urls: [
								"/solid-pattern-parent.js",
								"/solid-pattern-child.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, { value: "a" }],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child-b"],
							import_urls: [
								"/solid-pattern-parent.js",
								"/solid-pattern-child.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, { value: "b" }],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return createComponent(
						solid_adapter.VormaRootOutlet as any,
						{ idx: 0 },
					);
				}, container);

				try {
					await client.vormaNavigate("/solid-pattern-a");
					await wait_for_dom(() => {
						expect(mount_count).toBe(1);
					});

					await client.vormaNavigate("/solid-pattern-b");
					await wait_for_dom(() => {
						expect(mount_count).toBeGreaterThanOrEqual(2);
					});
					expect(cleanup_count).toBeGreaterThanOrEqual(1);
				} finally {
					dispose();
					container.remove();
				}
			});

			it("keeps parent instance when only child route changes", async () => {
				const client = await init_adapter_runtime();
				const solid_adapter = await import("vorma/solid");
				let parent_mount = 0;
				let parent_cleanup = 0;
				let set_parent_draft: ((v: string) => void) | undefined;
				let read_parent_draft: (() => string) | undefined;
				const ChildA = () => "solid-child-a";
				const ChildB = () => "solid-child-b";
				const Parent = (props: { Outlet: any }) => {
					parent_mount += 1;
					onCleanup(() => {
						parent_cleanup += 1;
					});
					let draft = "initial";
					set_parent_draft = (v: string) => {
						draft = v;
					};
					read_parent_draft = () => draft;
					return createComponent(props.Outlet as any, {});
				};
				vi.doMock("/solid-parent-state.js", () => ({
					default: Parent,
				}));
				vi.doMock("/solid-parent-child-a.js", () => ({
					default: ChildA,
				}));
				vi.doMock("/solid-parent-child-b.js", () => ({
					default: ChildB,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/solid-parent-state.js",
								"/solid-parent-child-a.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/solid-parent-state.js",
								"/solid-parent-child-b.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				const dispose = render_solid_tracked(() => {
					return createComponent(
						solid_adapter.VormaRootOutlet as any,
						{ idx: 0 },
					);
				}, container);

				try {
					await client.vormaNavigate("/solid-parent-a");
					await wait_for_dom(() => {
						expect(parent_mount).toBeGreaterThan(0);
						expect(read_parent_draft?.()).toBe("initial");
					});

					set_parent_draft?.("typed-value");
					expect(read_parent_draft?.()).toBe("typed-value");

					await client.vormaNavigate("/solid-parent-b");
					await vi.runAllTimersAsync();

					expect(read_parent_draft?.()).toBe("typed-value");
					expect(parent_mount).toBe(1);
					expect(parent_cleanup).toBe(0);
				} finally {
					dispose();
					container.remove();
				}
			});
		});

		describe("preact", () => {
			it("remounts child when export key changes", async () => {
				const client = await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				let child_mount_count = 0;
				let child_cleanup_count = 0;
				const ChildComp = () => {
					const ref = use_preact_ref<number | null>(null);
					if (ref.current === null) {
						child_mount_count += 1;
						ref.current = child_mount_count;
					}
					use_preact_effect(() => {
						return () => {
							child_cleanup_count += 1;
						};
					}, []);
					return h("div", {}, `child:${ref.current}`);
				};
				const Parent = (props: { Outlet: any }) =>
					h(props.Outlet as any, {});
				vi.doMock("/preact-export-parent.js", () => ({
					default: Parent,
				}));
				vi.doMock("/preact-export-child.js", () => ({
					childA: ChildComp,
					childB: ChildComp,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/preact-export-parent.js",
								"/preact-export-child.js",
							],
							export_keys: ["default", "childA"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/preact-export-parent.js",
								"/preact-export-child.js",
							],
							export_keys: ["default", "childB"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				try {
					render_preact(
						h(preact_adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						container,
					);

					await client.vormaNavigate("/preact-export-a");
					await wait_for_dom(() => {
						expect(container.textContent).toContain("child:1");
					});

					await client.vormaNavigate("/preact-export-b");
					await wait_for_dom(() => {
						expect(container.textContent).toContain("child:2");
					});
					expect(child_mount_count).toBeGreaterThanOrEqual(2);
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});

			it("keeps parent instance when only child route changes", async () => {
				const client = await init_adapter_runtime();
				const preact_adapter = await import("vorma/preact");
				let parent_mount_count = 0;
				let parent_unmount_count = 0;
				const ChildA = () => h("div", {}, "preact-child-a");
				const ChildB = () => h("div", {}, "preact-child-b");
				const Parent = (props: { Outlet: any }) => {
					const ref = use_preact_ref<number | null>(null);
					if (ref.current === null) {
						parent_mount_count += 1;
						ref.current = parent_mount_count;
					}
					use_preact_effect(() => {
						return () => {
							parent_unmount_count += 1;
						};
					}, []);
					const [draft, set_draft] = use_preact_state("initial");
					return h(
						"section",
						{},
						h("div", { "data-preact-parent-draft": true }, draft),
						h("input", {
							value: draft,
							onInput: (e: Event) => {
								set_draft((e.target as HTMLInputElement).value);
							},
						}),
						h(props.Outlet as any, {}),
					);
				};
				vi.doMock("/preact-parent-state.js", () => ({
					default: Parent,
				}));
				vi.doMock("/preact-parent-child-a.js", () => ({
					default: ChildA,
				}));
				vi.doMock("/preact-parent-child-b.js", () => ({
					default: ChildB,
				}));
				vi.spyOn(window, "fetch")
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/preact-parent-state.js",
								"/preact-parent-child-a.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					)
					.mockResolvedValueOnce(
						create_route_data_response({
							matched_patterns: ["/parent", "/parent/child"],
							import_urls: [
								"/preact-parent-state.js",
								"/preact-parent-child-b.js",
							],
							export_keys: ["default", "default"],
							error_export_keys: ["", ""],
							loaders_data: [{}, {}],
						} as any),
					);

				const container = document.createElement("div");
				document.body.appendChild(container);
				try {
					render_preact(
						h(preact_adapter.VormaRootOutlet as any, {
							idx: 0,
						}),
						container,
					);

					await client.vormaNavigate("/preact-parent-a");
					await wait_for_dom(() => {
						expect(container.textContent).toContain(
							"preact-child-a",
						);
					});

					const input = container.querySelector("input");
					if (!(input instanceof HTMLInputElement)) {
						throw new Error("Expected input.");
					}
					input.value = "typed-value";
					input.dispatchEvent(new Event("input", { bubbles: true }));
					await wait_for_dom(() => {
						expect(
							container.querySelector(
								"[data-preact-parent-draft]",
							)?.textContent,
						).toBe("typed-value");
					});

					await client.vormaNavigate("/preact-parent-b");
					await wait_for_dom(() => {
						expect(container.textContent).toContain(
							"preact-child-b",
						);
					});

					expect(
						container.querySelector("[data-preact-parent-draft]")
							?.textContent,
					).toBe("typed-value");
					expect(parent_mount_count).toBe(1);
					expect(parent_unmount_count).toBe(0);
				} finally {
					render_preact(null, container);
					container.remove();
				}
			});
		});
	});
});
