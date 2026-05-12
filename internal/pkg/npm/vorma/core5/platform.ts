/// <reference lib="dom" />

import type { Result } from "vorma/kit/result";

declare const timer_handle_brand: unique symbol;
export type TimerHandle = number & { readonly [timer_handle_brand]: true };

declare const abort_handle_brand: unique symbol;
export type AbortHandle = number & { readonly [abort_handle_brand]: true };

export type TimePlatform = {
	now: () => number;
};

export type RandomPlatform = {
	random_id: () => string;
};

export type TimerPlatform = {
	set_timeout: (callback: () => void, ms: number) => TimerHandle;
	clear_timeout: (handle: TimerHandle) => void;
};

export type AbortPlatform = {
	create_abort: () => { handle: AbortHandle; signal: AbortSignal };
	trigger_abort: (handle: AbortHandle) => void;
};

export type NetworkPlatform = {
	fetch: (
		url: string,
		init: RequestInit & { signal: AbortSignal },
	) => Promise<Response>;
};

export type LocationPlatform = {
	current_url: () => string;
	hard_redirect: (url: string) => void;
	reload: () => void;
};

export type HistoryPlatform = {
	history_state: () => unknown;
	push: (url: string, state: unknown) => void;
	replace: (url: string, state: unknown) => void;
	set_scroll_restoration_manual: () => void;
};

export type ScrollPlatform = {
	current_position: () => { x: number; y: number };
	scroll_to_xy: (x: number, y: number) => void;
	scroll_to_element_id: (id: string) => void;
};

export type StoragePlatform = {
	get: (key: string) => string | null;
	set: (key: string, value: string) => Result<void>;
	remove: (key: string) => void;
};

export type DOMPlatform = {
	ensure_root_element: (id: string) => HTMLElement;
	apply_head_and_title: (
		title: string | undefined,
		meta_head_els: readonly unknown[],
		rest_head_els: readonly unknown[],
	) => void;
	preload_css: (bundles: readonly string[]) => void;
	apply_css_bundles: (bundles: readonly string[]) => void;
	wait_for_css: (
		bundles: readonly string[],
		signal: AbortSignal,
	) => Promise<void>;
	preload_modules: (deps: readonly string[]) => void;
	import_module: (url: string) => Promise<Record<string, unknown>>;
	decode_html_entities: (raw: string) => string;
};

export type ViewTransitionPlatform = {
	supports_view_transitions: () => boolean;
	start_view_transition: (callback: () => void | Promise<void>) => {
		updateCallbackDone?: Promise<void>;
		finished?: Promise<void>;
	};
};

export type EventListenerPlatform = {
	add_window_listener: (
		event: "popstate" | "focus" | "beforeunload" | "visibilitychange",
		handler: () => void,
	) => () => void;
	document_visibility_state: () => "visible" | "hidden" | "prerender";
};

export type Platform = TimePlatform &
	RandomPlatform &
	TimerPlatform &
	AbortPlatform &
	NetworkPlatform &
	LocationPlatform &
	HistoryPlatform &
	ScrollPlatform &
	StoragePlatform &
	DOMPlatform &
	ViewTransitionPlatform &
	EventListenerPlatform;
