import type { Accessor } from "solid-js";
import type {
	APIRequestInitDecorator,
	ExtractApp,
	ParamsForPattern,
	RouteChangeEvent,
	StatusEvent,
	SubmitResult,
	VormaAppConfig,
	VormaLoaderOutput,
	VormaLoaderPattern,
	VormaMutationOutput,
	VormaMutationProps,
	VormaQueryOutput,
	VormaQueryProps,
} from "vorma/client";
import {
	addBuildIDListener,
	addLocationListener,
	addRouteChangeListener,
	addStatusListener,
	buildMutationURL,
	buildQueryURL,
	defaultErrorBoundary,
	getBuildID,
	getHistoryInstance,
	getLocation,
	getRootEl,
	getRouterData,
	getStatus,
	initClient,
	makeTypedAPIClient,
	makeTypedNavigate,
	resolveBody,
	revalidate,
	revalidateOnWindowFocus,
	setupGlobalLoadingIndicator,
	submit,
	vormaNavigate,
} from "vorma/client";
import {
	VormaLink as PreactVormaLink,
	VormaRootOutlet as PreactVormaRootOutlet,
	location as preactLocationSignal,
	makeTypedAddClientLoader as makeTypedPreactAddClientLoader,
	makeTypedLink as makeTypedPreactLink,
	makeTypedUseLoaderData as makeTypedPreactUseLoaderData,
	makeTypedUsePatternLoaderData as makeTypedPreactUsePatternLoaderData,
	makeTypedUseRouterData as makeTypedPreactUseRouterData,
	type VormaRouteProps as PreactVormaRouteProps,
} from "vorma/preact";
import {
	VormaLink as ReactVormaLink,
	VormaRootOutlet as ReactVormaRootOutlet,
	makeTypedAddClientLoader as makeTypedReactAddClientLoader,
	makeTypedLink as makeTypedReactLink,
	makeTypedUseLoaderData as makeTypedReactUseLoaderData,
	makeTypedUsePatternLoaderData as makeTypedReactUsePatternLoaderData,
	makeTypedUseRouterData as makeTypedReactUseRouterData,
	useLocation as useReactLocation,
	type VormaRouteProps as ReactVormaRouteProps,
} from "vorma/react";
import {
	VormaLink as SolidVormaLink,
	VormaRootOutlet as SolidVormaRootOutlet,
	location as readSolidLocation,
	makeTypedAddClientLoader as makeTypedSolidAddClientLoader,
	makeTypedLink as makeTypedSolidLink,
	makeTypedUseLoaderData as makeTypedSolidUseLoaderData,
	makeTypedUsePatternLoaderData as makeTypedSolidUsePatternLoaderData,
	makeTypedUseRouterData as makeTypedSolidUseRouterData,
	type VormaRouteProps as SolidVormaRouteProps,
} from "vorma/solid";

type Assert<Condition extends true> = Condition;
type IsExact<Actual, Expected> =
	(<T>() => T extends Actual ? 1 : 2) extends <T>() => T extends Expected
		? 1
		: 2
		? (<T>() => T extends Expected ? 1 : 2) extends <
				T,
			>() => T extends Actual ? 1 : 2
			? true
			: false
		: false;

function expectType<Expected>(_value: Expected): void {
	void _value;
}

type PublicTypeTestAppConfigContract = {
	actionsRouterMountRoot: "/api/";
	actionsDynamicRune: ":";
	actionsSplatRune: "*";
	loadersDynamicRune: ":";
	loadersSplatRune: "*";
	loadersExplicitIndexSegmentIdentifier: "_index";
};

