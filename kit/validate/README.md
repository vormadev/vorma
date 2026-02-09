# kit/validate

`github.com/vormadev/vorma/kit/validate`

`validate` combines two concerns:

- parsing request input into typed structs
- composable validation checks over values/objects

It is designed so decode/parse and validation failures share one error type
(`ValidationError`).

## Import

```go
import "github.com/vormadev/vorma/kit/validate"
```

## Core Building Blocks

- `JSONBodyInto`, `JSONBytesInto`, `JSONStrInto`: decode JSON and then run
  validation.
- `URLSearchParamsInto`: parse query params into a struct and then run
  validation.
- `Any(label, value)`: fluent validation over one value.
- `Object(object)`: fluent validation over named fields on a struct or map with
  string keys.
- `Validator` interface: model-owned validation hook (`Validate() error`).

## Quick Start

### Parse JSON request body + validate

```go
type CreateUserInput struct {
	Email string `json:"email"`
	Age   int    `json:"age"`
}

func (in *CreateUserInput) Validate() error {
	return validate.Object(in).
		Required("Email").Email().
		Required("Age").RangeInclusive(18, 120).
		Error()
}

func handler(w http.ResponseWriter, r *http.Request) {
	var in CreateUserInput
	if err := validate.JSONBodyInto(r, &in); err != nil {
		if validate.IsValidationError(err) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// in is decoded + validated
}
```

### Parse query params + validate

```go
type ListUsersQuery struct {
	Page int      `json:"page"`
	Tags []string `json:"tags"`
}

func (q *ListUsersQuery) Validate() error {
	return validate.Object(q).
		Optional("Page").Min(1).
		Optional("Tags").Max(10). // Max/Min operate on slice length
		Error()
}

var q ListUsersQuery
if err := validate.URLSearchParamsInto(r, &q); err != nil {
	// parse or validation failure
}
```

### Ad hoc value checks

```go
err := validate.Any("role", role).
	Required().
	In([]string{"admin", "member", "viewer"}).
	Error()
```

## Fluent Validation Semantics

### `AnyChecker`

Rules:

- lifecycle: `Required`, `Optional`, `If`, `Error`
- membership: `In`, `NotIn`
- string checks: `Email`, `URL`, `Regex`, `StartsWith`, `EndsWith`,
  `PermittedChars`
- numeric/length checks: `Min`, `Max`, `RangeInclusive`, `RangeExclusive`

Numeric rules apply to:

- numeric values directly
- length of strings, slices, arrays, and maps

### `ObjectChecker`

Field checks:

- `Required(field)`
- `Optional(field)`

Field-group relationships:

- `MutuallyExclusive(label, fields...)`
- `MutuallyRequired(label, fields...)`

Supported objects:

- struct (or pointer to struct)
- map with string keys (or pointer to map with string keys)

Unknown/unexported struct fields produce validation errors.

### Chaining behavior

`Required()`/`Optional()` initialize checker state. For optional zero values,
the checker is marked done and later chained checks are skipped.

## `Validator` Interface Behavior

If a value (or its addressable form) implements `Validate() error`, validation
is invoked recursively over:

- structs
- maps (keys and values)
- slices/arrays

Returned errors are wrapped as `ValidationError` when surfaced through checker
`.Error()`.

## Parsing Behavior Details

### JSON helpers (`JSONBodyInto`, `JSONBytesInto`, `JSONStrInto`)

- decode into destination
- run validation (`attemptValidation`)
- decode/validation failures return `*ValidationError`

Nil guards:

- `JSONBodyInto`: errors for nil request or nil body
- all JSON helpers: error when destination is nil

### URL query parsing (`URLSearchParamsInto`)

- destination must be non-nil pointer to struct
- uses field `json` names for mapping
- supports nested fields via dot paths (`address.city=...`)
- supports slices via repeated keys (`tags=a&tags=b`)
- supports maps via dotted key prefixes (`data.key=value`)

Pointer handling:

- empty string sets primitive pointer fields to `nil`
- non-empty values allocate pointer targets and parse values

## Error Handling

All validation/decode parse failures are represented as `*ValidationError`.

- detect using `validate.IsValidationError(err)`
- unwrap via `errors.As` / `errors.Is` (`ValidationError.Unwrap()` is
  implemented)

## Public API Reference

### Types

- `type Validator interface { Validate() error }`
- `type ValidationError struct{ Err error }`
- `type AnyChecker struct`
- `type ObjectChecker struct`

### Exported Struct Fields

- `ValidationError.Err error`
- `ObjectChecker.AnyChecker`
- `ObjectChecker.ChildCheckers []*AnyChecker`

### Constructors / Functions

- `func Any(label string, anything any) *AnyChecker`
- `func Object(object any) *ObjectChecker`
- `func IsValidationError(err error) bool`
- `func JSONBodyInto(r *http.Request, destStructPtr any) error`
- `func JSONBytesInto(data []byte, destStructPtr any) error`
- `func JSONStrInto(data string, destStructPtr any) error`
- `func URLSearchParamsInto(r *http.Request, destStructPtr any) error`

### Methods

`AnyChecker`:

- `func (c *AnyChecker) Required() *AnyChecker`
- `func (c *AnyChecker) Optional() *AnyChecker`
- `func (c *AnyChecker) If(condition bool, f func(*AnyChecker) *AnyChecker) *AnyChecker`
- `func (c *AnyChecker) In(permittedValuesSlice any) *AnyChecker`
- `func (c *AnyChecker) NotIn(prohibitedValuesSlice any) *AnyChecker`
- `func (c *AnyChecker) PermittedChars(allowedChars string) *AnyChecker`
- `func (c *AnyChecker) Email() *AnyChecker`
- `func (c *AnyChecker) Regex(regex *regexp.Regexp) *AnyChecker`
- `func (c *AnyChecker) StartsWith(prefix string) *AnyChecker`
- `func (c *AnyChecker) EndsWith(suffix string) *AnyChecker`
- `func (c *AnyChecker) URL() *AnyChecker`
- `func (c *AnyChecker) Min(min float64) *AnyChecker`
- `func (c *AnyChecker) Max(max float64) *AnyChecker`
- `func (c *AnyChecker) RangeInclusive(min, max float64) *AnyChecker`
- `func (c *AnyChecker) RangeExclusive(min, max float64) *AnyChecker`
- `func (c *AnyChecker) Error() error`

`ObjectChecker`:

- `func (oc *ObjectChecker) Required(field string) *AnyChecker`
- `func (oc *ObjectChecker) Optional(field string) *AnyChecker`
- `func (oc *ObjectChecker) MutuallyExclusive(label string, fields ...string) *ObjectChecker`
- `func (oc *ObjectChecker) MutuallyRequired(label string, fields ...string) *ObjectChecker`
- `func (oc *ObjectChecker) Error() error`

`ValidationError`:

- `func (e *ValidationError) Error() string`
- `func (e *ValidationError) Unwrap() error`
