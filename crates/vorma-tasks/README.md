# vorma-tasks

Standalone async task runtime with execution-context memoization.

Tasks memoize by task/input inside one execution context, can optionally cache successful
results across execution contexts, and support cancellation, parallel execution, typed
overrides, and passive observation.
