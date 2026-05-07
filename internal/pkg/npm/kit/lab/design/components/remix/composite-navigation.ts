import type { OrderedCollectionItem } from "./ordered-collection.ts";

export type CollectionNavigationInput<TItem extends OrderedCollectionItem> = {
	clamp?: boolean;
	currentValue: string | null | undefined;
	fallbackValue?: string | null | undefined;
	items: readonly TItem[];
	loop?: boolean;
	offset: number;
};

export type RovingTabIndexInput = {
	currentValue: string | null | undefined;
	fallbackValue?: string | null | undefined;
	itemValue: string;
};

export function get_collection_navigation_item<
	TItem extends OrderedCollectionItem,
>(input: CollectionNavigationInput<TItem>): TItem | undefined {
	if (input.items.length === 0) {
		return undefined;
	}

	const current_index = input.items.findIndex((item) => {
		return item.value === input.currentValue;
	});
	const fallback_index = input.items.findIndex((item) => {
		return item.value === input.fallbackValue;
	});
	const base_index = current_index >= 0 ? current_index : fallback_index;
	const next_index =
		base_index >= 0
			? base_index + input.offset
			: input.offset > 0
				? 0
				: input.items.length - 1;

	if (input.clamp === true) {
		const clamped_index = Math.max(
			0,
			Math.min(input.items.length - 1, next_index),
		);
		return input.items[clamped_index];
	}

	if (input.loop === true) {
		const wrapped_index =
			((next_index % input.items.length) + input.items.length) %
			input.items.length;
		return input.items[wrapped_index];
	}

	if (next_index < 0 || next_index >= input.items.length) {
		return undefined;
	}
	return input.items[next_index];
}

export function get_first_collection_item<TItem extends OrderedCollectionItem>(
	items: readonly TItem[],
): TItem | undefined {
	return items[0];
}

export function get_last_collection_item<TItem extends OrderedCollectionItem>(
	items: readonly TItem[],
): TItem | undefined {
	return items[items.length - 1];
}

export function get_roving_tab_index(input: RovingTabIndexInput): 0 | -1 {
	const active_value = input.currentValue ?? input.fallbackValue;
	return input.itemValue === active_value ? 0 : -1;
}
