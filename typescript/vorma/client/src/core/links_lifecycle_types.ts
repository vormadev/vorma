type LinkOnClickCallback<E extends Event> = (event: E) => void | Promise<void>;

export type LinkOnClickCallbacks<E extends Event> = {
	beforeBegin?: LinkOnClickCallback<E>;
	beforeRender?: LinkOnClickCallback<E>;
	afterRender?: LinkOnClickCallback<E>;
};

export type LinkLifecycleCallbacks<E extends Event> = {
	beforeRender?: (event: E) => void | Promise<void>;
	afterRender?: (event: E) => void | Promise<void>;
};

export type ClickNavigationOptions = {
	scrollToTop?: boolean;
	replace?: boolean;
	search?: string;
	hash?: string;
	state?: unknown;
};
