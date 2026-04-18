// @vitest-environment jsdom

import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
	LINK_ACTIVE_ANCESTOR_ATTR,
	LINK_ACTIVE_EXACT_ATTR,
	LINK_PENDING_ANCESTOR_ATTR,
	LINK_PENDING_EXACT_ATTR,
} from "./constants.ts";
import type { RouteState, WorkState } from "./create_client_core.ts";
import { make_link_props, type LinkNavFns } from "./make_link_props.ts";

const TEST_ROUTE_STATE: RouteState = {
	href: "/page",
	historyState: undefined,
	clientBuildID: "test-build",
	params: {},
	splatValues: [],
	matches: [],
	error: null,
};

const TEST_WORK_STATE: WorkState = {
	navigation: null,
	revalidation: null,
	prefetch: null,
	submissions: [],
};

function mock_nav(): LinkNavFns {
	return {
		navigate: vi.fn().mockResolvedValue({ didNavigate: true }),
		start_prefetch: vi.fn(),
		stop_prefetch: vi.fn(),
		save_current_scroll: vi.fn(),
		register_link_pattern: vi.fn(),
		get_link_attribute_state: vi.fn().mockReturnValue({
			active_exact: false,
			active_ancestor: false,
			pending_exact: false,
			pending_ancestor: false,
		}),
	};
}

function primary_click(overrides: Record<string, unknown> = {}) {
	return {
		button: 0,
		metaKey: false,
		ctrlKey: false,
		shiftKey: false,
		altKey: false,
		defaultPrevented: false,
		preventDefault: vi.fn(),
		...overrides,
	};
}

function mock_element() {
	return {
		addEventListener: vi.fn(),
	};
}

function pointer_down(overrides: Record<string, unknown> = {}) {
	return {
		button: 0,
		pointerType: "mouse",
		metaKey: false,
		ctrlKey: false,
		shiftKey: false,
		altKey: false,
		defaultPrevented: false,
		preventDefault: vi.fn(),
		currentTarget: mock_element(),
		...overrides,
	};
}

beforeEach(() => {
	vi.useFakeTimers();
});

afterEach(() => {
	vi.useRealTimers();
});

/////////////////////////////////////////////////////////////////////
/////// External detection
/////////////////////////////////////////////////////////////////////

describe("external detection", () => {
	it("detects cross-origin href as external", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "https://external.example/page" },
			nav,
		);
		expect(result.is_external).toBe(true);
	});

	it("detects mailto scheme as external", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "mailto:test@example.com" },
			nav,
		);
		expect(result.is_external).toBe(true);
	});

	it("detects tel scheme as external", () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "tel:+1234567890" }, nav);
		expect(result.is_external).toBe(true);
	});

	it("detects same-origin path as internal", () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/about" }, nav);
		expect(result.is_external).toBe(false);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Internal click interception
/////////////////////////////////////////////////////////////////////

describe("internal click interception", () => {
	it("prevents default and calls navigate on primary click", async () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/page" }, nav);
		const ev = primary_click();

		result.onClick!(ev);

		expect(ev.preventDefault).toHaveBeenCalled();
		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: undefined,
			scrollToTop: undefined,
			state: undefined,
		});
	});

	it("uses Vorma navigation for hash-only links so state is preserved", async () => {
		window.history.replaceState({}, "", "/page");

		const nav = mock_nav();
		const result = make_link_props(
			{
				href: "/page#details",
				replace: true,
				state: { from: "search" },
			},
			nav,
		);
		const ev = primary_click();

		result.onClick!(ev);

		expect(ev.preventDefault).toHaveBeenCalled();
		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page#details",
			replace: true,
			scrollToTop: undefined,
			state: { from: "search" },
		});
	});

	it("returns anchor props without vorma keys", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{
				href: "/page",
				className: "link",
				prefetch: "intent",
				prefetchDelayMs: 200,
				replace: true,
				scrollToTop: false,
			},
			nav,
		);

		expect(result.anchor_props).toHaveProperty("href", "/page");
		expect(result.anchor_props).toHaveProperty("className", "link");
		expect(result.anchor_props).not.toHaveProperty("prefetch");
		expect(result.anchor_props).not.toHaveProperty("prefetchDelayMs");
		expect(result.anchor_props).not.toHaveProperty("replace");
		expect(result.anchor_props).not.toHaveProperty("scrollToTop");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Fallthrough
/////////////////////////////////////////////////////////////////////

