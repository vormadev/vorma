interface ImportMetaEnv {
	readonly DEV: boolean;
	readonly [key: string]: string | boolean | undefined;
}

type ViteHMRUpdate = {
	type: string;
	path: string;
};

interface ImportMetaHot {
	on(
		event: "vite:afterUpdate",
		callback: (payload: { updates: Array<ViteHMRUpdate> }) => void,
	): void;
}

interface ImportMeta {
	readonly env: ImportMetaEnv;
	readonly hot?: ImportMetaHot;
}
