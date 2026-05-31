# Field Notes

Fully built: yes.

Field is the shared form-control coordinator. It owns the root/label/ description/error
anatomy, wires generated control/description/error IDs, merges `aria-describedby`,
preserves explicit control props, and propagates `disabled`, `invalid`, `readOnly`, and
`required` through DOM attributes and state data attributes.

Fully tested: WIP.

Current coverage exercises composed relationship wiring, matched error visibility,
explicit control prop precedence, and part-level consumer `mix` placement. Full testing
still needs broader coverage across real composed inputs, grouped field usage, and
integration with higher-level form components.
