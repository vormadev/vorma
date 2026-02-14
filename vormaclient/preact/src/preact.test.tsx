import { h } from "preact";
import { act } from "preact/test-utils";
import { render } from "preact";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const mockedBranchState = {
	isErrorIdx: false,
	shouldFallbackOutlet: false,
	nextRouteKey: "next-route",
	currentRouteKey: "current-route",
	currentComponent: undefined,
	errorComponent: undefined,
};

vi.mock("vorma/client", () => {
	return {
		addLocationListener: vi.fn(() => {
			return () => {};
		}),
		addRouteChangeListener: vi.fn(() => {
			return () => {};
		}),
	};
});

vi.mock("vorma/client/__internal", () => {
	return {
		applyScrollState: vi.fn(),
		areRouteOutletBranchInputsEqualByIdentity: vi.fn(() => false),
		areRouteOutletLocationsEqual: vi.fn(() => true),
		buildCurrentRouteOutletLocationState: vi.fn(() => {
			return {
				pathname: "/",
				search: "",
				hash: "",
				state: null,
			};
		}),
		buildInitialRouteOutletNavigationState: vi.fn(() => {
			return {
				loadersData: [],
				clientLoadersData: [],
				routerData: {
					buildID: "build-id",
					matchedPatterns: [],
					splatValues: [],
					params: {},
					rootData: null,
				},
				outermostError: undefined,
				outermostErrorIdx: undefined,
				activeComponents: [],
				activeErrorBoundary: undefined,
				importURLs: [],
				exportKeys: [],
			};
		}),
		buildNextRouteOutletNavigationState: vi.fn((state: unknown) => state),
		buildRouteOutletBranchInputState: vi.fn(() => {
			return {
				loaderCount: 0,
				outermostErrorIdx: undefined,
				activeComponents: [],
				activeErrorBoundary: undefined,
				importURLs: [],
				exportKeys: [],
			};
		}),
		buildRouteOutletBranchState: vi.fn(() => mockedBranchState),
	};
});

describe("preact adapter root outlet empty branch", () => {
	beforeEach(() => {
		vi.resetModules();
	});

	afterEach(() => {
		document.body.innerHTML = "";
	});

	it("renders no element when no component and no fallback branch are available", async () => {
		const adapter = await import("./preact.tsx");
		const container = document.createElement("div");
		document.body.appendChild(container);

		await act(async () => {
			render(h(adapter.VormaRootOutlet, { idx: 0 }), container);
		});

		expect(container.textContent).toBe("");
		expect(container.childElementCount).toBe(0);
	});
});
