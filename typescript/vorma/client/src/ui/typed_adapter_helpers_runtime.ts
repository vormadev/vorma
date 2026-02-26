import type { ClientLoaderAwaitedServerData } from "../app/context.ts";
import type {
	VormaAppBase,
	VormaLoaderOutput,
	VormaLoaderPattern,
} from "../app/helpers.ts";
import { __registerClientLoaderForAdapter as registerClientLoaderForAdapter } from "../core/extras.ts";
import type { ParamsForPattern } from "./helpers.ts";

const typedAdapterRouteInstanceTokenMarker = Symbol(
	"vorma_typed_adapter_route_instance_token",
);
const typedAdapterRouteInstanceStoreRecordMarker = Symbol(
	"vorma_typed_adapter_route_instance_store_record",
);

export const typedAdapterInternalRoutePropsRouteInstanceTokenPropName =
	"__vorma_internal_route_instance_token";

type VormaTypedAdapterRoutePropsWithIndex = {
	idx: number;
	[typedAdapterInternalRoutePropsRouteInstanceTokenPropName]?: unknown;
};

type TypedAdapterRouteInstanceStoreRecord = {
	status: "active" | "exiting" | "disposed";
	boundRoutePropsIndex: number;
	boundRouteKey: string;
	boundMatchedPattern: string;
	hasLoaderSnapshot: boolean;
	loaderSnapshot: unknown;
	hasClientLoaderSnapshot: boolean;
	clientLoaderSnapshot: unknown;
};

type TypedAdapterRouteInstanceToken = {
	[typedAdapterRouteInstanceTokenMarker]: true;
	[typedAdapterRouteInstanceStoreRecordMarker]: TypedAdapterRouteInstanceStoreRecord;
};

const typedAdapterRouteInstanceActiveRecords =
	new Set<TypedAdapterRouteInstanceStoreRecord>();

type VormaTypedAdapterClientLoaderFunction<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	LoaderData extends VormaLoaderOutput<App, Pattern>,
	T,
> = (props: {
	// Client loaders run speculatively during prefetch/navigation and can be
	// discarded when ownership changes; end users should keep side effects
	// idempotent and honor signal-based cancellation.
	params: Record<ParamsForPattern<App, Pattern>, string>;
	splatValues: string[];
	serverDataPromise: Promise<
		ClientLoaderAwaitedServerData<App["rootData"], LoaderData>
	>;
	signal: AbortSignal;
}) => Promise<T>;

export type VormaTypedAdapterAddClientLoaderProps<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	LoaderData extends VormaLoaderOutput<App, Pattern>,
	T,
> = {
	pattern: Pattern;
	clientLoader: VormaTypedAdapterClientLoaderFunction<
		App,
		Pattern,
		LoaderData,
		T
	>;
	reRunOnModuleChange?: ImportMeta;
};

/**
 * Registers a typed adapter client loader immediately when invoked.
 *
 * Re-registering the same pattern is intentional and overwrites the previous
 * wait function for that pattern. This enables adapter-level HMR/module
 * replacement without requiring explicit teardown hooks from applications.
 */
export function registerTypedAdapterClientLoader<
	App extends VormaAppBase,
	Pattern extends VormaLoaderPattern<App>,
	LoaderData extends VormaLoaderOutput<App, Pattern>,
	T,
>(
	props: VormaTypedAdapterAddClientLoaderProps<App, Pattern, LoaderData, T>,
): {
	pattern: Pattern;
	clientLoader: VormaTypedAdapterClientLoaderFunction<
		App,
		Pattern,
		LoaderData,
		T
	>;
} {
	registerClientLoaderForAdapter({
		pattern: props.pattern as string,
		waitFn: props.clientLoader as any,
		reRunOnModuleChange: props.reRunOnModuleChange,
	});

	return {
		pattern: props.pattern,
		clientLoader: props.clientLoader,
	};
}

function resolveTypedAdapterMatchedPatternIndex(props: {
	matchedPatterns: readonly string[];
	pattern: string;
}): number {
	const { matchedPatterns, pattern } = props;
	return matchedPatterns.findIndex(
		(matchedPattern) => matchedPattern === pattern,
	);
}

