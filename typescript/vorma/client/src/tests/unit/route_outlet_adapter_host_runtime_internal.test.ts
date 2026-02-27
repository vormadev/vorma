import { describe, expect, it } from "vitest";
import type { RouteOutletBranchInputState } from "../../../src/runtime.ts";
import {
	buildTypedAdapterRouteComponentMountProps,
	resolveRouteOutletAdapterRenderModel,
	shouldRemountRouteOutletComponentMount,
	shouldSyncRouteOutletRootMount,
	typedAdapterInternalRoutePropsRouteScopePropName,
} from "../../runtime.ts";

function createBranchInputState(
	overrides: Partial<RouteOutletBranchInputState>,
): RouteOutletBranchInputState {
	return {
		loaderCount: 1,
		outermostErrorIdx: undefined,
		activeComponents: [() => "Root"],
		activeErrorBoundary: undefined,
		importURLs: ["/routes/root.tsx"],
		exportKeys: ["Route"],
		matchedPatterns: ["/"],
		...overrides,
	};
}

describe("route outlet adapter host runtime", () => {
	it("resolves adapter render models for component/fallback/error branches", () => {
		const componentRenderModel = resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState: createBranchInputState({}),
			outermostError: "component-error",
			idx: 0,
		});
		expect(componentRenderModel.renderKind).toBe("component");
		expect(componentRenderModel).toEqual({
			renderKind: "component",
			currentComponent: expect.any(Function),
			currentRouteKey: expect.any(String),
			nextRouteKey: expect.any(String),
			matchedPattern: "/",
			outermostError: "component-error",
		});

		const fallbackRenderModel = resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState: createBranchInputState({
				activeComponents: [undefined, () => "Child"],
				importURLs: ["/routes/root.tsx", "/routes/child.tsx"],
				exportKeys: ["Route", "Route"],
				matchedPatterns: ["/", "/child"],
				loaderCount: 2,
			}),
			outermostError: "fallback-error",
			idx: 0,
		});
		expect(fallbackRenderModel).toEqual({
			renderKind: "fallback",
			currentRouteKey: expect.any(String),
			nextRouteKey: expect.any(String),
			outermostError: "fallback-error",
		});

		const errorBoundary = () => "ErrorBoundary";
		const errorRenderModel = resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState: createBranchInputState({
				outermostErrorIdx: 0,
				activeErrorBoundary: errorBoundary,
			}),
			outermostError: "fatal",
			idx: 0,
		});
		expect(errorRenderModel).toEqual({
			renderKind: "error",
			errorComponent: errorBoundary,
			currentRouteKey: expect.any(String),
			nextRouteKey: expect.any(String),
			outermostError: "fatal",
		});
	});

	it("resolves matched patterns through render-model binding and fails loudly on missing patterns", () => {
		const routeOutletBranchInputState = createBranchInputState({
			matchedPatterns: ["/root", "/child"],
			loaderCount: 2,
			activeComponents: [() => "Root", () => "Child"],
			importURLs: ["/routes/root.tsx", "/routes/child.tsx"],
			exportKeys: ["Route", "Route"],
		});

		const componentRenderModel = resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState,
			outermostError: "component-error",
			idx: 1,
		});
		expect(componentRenderModel.renderKind).toBe("component");
		if (componentRenderModel.renderKind === "component") {
			expect(componentRenderModel.matchedPattern).toBe("/child");
		}

		expect(() =>
			resolveRouteOutletAdapterRenderModel({
				routeOutletBranchInputState: createBranchInputState({
					matchedPatterns: [],
				}),
				outermostError: "component-error",
				idx: 0,
			}),
		).toThrow(
			"Route outlet adapter render model contract violated: missing matched pattern at index 0.",
		);
	});

	it("resolves adapter render model with matched pattern and outermost error payload", () => {
		const componentRenderModel = resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState: createBranchInputState({
				matchedPatterns: ["/projects"],
			}),
			outermostError: "component-error",
			idx: 0,
		});
		expect(componentRenderModel).toEqual({
			renderKind: "component",
			currentComponent: expect.any(Function),
			currentRouteKey: expect.any(String),
			nextRouteKey: expect.any(String),
			matchedPattern: "/projects",
			outermostError: "component-error",
		});
	});

	it("resolves render kinds directly from shared render models", () => {
		const componentRenderModel = resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState: createBranchInputState({}),
			outermostError: "component-error",
			idx: 0,
		});
		const fallbackRenderModel = resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState: createBranchInputState({
				activeComponents: [undefined, () => "Child"],
				importURLs: ["/routes/root.tsx", "/routes/child.tsx"],
				exportKeys: ["Route", "Route"],
				matchedPatterns: ["/", "/child"],
				loaderCount: 2,
			}),
			outermostError: "fallback-error",
			idx: 0,
		});
		const errorBoundary = () => "ErrorBoundary";
		const errorRenderModel = resolveRouteOutletAdapterRenderModel({
			routeOutletBranchInputState: createBranchInputState({
				outermostErrorIdx: 0,
				activeErrorBoundary: errorBoundary,
			}),
			outermostError: "fatal",
			idx: 0,
		});

		expect(componentRenderModel.renderKind).toBe("component");
		expect(fallbackRenderModel.renderKind).toBe("fallback");
		expect(errorRenderModel.renderKind).toBe("error");
	});

	it("builds route-component mount props with internal route-scope token", () => {
		const mountProps = buildTypedAdapterRouteComponentMountProps({
			routePropsIndex: 0,
			matchedPattern: "/projects",
		});
		expect(mountProps).toHaveProperty(
			typedAdapterInternalRoutePropsRouteScopePropName,
		);

		expect(() =>
			buildTypedAdapterRouteComponentMountProps({
				routePropsIndex: 0,
				matchedPattern: "",
			}),
		).toThrow(
			"Vorma route scope initialization violated: matched pattern is missing at route index.",
		);
	});

	it("delegates remount decisions through shared adapter host helper", () => {
		const previousComponent = () => "A";
		const nextComponent = () => "B";

		expect(
			shouldRemountRouteOutletComponentMount({
				idx: 1,
				previousRouteKey: "route-key",
				nextRouteKey: "route-key",
				previousRouteComponent: previousComponent,
				nextRouteComponent: nextComponent,
			}),
		).toBe(true);

		expect(
			shouldRemountRouteOutletComponentMount({
				idx: 1,
				previousRouteKey: "route-key-a",
				nextRouteKey: "route-key-b",
				previousRouteComponent: previousComponent,
				nextRouteComponent: previousComponent,
			}),
		).toBe(true);
	});

	it("resolves shared root-outlet lifecycle sync guard decisions", () => {
		expect(shouldSyncRouteOutletRootMount({ idx: 0 })).toBe(true);
		expect(shouldSyncRouteOutletRootMount({ idx: 1 })).toBe(false);
	});
});
