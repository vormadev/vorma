import { __getPrefetchHandlers, __makeLinkOnClickFn } from "../links.ts";
import { __resolvePath } from "../vorma_app_helpers/vorma_app_helpers.ts";
import { __vormaClientGlobal } from "../vorma_ctx/vorma_ctx.ts";

export type VormaLinkPropsBase<LinkOnClickCallback> = {
	href?: string;
	prefetch?: "intent";
	prefetchDelayMs?: number;
	beforeBegin?: LinkOnClickCallback;
	beforeRender?: LinkOnClickCallback;
	afterRender?: LinkOnClickCallback;
	scrollToTop?: boolean;
	replace?: boolean;
	state?: unknown;
};

function linkPropsToPrefetchObj<LinkOnClickCallback>(
	props: VormaLinkPropsBase<LinkOnClickCallback>,
) {
	if (!props.href || props.prefetch !== "intent") {
		return undefined;
	}

	return __getPrefetchHandlers({
		href: props.href,
		delayMs: props.prefetchDelayMs,
		beforeBegin: props.beforeBegin as any,
		beforeRender: props.beforeRender as any,
		afterRender: props.afterRender as any,
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	});
}

function linkPropsToOnClickFn<LinkOnClickCallback>(
	props: VormaLinkPropsBase<LinkOnClickCallback>,
) {
	return __makeLinkOnClickFn({
		beforeBegin: props.beforeBegin as any,
		beforeRender: props.beforeRender as any,
		afterRender: props.afterRender as any,
		scrollToTop: props.scrollToTop,
		replace: props.replace,
		state: props.state,
	});
}

type handlerKeys = {
	onPointerEnter: string;
	onFocus: string;
	onPointerLeave: string;
	onBlur: string;
	onTouchCancel: string;
	onClick: string;
};

const standardCamelHandlerKeys = {
	onPointerEnter: "onPointerEnter",
	onFocus: "onFocus",
	onPointerLeave: "onPointerLeave",
	onBlur: "onBlur",
	onTouchCancel: "onTouchCancel",
	onClick: "onClick",
} satisfies handlerKeys;

export function __makeFinalLinkProps<LinkOnClickCallback>(
	props: VormaLinkPropsBase<LinkOnClickCallback>,
	keys: {
		onPointerEnter: string;
		onFocus: string;
		onPointerLeave: string;
		onBlur: string;
		onTouchCancel: string;
		onClick: string;
	} = standardCamelHandlerKeys,
) {
	const prefetchObj = linkPropsToPrefetchObj(props);

	return {
		dataExternal: prefetchObj?.isExternal || undefined,
		onPointerEnter: (e: any) => {
			prefetchObj?.start(e);
			if (isFn((props as any)[keys.onPointerEnter])) {
				(props as any)[keys.onPointerEnter](e);
			}
		},
		onFocus: (e: any) => {
			prefetchObj?.start(e);
			if (isFn((props as any)[keys.onFocus])) {
				(props as any)[keys.onFocus](e);
			}
		},
		onPointerLeave: (e: any) => {
			// we don't want to stop on a touch device, because this triggers
			// even when the user "clicks" on the link for some reason
			if (!__vormaClientGlobal.get("isTouchDevice")) {
				prefetchObj?.stop();
			}
			if (isFn((props as any)[keys.onPointerLeave])) {
				(props as any)[keys.onPointerLeave](e);
			}
		},
		onBlur: (e: any) => {
			prefetchObj?.stop();
			if (isFn((props as any)[keys.onBlur])) {
				(props as any)[keys.onBlur](e);
			}
		},
		onTouchCancel: (e: any) => {
			prefetchObj?.stop();
			if (isFn((props as any)[keys.onTouchCancel])) {
				(props as any)[keys.onTouchCancel](e);
			}
		},
		onClick: async (e: any) => {
			if (isFn((props as any)[keys.onClick])) {
				(props as any)[keys.onClick](e);
			}
			if (prefetchObj) {
				await prefetchObj.onClick(e);
			} else {
				await linkPropsToOnClickFn(props)(e);
			}
		},
	};
}

function isFn(fn: any): fn is (...args: Array<any>) => any {
	return typeof fn === "function";
}