type PublicTypeTestApp = {
	appConfig: PublicTypeTestAppConfigContract;
	rootData: {
		sessionUserID: string | null;
	};
	routes: readonly [
		{
			_type: "loader";
			pattern: "/";
			phantomOutputType: {
				home: true;
			};
		},
		{
			_type: "loader";
			pattern: "/users/:userID";
			params: readonly ["userID"];
			phantomOutputType: {
				userName: string;
			};
		},
		{
			_type: "loader";
			pattern: "/docs/*";
			isSplat: true;
			phantomOutputType: {
				slugParts: string[];
			};
		},
		{
			_type: "loader";
			pattern: "/blog/_index";
			phantomOutputType: {
				posts: string[];
			};
		},
		{
			_type: "query";
			pattern: "/users/:userID";
			params: readonly ["userID"];
			method: "GET";
			phantomInputType: {
				includePosts: boolean;
			};
			phantomOutputType: {
				id: string;
				posts: number;
			};
		},
		{
			_type: "query";
			pattern: "/health";
			method: "GET";
			phantomInputType: null;
			phantomOutputType: {
				ok: true;
			};
		},
		{
			_type: "mutation";
			pattern: "/users/:userID";
			params: readonly ["userID"];
			method: "PATCH";
			phantomInputType: {
				nickname: string;
			};
			phantomOutputType: {
				saved: true;
			};
		},
		{
			_type: "mutation";
			pattern: "/sessions";
			method: "POST";
			phantomInputType: {
				email: string;
				password: string;
			};
			phantomOutputType: {
				token: string;
			};
		},
		{
			_type: "mutation";
			pattern: "/logout";
			method: "POST";
			phantomInputType: undefined;
			phantomOutputType: {
				done: true;
			};
		},
	];
};

const PUBLIC_TYPE_TEST_APP_CONFIG = {
	actionsRouterMountRoot: "/api/",
	actionsDynamicRune: ":",
	actionsSplatRune: "*",
	loadersDynamicRune: ":",
	loadersSplatRune: "*",
	loadersExplicitIndexSegmentIdentifier: "_index",
	__phantom: undefined as unknown as PublicTypeTestApp,
} as const satisfies VormaAppConfig;

type PublicApp = ExtractApp<typeof PUBLIC_TYPE_TEST_APP_CONFIG>;

type _assertUserIDParam = Assert<
	IsExact<ParamsForPattern<PublicApp, "/users/:userID">, "userID">
>;
type _assertDocsParams = Assert<
	IsExact<ParamsForPattern<PublicApp, "/docs/*">, never>
>;

