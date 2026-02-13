import React, { act } from "react";
import { createRoot } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

type RuntimeRenderState = {
	loadersData: unknown[];
	clientLoadersData: unknown[];
	outermostError: string | undefined;
	outermostErrorIdx: number | undefined;
	activeComponents: Array<unknown> | null;
	activeErrorBoundary: unknown;
	importURLs: string[];
	exportKeys: string[];
};

const {
	applyScrollStateSpy,
	getClientRuntimeRenderStateSpy,
	addRouteChangeListenerSpy,
	addLocationListenerSpy,
	getLocationSpy,
	getRouterDataSpy,
	__dispatchRouteChange,
	__resetRuntimeState,
	__setRuntimeState,
} = vi.hoisted(() => {
	let currentRuntimeState: RuntimeRenderState = {
		loadersData: [],
		clientLoadersData: [],
		outermostError: undefined,
		outermostErrorIdx: undefined,
		activeComponents: [],
		activeErrorBoundary: undefined,
		importURLs: [],
		exportKeys: [],
	};
	let routeChangeListener: ((event: any) => void) | null = null;
	let locationListener: (() => void) | null = null;
	const locationState = {
		pathname: "/",
		search: "",
		hash: "",
		href: "http://localhost/",
		key: "k1",
		state: null,
	};
	const routerState = {
		buildID: "1",
		matchedPatterns: ["/"],
		splatValues: [],
		params: {},
		rootData: null,
	};

	return {
		applyScrollStateSpy: vi.fn(),
		getClientRuntimeRenderStateSpy: vi.fn(() => currentRuntimeState),
		addRouteChangeListenerSpy: vi.fn((listener: (event: any) => void) => {
			routeChangeListener = listener;
			return () => {
				routeChangeListener = null;
			};
		}),
		addLocationListenerSpy: vi.fn((listener: () => void) => {
			locationListener = listener;
			return () => {
				locationListener = null;
			};
		}),
		getLocationSpy: vi.fn(() => locationState),
		getRouterDataSpy: vi.fn(() => routerState),
		__setRuntimeState(nextState: RuntimeRenderState) {
			currentRuntimeState = nextState;
		},
		__dispatchRouteChange(event: any) {
			if (!routeChangeListener) {
				throw new Error("Route change listener was not registered.");
			}
			routeChangeListener(event);
		},
		__dispatchLocation() {
			if (!locationListener) {
				throw new Error("Location listener was not registered.");
			}
			locationListener();
		},
		__resetRuntimeState() {
			currentRuntimeState = {
				loadersData: [],
				clientLoadersData: [],
				outermostError: undefined,
				outermostErrorIdx: undefined,
				activeComponents: [],
				activeErrorBoundary: undefined,
				importURLs: [],
				exportKeys: [],
			};
			routeChangeListener = null;
			locationListener = null;
		},
	};
});

vi.mock("vorma/client", () => {
	return {
		__applyScrollState: applyScrollStateSpy,
		__getClientRuntimeRenderState: getClientRuntimeRenderStateSpy,
		addLocationListener: addLocationListenerSpy,
		addRouteChangeListener: addRouteChangeListenerSpy,
		getLocation: getLocationSpy,
		getRouterData: getRouterDataSpy,
	};
});

describe("react root adapter", () => {
	let originalRAF: typeof window.requestAnimationFrame;
	let originalActEnvironment: unknown;
	let container: HTMLDivElement;

	beforeEach(() => {
		vi.resetModules();
		vi.clearAllMocks();
		__resetRuntimeState();

		__setRuntimeState({
			loadersData: [{}],
			clientLoadersData: [],
			outermostError: undefined,
			outermostErrorIdx: undefined,
			activeComponents: [
				(props: { idx: number }) => {
					return React.createElement("div", {}, `first:${props.idx}`);
				},
			],
			activeErrorBoundary: undefined,
			importURLs: ["/first.js"],
			exportKeys: ["default"],
		});

		originalRAF = window.requestAnimationFrame;
		originalActEnvironment = (globalThis as any).IS_REACT_ACT_ENVIRONMENT;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = true;
		window.requestAnimationFrame = (callback) => {
			callback(0);
			return 0;
		};
		container = document.createElement("div");
		document.body.appendChild(container);
	});

	afterEach(() => {
		window.requestAnimationFrame = originalRAF;
		(globalThis as any).IS_REACT_ACT_ENVIRONMENT = originalActEnvironment;
		container?.remove();
		__resetRuntimeState();
	});

	it("syncs runtime state on route changes and applies scroll state", async () => {
		const { VormaRootOutlet } = await import("./react.tsx");
		const root = createRoot(container);

		await act(async () => {
			root.render(React.createElement(VormaRootOutlet, { idx: 0 }));
		});
		expect(container.textContent).toContain("first:0");
		expect(addRouteChangeListenerSpy).toHaveBeenCalledTimes(1);
		expect(addLocationListenerSpy).toHaveBeenCalledTimes(1);

		__setRuntimeState({
			loadersData: [{}],
			clientLoadersData: [],
			outermostError: undefined,
			outermostErrorIdx: undefined,
			activeComponents: [
				(props: { idx: number }) => {
					return React.createElement(
						"div",
						{},
						`second:${props.idx}`,
					);
				},
			],
			activeErrorBoundary: undefined,
			importURLs: ["/second.js"],
			exportKeys: ["default"],
		});

		await act(async () => {
			__dispatchRouteChange({
				detail: {
					__scrollState: { x: 7, y: 11 },
				},
			});
		});

		expect(container.textContent).toContain("second:0");
		expect(applyScrollStateSpy).toHaveBeenCalledWith({ x: 7, y: 11 });

		await act(async () => {
			root.unmount();
		});
	});
});
