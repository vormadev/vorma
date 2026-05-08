import { ref, type MixInput } from "remix/ui";
import { releasePopoverScrollLock } from "./popover-scroll-lock.ts";

export type PopupRelationship = {
	containsTarget: (target: EventTarget | null) => boolean;
	focusTrigger: () => void;
	getPopup: () => HTMLElement | null;
	getTrigger: () => HTMLElement | null;
	registerPopup: (node: HTMLElement | null) => void;
	registerTrigger: (node: HTMLElement | null) => void;
	syncPopup: (open: boolean) => void;
};

export type PopupRefMixInput = {
	getOpen: () => boolean;
	onInteractOutside?: (event: PointerEvent) => void;
	relationship: PopupRelationship;
};

export function containsEventTarget(
	node: HTMLElement | null,
	target: EventTarget | null,
): boolean {
	return target instanceof Node && node?.contains(target) === true;
}

export function syncNativePopover(node: HTMLElement, open: boolean): void {
	if (open) {
		node.hidden = false;
		if ("showPopover" in node && !node.matches(":popover-open")) {
			node.showPopover();
		}
		return;
	}

	if ("hidePopover" in node && node.matches(":popover-open")) {
		node.hidePopover();
	}
	node.hidden = true;
	releasePopoverScrollLock(node.ownerDocument);
}

export function createPopupRelationship(): PopupRelationship {
	let popup_node: HTMLElement | null = null;
	let trigger_node: HTMLElement | null = null;

	return {
		containsTarget: (target) => {
			return (
				containsEventTarget(trigger_node, target) ||
				containsEventTarget(popup_node, target)
			);
		},
		focusTrigger: () => {
			trigger_node?.focus();
		},
		getPopup: () => {
			return popup_node;
		},
		getTrigger: () => {
			return trigger_node;
		},
		registerPopup: (node) => {
			popup_node = node;
		},
		registerTrigger: (node) => {
			trigger_node = node;
		},
		syncPopup: (open) => {
			if (popup_node) {
				syncNativePopover(popup_node, open);
			}
		},
	};
}

export function createPopupRefMix(
	input: PopupRefMixInput,
): MixInput<HTMLElement> {
	return ref<HTMLElement>((node, signal) => {
		input.relationship.registerPopup(node);
		syncNativePopover(node, input.getOpen());
		const on_pointer_down = (event: PointerEvent): void => {
			if (!input.getOpen()) {
				return;
			}
			if (input.relationship.containsTarget(event.target)) {
				return;
			}
			input.onInteractOutside?.(event);
		};
		node.ownerDocument.addEventListener(
			"pointerdown",
			on_pointer_down,
			true,
		);
		signal.addEventListener("abort", () => {
			input.relationship.registerPopup(null);
			node.ownerDocument.removeEventListener(
				"pointerdown",
				on_pointer_down,
				true,
			);
		});
	});
}
