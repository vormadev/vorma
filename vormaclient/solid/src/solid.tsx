import {
	batch,
	createEffect,
	createMemo,
	createRenderEffect,
	createSignal,
	type JSX,
	Show,
} from "solid-js";
import { Dynamic } from "solid-js/web";
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

const [latestEvent, setLatestEvent] = createSignal<RouteChangeEvent | null>(
	null,
);
const [loadersData, setLoadersData] = createSignal(ctx.get("loadersData"));
const [clientLoadersData, setClientLoadersData] = createSignal(
	ctx.get("clientLoadersData"),
);
const [routerData, setRouterData] = createSignal(getRouterData());
const [outermostErrorIdx, setOutermostErrorIdx] = createSignal(
	ctx.get("outermostErrorIdx"),
);
const [outermostError, setOutermostError] = createSignal(
	ctx.get("outermostError"),
);
const [activeComponents, setActiveComponents] = createSignal(
	ctx.get("activeComponents"),
);
const [activeErrorBoundary, setActiveErrorBoundary] = createSignal(
	ctx.get("activeErrorBoundary"),
);
const [importURLs, setImportURLs] = createSignal(ctx.get("importURLs"));
const [exportKeys, setExportKeys] = createSignal(ctx.get("exportKeys"));

export { clientLoadersData, loadersData, routerData };

let isInited = false;

function initUIListeners() {
	if (isInited) return;
	isInited = true;

	addRouteChangeListener((e) => {
		batch(() => {
			setLatestEvent(e);
			setLoadersData(ctx.get("loadersData"));
			setClientLoadersData(ctx.get("clientLoadersData"));
			setRouterData(getRouterData());
			setOutermostErrorIdx(ctx.get("outermostErrorIdx"));
			setOutermostError(ctx.get("outermostError"));
			setActiveComponents(ctx.get("activeComponents"));
			setActiveErrorBoundary(ctx.get("activeErrorBoundary"));
			setImportURLs(ctx.get("importURLs"));
			setExportKeys(ctx.get("exportKeys"));
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
			setLoadersData(ctx.get("loadersData"));
			setClientLoadersData(ctx.get("clientLoadersData"));
			setRouterData(getRouterData());
			setOutermostErrorIdx(ctx.get("outermostErrorIdx"));
			setOutermostError(ctx.get("outermostError"));
			setActiveComponents(ctx.get("activeComponents"));
			setActiveErrorBoundary(ctx.get("activeErrorBoundary"));
			setImportURLs(ctx.get("importURLs"));
			setExportKeys(ctx.get("exportKeys"));
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

	const currentCompMemo = createMemo(() => {
		if (isErrorIdxMemo()) {
			return null;
		}
		currentImportURL();
		currentExportKey();
		return activeComponents()?.[idx];
	});

	const shouldFallbackOutletMemo = createMemo(() => {
		if (isErrorIdxMemo() || currentCompMemo()) {
			return false;
		}
		return idx + 1 < loadersData().length;
	});

	const errorCompMemo = createMemo(() => {
		if (!isErrorIdxMemo()) {
			return null;
		}
		return activeErrorBoundary();
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
