type ViteHMRUpdate = {
	type: string;
	path: string;
};

type ImportMetaHot = {
	on: (
		event: "vite:afterUpdate",
		listener: (payload: { updates: Array<ViteHMRUpdate> }) => void,
	) => void;
};

function normalizeHMRModulePath(path: string): string {
	return new URL(path, window.location.href).pathname;
}

const registeredViteHotRuntimes = new WeakSet<ImportMetaHot>();

export function shouldRevalidateForHMRUpdate(props: {
	updates: Array<ViteHMRUpdate>;
	currentImportURLs: string[];
}): boolean {
	const importPathsForCurrentSnapshot = new Set(
		props.currentImportURLs.map(normalizeHMRModulePath),
	);
	if (importPathsForCurrentSnapshot.size === 0) {
		return false;
	}
	for (const update of props.updates) {
		if (update.type !== "js-update") {
			continue;
		}
		if (
			importPathsForCurrentSnapshot.has(
				normalizeHMRModulePath(update.path),
			)
		) {
			return true;
		}
	}
	return false;
}

export function applyViteAfterUpdatePayload(props: {
	updates: Array<ViteHMRUpdate>;
	currentImportURLs: string[];
	triggerRevalidate: () => void;
}): void {
	if (
		!shouldRevalidateForHMRUpdate({
			updates: props.updates,
			currentImportURLs: props.currentImportURLs,
		})
	) {
		return;
	}
	props.triggerRevalidate();
}

export function registerViteAfterUpdateListenerIfNeeded(props: {
	hotRuntime: ImportMetaHot | undefined;
	getCurrentImportURLs: () => string[];
	triggerRevalidate: () => void;
}): void {
	if (!import.meta.env.DEV) {
		return;
	}
	if (!props.hotRuntime) {
		return;
	}
	if (registeredViteHotRuntimes.has(props.hotRuntime)) {
		return;
	}
	props.hotRuntime.on("vite:afterUpdate", ({ updates }) => {
		applyViteAfterUpdatePayload({
			updates,
			currentImportURLs: props.getCurrentImportURLs(),
			triggerRevalidate: props.triggerRevalidate,
		});
	});
	registeredViteHotRuntimes.add(props.hotRuntime);
}