describe("fallthrough", () => {
	it("does not intercept meta-click", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page" },
			nav,
			TEST_ROUTE_STATE,
			TEST_WORK_STATE,
		);
		const ev = primary_click({ metaKey: true });

		result.onClick!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept ctrl-click", async () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/page" }, nav);
		const ev = primary_click({ ctrlKey: true });

		result.onClick!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept shift-click", async () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/page" }, nav);
		const ev = primary_click({ shiftKey: true });

		result.onClick!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept alt-click", async () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/page" }, nav);
		const ev = primary_click({ altKey: true });

		result.onClick!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept non-primary button", async () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/page" }, nav);
		const ev = primary_click({ button: 1 });

		result.onClick!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept target=_blank", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", target: "_blank" },
			nav,
		);
		const ev = primary_click();

		result.onClick!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept when consumer called preventDefault", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{
				href: "/page",
				onClick: (e: any) => {
					e.defaultPrevented = true;
				},
			},
			nav,
		);
		const ev = primary_click();

		result.onClick!(ev);

		expect(nav.navigate).not.toHaveBeenCalled();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Consumer event forwarding
/////////////////////////////////////////////////////////////////////

describe("consumer event forwarding", () => {
	it("forwards onClick to consumer", async () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onClick: consumer },
			nav,
		);
		const ev = primary_click();

		result.onClick!(ev);

		expect(consumer).toHaveBeenCalledWith(ev);
	});

	it("forwards onPointerDown to consumer", () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onPointerDown: consumer },
			nav,
		);
		const ev = pointer_down();

		result.onPointerDown!(ev);

		expect(consumer).toHaveBeenCalledWith(ev);
	});

	it("forwards onPointerDown to consumer even without visitOnPointerDown", () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onPointerDown: consumer },
			nav,
		);
		const ev = pointer_down();

		result.onPointerDown!(ev);

		expect(consumer).toHaveBeenCalledWith(ev);
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("forwards onPointerEnter to consumer", () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onPointerEnter: consumer, prefetch: "intent" },
			nav,
		);
		const ev = {};

		result.onPointerEnter!(ev);

		expect(consumer).toHaveBeenCalledWith(ev);
	});

	it("forwards onFocus to consumer", () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onFocus: consumer, prefetch: "intent" },
			nav,
		);
		const ev = {};

		result.onFocus!(ev);

		expect(consumer).toHaveBeenCalledWith(ev);
	});

	it("forwards onPointerLeave to consumer", () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onPointerLeave: consumer, prefetch: "intent" },
			nav,
		);
		const ev = {};

		result.onPointerLeave!(ev);

		expect(consumer).toHaveBeenCalledWith(ev);
	});

	it("forwards onBlur to consumer", () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onBlur: consumer, prefetch: "intent" },
			nav,
		);
		const ev = {};

		result.onBlur!(ev);

		expect(consumer).toHaveBeenCalledWith(ev);
	});

	it("forwards onTouchCancel to consumer", () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onTouchCancel: consumer, prefetch: "intent" },
			nav,
		);
		const ev = {};

		result.onTouchCancel!(ev);

		expect(consumer).toHaveBeenCalledWith(ev);
	});
});

/////////////////////////////////////////////////////////////////////
/////// Prefetch intent
/////////////////////////////////////////////////////////////////////

describe("prefetch intent", () => {
	it("starts prefetch after delay on pointerenter", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "intent" },
			nav,
		);

		result.onPointerEnter!({});

		expect(nav.start_prefetch).not.toHaveBeenCalled();
		vi.advanceTimersByTime(100);
		expect(nav.start_prefetch).toHaveBeenCalledWith("/page");
	});

	it("starts prefetch after delay on focus", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "intent" },
			nav,
		);

		result.onFocus!({});

		expect(nav.start_prefetch).not.toHaveBeenCalled();
		vi.advanceTimersByTime(100);
		expect(nav.start_prefetch).toHaveBeenCalledWith("/page");
	});

	it("respects custom prefetchDelayMs", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "intent", prefetchDelayMs: 300 },
			nav,
		);

		result.onPointerEnter!({});

		vi.advanceTimersByTime(299);
		expect(nav.start_prefetch).not.toHaveBeenCalled();
		vi.advanceTimersByTime(1);
		expect(nav.start_prefetch).toHaveBeenCalledWith("/page");
	});

	it("clears timer and stops prefetch on blur", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "intent" },
			nav,
		);

		result.onFocus!({});
		vi.advanceTimersByTime(50);
		result.onBlur!({});
		vi.advanceTimersByTime(100);

		expect(nav.start_prefetch).not.toHaveBeenCalled();
		expect(nav.stop_prefetch).toHaveBeenCalledWith("/page");
	});

	it("clears timer and stops prefetch on pointerleave", () => {
		// Ensure mouse modality so pointerleave cancels
		window.dispatchEvent(
			new PointerEvent("pointermove", { pointerType: "mouse" }),
		);

		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "intent" },
			nav,
		);

		result.onPointerEnter!({});
		vi.advanceTimersByTime(50);
		result.onPointerLeave!({});
		vi.advanceTimersByTime(100);

		expect(nav.start_prefetch).not.toHaveBeenCalled();
		expect(nav.stop_prefetch).toHaveBeenCalledWith("/page");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Prefetch cancel and restart
