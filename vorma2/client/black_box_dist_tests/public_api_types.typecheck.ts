import type { Accessor } from "solid-js";
import type {
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
	addClientBuildIDListener,
	addRouteChangeListener,
	addStatusListener,
	getClientBuildID,
	getRootEl,
	getRouterData,
	getStatus,
	initClient,
	makeTypedAPIClient,
	makeTypedNavigate,
	revalidate,
	revalidateOnWindowFocus,
	setupGlobalLoadingIndicator,
	submit,
	vormaNavigate,
} from "vorma/client";
import {
	VormaLink as Preact__VormaLink,
	VormaRootOutlet as Preact__VormaRootOutlet,
	makeTypedAddClientLoader as preact__makeTypedAddClientLoader,
	makeTypedLink as preact__makeTypedLink,
	makeTypedUseLoaderData as preact__makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData as preact__makeTypedUsePatternLoaderData,
	makeTypedUseRouterData as preact__makeTypedUseRouterData,
	type VormaRouteProps as Preact__VormaRouteProps,
} from "vorma/preact";
import {
	VormaLink as React__VormaLink,
	VormaRootOutlet as React__VormaRootOutlet,
	makeTypedAddClientLoader as react__makeTypedAddClientLoader,
	makeTypedLink as react__makeTypedLink,
	makeTypedUseLoaderData as react__makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData as react__makeTypedUsePatternLoaderData,
	makeTypedUseRouterData as react__makeTypedUseRouterData,
	type VormaRouteProps as React__VormaRouteProps,
} from "vorma/react";
import {
	VormaLink as Solid__VormaLink,
	VormaRootOutlet as Solid__VormaRootOutlet,
	makeTypedAddClientLoader as solid__makeTypedAddClientLoader,
	makeTypedLink as solid__makeTypedLink,
	makeTypedUseLoaderData as solid__makeTypedUseLoaderData,
	makeTypedUsePatternLoaderData as solid__makeTypedUsePatternLoaderData,
	makeTypedUseRouterData as solid__makeTypedUseRouterData,
	type VormaRouteProps as Solid__VormaRouteProps,
} from "vorma/solid";
import type {
	APIRequestInitDecorator,
	ExtractApp,
	ParamsForPattern,
} from "../types.ts";

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
	const useLoaderData = react__makeTypedUseLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const usePatternLoaderData = react__makeTypedUsePatternLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const useRouterData = react__makeTypedUseRouterData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const addClientLoader = react__makeTypedAddClientLoader(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const TypedLink = react__makeTypedLink(PUBLIC_TYPE_TEST_APP_CONFIG);
	const routeProps = null as unknown as React__VormaRouteProps<
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
	const useLoaderData = preact__makeTypedUseLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const usePatternLoaderData = preact__makeTypedUsePatternLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const useRouterData = preact__makeTypedUseRouterData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const addClientLoader = preact__makeTypedAddClientLoader(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const TypedLink = preact__makeTypedLink(PUBLIC_TYPE_TEST_APP_CONFIG);
	const routeProps = null as unknown as Preact__VormaRouteProps<
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
	const useLoaderData = solid__makeTypedUseLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const usePatternLoaderData = solid__makeTypedUsePatternLoaderData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const useRouterData = solid__makeTypedUseRouterData(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const addClientLoader = solid__makeTypedAddClientLoader(
		PUBLIC_TYPE_TEST_APP_CONFIG,
	);
	const TypedLink = solid__makeTypedLink(PUBLIC_TYPE_TEST_APP_CONFIG);
	const routeProps = null as unknown as Solid__VormaRouteProps<
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

	const removeClientBuildIDListener = addClientBuildIDListener((event) => {
		expectType<string>(event.detail.oldClientBuildID);
		expectType<string>(event.detail.newClientBuildID);
	});
	expectType<() => void>(removeClientBuildIDListener);

	expectType<string>(getClientBuildID());
	expectType<HTMLElement>(getRootEl());
	expectType<boolean>(getStatus().isNavigating);

	const unscopedRouterData = getRouterData<PublicApp>();
	expectType<Record<string, string>>(unscopedRouterData.params);
	expectType<{ sessionUserID: string | null }>(unscopedRouterData.rootData);

	const scopedRouteProps = null as unknown as React__VormaRouteProps<
		PublicApp,
		"/users/:userID"
	>;
	const scopedRouterData = getRouterData<PublicApp, "/users/:userID">(
		scopedRouteProps,
	);
	expectType<Record<"userID", string>>(scopedRouterData.params);

	const initClientPromise = initClient({
		vormaAppConfig: PUBLIC_TYPE_TEST_APP_CONFIG,
		isDev: true,
		publicPathPrefix: "/",
		routeManifestURL: "/route-manifest.json",
		useViewTransitions: true,
		renderFn: async () => {},
	});
	expectType<Promise<void>>(initClientPromise);

	const navigatePromise = vormaNavigate("/users/u-1", {
		replace: true,
		scrollToTop: false,
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

	void React__VormaRootOutlet({
		idx: 0,
	});
	void React__VormaLink({
		href: "/users/u-1",
		prefetch: "intent",
		prefetchDelayMs: 0,
		replace: true,
		scrollToTop: false,
		beforeNavigate: async () => {},
		beforeRender: () => {},
		afterRender: () => {},
	});
	// @ts-expect-error prefetch only accepts intent or none.
	void React__VormaLink({ prefetch: "hover" });

	void Preact__VormaRootOutlet({
		idx: 0,
	});
	void Preact__VormaLink({
		href: "/users/u-1",
		prefetch: "intent",
	});

	void Solid__VormaRootOutlet({
		idx: 0,
	});
	void Solid__VormaLink({
		href: "/users/u-1",
		prefetch: "intent",
	});
}
void assertPublicRuntimeAndAdapterComponentContracts;
