import { batch, computed, effect, signal } from "@preact/signals";
import { h, type ComponentType } from "preact";
import { useEffect, useLayoutEffect, useMemo, useRef } from "preact/hooks";
import {
	__applyScrollState,
	__getClientRuntimeRenderState,
	addLocationListener,
	addRouteChangeListener,
	getLocation,
	getRouterData,
	type RouteChangeEvent,
} from "vorma/client";

/////////////////////////////////////////////////////////////////////
/////// CORE SETUP
/////////////////////////////////////////////////////////////////////

type VormaOutletProps = {
	idx: number;
	Outlet: (localProps: Record<string, any> | undefined) => h.JSX.Element;
};

type VormaOutletComponent = ComponentType<VormaOutletProps>;

type VormaErrorBoundaryProps = {
	error: string | undefined;
};

type VormaErrorBoundaryComponent = ComponentType<VormaErrorBoundaryProps>;

const latestEvent = signal<RouteChangeEvent | null>(null);
const initialRenderState = __getClientRuntimeRenderState();
const loadersData = signal(initialRenderState.loadersData);
const clientLoadersData = signal(initialRenderState.clientLoadersData);
const routerData = signal(getRouterData());
const outermostErrorIdx = signal(initialRenderState.outermostErrorIdx);
const outermostError = signal(initialRenderState.outermostError);
const activeComponents = signal<Array<VormaOutletComponent> | null>(
	initialRenderState.activeComponents as Array<VormaOutletComponent> | null,
);
const activeErrorBoundary = signal<VormaErrorBoundaryComponent | undefined>(
	initialRenderState.activeErrorBoundary as
		| VormaErrorBoundaryComponent
		| undefined,
);
const importURLs = signal(initialRenderState.importURLs);
const exportKeys = signal(initialRenderState.exportKeys);

export { clientLoadersData, loadersData, routerData };

let isInited = false;

function syncRuntimeRenderState(): void {
	const renderState = __getClientRuntimeRenderState();
	loadersData.value = renderState.loadersData;
	clientLoadersData.value = renderState.clientLoadersData;
	outermostErrorIdx.value = renderState.outermostErrorIdx;
	outermostError.value = renderState.outermostError;
	activeComponents.value =
		renderState.activeComponents as Array<VormaOutletComponent> | null;
	activeErrorBoundary.value = renderState.activeErrorBoundary as
		| VormaErrorBoundaryComponent
		| undefined;
	importURLs.value = renderState.importURLs;
	exportKeys.value = renderState.exportKeys;
}

function initUIListeners() {
	if (isInited) return;
	isInited = true;

	addRouteChangeListener((e) => {
		batch(() => {
			latestEvent.value = e;
			syncRuntimeRenderState();
			routerData.value = getRouterData();
		});
	});

	addLocationListener(() => {
		location.value = getLocation();
	});
}

export const location = signal(getLocation());

/////////////////////////////////////////////////////////////////////
/////// COMPONENT
/////////////////////////////////////////////////////////////////////

export function VormaRootOutlet(props: { idx?: number }): h.JSX.Element {
	const idx = props.idx ?? 0;

	const initialRenderRef = useRef(true);

	if (idx === 0 && initialRenderRef.current) {
		initUIListeners();

		initialRenderRef.current = false;
		batch(() => {
			syncRuntimeRenderState();
			routerData.value = getRouterData();
		});
	}

	const currentImportURL = signal(importURLs.value[idx]);
	const currentExportKey = signal(exportKeys.value[idx]);
	const nextImportURL = signal(importURLs.value[idx + 1]);
	const nextExportKey = signal(exportKeys.value[idx + 1]);

	useEffect(() => {
		const dispose = effect(() => {
			if (!currentImportURL.value || !latestEvent.value) {
				return;
			}

			batch(() => {
				const newCurrentImportURL = importURLs.value[idx];
				const newCurrentExportKey = exportKeys.value[idx];

				if (currentImportURL.value !== newCurrentImportURL) {
					currentImportURL.value = newCurrentImportURL;
				}
				if (currentExportKey.value !== newCurrentExportKey) {
					currentExportKey.value = newCurrentExportKey;
				}

				// these are also needed for Outlets to render correctly
				const newNextImportURL = importURLs.value[idx + 1];
				const newNextExportKey = exportKeys.value[idx + 1];

				if (nextImportURL.value !== newNextImportURL) {
					nextImportURL.value = newNextImportURL;
				}
				if (nextExportKey.value !== newNextExportKey) {
					nextExportKey.value = newNextExportKey;
				}
			});
		});

		return dispose;
	}, [idx]);

	useLayoutEffect(() => {
		const dispose = effect(() => {
			const event = latestEvent.value;
			if (!event || idx !== 0) {
				return;
			}
			window.requestAnimationFrame(() => {
				__applyScrollState(event.detail.__scrollState);
			});
		});

		return dispose;
	}, [idx]);

	const isErrorIdx = computed(() => idx === outermostErrorIdx.value);

	const CurrentComp = computed<VormaOutletComponent | null>(() => {
		if (isErrorIdx.value) {
			return null;
		}
		void currentImportURL.value;
		void currentExportKey.value;
		return activeComponents.value?.[idx] ?? null;
	});

	const Outlet = useMemo(
		() => (localProps: Record<string, any> | undefined) => {
			return h(VormaRootOutlet, {
				...localProps,
				...props,
				idx: idx + 1,
			});
		},
		[nextImportURL.value, nextExportKey.value],
	);

	const shouldFallbackOutlet = computed(() => {
		if (isErrorIdx.value) {
			return false;
		}
		if (CurrentComp.value) {
			return false;
		}
		return idx + 1 < loadersData.value.length;
	});

	const ErrorComp = computed<VormaErrorBoundaryComponent | null>(() => {
		if (!isErrorIdx.value) {
			return null;
		}
		return activeErrorBoundary.value ?? null;
	});

	if (isErrorIdx.value) {
		if (ErrorComp.value) {
			return h(ErrorComp.value, { error: outermostError.value });
		}
		return h("div", {}, `Error: ${outermostError.value || "unknown"}`);
	}

	if (!CurrentComp.value) {
		if (shouldFallbackOutlet.value) {
			return h(Outlet, {});
		}
		return h("div", {});
	}

	return h(CurrentComp.value, { idx, Outlet });
}