function assertTypedClientAPIContracts(): void {
	const typedNavigate = makeTypedNavigate(PUBLIC_TYPE_TEST_APP_CONFIG);
	void typedNavigate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});
	void typedNavigate({
		pattern: "/docs/*",
		splatValues: ["guide", "install"],
		hash: "#top",
	});
	void typedNavigate({
		pattern: "/blog",
	});

	// @ts-expect-error route params are required for /users/:userID.
	void typedNavigate({ pattern: "/users/:userID" });
	void typedNavigate({
		pattern: "/users/:userID",
		// @ts-expect-error params key must match route parameter names.
		params: { slug: "u-1" },
	});
	void typedNavigate({
		pattern: "/users/:userID",
		// @ts-expect-error route params must be string values.
		params: { userID: 123 },
	});
	// @ts-expect-error splatValues are required for splat loader routes.
	void typedNavigate({ pattern: "/docs/*" });
	void typedNavigate({
		pattern: "/docs/*",
		// @ts-expect-error splatValues must be an array of strings.
		splatValues: "guide",
	});
	// @ts-expect-error unknown route patterns are rejected.
	void typedNavigate({ pattern: "/does-not-exist" });

	const typedDecorator: APIRequestInitDecorator<PublicApp> = async (
		context,
	) => {
		if (context.type === "query") {
			expectType<"/users/:userID" | "/health">(context.pattern);
			return {
				headers: [["x-query", "1"]],
			};
		}

		expectType<"/users/:userID" | "/sessions" | "/logout">(context.pattern);
		return {
			headers: [["x-mutation", "1"]],
		};
	};
	const apiClient = makeTypedAPIClient(
		PUBLIC_TYPE_TEST_APP_CONFIG,
		typedDecorator,
	);

	const userQueryResult = apiClient.query({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	});
	expectType<
		Promise<SubmitResult<VormaQueryOutput<PublicApp, "/users/:userID">>>
	>(userQueryResult);

	void apiClient.query({
		pattern: "/health",
	});
	void apiClient.query({
		pattern: "/health",
		input: null,
	});

	// @ts-expect-error object query inputs are required when input type is non-empty.
	void apiClient.query({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});
	void apiClient.query({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
		// @ts-expect-error query requestInit method must remain GET.
		requestInit: { method: "POST" },
	});
	void apiClient.query({
		pattern: "/users/:userID",
		// @ts-expect-error query params must match route parameter names.
		params: { id: "u-1" },
		input: { includePosts: true },
	});
	void apiClient.query({
		pattern: "/health",
		// @ts-expect-error query input root must be object, null, or undefined.
		input: "invalid",
	});
	// @ts-expect-error query route must come from query patterns.
	void apiClient.query<"/docs/*">({
		pattern: "/docs/*",
		splatValues: ["x"],
		input: {},
	});

	const patchMutationResult = apiClient.mutate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
		requestInit: { method: "PATCH" },
	});
	expectType<
		Promise<SubmitResult<VormaMutationOutput<PublicApp, "/users/:userID">>>
	>(patchMutationResult);

	const postMutationResult = apiClient.mutate({
		pattern: "/sessions",
		input: {
			email: "a@b.com",
			password: "pw",
		},
	});
	expectType<
		Promise<SubmitResult<VormaMutationOutput<PublicApp, "/sessions">>>
	>(postMutationResult);

	void apiClient.mutate({
		pattern: "/logout",
	});

	// @ts-expect-error PATCH mutations must explicitly provide requestInit.method.
	void apiClient.mutate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
	});
	void apiClient.mutate({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
		// @ts-expect-error requestInit method must match mutation method.
		requestInit: { method: "POST" },
	});
	// @ts-expect-error non-empty mutation input is required.
	void apiClient.mutate({ pattern: "/sessions" });
	void apiClient.mutate({
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
		// @ts-expect-error POST mutation requestInit cannot force another method.
		requestInit: { method: "PUT" },
	});
	// @ts-expect-error mutation route must come from mutation patterns.
	void apiClient.mutate<"/health">({
		pattern: "/health",
		input: {},
	});
}
void assertTypedClientAPIContracts;

