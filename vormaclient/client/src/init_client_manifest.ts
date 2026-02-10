import { registerPattern } from "vorma/kit/matcher/register";
import { __vormaClientGlobal } from "./vorma_ctx/vorma_ctx.ts";

export function loadRouteManifestProgressively(): void {
	const manifestURL = __vormaClientGlobal.get("routeManifestURL");
	if (!manifestURL) {
		return;
	}

	const patternRegistry = __vormaClientGlobal.get("patternRegistry");
	if (!patternRegistry) {
		return;
	}

	fetch(manifestURL)
		.then((response) => response.json())
		.then((manifest) => {
			__vormaClientGlobal.set("routeManifest", manifest);

			// Register all patterns from manifest into the existing registry
			for (const pattern of Object.keys(manifest)) {
				registerPattern(patternRegistry, pattern);
			}
		})
		.catch((error) => {
			// This is no biggie -- it's a progressive enhancement
			console.warn("Failed to load route manifest:", error);
		});
}
