import React from "react";
import { createRoot as createReactRoot } from "react-dom/client";
import { act } from "react-dom/test-utils";
import { h, render as renderPreact } from "preact";
import { describe, expect, it, vi } from "vitest";

import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";
import {
	describeNavigationTestSuite,
	setupGlobalVormaContext,
} from "./client.test.helpers.ts";

type AdapterName = "react" | "preact" | "solid";

type AdapterSnapshot = {
	loadersData: any;
	clientLoadersData: any;
	routerData: any;
	location: any;
};

type AdapterHarness = {
	getSnapshot: () => AdapterSnapshot;
	cleanup: () => void;
};

type OutletRenderHarness = {
	getTextContent: () => string;
	cleanup: () => void;
};

function setupBaseAdapterState() {
	setupGlobalVormaContext({
		hasRootData: true,
		loadersData: ["root-initial"],
		clientLoadersData: ["client-initial"],
		matchedPatterns: ["/initial"],
		params: { id: "0" },
		splatValues: ["initial"],
		buildID: "build-initial",
		importURLs: [],
		exportKeys: [],
		activeComponents: [],
		activeErrorBoundary: undefined,
	});
}

async function mountAdapterHarness(adapter: AdapterName): Promise<AdapterHarness> {
	if (adapter === "react") {
		const reactAdapter = await import("../../react/src/react.tsx");
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createReactRoot(container);

		let latestSnapshot: AdapterSnapshot = {
			loadersData: null,
			clientLoadersData: null,
			routerData: null,
			location: null,
		};

		function Probe() {
			latestSnapshot = {
				loadersData: reactAdapter.useLoadersData(),
				clientLoadersData: reactAdapter.useClientLoadersData(),
				routerData: reactAdapter.useRouterData(),
				location: reactAdapter.useLocation(),
			};
			return null;
		}

		await act(async () => {
			root.render(
				React.createElement(
					React.Fragment,
					null,
					React.createElement(reactAdapter.VormaRootOutlet, {}),
					React.createElement(Probe, {}),
				),
			);
		});

		return {
			getSnapshot: () => latestSnapshot,
			cleanup: () => {
				act(() => {
					root.unmount();
				});
				container.remove();
			},
		};
	}

	if (adapter === "preact") {
		const preactAdapter = await import("../../preact/src/preact.tsx");
		const container = document.createElement("div");
		document.body.appendChild(container);
		renderPreact(h(preactAdapter.VormaRootOutlet, {}), container);

		return {
			getSnapshot: () => ({
				loadersData: preactAdapter.loadersData.value,
				clientLoadersData: preactAdapter.clientLoadersData.value,
				routerData: preactAdapter.routerData.value,
				location: preactAdapter.location.value,
			}),
			cleanup: () => {
				renderPreact(null, container);
				container.remove();
			},
		};
	}

	const solidAdapter = await import("../../solid/src/solid.tsx");
	const { render: renderSolid } = await import("solid-js/web");
	const container = document.createElement("div");
	document.body.appendChild(container);
	const dispose = renderSolid(() => solidAdapter.VormaRootOutlet({}), container);

	return {
		getSnapshot: () => ({
			loadersData: solidAdapter.loadersData(),
			clientLoadersData: solidAdapter.clientLoadersData(),
			routerData: solidAdapter.routerData(),
			location: solidAdapter.location(),
		}),
		cleanup: () => {
			dispose();
			container.remove();
		},
	};
}

async function mountAdapterOutlet(adapter: AdapterName): Promise<OutletRenderHarness> {
	if (adapter === "react") {
		const reactAdapter = await import("../../react/src/react.tsx");
		const container = document.createElement("div");
		document.body.appendChild(container);
		const root = createReactRoot(container);

		await act(async () => {
			root.render(React.createElement(reactAdapter.VormaRootOutlet, {}));
		});

		return {
			getTextContent: () => container.textContent || "",
			cleanup: () => {
				act(() => {
					root.unmount();
				});
				container.remove();
			},
		};
	}

	if (adapter === "preact") {
		const preactAdapter = await import("../../preact/src/preact.tsx");
		const container = document.createElement("div");
		document.body.appendChild(container);
		renderPreact(h(preactAdapter.VormaRootOutlet, {}), container);

		return {
			getTextContent: () => container.textContent || "",
			cleanup: () => {
				renderPreact(null, container);
				container.remove();
			},
		};
	}

	const solidAdapter = await import("../../solid/src/solid.tsx");
	const { render: renderSolid } = await import("solid-js/web");
	const container = document.createElement("div");
	document.body.appendChild(container);
	const dispose = renderSolid(() => solidAdapter.VormaRootOutlet({}), container);

	return {
		getTextContent: () => container.textContent || "",
		cleanup: () => {
			dispose();
			container.remove();
		},
	};
}