function buildRouteInstanceRouteKey(props: {
	routePropsIndex: number;
	importURLs: ReadonlyArray<string> | null | undefined;
	exportKeys: ReadonlyArray<string> | null | undefined;
	matchedPatterns: ReadonlyArray<string> | null | undefined;
}): string {
	const importURL = props.importURLs?.[props.routePropsIndex] || "";
	const exportKey = props.exportKeys?.[props.routePropsIndex] || "";
	const matchedPattern = props.matchedPatterns?.[props.routePropsIndex] || "";
	return JSON.stringify([
		props.routePropsIndex,
		importURL,
		exportKey,
		matchedPattern,
	]);
}

function isTypedAdapterRouteInstanceToken(
	value: unknown,
): value is TypedAdapterRouteInstanceToken {
	if (typeof value !== "object" || value === null) {
		return false;
	}
	return (
		(value as Record<PropertyKey, unknown>)[
			typedAdapterRouteInstanceTokenMarker
		] === true
	);
}

function resolveTypedAdapterRouteInstanceStoreRecordFromToken(props: {
	routeInstanceToken: TypedAdapterRouteInstanceToken;
}): TypedAdapterRouteInstanceStoreRecord | undefined {
	const routeInstanceStoreRecord =
		props.routeInstanceToken[typedAdapterRouteInstanceStoreRecordMarker];
	if (
		typeof routeInstanceStoreRecord !== "object" ||
		routeInstanceStoreRecord === null
	) {
		return undefined;
	}
	return routeInstanceStoreRecord;
}

function resolveTypedAdapterRouteInstanceStoreRecordOrThrow(props: {
	hookName: "useLoaderData" | "useClientLoaderData";
	routeProps: VormaTypedAdapterRoutePropsWithIndex;
}): TypedAdapterRouteInstanceStoreRecord {
	const maybeRouteInstanceToken =
		props.routeProps[
			typedAdapterInternalRoutePropsRouteInstanceTokenPropName
		];
	if (!isTypedAdapterRouteInstanceToken(maybeRouteInstanceToken)) {
		throw new Error(
			`${props.hookName}(routeProps) contract violated: route instance token is missing or invalid.`,
		);
	}

	const routeInstanceStoreRecord =
		resolveTypedAdapterRouteInstanceStoreRecordFromToken({
			routeInstanceToken: maybeRouteInstanceToken,
		});
	if (!routeInstanceStoreRecord) {
		throw new Error(
			`${props.hookName}(routeProps) contract violated: route instance token is missing or invalid.`,
		);
	}
	if (routeInstanceStoreRecord.status === "disposed") {
		throw new Error(
			`${props.hookName}(routeProps) contract violated: route instance has been disposed.`,
		);
	}

	const routePropsIndex = props.routeProps.idx;
	if (
		!Number.isInteger(routePropsIndex) ||
		routePropsIndex !== routeInstanceStoreRecord.boundRoutePropsIndex
	) {
		throw new Error(
			`${props.hookName}(routeProps) contract violated: routeProps.idx changed after route instance binding.`,
		);
	}

	return routeInstanceStoreRecord;
}

function markTypedAdapterRouteInstanceStoreRecordExiting(props: {
	routeInstanceStoreRecord: TypedAdapterRouteInstanceStoreRecord;
}): void {
	const { routeInstanceStoreRecord } = props;
	if (routeInstanceStoreRecord.status === "disposed") {
		return;
	}
	routeInstanceStoreRecord.status = "exiting";
}

