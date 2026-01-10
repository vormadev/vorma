---
title: validate
description:
    Struct validation with recursive Validator interface support, HTTP request
    parsing, and fluent field checks.
---

```go
import "github.com/vormadev/vorma/kit/validate"
```

## Validator Interface

Implement on structs for automatic recursive validation:

```go
type Validator interface {
    Validate() error
}
```

## HTTP Request Parsing + Validation

All decode into struct and auto-validate fields implementing `Validator`:

```go
func JSONBodyInto(r *http.Request, destStructPtr any) error
func JSONBytesInto(data []byte, destStructPtr any) error
func JSONStrInto(data string, destStructPtr any) error
func URLSearchParamsInto(r *http.Request, destStructPtr any) error
```

## ValidationError

```go
type ValidationError struct { Err error }
func IsValidationError(err error) bool
```

## Entry Points

Validate any value (recursively validates nested Validators):

```go
func Any(label string, anything any) *AnyChecker
```

Validate struct/map with field-level rules:

```go
func Object(object any) *ObjectChecker
```

## AnyChecker Methods

### Initialization

```go
func (c *AnyChecker) Required() *AnyChecker
func (c *AnyChecker) Optional() *AnyChecker
func (c *AnyChecker) Error() error
```

### Conditional

```go
func (c *AnyChecker) If(condition bool, f func(*AnyChecker) *AnyChecker) *AnyChecker
```

### Membership

```go
func (c *AnyChecker) In(permittedValuesSlice any) *AnyChecker
func (c *AnyChecker) NotIn(prohibitedValuesSlice any) *AnyChecker
```

### String Validation

```go
func (c *AnyChecker) Email() *AnyChecker
func (c *AnyChecker) URL() *AnyChecker
func (c *AnyChecker) Regex(regex *regexp.Regexp) *AnyChecker
func (c *AnyChecker) StartsWith(prefix string) *AnyChecker
func (c *AnyChecker) EndsWith(suffix string) *AnyChecker
func (c *AnyChecker) PermittedChars(allowedChars string) *AnyChecker
```

### Numeric Validation (also works on string/slice/map length)

```go
func (c *AnyChecker) Min(min float64) *AnyChecker
func (c *AnyChecker) Max(max float64) *AnyChecker
func (c *AnyChecker) RangeInclusive(min, max float64) *AnyChecker
func (c *AnyChecker) RangeExclusive(min, max float64) *AnyChecker
```

## ObjectChecker Methods

Field validation:

```go
func (oc *ObjectChecker) Required(field string) *AnyChecker
func (oc *ObjectChecker) Optional(field string) *AnyChecker
func (oc *ObjectChecker) Error() error
```

Field relationships:

```go
func (oc *ObjectChecker) MutuallyExclusive(label string, fields ...string) *ObjectChecker
func (oc *ObjectChecker) MutuallyRequired(label string, fields ...string) *ObjectChecker
```

## Example

```go
type CreateUserRequest struct {
    Email    string `json:"email"`
    Age      int    `json:"age"`
    Role     string `json:"role"`
    Password string `json:"password"`
    Confirm  string `json:"confirm"`
}

func (r *CreateUserRequest) Validate() error {
    return validate.Object(r).
        Required("Email").Email().
        Required("Age").RangeInclusive(18, 120).
        Required("Role").In([]string{"admin", "user", "guest"}).
        MutuallyRequired("password-confirm", "Password", "Confirm").
        Error()
}

// In handler
var req CreateUserRequest
if err := validate.JSONBodyInto(r, &req); err != nil {
    if validate.IsValidationError(err) {
        // 400 Bad Request
    }
}
```
