type PathResolutionProps = {
	pattern: string;
	params?: Record<string, unknown>;
	splatValues?: Array<string>;
};

type PathResolutionConfig = {
	actionsDynamicRune: string;
	actionsSplatRune: string;
	loadersDynamicRune: string;
	loadersSplatRune: string;
	loadersExplicitIndexSegment: string;
};

type ResolvePathInput = {
	vormaAppConfig: PathResolutionConfig;
	type: "loader" | "query" | "mutation";
	props: PathResolutionProps;
};

export function resolveVormaPath(input: ResolvePathInput): string {
	const { props, vormaAppConfig } = input;
	let path = props.pattern;

	let dynamicParamPrefixRune = vormaAppConfig.actionsDynamicRune;
	let splatSegmentRune = vormaAppConfig.actionsSplatRune;

	if (input.type === "loader") {
		dynamicParamPrefixRune = vormaAppConfig.loadersDynamicRune;
		splatSegmentRune = vormaAppConfig.loadersSplatRune;
	}

	if ("params" in props && props.params) {
		for (const [key, value] of Object.entries(props.params)) {
			path = path.replace(
				`${dynamicParamPrefixRune}${key}`,
				String(value),
			);
		}
	}

	if ("splatValues" in props && props.splatValues) {
		const splatPath = props.splatValues.join("/");
		path = path.replace(splatSegmentRune, splatPath);
	}

	// Strip explicit index segment
	if (input.type === "loader" && vormaAppConfig.loadersExplicitIndexSegment) {
		const indexSegment = `/${vormaAppConfig.loadersExplicitIndexSegment}`;
		if (path.endsWith(indexSegment)) {
			path = path.slice(0, -indexSegment.length) || "/";
		}
	}

	return path;
}
