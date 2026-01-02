import {
	type JSX,
	useEffect,
	useLayoutEffect,
	useMemo,
	useRef,
	useState,
	useSyncExternalStore,
} from "react";
import {
	__applyScrollState,
	addLocationListener,
	addRouteChangeListener,
	__vormaClientGlobal as ctx,
	getLocation,
	getRouterData,
	type RouteChangeEvent,
} from "vorma/client";

/////////////////////////////////////////////////////////////////////
/////// STORE
/////////////////////////////////////////////////////////////////////

type NavigationState = {
	latestEvent: RouteChangeEvent | null;
	loadersData: any;
	clientLoadersData: any;
	routerData: ReturnType<typeof getRouterData>;
	outermostError: any;
	outermostErrorIdx: number | undefined;
	activeComponents: any[] | null;
	activeErrorBoundary: any;
	importURLs: string[];
	exportKeys: string[];
};

type StoreState = {
	navigation: NavigationState;
	location: ReturnType<typeof getLocation>;
};

function getInitialState(): StoreState {
	return {
		navigation: {
			latestEvent: null,
			loadersData: ctx.get("loadersData"),
			clientLoadersData: ctx.get("clientLoadersData"),
			routerData: getRouterData(),
			outermostError: ctx.get("outermostError"),
			outermostErrorIdx: ctx.get("outermostErrorIdx"),
			activeComponents: ctx.get("activeComponents"),
			activeErrorBoundary: ctx.get("activeErrorBoundary"),
			importURLs: ctx.get("importURLs"),
			exportKeys: ctx.get("exportKeys"),
		},
		location: getLocation(),
	};
}

let state = getInitialState();
const listeners = new Set<() => void>();

const store = {
	getSnapshot: () => state,
	subscribe: (listener: () => void) => {
		listeners.add(listener);
		return () => listeners.delete(listener);
	},
	setState: (updater: (prevState: StoreState) => StoreState) => {
		const nextState = updater(state);
		if (nextState !== state) {
			state = nextState;
			listeners.forEach((listener) => listener());
		}
	},
};

function useStoreSelector<T>(selector: (state: StoreState) => T): T {
	const getSelectedSnapshot = useMemo(() => {
		let selectedSnapshot: T;
		return () => {
			const nextSnapshot = selector(store.getSnapshot());
			if (
				selectedSnapshot === undefined ||
				!Object.is(selectedSnapshot, nextSnapshot)
			) {
				selectedSnapshot = nextSnapshot;
			}
			return selectedSnapshot;
		};
	}, [selector]);

	return useSyncExternalStore(store.subscribe, getSelectedSnapshot);
}

export function useLoadersData() {
	return useStoreSelector((s) => s.navigation.loadersData);
}
export function useClientLoadersData() {
	return useStoreSelector((s) => s.navigation.clientLoadersData);
}
export function useRouterData() {
	return useStoreSelector((s) => s.navigation.routerData);
}
function useLatestEvent() {
	return useStoreSelector((s) => s.navigation.latestEvent);
}
function useOutermostError() {
	return useStoreSelector((s) => s.navigation.outermostError);
}
function useOutermostErrorIdx() {
	return useStoreSelector((s) => s.navigation.outermostErrorIdx);
}
function useActiveComponents() {
	return useStoreSelector((s) => s.navigation.activeComponents);
}
function useActiveErrorBoundary() {
	return useStoreSelector((s) => s.navigation.activeErrorBoundary);
}
function useImportURLs() {
	return useStoreSelector((s) => s.navigation.importURLs);
}
function useExportKeys() {
	return useStoreSelector((s) => s.navigation.exportKeys);
}

let isInited = false;

function initUIListeners() {
	if (isInited) return;
	isInited = true;

	addRouteChangeListener((e) => {
		store.setState((prev) => {
			return {
				...prev,
				navigation: {
					latestEvent: e,
					loadersData: ctx.get("loadersData"),
					clientLoadersData: ctx.get("clientLoadersData"),
					routerData: getRouterData(),
					outermostError: ctx.get("outermostError"),
					outermostErrorIdx: ctx.get("outermostErrorIdx"),
					activeComponents: ctx.get("activeComponents"),
					activeErrorBoundary: ctx.get("activeErrorBoundary"),
					importURLs: ctx.get("importURLs"),
					exportKeys: ctx.get("exportKeys"),
				},
			};
		});
	});

	addLocationListener(() => {
		store.setState((prev) => {
			return {
				...prev,
				location: getLocation(),
			};
		});
	});
}

