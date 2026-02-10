export type SkipCheckContext = {
	routeManifest: Record<string, number>;
	patternRegistry: any;
	patternToWaitFnMap: Record<string, any>;
	clientModuleMap: Record<
		string,
		{ importURL: string; exportKey: string; errorExportKey: string }
	>;
	currentMatchedPatterns: string[];
	currentParams: Record<string, string>;
	currentSplatValues: string[];
	currentLoadersData: any[];
	url: URL;
	matchResult: any;
};

export type SkipCheckResult =
	| { canSkip: false }
	| {
			canSkip: true;
			matchResult: any;
			importURLs: string[];
			exportKeys: string[];
			loadersData: any[];
	  };
