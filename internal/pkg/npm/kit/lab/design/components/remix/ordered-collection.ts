export type OrderedCollectionItem<TValue extends string = string> = {
	disabled: boolean;
	id: string;
	node: HTMLElement;
	text: string;
	value: TValue;
};

export type OrderedCollection<TItem extends OrderedCollectionItem> = {
	findByValue: (value: string | null | undefined) => TItem | undefined;
	getEnabledItems: () => TItem[];
	getItems: () => TItem[];
	register: (item: TItem) => boolean;
	unregister: (id: string) => TItem | undefined;
};

function compare_collection_items<TItem extends OrderedCollectionItem>(
	left: TItem,
	right: TItem,
): number {
	if (left.node === right.node) {
		return 0;
	}

	const position = left.node.compareDocumentPosition(right.node);
	if ((position & Node.DOCUMENT_POSITION_PRECEDING) !== 0) {
		return 1;
	}
	if ((position & Node.DOCUMENT_POSITION_FOLLOWING) !== 0) {
		return -1;
	}
	return 0;
}

function item_changed<TItem extends OrderedCollectionItem>(
	current: TItem | undefined,
	next: TItem,
): boolean {
	return (
		current === undefined ||
		current.disabled !== next.disabled ||
		current.node !== next.node ||
		current.text !== next.text ||
		current.value !== next.value
	);
}

export function create_ordered_collection<
	TItem extends OrderedCollectionItem,
>(): OrderedCollection<TItem> {
	const items_by_id = new Map<string, TItem>();

	function get_items(): TItem[] {
		return Array.from(items_by_id.values())
			.filter((item) => {
				return item.node.isConnected;
			})
			.sort(compare_collection_items);
	}

	function get_enabled_items(): TItem[] {
		return get_items().filter((item) => {
			return !item.disabled;
		});
	}

	function find_by_value(
		value: string | null | undefined,
	): TItem | undefined {
		if (value == null) {
			return undefined;
		}
		return get_items().find((item) => {
			return item.value === value;
		});
	}

	function register(item: TItem): boolean {
		const current = items_by_id.get(item.id);
		items_by_id.set(item.id, item);
		return item_changed(current, item);
	}

	function unregister(id: string): TItem | undefined {
		const item = items_by_id.get(id);
		items_by_id.delete(id);
		return item;
	}

	return {
		findByValue: find_by_value,
		getEnabledItems: get_enabled_items,
		getItems: get_items,
		register,
		unregister,
	};
}
