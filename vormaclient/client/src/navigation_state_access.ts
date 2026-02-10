import type {
	NavigateProps,
	NavigationStateManager,
} from "./navigation_runtime/types.ts";

export type NavigationStateAccess = Pick<
	NavigationStateManager,
	"navigate" | "removeNavigation" | "getNavigations"
> & {
	navigate: (props: NavigateProps) => Promise<{ didNavigate: boolean }>;
};

let navigationStateAccess: NavigationStateAccess | null = null;

export function setNavigationStateAccess(access: NavigationStateAccess): void {
	navigationStateAccess = access;
}

export function getNavigationStateAccess(): NavigationStateAccess {
	if (!navigationStateAccess) {
		throw new Error("Navigation state access has not been initialized.");
	}
	return navigationStateAccess;
}