function syncTypedAdapterRouteInstanceStoreRecordFromNavigationState(props: {
	routeInstanceStoreRecord: TypedAdapterRouteInstanceStoreRecord;
	matchedPatterns: readonly string[];
	loadersData: readonly unknown[];
	clientLoadersData: readonly unknown[];
	importURLs: readonly string[];
	exportKeys: readonly string[];
}): void {
	const { routeInstanceStoreRecord } = props;
	if (routeInstanceStoreRecord.status === "disposed") {
		return;
	}

	const routePropsIndex = routeInstanceStoreRecord.boundRoutePropsIndex;
	const currentRouteKeyAtIndex = buildRouteInstanceRouteKey({
		routePropsIndex,
		importURLs: props.importURLs,
		exportKeys: props.exportKeys,
		matchedPatterns: props.matchedPatterns,
	});
	const currentMatchedPattern = props.matchedPatterns[routePropsIndex];

	if (
		currentRouteKeyAtIndex !== routeInstanceStoreRecord.boundRouteKey ||
		currentMatchedPattern !== routeInstanceStoreRecord.boundMatchedPattern
	) {
		markTypedAdapterRouteInstanceStoreRecordExiting({
			routeInstanceStoreRecord,
		});
		return;
	}

	routeInstanceStoreRecord.status = "active";
	routeInstanceStoreRecord.hasLoaderSnapshot = true;
	routeInstanceStoreRecord.loaderSnapshot =
		props.loadersData[routePropsIndex];
	routeInstanceStoreRecord.hasClientLoaderSnapshot = true;
	routeInstanceStoreRecord.clientLoaderSnapshot =
		props.clientLoadersData[routePropsIndex];
}

export function syncTypedAdapterRouteInstanceStoreFromNavigationState(props: {
	matchedPatterns: readonly string[];
	loadersData: readonly unknown[];
	clientLoadersData: readonly unknown[];
	importURLs: readonly string[];
	exportKeys: readonly string[];
}): void {
	typedAdapterRouteInstanceActiveRecords.forEach(
		(routeInstanceStoreRecord) => {
			syncTypedAdapterRouteInstanceStoreRecordFromNavigationState({
				routeInstanceStoreRecord,
				matchedPatterns: props.matchedPatterns,
				loadersData: props.loadersData,
				clientLoadersData: props.clientLoadersData,
				importURLs: props.importURLs,
				exportKeys: props.exportKeys,
			});
		},
	);
}

export function createTypedAdapterRouteInstanceToken(props: {
	routePropsIndex: number;
	routeKey: string;
	matchedPatterns: readonly string[];
	loadersData: readonly unknown[];
	clientLoadersData: readonly unknown[];
}): unknown {
	const routePropsIndex = props.routePropsIndex;
	if (
		!Number.isInteger(routePropsIndex) ||
		routePropsIndex < 0 ||
		routePropsIndex >= props.matchedPatterns.length
	) {
		throw new Error(
			"Vorma route instance token initialization violated: route index is out of bounds for matched patterns.",
		);
	}
	if (typeof props.routeKey !== "string" || props.routeKey === "") {
		throw new Error(
			"Vorma route instance token initialization violated: route key is missing.",
		);
	}

	const matchedPattern = props.matchedPatterns[routePropsIndex];
	if (typeof matchedPattern !== "string" || matchedPattern === "") {
		throw new Error(
			"Vorma route instance token initialization violated: matched pattern is missing at route index.",
		);
	}

	const routeInstanceToken: TypedAdapterRouteInstanceToken = {
		[typedAdapterRouteInstanceTokenMarker]: true,
		[typedAdapterRouteInstanceStoreRecordMarker]: {
			status: "active",
			boundRoutePropsIndex: routePropsIndex,
			boundRouteKey: props.routeKey,
			boundMatchedPattern: matchedPattern,
			hasLoaderSnapshot: true,
			loaderSnapshot: props.loadersData[routePropsIndex],
			hasClientLoaderSnapshot: true,
			clientLoaderSnapshot: props.clientLoadersData[routePropsIndex],
		},
	};

	return routeInstanceToken;
}

export function markTypedAdapterRouteInstanceTokenActive(props: {
	routeInstanceToken: unknown;
}): void {
	if (!isTypedAdapterRouteInstanceToken(props.routeInstanceToken)) {
		return;
	}
	const routeInstanceStoreRecord =
		resolveTypedAdapterRouteInstanceStoreRecordFromToken({
			routeInstanceToken: props.routeInstanceToken,
		});
	if (!routeInstanceStoreRecord) {
		return;
	}
	routeInstanceStoreRecord.status = "active";
	typedAdapterRouteInstanceActiveRecords.add(routeInstanceStoreRecord);
}

