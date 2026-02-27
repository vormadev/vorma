import { beforeEach, describe, expect, it, vi } from "vitest";
import { createPatternRegistry } from "vorma/kit/matcher/register";
import {
	__vormaClientGlobal,
	findPartialMatchesOnClient,
	registerClientLoaderForAdapter,
	VORMA_SYMBOL,
} from "../../runtime.ts";

const TEST_VORMA_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
};

function installVormaGlobalForClientLoaderAdapterTests(props?: {
	patternRegistry?: unknown;
}) {
	const patternRegistry =
		props?.patternRegistry === undefined
			? createPatternRegistry({
					dynamicParamPrefixRune:
						TEST_VORMA_APP_CONFIG.loadersDynamicRune,
					splatSegmentRune: TEST_VORMA_APP_CONFIG.loadersSplatRune,
					explicitIndexSegment:
						TEST_VORMA_APP_CONFIG.loadersExplicitIndexSegmentIdentifier,
				})
			: props.patternRegistry;

	(globalThis as any)[VORMA_SYMBOL] = {
		isDev: false,
		viteDevURL: "",
		publicPathPrefix: "",
		isTouchInputModalityActive: false,
		patternToWaitFnMap: {},
		defaultErrorBoundary: () => null,
		useViewTransitions: false,
		deploymentID: "",
		vormaAppConfig: TEST_VORMA_APP_CONFIG,
		routeManifestURL: "",
		routeManifest: undefined,
		patternRegistry,
		runtimeRouteSnapshot: {
			outermostServerError: undefined,
			outermostServerErrorIdx: undefined,
			matchedPatterns: [],
			loadersData: [],
			importURLs: [],
			exportKeys: [],
			errorExportKeys: [],
			hasRootData: false,
			params: {},
			splatValues: [],
			outermostClientError: undefined,
			outermostClientErrorIdx: undefined,
			outermostError: undefined,
			outermostErrorIdx: undefined,
			buildID: "1",
			rootElementID: undefined,
			activeComponents: [],
			activeErrorBoundary: undefined,
			clientLoadersData: [],
		},
	};
}

describe("client loader adapter registration", () => {
	beforeEach(() => {
		installVormaGlobalForClientLoaderAdapterTests();
	});

	it("registers pattern and installs the wait function", async () => {
		const waitFn = vi.fn(async () => "ok");
		registerClientLoaderForAdapter({
			pattern: "/items/:id",
			waitFn: waitFn as any,
		});

		const patternToWaitFnMap =
			__vormaClientGlobal.get("patternToWaitFnMap");
		expect(patternToWaitFnMap["/items/:id"]).toBe(waitFn);
		await expect(
			findPartialMatchesOnClient("/items/123"),
		).resolves.toMatchObject({
			matches: [
				{
					registeredPattern: {
						originalPattern: "/items/:id",
					},
				},
			],
		});
	});

	it("throws with pattern context when registration fails", () => {
		installVormaGlobalForClientLoaderAdapterTests({
			patternRegistry: null,
		});

		expect(() => {
			registerClientLoaderForAdapter({
				pattern: "/broken",
				waitFn: vi.fn(async () => "ok") as any,
			});
		}).toThrow(
			'Failed to register client loader pattern "/broken": Pattern registry has not been initialized.',
		);

		expect(__vormaClientGlobal.get("patternToWaitFnMap")).toEqual({});
	});

	it("delegates failures to onRegistrationError without installing wait function", () => {
		installVormaGlobalForClientLoaderAdapterTests({
			patternRegistry: null,
		});
		const onRegistrationError = vi.fn();

		expect(() => {
			registerClientLoaderForAdapter({
				pattern: "/broken",
				waitFn: vi.fn(async () => "ok") as any,
				onRegistrationError,
			});
		}).not.toThrow();

		expect(onRegistrationError).toHaveBeenCalledTimes(1);
		expect(onRegistrationError.mock.calls[0]?.[0]).toBeInstanceOf(Error);
		expect((onRegistrationError.mock.calls[0]?.[0] as Error).message).toBe(
			"Pattern registry has not been initialized.",
		);
		expect(__vormaClientGlobal.get("patternToWaitFnMap")).toEqual({});
	});
});
