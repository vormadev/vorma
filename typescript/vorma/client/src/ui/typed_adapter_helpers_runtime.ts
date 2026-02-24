import type { ClientLoaderAwaitedServerData } from "../app/context.ts";
import type {
	VormaAppBase,
	VormaLoaderOutput,
	VormaLoaderPattern,
} from "../app/helpers.ts";
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
