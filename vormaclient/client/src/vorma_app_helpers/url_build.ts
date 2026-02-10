import { serializeToSearchParams } from "vorma/kit/json";
import { resolveVormaPath } from "./path_resolution.ts";

type URLBuildProps = {
	pattern: string;
	params?: Record<string, unknown>;
	splatValues?: Array<string>;
	input?: unknown;
};

type URLBuildConfig = {
	actionsRouterMountRoot: string;
	actionsDynamicRune: string;
	actionsSplatRune: string;
	loadersDynamicRune: string;
	loadersSplatRune: string;
	loadersExplicitIndexSegment: string;
};

type URLBuildInput = {
	vormaAppConfig: URLBuildConfig;
	type: "loader" | "query" | "mutation";
	props: URLBuildProps;
};

export function buildVormaURL(input: URLBuildInput): URL {
	const basePath = stripTrailingSlash(
		input.vormaAppConfig.actionsRouterMountRoot,
	);
	const resolvedPath = resolveVormaPath(input);
	const url = new URL(basePath + resolvedPath, getCurrentOrigin());

	if (input.type === "query" && input.props.input) {
		url.search = serializeToSearchParams(input.props.input).toString();
	}

	return url;
}

function getCurrentOrigin(): string {
	return new URL(window.location.href).origin;
}

function stripTrailingSlash(path: string): string {
	return path.endsWith("/") ? path.slice(0, -1) : path;
}
