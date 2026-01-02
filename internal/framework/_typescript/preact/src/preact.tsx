import { batch, computed, effect, signal } from "@preact/signals";
import { h } from "preact";
import { useEffect, useLayoutEffect, useMemo, useRef } from "preact/hooks";
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
/////// CORE SETUP
/////////////////////////////////////////////////////////////////////

const latestEvent = signal<RouteChangeEvent | null>(null);
const loadersData = signal(ctx.get("loadersData"));
const clientLoadersData = signal(ctx.get("clientLoadersData"));
const routerData = signal(getRouterData());
const outermostErrorIdx = signal(ctx.get("outermostErrorIdx"));
const outermostError = signal(ctx.get("outermostError"));
const activeComponents = signal(ctx.get("activeComponents"));
const activeErrorBoundary = signal(ctx.get("activeErrorBoundary"));
const importURLs = signal(ctx.get("importURLs"));
const exportKeys = signal(ctx.get("exportKeys"));

export { clientLoadersData, loadersData, routerData };

let isInited = false;

function initUIListeners() {
	if (isInited) return;
	isInited = true;

	addRouteChangeListener((e) => {
		batch(() => {
			latestEvent.value = e;
			loadersData.value = ctx.get("loadersData");
			clientLoadersData.value = ctx.get("clientLoadersData");
			routerData.value = getRouterData();
			outermostErrorIdx.value = ctx.get("outermostErrorIdx");
			outermostError.value = ctx.get("outermostError");
			activeComponents.value = ctx.get("activeComponents");
			activeErrorBoundary.value = ctx.get("activeErrorBoundary");
			importURLs.value = ctx.get("importURLs");
			exportKeys.value = ctx.get("exportKeys");
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
			loadersData.value = ctx.get("loadersData");
			clientLoadersData.value = ctx.get("clientLoadersData");
			routerData.value = getRouterData();
			outermostError.value = ctx.get("outermostError");
			outermostErrorIdx.value = ctx.get("outermostErrorIdx");
			activeComponents.value = ctx.get("activeComponents");
			activeErrorBoundary.value = ctx.get("activeErrorBoundary");
			importURLs.value = ctx.get("importURLs");
			exportKeys.value = ctx.get("exportKeys");
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

	const CurrentComp = computed(() => {
		if (isErrorIdx.value) {
			return null;
		}
		currentImportURL.value;
		currentExportKey.value;
		return activeComponents.value?.[idx];
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

	const ErrorComp = computed(() => {
		if (!isErrorIdx.value) {
			return null;
		}
		return activeErrorBoundary.value;
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
