import {
	beginPrefetch as executeBeginPrefetch,
	beginRevalidation as executeBeginRevalidation,
	beginUserNavigation as executeBeginUserNavigation,
	type BeginNavigationContext,
} from "./begin_navigation.ts";
import { beginNavigationWithHandlers } from "./navigate.ts";
import type {
	NavigateProps,
	NavigationControl,
	NavigationIntent,
} from "./types.ts";

export type RuntimeBeginNavigation = (
	props: NavigateProps,
) => NavigationControl;

export type CreateRuntimeBeginNavigationOptions = {
	beginNavigationContext: BeginNavigationContext;
	createActiveNavigation: (
		props: NavigateProps,
		intent: NavigationIntent,
	) => NavigationControl;
};

export function createRuntimeBeginNavigation(
	options: CreateRuntimeBeginNavigationOptions,
): RuntimeBeginNavigation {
	const { beginNavigationContext, createActiveNavigation } = options;

	function beginUserNavigation(
		props: NavigateProps,
		targetUrl: string,
	): NavigationControl {
		return executeBeginUserNavigation(
			beginNavigationContext,
			props,
			targetUrl,
		);
	}

	function beginPrefetch(
		props: NavigateProps,
		targetUrl: string,
	): NavigationControl {
		return executeBeginPrefetch(beginNavigationContext, props, targetUrl);
	}

	function beginRevalidation(props: NavigateProps): NavigationControl {
		return executeBeginRevalidation(beginNavigationContext, props);
	}

	return function beginNavigation(props: NavigateProps): NavigationControl {
		return beginNavigationWithHandlers(
			{
				beginUserNavigation,
				beginPrefetch,
				beginRevalidation,
				createActiveNavigation,
			},
			props,
		);
	};
}
