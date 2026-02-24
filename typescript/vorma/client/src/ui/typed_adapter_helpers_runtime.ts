import type { ClientLoaderAwaitedServerData } from "../app/context.ts";
import type {
	VormaAppBase,
	VormaLoaderOutput,
	VormaLoaderPattern,
} from "../app/helpers.ts";
import { __registerClientLoaderForAdapter as registerClientLoaderForAdapter } from "../core/extras.ts";
import type { ParamsForPattern } from "./helpers.ts";

type VormaTypedAdapterRoutePropsWithIndex = {
	idx: number;
};

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

export function resolveTypedAdapterIndexedDataForPatternOrRouteProps<
	Data,
>(props: {
	pattern: string;
	matchedPatterns: readonly string[];
	indexedData: readonly unknown[];
	routeProps?: VormaTypedAdapterRoutePropsWithIndex;
}): Data | undefined {
	const idx =
		props.routeProps?.idx ??
		resolveTypedAdapterMatchedPatternIndex({
			matchedPatterns: props.matchedPatterns,
			pattern: props.pattern,
		});

	if (idx === -1) {
		return undefined;
	}
	return props.indexedData[idx] as Data | undefined;
}
