import { createPatternRegistry } from "vorma/kit/matcher/register";
import type { VormaAppConfig } from "./vorma_app_helpers/vorma_app_helpers.ts";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export function initializeClientPatternRegistry(
	vormaAppConfig: VormaAppConfig,
): void {
	const patternRegistry = createPatternRegistry({
		dynamicParamPrefixRune: vormaAppConfig.loadersDynamicRune,
		splatSegmentRune: vormaAppConfig.loadersSplatRune,
		explicitIndexSegment: vormaAppConfig.loadersExplicitIndexSegment,
	});
	__vormaClientGlobal.set("patternRegistry", patternRegistry);
}
