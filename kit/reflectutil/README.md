# kit/reflectutil

`github.com/vormadev/vorma/kit/reflectutil`

Small reflection helpers for interface checks, nil/pointer inspection, and JSON
tag name extraction.

## Import

```go
import "github.com/vormadev/vorma/kit/reflectutil"
```

## Quick Start

### Check interface implementation (value or pointer receiver)

```go
readerIface := reflectutil.ToInterfaceReflectType[io.Reader]()
if reflectutil.ImplementsInterface(t, readerIface) {
	// supports io.Reader
}
```

`ImplementsInterface` returns false for nil types and panics if `iface` is not
an interface type.

### Nil-or-points-to-nil checks

```go
isNil := reflectutil.ExcludingNoneGetIsNilOrUltimatelyPointsToNil(v)
```

This follows pointers/interfaces recursively and treats `struct{}`/`*struct{}`
("None" sentinel) as non-nil.

### JSON field name extraction

```go
name := reflectutil.GetJSONFieldName(field)
```

- returns tag name if present (`json:"id,omitempty"` -> `id`)
- returns `""` for ignored fields (`json:"-"`)
- falls back to field name when tag key is empty

## API Coverage

### Functions

- `func ExcludingNoneGetIsNilOrUltimatelyPointsToNil(v any) bool`
- `func GetJSONFieldName(field reflect.StructField) string`
- `func ImplementsInterface(t reflect.Type, iface reflect.Type) bool`
- `func ToInterfaceReflectType[T any]() reflect.Type`
