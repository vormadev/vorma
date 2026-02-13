import {
	batch,
	createEffect,
	createMemo,
	createRenderEffect,
	createSignal,
	type ValidComponent,
	type JSX,
	Show,
} from "solid-js";
import { Dynamic } from "solid-js/web";
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

const [latestEvent, setLatestEvent] = createSignal<RouteChangeEvent | null>(
	null,
);
const initialRenderState = __getClientRuntimeRenderState();
const [loadersData, setLoadersData] = createSignal(
	initialRenderState.loadersData,
);
const [clientLoadersData, setClientLoadersData] = createSignal(
	initialRenderState.clientLoadersData,
);
const [routerData, setRouterData] = createSignal(getRouterData());
const [outermostErrorIdx, setOutermostErrorIdx] = createSignal(
	initialRenderState.outermostErrorIdx,
);
const [outermostError, setOutermostError] = createSignal(
	initialRenderState.outermostError,
);
const [activeComponents, setActiveComponents] = createSignal(
	initialRenderState.activeComponents as Array<ValidComponent> | null,
);
const [activeErrorBoundary, setActiveErrorBoundary] = createSignal(
	initialRenderState.activeErrorBoundary as ValidComponent | undefined,
);
const [importURLs, setImportURLs] = createSignal(initialRenderState.importURLs);
const [exportKeys, setExportKeys] = createSignal(initialRenderState.exportKeys);

export { clientLoadersData, loadersData, routerData };

let isInited = false;

function syncRuntimeRenderState(): void {
	const renderState = __getClientRuntimeRenderState();
	setLoadersData(renderState.loadersData);
	setClientLoadersData(renderState.clientLoadersData);
	setOutermostErrorIdx(renderState.outermostErrorIdx);
	setOutermostError(renderState.outermostError);
	setActiveComponents(
		renderState.activeComponents as Array<ValidComponent> | null,
	);
	setActiveErrorBoundary(() => {
		return renderState.activeErrorBoundary as ValidComponent | undefined;
	});
	setImportURLs(renderState.importURLs);
	setExportKeys(renderState.exportKeys);
}

function initUIListeners() {
	if (isInited) return;
	isInited = true;

	addRouteChangeListener((e) => {
		batch(() => {
			setLatestEvent(e);
			syncRuntimeRenderState();
			setRouterData(getRouterData());
		});
	});

	addLocationListener(() => {
		setLocation(getLocation());
	});
}

const [location, setLocation] = createSignal(getLocation());

export { location };

/////////////////////////////////////////////////////////////////////
/////// COMPONENT
/////////////////////////////////////////////////////////////////////

export function VormaRootOutlet(props: { idx?: number }): JSX.Element {
	const idx = props.idx ?? 0;

	if (idx === 0) {
		initUIListeners();

		batch(() => {
			syncRuntimeRenderState();
			setRouterData(getRouterData());
		});
	}

	const [currentImportURL, setCurrentImportURL] = createSignal(
		importURLs()?.[idx],
	);
	const [currentExportKey, setCurrentExportKey] = createSignal(
		exportKeys()?.[idx],
	);

	createEffect(() => {
		if (!currentImportURL()) {
			return;
		}
		const e = latestEvent();
		if (!e) {
			return;
		}

		const newCurrentImportURL = importURLs()?.[idx];
		const newCurrentExportKey = exportKeys()?.[idx];

		if (currentImportURL() !== newCurrentImportURL) {
			setCurrentImportURL(newCurrentImportURL);
		}
		if (currentExportKey() !== newCurrentExportKey) {
			setCurrentExportKey(newCurrentExportKey);
		}
	});

	createRenderEffect(() => {
		const e = latestEvent();
		if (!e || idx !== 0) {
			return;
		}
		window.requestAnimationFrame(() => {
			__applyScrollState(e.detail.__scrollState);
		});
	});

	const isErrorIdxMemo = createMemo(() => {
		return idx === outermostErrorIdx();
	});

	const currentCompMemo = createMemo<ValidComponent | undefined>(() => {
		if (isErrorIdxMemo()) {
			return undefined;
		}
		currentImportURL();
		currentExportKey();
		return activeComponents()?.[idx] ?? undefined;
	});

	const shouldFallbackOutletMemo = createMemo(() => {
		if (isErrorIdxMemo() || currentCompMemo()) {
			return false;
		}
		return idx + 1 < loadersData().length;
	});

	const errorCompMemo = createMemo<ValidComponent | undefined>(() => {
		if (!isErrorIdxMemo()) {
			return undefined;
		}
		return activeErrorBoundary() ?? undefined;
	});

	const remountKeyNext = createMemo(
		() => `${importURLs()[idx + 1]}|${exportKeys()[idx + 1]}`,
	);

	const Outlet = (localProps: Record<string, any> | undefined) => (
		<Show when={remountKeyNext()} keyed>
			<VormaRootOutlet {...localProps} {...props} idx={idx + 1} />
		</Show>
	);

	return (
		<>
			<Show when={currentCompMemo()}>
				<Dynamic
					component={currentCompMemo()}
					idx={idx}
					Outlet={Outlet}
				/>
			</Show>

			<Show when={shouldFallbackOutletMemo()}>
				<Outlet />
			</Show>

			<Show when={isErrorIdxMemo()}>
				<Show
					when={errorCompMemo()}
					fallback={`Error: ${outermostError() || "unknown"}`}
				>
					<Dynamic
						component={errorCompMemo()}
						error={outermostError()}
					/>
				</Show>
			</Show>
		</>
	);
}