function assertTypedQueryAndMutationPropsContracts(): void {
	const requiredQueryProps: VormaQueryProps<PublicApp, "/users/:userID"> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	};
	expectType<{ includePosts: boolean }>(requiredQueryProps.input);

	const optionalQueryPropsWithoutInput: VormaQueryProps<
		PublicApp,
		"/health"
	> = {
		pattern: "/health",
	};
	const optionalQueryPropsWithNullInput: VormaQueryProps<
		PublicApp,
		"/health"
	> = {
		pattern: "/health",
		input: null,
	};
	const optionalQueryPropsWithUndefinedInput: VormaQueryProps<
		PublicApp,
		"/health"
	> = {
		pattern: "/health",
		input: undefined,
	};
	void optionalQueryPropsWithoutInput;
	void optionalQueryPropsWithNullInput;
	void optionalQueryPropsWithUndefinedInput;

	// @ts-expect-error non-empty query input must be required in VormaQueryProps.
	const missingRequiredQueryInput: VormaQueryProps<
		PublicApp,
		"/users/:userID"
	> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	};
	void missingRequiredQueryInput;

	const wrongQueryMethodProps: VormaQueryProps<PublicApp, "/users/:userID"> =
		{
			pattern: "/users/:userID",
			params: { userID: "u-1" },
			input: { includePosts: true },
			requestInit: {
				// @ts-expect-error VormaQueryProps requestInit.method must remain GET.
				method: "POST",
			},
		};
	void wrongQueryMethodProps;

	const wrongQueryParamsProps: VormaQueryProps<PublicApp, "/users/:userID"> =
		{
			pattern: "/users/:userID",
			// @ts-expect-error VormaQueryProps params keys must match route params.
			params: { id: "u-1" },
			input: { includePosts: true },
		};
	void wrongQueryParamsProps;

	const requiredPatchMutationProps: VormaMutationProps<
		PublicApp,
		"/users/:userID"
	> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
		requestInit: { method: "PATCH" },
	};
	expectType<{ nickname: string }>(requiredPatchMutationProps.input);

	const requiredPostMutationProps: VormaMutationProps<
		PublicApp,
		"/sessions"
	> = {
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
	};
	const requiredPostMutationPropsWithMethod: VormaMutationProps<
		PublicApp,
		"/sessions"
	> = {
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
		requestInit: { method: "POST" },
	};
	const optionalLogoutMutationPropsWithoutInput: VormaMutationProps<
		PublicApp,
		"/logout"
	> = {
		pattern: "/logout",
	};
	const optionalLogoutMutationPropsWithUndefinedInput: VormaMutationProps<
		PublicApp,
		"/logout"
	> = {
		pattern: "/logout",
		input: undefined,
	};
	void requiredPostMutationProps;
	void requiredPostMutationPropsWithMethod;
	void optionalLogoutMutationPropsWithoutInput;
	void optionalLogoutMutationPropsWithUndefinedInput;

	// @ts-expect-error non-empty mutation input must be required in VormaMutationProps.
	const missingRequiredMutationInput: VormaMutationProps<
		PublicApp,
		"/sessions"
	> = {
		pattern: "/sessions",
	};
	void missingRequiredMutationInput;

	// @ts-expect-error non-POST mutation methods must require explicit requestInit.method.
	const missingPatchMutationMethodProps: VormaMutationProps<
		PublicApp,
		"/users/:userID"
	> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
	};
	void missingPatchMutationMethodProps;

	const wrongPatchMutationMethodProps: VormaMutationProps<
		PublicApp,
		"/users/:userID"
	> = {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
		requestInit: {
			// @ts-expect-error non-POST mutation method must match route declaration.
			method: "POST",
		},
	};
	void wrongPatchMutationMethodProps;

	const wrongPostMutationMethodProps: VormaMutationProps<
		PublicApp,
		"/sessions"
	> = {
		pattern: "/sessions",
		input: { email: "a@b.com", password: "pw" },
		requestInit: {
			// @ts-expect-error POST mutation requestInit.method cannot be changed.
			method: "PUT",
		},
	};
	void wrongPostMutationMethodProps;

	const wrongMutationParamsProps: VormaMutationProps<
		PublicApp,
		"/users/:userID"
	> = {
		pattern: "/users/:userID",
		// @ts-expect-error VormaMutationProps params keys must match route params.
		params: { id: "u-1" },
		input: { nickname: "neo" },
		requestInit: { method: "PATCH" },
	};
	void wrongMutationParamsProps;
}
void assertTypedQueryAndMutationPropsContracts;

