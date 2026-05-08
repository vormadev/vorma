/// <reference types="vite/client" />

import { Data, Effect } from "effect";

export type ModulePreloadOptions = {
	dev?: boolean;
};

export class ModulePreloadFailed extends Data.TaggedError(
	"ModulePreloadFailed",
)<{
	readonly error: unknown;
}> {}

export function preload_modules(deps: string[]): void {
	Effect.runSync(
		preload_modules_effect(deps).pipe(
			Effect.mapError((error) => {
				return error.error;
			}),
		),
	);
}

export function preload_modules_effect(
	deps: string[],
	options: ModulePreloadOptions = {},
): Effect.Effect<void, ModulePreloadFailed> {
	return Effect.try({
		try: () => {
			preload_modules_sync(deps, options);
		},
		catch: (error) => {
			return new ModulePreloadFailed({ error });
		},
	});
}

function preload_modules_sync(
	deps: string[],
	options: ModulePreloadOptions,
): void {
	if (options.dev ?? import.meta.env.DEV) {
		return;
	}
	for (const dep of new Set(deps)) {
		if (
			document.head.querySelector(
				`link[rel="modulepreload"][href="${dep}"]`,
			)
		) {
			continue;
		}
		const link = document.createElement("link");
		link.rel = "modulepreload";
		link.href = dep;
		document.head.appendChild(link);
	}
}