/////////////////////////////////////////////////////////////////////

describe("prefetch cancel and restart", () => {
	it("re-focus restarts the timer", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "intent" },
			nav,
		);

		result.onFocus!({});
		vi.advanceTimersByTime(50);
		result.onBlur!({});

		result.onFocus!({});
		vi.advanceTimersByTime(100);

		expect(nav.start_prefetch).toHaveBeenCalledTimes(1);
		expect(nav.start_prefetch).toHaveBeenCalledWith("/page");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Prefetch none
/////////////////////////////////////////////////////////////////////

describe("prefetch none", () => {
	it("does not set up prefetch handlers when prefetch is not intent", () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/page" }, nav);

		expect(result.onPointerEnter).toBeUndefined();
		expect(result.onFocus).toBeUndefined();
		expect(result.onPointerLeave).toBeUndefined();
		expect(result.onBlur).toBeUndefined();
	});

	it("does not prefetch on hover when prefetch is none", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "none" },
			nav,
		);

		// Handlers are undefined when no consumer handlers either
		expect(result.onPointerEnter).toBeUndefined();
		expect(result.onFocus).toBeUndefined();
	});
});

/////////////////////////////////////////////////////////////////////
/////// Touch modality
/////////////////////////////////////////////////////////////////////

describe("touch modality", () => {
	it("pointerleave does not cancel during touch", () => {
		window.dispatchEvent(
			new PointerEvent("pointerdown", { pointerType: "touch" }),
		);

		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "intent" },
			nav,
		);

		result.onPointerEnter!({});
		vi.advanceTimersByTime(100);
		expect(nav.start_prefetch).toHaveBeenCalled();

		result.onPointerLeave!({});

		expect(nav.stop_prefetch).not.toHaveBeenCalled();
	});

	it("pointerleave cancels after switch to mouse", () => {
		window.dispatchEvent(
			new PointerEvent("pointerdown", { pointerType: "touch" }),
		);
		window.dispatchEvent(
			new PointerEvent("pointermove", { pointerType: "mouse" }),
		);

		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", prefetch: "intent" },
			nav,
		);

		result.onPointerEnter!({});
		vi.advanceTimersByTime(50);
		result.onPointerLeave!({});
		vi.advanceTimersByTime(100);

		expect(nav.start_prefetch).not.toHaveBeenCalled();
		expect(nav.stop_prefetch).toHaveBeenCalledWith("/page");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Prop stripping
/////////////////////////////////////////////////////////////////////

describe("prop stripping", () => {
	it("removes vorma keys from anchor props for internal links", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{
				href: "/page",
				attributeMatchRules: { includeSearch: true },
				pattern: "/page",
				prefetch: "intent",
				prefetchDelayMs: 200,
				replace: true,
				scrollToTop: false,
				visitOnPointerDown: true,
				id: "my-link",
			},
			nav,
		);

		expect(result.anchor_props).not.toHaveProperty("prefetch");
		expect(result.anchor_props).not.toHaveProperty("attributeMatchRules");
		expect(result.anchor_props).not.toHaveProperty("pattern");
		expect(result.anchor_props).not.toHaveProperty("prefetchDelayMs");
		expect(result.anchor_props).not.toHaveProperty("replace");
		expect(result.anchor_props).not.toHaveProperty("scrollToTop");
		expect(result.anchor_props).not.toHaveProperty("visitOnPointerDown");
		expect(result.anchor_props).toHaveProperty("id", "my-link");
		expect(result.anchor_props).toHaveProperty("href", "/page");
	});

	it("removes vorma keys from anchor props for external links", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{
				href: "https://external.example",
				prefetch: "intent",
				replace: true,
				scrollToTop: true,
				visitOnPointerDown: true,
			},
			nav,
		);

		expect(result.anchor_props).not.toHaveProperty("prefetch");
		expect(result.anchor_props).not.toHaveProperty("replace");
		expect(result.anchor_props).not.toHaveProperty("scrollToTop");
		expect(result.anchor_props).not.toHaveProperty("visitOnPointerDown");
	});

	it("strips composed event keys from internal anchor props", () => {
		const nav = mock_nav();
		const result = make_link_props(
			{
				href: "/page",
				onClick: vi.fn(),
				onPointerDown: vi.fn(),
				onPointerEnter: vi.fn(),
				onFocus: vi.fn(),
				onPointerLeave: vi.fn(),
				onBlur: vi.fn(),
			},
			nav,
		);

		expect(result.anchor_props).not.toHaveProperty("onClick");
		expect(result.anchor_props).not.toHaveProperty("onPointerDown");
		expect(result.anchor_props).not.toHaveProperty("onPointerEnter");
		expect(result.anchor_props).not.toHaveProperty("onFocus");
		expect(result.anchor_props).not.toHaveProperty("onPointerLeave");
		expect(result.anchor_props).not.toHaveProperty("onBlur");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Route state attributes
/////////////////////////////////////////////////////////////////////

describe("route state attributes", () => {
	it("adds Vorma link state attributes from nav state", () => {
		const nav = mock_nav();
		vi.mocked(nav.get_link_attribute_state).mockReturnValue({
			active_exact: true,
			active_ancestor: true,
			pending_exact: true,
			pending_ancestor: true,
		});

		const result = make_link_props(
			{ href: "/page" },
			nav,
			TEST_ROUTE_STATE,
			TEST_WORK_STATE,
		);

		expect(result.anchor_props).toHaveProperty(LINK_ACTIVE_EXACT_ATTR, "");
		expect(result.anchor_props).toHaveProperty(
			LINK_ACTIVE_ANCESTOR_ATTR,
			"",
		);
		expect(result.anchor_props).toHaveProperty(LINK_PENDING_EXACT_ATTR, "");
		expect(result.anchor_props).toHaveProperty(
			LINK_PENDING_ANCESTOR_ATTR,
			"",
		);
		expect(result.anchor_props).toHaveProperty("aria-current", "page");
	});

	it("does not overwrite user aria-current", () => {
		const nav = mock_nav();
		vi.mocked(nav.get_link_attribute_state).mockReturnValue({
			active_exact: true,
			active_ancestor: false,
			pending_exact: false,
			pending_ancestor: false,
		});

		const result = make_link_props(
			{
				href: "/page",
				"aria-current": "step",
			},
			nav,
			TEST_ROUTE_STATE,
			TEST_WORK_STATE,
		);

		expect(result.anchor_props).toHaveProperty("aria-current", "step");
	});

	it("registers object-form link patterns", () => {
		const nav = mock_nav();

		make_link_props({ href: "/page", pattern: "/page" }, nav);

		expect(nav.register_link_pattern).toHaveBeenCalledWith("/page");
	});
});

/////////////////////////////////////////////////////////////////////
/////// Navigate options
/////////////////////////////////////////////////////////////////////

describe("navigate options", () => {
	it("forwards replace to navigate", async () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/page", replace: true }, nav);

		result.onClick!(primary_click());

		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: true,
			scrollToTop: undefined,
			state: undefined,
		});
	});

	it("forwards scrollToTop to navigate", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", scrollToTop: false },
			nav,
		);

		result.onClick!(primary_click());

		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: undefined,
			scrollToTop: false,
			state: undefined,
		});
	});

	it("forwards both replace and scrollToTop to navigate", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", replace: true, scrollToTop: false },
			nav,
		);

		result.onClick!(primary_click());

		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: true,
			scrollToTop: false,
			state: undefined,
		});
	});
});

