export type ControllableStateChangeHandler<TValue, TDetails> = (
	value: TValue,
	details: TDetails,
) => void;

export type ControllableState<TValue, TDetails> = {
	get: () => TValue;
	isControlled: () => boolean;
	set: (value: TValue, details: TDetails) => boolean;
};

export type ControllableStateInput<TValue, TDetails> = {
	equals?: (left: TValue, right: TValue) => boolean;
	getControlled: () => TValue | undefined;
	getLocal: () => TValue;
	getOnChange: () =>
		| ControllableStateChangeHandler<TValue, TDetails>
		| undefined;
	setLocal: (value: TValue) => void;
};

function default_state_equals<TValue>(left: TValue, right: TValue): boolean {
	return Object.is(left, right);
}

export function create_controllable_state<TValue, TDetails>(
	input: ControllableStateInput<TValue, TDetails>,
): ControllableState<TValue, TDetails> {
	const equals = input.equals ?? default_state_equals;

	function is_controlled(): boolean {
		return input.getControlled() !== undefined;
	}

	function get(): TValue {
		const controlled = input.getControlled();
		if (controlled !== undefined) {
			return controlled;
		}
		return input.getLocal();
	}

	function set(value: TValue, details: TDetails): boolean {
		if (equals(get(), value)) {
			return false;
		}

		if (!is_controlled()) {
			input.setLocal(value);
		}
		input.getOnChange()?.(value, details);
		return true;
	}

	return {
		get,
		isControlled: is_controlled,
		set,
	};
}