export function useLocation() {
	return useStoreSelector((s) => s.location);
}

/////////////////////////////////////////////////////////////////////
/////// COMPONENT
/////////////////////////////////////////////////////////////////////

export function VormaRootOutlet(props: { idx?: number }): JSX.Element {
	const idx = props.idx ?? 0;

	const initialRenderRef = useRef(true);

	if (idx === 0 && initialRenderRef.current) {
		initUIListeners();

		initialRenderRef.current = false;
		store.setState((prev) => {
			return {
				...prev,
				navigation: {
					latestEvent: null,
					loadersData: ctx.get("loadersData"),
					clientLoadersData: ctx.get("clientLoadersData"),
					routerData: getRouterData(),
					outermostError: ctx.get("outermostError"),
					outermostErrorIdx: ctx.get("outermostErrorIdx"),
					activeComponents: ctx.get("activeComponents"),
					activeErrorBoundary: ctx.get("activeErrorBoundary"),
					importURLs: ctx.get("importURLs"),
					exportKeys: ctx.get("exportKeys"),
				},
			};
		});
	}

	const latestEvent = useLatestEvent();
	const loadersData = useLoadersData();
	const outermostError = useOutermostError();
	const outermostErrorIdx = useOutermostErrorIdx();
	const activeComponents = useActiveComponents();
	const activeErrorBoundary = useActiveErrorBoundary();
	const importURLs = useImportURLs();
	const exportKeys = useExportKeys();

	const [currentImportURL, setCurrentImportURL] = useState(importURLs[idx]);
	const [currentExportKey, setCurrentExportKey] = useState(exportKeys[idx]);
	const [nextImportURL, setNextImportURL] = useState(importURLs[idx + 1]);
	const [nextExportKey, setNextExportKey] = useState(exportKeys[idx + 1]);

	useEffect(() => {
		if (!currentImportURL || !latestEvent) {
			return;
		}

		const newCurrentImportURL = importURLs[idx];
		const newCurrentExportKey = exportKeys[idx];

		if (currentImportURL !== newCurrentImportURL) {
			setCurrentImportURL(newCurrentImportURL);
		}
		if (currentExportKey !== newCurrentExportKey) {
			setCurrentExportKey(newCurrentExportKey);
		}

		// these are also needed for Outlets to render correctly
		const newNextImportURL = importURLs[idx + 1];
		const newNextExportKey = exportKeys[idx + 1];

		if (nextImportURL !== newNextImportURL) {
			setNextImportURL(newNextImportURL);
		}
		if (nextExportKey !== newNextExportKey) {
			setNextExportKey(newNextExportKey);
		}
	}, [latestEvent, importURLs, exportKeys]);

	useLayoutEffect(() => {
		if (!latestEvent || idx !== 0) {
			return;
		}
		window.requestAnimationFrame(() => {
			__applyScrollState(latestEvent.detail.__scrollState);
		});
	}, [latestEvent, idx]);

	const isErrorIdxMemo = useMemo(() => {
		return idx === outermostErrorIdx;
	}, [idx, outermostErrorIdx]);

	const CurrentCompMemo = useMemo(() => {
		if (isErrorIdxMemo) {
			return null;
		}
		return activeComponents?.[idx];
	}, [isErrorIdxMemo, currentImportURL, currentExportKey, activeComponents]);

	const Outlet = useMemo(
		() => (localProps: Record<string, any> | undefined) => {
			return <VormaRootOutlet {...localProps} {...props} idx={idx + 1} />;
		},
		[nextImportURL, nextExportKey],
	);

	const shouldFallbackOutletMemo = useMemo(() => {
		if (isErrorIdxMemo) {
			return false;
		}
		if (CurrentCompMemo) {
			return false;
		}
		return idx + 1 < loadersData.length;
	}, [isErrorIdxMemo, CurrentCompMemo, idx, loadersData]);

	const ErrorCompMemo = useMemo(() => {
		if (!isErrorIdxMemo) {
			return null;
		}
		return activeErrorBoundary;
	}, [isErrorIdxMemo, activeErrorBoundary]);

	if (isErrorIdxMemo) {
		if (ErrorCompMemo) {
			return <ErrorCompMemo error={outermostError} />;
		}
		return <>{`Error: ${outermostError || "unknown"}`}</>;
	}

	if (!CurrentCompMemo) {
		if (shouldFallbackOutletMemo) {
			return <Outlet />;
		}
		return <></>;
	}

	return <CurrentCompMemo idx={idx} Outlet={Outlet} />;
}