/////////////////////////////////////////////////////////////////////
/////// visitOnPointerDown
/////////////////////////////////////////////////////////////////////

describe("visitOnPointerDown", () => {
	it("navigates on pointerdown for mouse", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);
		const ev = pointer_down({ pointerType: "mouse" });

		result.onPointerDown!(ev);

		expect(ev.preventDefault).toHaveBeenCalled();
		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: undefined,
			scrollToTop: undefined,
		});
	});

	it("navigates on pointerdown for pen", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);
		const ev = pointer_down({ pointerType: "pen" });

		result.onPointerDown!(ev);

		expect(ev.preventDefault).toHaveBeenCalled();
		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: undefined,
			scrollToTop: undefined,
		});
	});

	it("does not navigate on pointerdown for touch", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);
		const ev = pointer_down({ pointerType: "touch" });

		result.onPointerDown!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
		expect(ev.currentTarget.addEventListener).not.toHaveBeenCalled();
	});

	it("falls back to click navigation for touch users", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);

		// Touch pointerdown does nothing
		result.onPointerDown!(pointer_down({ pointerType: "touch" }));
		expect(nav.navigate).not.toHaveBeenCalled();

		// Subsequent click navigates normally
		const click_ev = primary_click();
		result.onClick!(click_ev);
		expect(click_ev.preventDefault).toHaveBeenCalled();
		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: undefined,
			scrollToTop: undefined,
			state: undefined,
		});
	});

	it("registers a one-time native click listener to suppress default", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);
		const el = mock_element();
		const ev = pointer_down({ currentTarget: el });

		result.onPointerDown!(ev);

		expect(el.addEventListener).toHaveBeenCalledWith(
			"click",
			expect.any(Function),
			{ once: true },
		);
	});

	it("native click listener calls preventDefault on the click event", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);
		const el = mock_element();
		const ev = pointer_down({ currentTarget: el });

		result.onPointerDown!(ev);

		const click_handler = el.addEventListener.mock.calls[0]?.[1];
		const click_ev = { preventDefault: vi.fn() };
		click_handler(click_ev);

		expect(click_ev.preventDefault).toHaveBeenCalled();
	});

	it("does not intercept modified pointerdown", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);

		result.onPointerDown!(pointer_down({ metaKey: true }));
		expect(nav.navigate).not.toHaveBeenCalled();

		result.onPointerDown!(pointer_down({ ctrlKey: true }));
		expect(nav.navigate).not.toHaveBeenCalled();

		result.onPointerDown!(pointer_down({ shiftKey: true }));
		expect(nav.navigate).not.toHaveBeenCalled();

		result.onPointerDown!(pointer_down({ altKey: true }));
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept non-primary button pointerdown", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);
		const ev = pointer_down({ button: 2 });

		result.onPointerDown!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept pointerdown with target=_blank", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true, target: "_blank" },
			nav,
		);
		const ev = pointer_down();

		result.onPointerDown!(ev);

		expect(ev.preventDefault).not.toHaveBeenCalled();
		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("does not intercept pointerdown when consumer called preventDefault", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{
				href: "/page",
				visitOnPointerDown: true,
				onPointerDown: (e: any) => {
					e.defaultPrevented = true;
				},
			},
			nav,
		);
		const ev = pointer_down();

		result.onPointerDown!(ev);

		expect(nav.navigate).not.toHaveBeenCalled();
	});

	it("forwards replace and scrollToTop on pointerdown navigation", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{
				href: "/page",
				visitOnPointerDown: true,
				replace: true,
				scrollToTop: false,
			},
			nav,
		);

		result.onPointerDown!(pointer_down());

		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: true,
			scrollToTop: false,
		});
	});

	it("does not set onPointerDown handler when not enabled and no consumer", () => {
		const nav = mock_nav();
		const result = make_link_props({ href: "/page" }, nav);

		expect(result.onPointerDown).toBeUndefined();
	});

	it("sets onPointerDown handler when only consumer is provided", () => {
		const consumer = vi.fn();
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", onPointerDown: consumer },
			nav,
		);

		expect(result.onPointerDown).toBeDefined();
	});

	it("allows keyboard navigation via click even when visitOnPointerDown is set", async () => {
		const nav = mock_nav();
		const result = make_link_props(
			{ href: "/page", visitOnPointerDown: true },
			nav,
		);

		// No pointerdown fires for keyboard Enter — click fires directly
		const click_ev = primary_click();
		result.onClick!(click_ev);

		expect(click_ev.preventDefault).toHaveBeenCalled();
		expect(nav.navigate).toHaveBeenCalledWith({
			href: "/page",
			replace: undefined,
			scrollToTop: undefined,
			state: undefined,
		});
	});
});