function assertTypedReactAdapterContracts(): void {
	const useLoaderData = makeTypedReactUseLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const usePatternLoaderData = makeTypedReactUsePatternLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const useRouterData = makeTypedReactUseRouterData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const addClientLoader = makeTypedReactAddClientLoader(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const TypedLink = makeTypedReactLink(PUBLIC_TYPE_TEST_APP_CONFIG);
	const routeProps = null as unknown as ReactVormaRouteProps<
		PublicApp,
		"/users/:userID"
	>;

	const loaderData = useLoaderData(routeProps);
	expectType<VormaLoaderOutput<PublicApp, "/users/:userID">>(loaderData);
	const maybePatternData = usePatternLoaderData("/docs/*");
	expectType<VormaLoaderOutput<PublicApp, "/docs/*"> | undefined>(
		maybePatternData,
	);

	const scopedRouterData = useRouterData(routeProps);
	expectType<Record<"userID", string>>(scopedRouterData.params);
	const unscopedRouterData = useRouterData();
	expectType<Record<string, string>>(unscopedRouterData.params);

	const useClientLoaderData = addClientLoader({
		pattern: "/users/:userID",
		clientLoader: async ({ params, serverDataPromise }) => {
			expectType<string>(params.userID);
			const serverData = await serverDataPromise;
			expectType<VormaLoaderOutput<PublicApp, "/users/:userID">>(
				serverData.loaderData,
			);
			return serverData.loaderData.userName.length;
		},
		reRunOnModuleChange: import.meta,
	});
	const maybeClientLoaderData = useClientLoaderData();
	expectType<number | undefined>(maybeClientLoaderData);
	const scopedClientLoaderData = useClientLoaderData(routeProps);
	expectType<number>(scopedClientLoaderData);

	void TypedLink({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});
	void TypedLink({
		pattern: "/docs/*",
		splatValues: ["guide"],
	});
	void TypedLink({
		pattern: "/blog",
	});

	void usePatternLoaderData(
		// @ts-expect-error typed pattern loader data rejects unknown patterns.
		"/not-a-route",
	);
	void addClientLoader({
		// @ts-expect-error client loader pattern must exist in loader route patterns.
		pattern: "/not-a-route",
		clientLoader: async () => 1,
	});
	void addClientLoader({
		pattern: "/users/:userID",
		clientLoader: async () => 1,
		// @ts-expect-error reRunOnModuleChange must be ImportMeta when provided.
		reRunOnModuleChange: true,
	});
	// @ts-expect-error typed links require params for dynamic routes.
	void TypedLink({
		pattern: "/users/:userID",
	});
	void TypedLink({
		pattern: "/users/:userID",
		// @ts-expect-error typed links enforce exact dynamic param keys.
		params: { id: "u-1" },
	});
	void TypedLink({
		pattern: "/users/:userID",
		// @ts-expect-error typed links require string param values.
		params: { userID: 123 },
	});
	// @ts-expect-error typed links require splatValues for splat routes.
	void TypedLink({
		pattern: "/docs/*",
	});
	void TypedLink({
		pattern: "/docs/*",
		// @ts-expect-error typed links require splatValues to be string[].
		splatValues: "guide",
	});
	void TypedLink({
		// @ts-expect-error typed links reject unknown route patterns.
		pattern: "/not-a-route",
	});
}
void assertTypedReactAdapterContracts;

function assertTypedPreactAdapterContracts(): void {
	const useLoaderData = makeTypedPreactUseLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const usePatternLoaderData = makeTypedPreactUsePatternLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const useRouterData = makeTypedPreactUseRouterData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const addClientLoader = makeTypedPreactAddClientLoader(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const TypedLink = makeTypedPreactLink(PUBLIC_TYPE_TEST_APP_CONFIG);
	const routeProps = null as unknown as PreactVormaRouteProps<
		PublicApp,
		"/users/:userID"
	>;

	const loaderData = useLoaderData(routeProps);
	expectType<VormaLoaderOutput<PublicApp, "/users/:userID">>(loaderData);
	const maybePatternData = usePatternLoaderData("/docs/*");
	expectType<VormaLoaderOutput<PublicApp, "/docs/*"> | undefined>(
		maybePatternData,
	);

	const scopedRouterData = useRouterData(routeProps);
	expectType<Record<"userID", string>>(scopedRouterData.params);
	const unscopedRouterData = useRouterData();
	expectType<Record<string, string>>(unscopedRouterData.params);

	const useClientLoaderData = addClientLoader({
		pattern: "/users/:userID",
		clientLoader: async ({ params, serverDataPromise }) => {
			expectType<string>(params.userID);
			const serverData = await serverDataPromise;
			expectType<VormaLoaderOutput<PublicApp, "/users/:userID">>(
				serverData.loaderData,
			);
			return serverData.loaderData.userName.length;
		},
		reRunOnModuleChange: import.meta,
	});
	const maybeClientLoaderData = useClientLoaderData();
	expectType<number | undefined>(maybeClientLoaderData);
	const scopedClientLoaderData = useClientLoaderData(routeProps);
	expectType<number>(scopedClientLoaderData);

	void TypedLink({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});
	void TypedLink({
		pattern: "/docs/*",
		splatValues: ["guide"],
	});
	void TypedLink({
		pattern: "/blog",
	});

	void usePatternLoaderData(
		// @ts-expect-error typed pattern loader data rejects unknown patterns.
		"/not-a-route",
	);
	void addClientLoader({
		// @ts-expect-error client loader pattern must exist in loader route patterns.
		pattern: "/not-a-route",
		clientLoader: async () => 1,
	});
	void addClientLoader({
		pattern: "/users/:userID",
		clientLoader: async () => 1,
		// @ts-expect-error reRunOnModuleChange must be ImportMeta when provided.
		reRunOnModuleChange: true,
	});
	// @ts-expect-error typed links require params for dynamic routes.
	void TypedLink({
		pattern: "/users/:userID",
	});
	void TypedLink({
		pattern: "/users/:userID",
		// @ts-expect-error typed links enforce exact dynamic param keys.
		params: { id: "u-1" },
	});
	void TypedLink({
		pattern: "/users/:userID",
		// @ts-expect-error typed links require string param values.
		params: { userID: 123 },
	});
	// @ts-expect-error typed links require splatValues for splat routes.
	void TypedLink({
		pattern: "/docs/*",
	});
	void TypedLink({
		pattern: "/docs/*",
		// @ts-expect-error typed links require splatValues to be string[].
		splatValues: "guide",
	});
	void TypedLink({
		// @ts-expect-error typed links reject unknown route patterns.
		pattern: "/not-a-route",
	});
}
void assertTypedPreactAdapterContracts;

function assertTypedSolidAdapterContracts(): void {
	const useLoaderData = makeTypedSolidUseLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const usePatternLoaderData = makeTypedSolidUsePatternLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const useRouterData = makeTypedSolidUseRouterData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const addClientLoader = makeTypedSolidAddClientLoader(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const TypedLink = makeTypedSolidLink(PUBLIC_TYPE_TEST_APP_CONFIG);
	const routeProps = null as unknown as SolidVormaRouteProps<
		PublicApp,
		"/users/:userID"
	>;

	const loaderData = useLoaderData(routeProps);
	expectType<Accessor<VormaLoaderOutput<PublicApp, "/users/:userID">>>(
		loaderData,
	);
	const maybePatternData = usePatternLoaderData("/docs/*");
	expectType<Accessor<VormaLoaderOutput<PublicApp, "/docs/*"> | undefined>>(
		maybePatternData,
	);

	const scopedRouterData = useRouterData(routeProps);
	expectType<Accessor<{ params: Record<"userID", string> }>>(
		scopedRouterData,
	);
	const unscopedRouterData = useRouterData();
	expectType<Accessor<{ params: Record<string, string> }>>(
		unscopedRouterData,
	);

	const useClientLoaderData = addClientLoader({
		pattern: "/users/:userID",
		clientLoader: async ({ params, serverDataPromise }) => {
			expectType<string>(params.userID);
			const serverData = await serverDataPromise;
			expectType<VormaLoaderOutput<PublicApp, "/users/:userID">>(
				serverData.loaderData,
			);
			return serverData.loaderData.userName.length;
		},
		reRunOnModuleChange: import.meta,
	});
	const maybeClientLoaderData = useClientLoaderData();
	expectType<Accessor<number | undefined>>(maybeClientLoaderData);
	const scopedClientLoaderData = useClientLoaderData(routeProps);
	expectType<Accessor<number>>(scopedClientLoaderData);

	void TypedLink({
		pattern: "/users/:userID",
		params: { userID: "u-1" },
	});
	void TypedLink({
		pattern: "/docs/*",
		splatValues: ["guide"],
	});
	void TypedLink({
		pattern: "/blog",
	});

	void usePatternLoaderData(
		// @ts-expect-error typed pattern loader data rejects unknown patterns.
		"/not-a-route",
	);
	void addClientLoader({
		// @ts-expect-error client loader pattern must exist in loader route patterns.
		pattern: "/not-a-route",
		clientLoader: async () => 1,
	});
	void addClientLoader({
		pattern: "/users/:userID",
		clientLoader: async () => 1,
		// @ts-expect-error reRunOnModuleChange must be ImportMeta when provided.
		reRunOnModuleChange: true,
	});
	// @ts-expect-error typed links require params for dynamic routes.
	void TypedLink({
		pattern: "/users/:userID",
	});
	void TypedLink({
		pattern: "/users/:userID",
		// @ts-expect-error typed links enforce exact dynamic param keys.
		params: { id: "u-1" },
	});
	void TypedLink({
		pattern: "/users/:userID",
		// @ts-expect-error typed links require string param values.
		params: { userID: 123 },
	});
	// @ts-expect-error typed links require splatValues for splat routes.
	void TypedLink({
		pattern: "/docs/*",
	});
	void TypedLink({
		pattern: "/docs/*",
		// @ts-expect-error typed links require splatValues to be string[].
		splatValues: "guide",
	});
	void TypedLink({
		// @ts-expect-error typed links reject unknown route patterns.
		pattern: "/not-a-route",
	});
}
void assertTypedSolidAdapterContracts;

type _assertTypedLoaderPattern = Assert<
	IsExact<
		VormaLoaderPattern<PublicApp>,
		"/" | "/users/:userID" | "/docs/*" | "/blog/_index"
	>
>;

function assertPublicRuntimeAndAdapterComponentContracts(): void {
	const removeStatusListener = addStatusListener((event) => {
		expectType<StatusEvent>(event);
		expectType<boolean>(event.detail.isNavigating);
		expectType<boolean>(event.detail.isSubmitting);
		expectType<boolean>(event.detail.isRevalidating);
	});
	expectType<() => void>(removeStatusListener);

	const removeRouteChangeListener = addRouteChangeListener((event) => {
		expectType<RouteChangeEvent>(event);
		const scrollState = event.detail.__scrollState;
		if (scrollState === undefined) {
			return;
		}
		if ("hash" in scrollState) {
			expectType<string>(scrollState.hash);
			return;
		}
		expectType<number>(scrollState.x);
		expectType<number>(scrollState.y);
	});
	expectType<() => void>(removeRouteChangeListener);

	const removeLocationListener = addLocationListener((event) => {
		expectType<string>(event.detail.pathname);
		expectType<string>(event.detail.search);
		expectType<string>(event.detail.hash);
		expectType<unknown>(event.detail.state);
	});
	expectType<() => void>(removeLocationListener);

	const removeBuildIDListener = addBuildIDListener((event) => {
		expectType<string>(event.detail.oldID);
		expectType<string>(event.detail.newID);
	});
	expectType<() => void>(removeBuildIDListener);

	const builtQueryURL = buildQueryURL(PUBLIC_TYPE_TEST_APP_CONFIG, {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { includePosts: true },
	});
	expectType<URL>(builtQueryURL);

	const builtMutationURL = buildMutationURL(PUBLIC_TYPE_TEST_APP_CONFIG, {
		pattern: "/users/:userID",
		params: { userID: "u-1" },
		input: { nickname: "neo" },
	});
	expectType<URL>(builtMutationURL);

	const resolvedBody = resolveBody({
		input: { hello: "world" },
	});
	expectType<BodyInit | null | undefined>(resolvedBody);

	const formattedError = defaultErrorBoundary({
		error: new Error("boom"),
	});
	expectType<string>(formattedError);

	expectType<string>(getBuildID());
	expectType<HTMLElement>(getRootEl());
	expectType<string>(getLocation().pathname);
	expectType<boolean>(getStatus().isNavigating);
	void getHistoryInstance();

	const unscopedRouterData = getRouterData<PublicApp>();
	expectType<Record<string, string>>(unscopedRouterData.params);
	expectType<{ sessionUserID: string | null }>(unscopedRouterData.rootData);

	const scopedRouteProps = null as unknown as ReactVormaRouteProps<
		PublicApp,
		"/users/:userID"
	>;
	const scopedRouterData = getRouterData<PublicApp, false, "/users/:userID">(
		scopedRouteProps,
	);
	expectType<Record<"userID", string>>(scopedRouterData.params);

	const initClientPromise = initClient({
		vormaAppConfig: PUBLIC_TYPE_TEST_APP_CONFIG,
		rootElementID: "vorma-root",
		isDev: true,
		viteDevURL: "http://127.0.0.1:5173",
		publicPathPrefix: "/",
		routeManifestURL: "/route-manifest.json",
		useViewTransitions: true,
		renderFn: async () => {},
	});
	expectType<Promise<void>>(initClientPromise);

	const navigatePromise = vormaNavigate("/users/u-1", {
		replace: true,
		scrollToTop: false,
		state: { source: "tests" },
	});
	expectType<Promise<{ didNavigate: boolean }>>(navigatePromise);

	const revalidatePromise = revalidate();
	expectType<Promise<{ didNavigate: boolean }>>(revalidatePromise);

	const submitUntypedPromise = submit("/api/users", {
		method: "POST",
	});
	expectType<Promise<SubmitResult<unknown>>>(submitUntypedPromise);

	const submitTypedPromise = submit<{ ok: true }>("/api/users", {
		method: "POST",
	});
	expectType<Promise<SubmitResult<{ ok: true }>>>(submitTypedPromise);

	const stopFocusRevalidate = revalidateOnWindowFocus({
		staleTimeMS: 3000,
	});
	expectType<() => void>(stopFocusRevalidate);

	const stopGlobalLoadingIndicator = setupGlobalLoadingIndicator({
		start: () => {},
		stop: () => {},
		isRunning: () => false,
		include: ["navigations", "submissions", "revalidations"],
		startDelayMS: 30,
		stopDelayMS: 40,
	});
	expectType<() => void>(stopGlobalLoadingIndicator);

	expectType<string>(useReactLocation().pathname);
	void ReactVormaRootOutlet({
		idx: 0,
	});
	void ReactVormaLink({
		href: "/users/u-1",
		prefetch: "intent",
		prefetchDelayMs: 0,
		replace: true,
		scrollToTop: false,
		state: { via: "react" },
		beforeBegin: async () => {},
		beforeRender: () => {},
		afterRender: () => {},
	});
	// @ts-expect-error prefetch only accepts intent or none.
	void ReactVormaLink({ prefetch: "hover" });

	expectType<string>(preactLocationSignal.value.pathname);
	void PreactVormaRootOutlet({
		idx: 0,
	});
	void PreactVormaLink({
		href: "/users/u-1",
		prefetch: "intent",
	});

	expectType<string>(readSolidLocation().pathname);
	void SolidVormaRootOutlet({
		idx: 0,
	});
	void SolidVormaLink({
		href: "/users/u-1",
		prefetch: "intent",
	});
}
void assertPublicRuntimeAndAdapterComponentContracts;
