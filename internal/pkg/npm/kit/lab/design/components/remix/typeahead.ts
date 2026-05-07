export type TypeaheadItem<TValue extends string = string> = {
	text: string;
	value: TValue;
};

export type Typeahead<TItem extends TypeaheadItem> = {
	reset: () => void;
	search: (input: TypeaheadSearchInput<TItem>) => TItem | undefined;
};

export type TypeaheadInput = {
	timeoutMs: number;
};

export type TypeaheadSearchInput<TItem extends TypeaheadItem> = {
	currentValue: string | null | undefined;
	items: readonly TItem[];
	key: string;
};

function is_repeated_character_search(value: string): boolean {
	return [...value].every((character) => {
		return character === value[0];
	});
}

function order_items_after_current<TItem extends TypeaheadItem>(
	items: readonly TItem[],
	current_value: string | null | undefined,
): readonly TItem[] {
	const current_index = items.findIndex((item) => {
		return item.value === current_value;
	});
	const start_index = current_index >= 0 ? current_index : -1;
	return [
		...items.slice(start_index + 1),
		...items.slice(0, start_index + 1),
	];
}

export function create_typeahead<TItem extends TypeaheadItem>(
	input: TypeaheadInput,
): Typeahead<TItem> {
	let buffer = "";
	let timer: ReturnType<typeof setTimeout> | undefined;

	function reset(): void {
		if (timer !== undefined) {
			clearTimeout(timer);
			timer = undefined;
		}
		buffer = "";
	}

	function search(
		search_input: TypeaheadSearchInput<TItem>,
	): TItem | undefined {
		if (timer !== undefined) {
			clearTimeout(timer);
			timer = undefined;
		}
		buffer += search_input.key.toLowerCase();
		timer = setTimeout(() => {
			buffer = "";
			timer = undefined;
		}, input.timeoutMs);

		const query = is_repeated_character_search(buffer)
			? search_input.key.toLowerCase()
			: buffer;
		return order_items_after_current(
			search_input.items,
			search_input.currentValue,
		).find((item) => {
			return item.text.toLowerCase().startsWith(query);
		});
	}

	return {
		reset,
		search,
	};
}
