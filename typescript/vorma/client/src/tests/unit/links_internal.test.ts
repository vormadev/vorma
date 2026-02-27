import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { navigationStateManager } from "../../runtime.ts";
import {
	createLinkOnClickFn as __makeLinkOnClickFn,
	getEligibleInternalAnchorDetails,
	navigateEligibleInternalAnchorClick,
} from "../../runtime.ts";

function createClickEvent(props: {
	href: string;
	target?: string;
	ctrlKey?: boolean;
}): { event: MouseEvent; anchor: HTMLAnchorElement } {
	const event = new MouseEvent("click", {
		bubbles: true,
		cancelable: true,
		ctrlKey: props.ctrlKey,
	});
	const anchor = document.createElement("a");
	anchor.href = props.href;
	if (props.target) {
		anchor.target = props.target;
	}
	document.body.appendChild(anchor);
	Object.defineProperty(event, "target", { value: anchor });
	return { event, anchor };
}

describe("links internal branches", () => {
	beforeEach(() => {
		window.history.replaceState({}, "", "/");
		vi.spyOn(console, "error").mockImplementation(() => {});
	});

	afterEach(() => {
		vi.restoreAllMocks();
		document.body.innerHTML = "";
	});

	it("delegates eligible navigate clicks to navigationStateManager.navigate", async () => {
		const navigateSpy = vi
			.spyOn(navigationStateManager, "navigate")
			.mockResolvedValue({ didNavigate: true });
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const onClick = __makeLinkOnClickFn({
			beforeBegin,
			beforeRender,
			afterRender,
			scrollToTop: false,
			replace: true,
			state: { source: "unit" },
		});
		const { event } = createClickEvent({ href: "/target" });
		const preventDefault = vi.spyOn(event, "preventDefault");

		await onClick(event);

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(beforeBegin).toHaveBeenCalledTimes(1);
		expect(beforeRender).toHaveBeenCalledTimes(1);
		expect(afterRender).toHaveBeenCalledTimes(1);
		expect(navigateSpy).toHaveBeenCalledWith({
			href: "http://localhost:3000/target",
			navigationType: "userNavigation",
			scrollToTop: false,
			replace: true,
			state: { source: "unit" },
		});
		const beforeBeginCallOrder = beforeBegin.mock.invocationCallOrder[0];
		const beforeRenderCallOrder = beforeRender.mock.invocationCallOrder[0];
		const afterRenderCallOrder = afterRender.mock.invocationCallOrder[0];
		if (
			beforeBeginCallOrder === undefined ||
			beforeRenderCallOrder === undefined ||
			afterRenderCallOrder === undefined
		) {
			throw new Error(
				"Expected callback call order values to be defined.",
			);
		}
		expect(beforeBeginCallOrder).toBeLessThan(beforeRenderCallOrder);
		expect(beforeRenderCallOrder).toBeLessThan(afterRenderCallOrder);
	});

	it("treats same-document hash changes as runtime-only navigations without link callbacks", async () => {
		window.history.replaceState({}, "", "/current");
		const navigateSpy = vi
			.spyOn(navigationStateManager, "navigate")
			.mockResolvedValue({ didNavigate: true });
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const onClick = __makeLinkOnClickFn({
			beforeBegin,
			beforeRender,
			afterRender,
		});
		const { event } = createClickEvent({ href: "/current#section-a" });
		const preventDefault = vi.spyOn(event, "preventDefault");

		await onClick(event);

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(navigateSpy).toHaveBeenCalledTimes(1);
		expect(beforeBegin).not.toHaveBeenCalled();
		expect(beforeRender).not.toHaveBeenCalled();
		expect(afterRender).not.toHaveBeenCalled();
	});

	it("treats same-document no-op targets as runtime-only no-op navigations without link callbacks", async () => {
		window.history.replaceState({}, "", "/current#~");
		const navigateSpy = vi
			.spyOn(navigationStateManager, "navigate")
			.mockResolvedValue({ didNavigate: false });
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const onClick = __makeLinkOnClickFn({
			beforeBegin,
			beforeRender,
			afterRender,
		});
		const { event } = createClickEvent({ href: "/current#%7E" });
		const preventDefault = vi.spyOn(event, "preventDefault");

		await onClick(event);

		expect(preventDefault).toHaveBeenCalledTimes(1);
		expect(navigateSpy).toHaveBeenCalledTimes(1);
		expect(beforeBegin).not.toHaveBeenCalled();
		expect(beforeRender).not.toHaveBeenCalled();
		expect(afterRender).not.toHaveBeenCalled();
	});

	it("does not handle ineligible anchor clicks", async () => {
		const navigateSpy = vi.spyOn(navigationStateManager, "navigate");
		const onClick = __makeLinkOnClickFn({});
		const { event } = createClickEvent({
			href: "/target",
			target: "_top",
		});
		const preventDefault = vi.spyOn(event, "preventDefault");

		await onClick(event);

		expect(preventDefault).not.toHaveBeenCalled();
		expect(navigateSpy).not.toHaveBeenCalled();
	});

	it("does not call afterRender when runtime navigate reports no commit", async () => {
		const navigateSpy = vi
			.spyOn(navigationStateManager, "navigate")
			.mockResolvedValue({ didNavigate: false });
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const onClick = __makeLinkOnClickFn({
			beforeBegin,
			beforeRender,
			afterRender,
		});
		const { event } = createClickEvent({ href: "/target" });

		await onClick(event);

		expect(navigateSpy).toHaveBeenCalledTimes(1);
		expect(beforeBegin).toHaveBeenCalledTimes(1);
		expect(beforeRender).toHaveBeenCalledTimes(1);
		expect(afterRender).not.toHaveBeenCalled();
	});

	it("swallows runtime navigate rejections and logs errors", async () => {
		vi.spyOn(navigationStateManager, "navigate").mockRejectedValue(
			new Error("network failed"),
		);
		const consoleErrorSpy = vi.spyOn(console, "error");
		const onClick = __makeLinkOnClickFn({});
		const { event } = createClickEvent({ href: "/target" });

		await expect(onClick(event)).resolves.toBeUndefined();
		expect(consoleErrorSpy).toHaveBeenCalled();
	});

	it("can skip beforeBegin execution while still navigating for prefetch-started clicks", async () => {
		const navigateSpy = vi
			.spyOn(navigationStateManager, "navigate")
			.mockResolvedValue({ didNavigate: true });
		const beforeBegin = vi.fn();
		const beforeRender = vi.fn();
		const afterRender = vi.fn();
		const { event } = createClickEvent({ href: "/target" });
		const anchorDetails = getEligibleInternalAnchorDetails(event);
		if (!anchorDetails) {
			throw new Error("Expected eligible internal anchor details.");
		}

		await navigateEligibleInternalAnchorClick({
			event,
			anchorDetails,
			beforeBegin,
			beforeRender,
			afterRender,
			shouldRunBeforeBegin: false,
		});

		expect(navigateSpy).toHaveBeenCalledTimes(1);
		expect(beforeBegin).not.toHaveBeenCalled();
		expect(beforeRender).toHaveBeenCalledTimes(1);
		expect(afterRender).toHaveBeenCalledTimes(1);
	});
});