export function markTypedAdapterRouteInstanceTokenDisposed(props: {
	routeInstanceToken: unknown;
}): void {
	if (!isTypedAdapterRouteInstanceToken(props.routeInstanceToken)) {
		return;
	}
	const routeInstanceStoreRecord =
		resolveTypedAdapterRouteInstanceStoreRecordFromToken({
			routeInstanceToken: props.routeInstanceToken,
		});
	if (!routeInstanceStoreRecord) {
		return;
	}
	routeInstanceStoreRecord.status = "disposed";
	typedAdapterRouteInstanceActiveRecords.delete(routeInstanceStoreRecord);
}

export function buildTypedAdapterRoutePropsWithInternalRouteInstanceToken(props: {
	routeInstanceToken: unknown;
}): Record<
	typeof typedAdapterInternalRoutePropsRouteInstanceTokenPropName,
	unknown
> {
	return {
		[typedAdapterInternalRoutePropsRouteInstanceTokenPropName]:
			props.routeInstanceToken,
	};
}

export function resolveTypedAdapterLoaderDataForRoutePropsOrThrow<Data>(props: {
	routeProps: VormaTypedAdapterRoutePropsWithIndex;
	matchedPatterns: readonly string[];
	loadersData: readonly unknown[];
	clientLoadersData: readonly unknown[];
}): Data {
	void props.matchedPatterns;
	void props.loadersData;
	void props.clientLoadersData;

	const routeInstanceStoreRecord =
		resolveTypedAdapterRouteInstanceStoreRecordOrThrow({
			hookName: "useLoaderData",
			routeProps: props.routeProps,
		});

	if (!routeInstanceStoreRecord.hasLoaderSnapshot) {
		throw new Error(
			"useLoaderData(routeProps) contract violated: no route-scoped loader snapshot is available.",
		);
	}

	return routeInstanceStoreRecord.loaderSnapshot as Data;
}

export function resolveTypedAdapterIndexedDataForPattern<Data>(props: {
	pattern: string;
	matchedPatterns: readonly string[];
	indexedData: readonly unknown[];
}): Data | undefined {
	const idx = resolveTypedAdapterMatchedPatternIndex({
		matchedPatterns: props.matchedPatterns,
		pattern: props.pattern,
	});
	if (idx === -1) {
		return undefined;
	}
	return props.indexedData[idx] as Data | undefined;
}

export function resolveTypedAdapterClientLoaderDataForPatternOrRouteProps<
	Data,
>(props: {
	pattern: string;
	routeProps?: VormaTypedAdapterRoutePropsWithIndex;
	matchedPatterns: readonly string[];
	loadersData: readonly unknown[];
	clientLoadersData: readonly unknown[];
}): Data | undefined {
	if (!props.routeProps) {
		return resolveTypedAdapterIndexedDataForPattern<Data>({
			pattern: props.pattern,
			matchedPatterns: props.matchedPatterns,
			indexedData: props.clientLoadersData,
		});
	}

	void props.matchedPatterns;
	void props.loadersData;
	void props.clientLoadersData;

	const routeInstanceStoreRecord =
		resolveTypedAdapterRouteInstanceStoreRecordOrThrow({
			hookName: "useClientLoaderData",
			routeProps: props.routeProps,
		});

	if (routeInstanceStoreRecord.boundMatchedPattern !== props.pattern) {
		throw new Error(
			`useClientLoaderData(routeProps) contract violated for pattern "${props.pattern}": route instance is bound to pattern "${routeInstanceStoreRecord.boundMatchedPattern}".`,
		);
	}
	if (!routeInstanceStoreRecord.hasClientLoaderSnapshot) {
		throw new Error(
			`useClientLoaderData(routeProps) contract violated for pattern "${props.pattern}": no route-scoped client-loader snapshot is available.`,
		);
	}

	return routeInstanceStoreRecord.clientLoaderSnapshot as Data | undefined;
}
