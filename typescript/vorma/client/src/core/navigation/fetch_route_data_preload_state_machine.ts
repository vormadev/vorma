export type ServerSuccessPreloadExecutionPlan =
	| {
			type: "skip";
			reason: "server_success_preload_skipped_signal_aborted";
	  }
	| {
			type: "preload";
			moduleDependenciesToPreload: string[];
			cssBundlesToPreload: string[];
			reason: "server_success_preload_allowed";
	  };

export function decideServerSuccessPreloadExecutionPlan(props: {
	signalAborted: boolean;
	isDev: boolean;
	importURLs: string[];
	deps: string[];
	cssBundles: string[];
}): ServerSuccessPreloadExecutionPlan {
	if (props.signalAborted) {
		return {
			type: "skip",
			reason: "server_success_preload_skipped_signal_aborted",
		};
	}

	const moduleDependenciesToPreload = props.isDev
		? [...new Set(props.importURLs)]
		: props.deps;

	return {
		type: "preload",
		moduleDependenciesToPreload,
		cssBundlesToPreload: props.cssBundles,
		reason: "server_success_preload_allowed",
	};
}