describeNavigationTestSuite(() => {
	describe("UI root adapter parity conformance", () => {
		it("FEC-UI-001_FE-UI-001_nested_outlet_error_boundary_and_fallback_behavior_are_equivalent_across_react_preact_and_solid", async () => {
			for (const adapter of ["react", "preact", "solid"] as const) {
				vi.resetModules();

				const nestedCalls = { root: 0, child: 0 };
				const NestedRoot = (props: any) => {
					nestedCalls.root += 1;
					return props.Outlet({});
				};
				const NestedChild = () => {
					nestedCalls.child += 1;
					return "nested-child";
				};

				setupGlobalVormaContext({
					hasRootData: true,
					loadersData: ["root-data", "child-data"],
					clientLoadersData: [],
					matchedPatterns: ["/", "/child"],
					params: {},
					splatValues: [],
					buildID: "build-ui-001",
					importURLs: ["/root.js", "/child.js"],
					exportKeys: ["default", "default"],
					activeComponents: [NestedRoot, NestedChild],
					outermostError: undefined,
					outermostErrorIdx: undefined,
					activeErrorBoundary: undefined,
				});

				const nestedHarness = await mountAdapterOutlet(adapter);
				expect(nestedHarness.getTextContent()).toContain("nested-child");
				expect(nestedCalls.root).toBeGreaterThan(0);
				expect(nestedCalls.child).toBeGreaterThan(0);
				nestedHarness.cleanup();

				const errorCalls = { root: 0, child: 0, boundary: 0 };
				const ErrorRoot = (props: any) => {
					errorCalls.root += 1;
					return props.Outlet({});
				};
				const ErrorChild = () => {
					errorCalls.child += 1;
					return "should-not-render";
				};
				const ErrorBoundary = ({ error }: { error: string }) => {
					errorCalls.boundary += 1;
					return `error-boundary:${error}`;
				};

				setupGlobalVormaContext({
					hasRootData: true,
					loadersData: ["root-data", "child-data"],
					clientLoadersData: [],
					matchedPatterns: ["/", "/child"],
					params: {},
					splatValues: [],
					buildID: "build-ui-001",
					importURLs: ["/root.js", "/child.js"],
					exportKeys: ["default", "default"],
					activeComponents: [ErrorRoot, ErrorChild],
					outermostError: "boom",
					outermostErrorIdx: 1,
					activeErrorBoundary: ErrorBoundary,
				});

				const errorHarness = await mountAdapterOutlet(adapter);
				const errorText = errorHarness.getTextContent();
				expect(errorText).toContain("error-boundary:boom");
				expect(errorText).not.toContain("should-not-render");
				expect(errorCalls.root).toBeGreaterThan(0);
				expect(errorCalls.child).toBe(0);
				expect(errorCalls.boundary).toBeGreaterThan(0);
				errorHarness.cleanup();

				const fallbackCalls = { child: 0 };
				const FallbackChild = () => {
					fallbackCalls.child += 1;
					return "fallback-child";
				};

				setupGlobalVormaContext({
					hasRootData: true,
					loadersData: ["root-data", "child-data"],
					clientLoadersData: [],
					matchedPatterns: ["/", "/child"],
					params: {},
					splatValues: [],
					buildID: "build-ui-001",
					importURLs: ["/root.js", "/child.js"],
					exportKeys: ["default", "default"],
					activeComponents: [undefined, FallbackChild],
					outermostError: undefined,
					outermostErrorIdx: undefined,
					activeErrorBoundary: undefined,
				});

				const fallbackHarness = await mountAdapterOutlet(adapter);
				expect(fallbackHarness.getTextContent()).toContain(
					"fallback-child",
				);
				expect(fallbackCalls.child).toBeGreaterThan(0);
				fallbackHarness.cleanup();
			}
		});

		it("FEC-UI-002_FE-UI-002_route_change_syncs_snapshots_from_client_global_across_adapters", async () => {
			const expected = {
				loadersData: ["root-next"],
				clientLoadersData: ["client-next"],
				routerData: {
					buildID: "build-next",
					matchedPatterns: ["/users/:id"],
					splatValues: ["profile"],
					params: { id: "42" },
					rootData: "root-next",
				},
			};

			for (const adapter of ["react", "preact", "solid"] as const) {
				vi.resetModules();
				setupBaseAdapterState();

				const harness = await mountAdapterHarness(adapter);

				__vormaClientGlobal.set("loadersData", expected.loadersData);
				__vormaClientGlobal.set(
					"clientLoadersData",
					expected.clientLoadersData,
				);
				__vormaClientGlobal.set("buildID", expected.routerData.buildID);
				__vormaClientGlobal.set(
					"matchedPatterns",
					expected.routerData.matchedPatterns,
				);
				__vormaClientGlobal.set("params", expected.routerData.params);
				__vormaClientGlobal.set(
					"splatValues",
					expected.routerData.splatValues,
				);
				__vormaClientGlobal.set("hasRootData", true);
				__vormaClientGlobal.set("importURLs", []);
				__vormaClientGlobal.set("exportKeys", []);
				__vormaClientGlobal.set("activeComponents", []);
				__vormaClientGlobal.set("activeErrorBoundary", undefined);

				await act(async () => {
					window.dispatchEvent(
						new CustomEvent("vorma:route-change", {
							detail: { __scrollState: { x: 9, y: 11 } },
						}),
					);
				});

				const snapshot = harness.getSnapshot();
				expect(snapshot.loadersData).toEqual(expected.loadersData);
				expect(snapshot.clientLoadersData).toEqual(
					expected.clientLoadersData,
				);
				expect(snapshot.routerData).toEqual(expected.routerData);
				harness.cleanup();
			}
		});

		it("FEC-UI-003_FE-UI-003_scroll_apply_timing_is_observed_for_react_preact_and_solid", async () => {
			for (const adapter of ["react", "preact", "solid"] as const) {
				vi.resetModules();
				setupBaseAdapterState();

				const rafCallbacks: Array<(time: number) => void> = [];
				const rafSpy = vi
					.spyOn(window, "requestAnimationFrame")
					.mockImplementation((cb: any) => {
						rafCallbacks.push(cb);
						return rafCallbacks.length;
					});

				const harness = await mountAdapterHarness(adapter);

				vi.mocked(window.scrollTo).mockClear();
				await act(async () => {
					window.dispatchEvent(
						new CustomEvent("vorma:route-change", {
							detail: { __scrollState: { x: 21, y: 34 } },
						}),
					);
				});

				if (rafSpy.mock.calls.length < 1) {
					throw new Error(
						`expected requestAnimationFrame scheduling for adapter=${adapter}`,
					);
				}
				expect(window.scrollTo).not.toHaveBeenCalled();
				for (const cb of rafCallbacks) {
					cb(0);
				}
				expect(window.scrollTo).toHaveBeenCalledWith(21, 34);
				harness.cleanup();
			}
		});

		it("FEC-UI-003_FE-UI-004_location_state_contract_is_equivalent_across_react_preact_and_solid", async () => {
			for (const adapter of ["react", "preact", "solid"] as const) {
				vi.resetModules();
				setupBaseAdapterState();

				const harness = await mountAdapterHarness(adapter);

				window.history.replaceState(
					{ from: adapter },
					"",
					"http://localhost:3000/adapter-location?tab=details#top",
				);
				await act(async () => {
					window.dispatchEvent(new CustomEvent("vorma:location"));
				});

				const snapshot = harness.getSnapshot();
				expect(snapshot.location.pathname).toBe("/adapter-location");
				expect(snapshot.location.search).toBe("?tab=details");
				expect(snapshot.location.hash).toBe("#top");
				harness.cleanup();
			}
		});
	});
});
