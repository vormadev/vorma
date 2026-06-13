import type {
	AppConfig,
	AppView,
	ConditionalSplat,
	ConditionalViewParams,
	ViewByPattern,
	ViewParamsRecord,
} from "./generated_contract_types.ts";
import type {
	BeforeRouteCommitFn,
	BeforeRouteYieldFn,
	LinkPropsBase,
} from "./route_types.ts";

/////// VIEW PATTERNS

export type ToViewPattern<A extends AppConfig> = AppView<A>["pattern"];

type PermissiveViewPattern<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = P extends `${infer Prefix}/_index` ? P | (Prefix extends "" ? "/" : Prefix) : P;

/////// VIEW I/O

export type ToViewOutput<A extends AppConfig, P extends ToViewPattern<A>> =
	ViewByPattern<A, P> extends { __o: infer O } ? O : never;

export type ToViewInput<A extends AppConfig, P extends ToViewPattern<A>> =
	ViewByPattern<A, P> extends { __i: infer I } ? I : never;

/////// PARENT VIEW INPUT

type ViewParents<A extends AppConfig, P extends string> =
	ViewByPattern<A, P> extends {
		parents: ReadonlyArray<infer Parent>;
	}
		? Extract<Parent, ToViewPattern<A>>
		: never;

/*
A matched URL's query feeds EVERY matched view's input, so the typed
search is the intersection of the whole chain's inputs. Views with no
input contribute nothing — without the `undefined -> {}` normalization a
plain layout would annihilate the intersection and type every child's
search as `undefined`.
*/
type ViewInputWithParentViews<A extends AppConfig, P extends ToViewPattern<A>> = (
	P | ViewParents<A, P> extends infer Pattern
		? Pattern extends ToViewPattern<A>
			? (
					input: [ToViewInput<A, Pattern>] extends [undefined]
						? // oxlint-disable-next-line no-empty-object-type -- identity for intersection
							{}
						: ToViewInput<A, Pattern>,
				) => void
			: never
		: never
) extends (input: infer Input) => void
	? Input
	: never;

/////// ROUTE TARGETS

export type ToRouteDestination<A extends AppConfig, P extends ToViewPattern<A>> = {
	href?: never;
	pattern: PermissiveViewPattern<A, P>;
	search?: ViewInputWithParentViews<A, P>;
	hash?: string;
} & ConditionalViewParams<A, P> &
	ConditionalSplat<P>;

export type ToNavigationTarget<A extends AppConfig, P extends ToViewPattern<A>> =
	| {
			href: string;
			pattern?: never;
			params?: never;
			splatValues?: never;
			search?: never;
			hash?: never;
	  }
	| ToRouteDestination<A, P>;

export type ToNavigateArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = ToNavigationTarget<A, P> & {
	replace?: boolean;
	scrollToTop?: boolean;
	state?: unknown;
	skipWorkIndicator?: boolean;
};

export type ToRouteSyncArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
> = ToRouteDestination<A, P> & {
	enabled?: boolean;
	debounceMs?: number;
	replace?: boolean;
	scrollToTop?: boolean;
};

/////// VIEW COMPONENT PROPS

export type ToViewComponentProps<
	A extends AppConfig,
	P extends ToViewPattern<A>,
	ClientLoaderData = unknown,
> = {
	idx: number;
	Outlet: (local?: Record<string, unknown>) => any;
	__phantom_pattern?: P;
	__phantom_client_loader_data?: ClientLoaderData;
};

/////// CLIENT LOADER PROPS

export type ClientLoaderKnownMatch = {
	pattern: string;
	input: unknown;
};

export type ClientLoaderServerState<A extends AppConfig, P extends ToViewPattern<A>> = {
	clientBuildId: string;
	matches: Array<{
		pattern: string;
		input: unknown;
		viewData: unknown;
	}>;
	outermostServerError: null | {
		idx: number;
		error: unknown;
	};
	viewData: ToViewOutput<A, P>;
};

export type ToClientLoaderArgs<A extends AppConfig, P extends ToViewPattern<A>> = {
	trigger: "boot" | "navigation" | "revalidation" | "prefetch";
	href: string;
	historyState: unknown;
	pattern: P;
	params: ViewParamsRecord<A, P>;
	splatValues: string[];
	input: ToViewInput<A, P>;
	knownMatches: ClientLoaderKnownMatch[];
	serverPromise: Promise<ClientLoaderServerState<A, P>>;
	signal: AbortSignal;
};

/////// DEFINE VIEW INPUT

export type ToDefineViewArgs<
	A extends AppConfig,
	P extends ToViewPattern<A>,
	T = any,
	Element = unknown,
> = {
	pattern: P;
	component: (props: ToViewComponentProps<A, P, T>) => Element;
	errorBoundary?: (props: { error: unknown }) => Element;
	clientLoader?: (args: ToClientLoaderArgs<A, P>) => Promise<T>;
	beforeRouteCommit?: BeforeRouteCommitFn;
	beforeRouteYield?: BeforeRouteYieldFn;
	runClientLoaderOnHmr?: boolean;
};

/////// LINK PROPS

export type ToLinkProps<A extends AppConfig, P extends ToViewPattern<A>> = LinkPropsBase &
	ToNavigationTarget<A, P> & {
		state?: unknown;
	};
