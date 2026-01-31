// Package genericsutil provides helpers for implementing type erasure patterns
package genericsutil

/////////////////////////////////////////////////////////////////////
/////// ZERO HELPERS
/////////////////////////////////////////////////////////////////////

type AnyZeroHelper interface {
	I() any    // returns direct I zero val
	O() any    // returns direct O zero val
	IPtr() any // returns `new(I)` (pointer to I)
	OPtr() any // returns `new(O)` (pointer to O)
}

type ZeroHelper[I any, O any] struct{}

func (ZeroHelper[I, O]) I() any    { return Zero[I]() }
func (ZeroHelper[I, O]) O() any    { return Zero[O]() }
func (ZeroHelper[I, O]) IPtr() any { return new(I) }
func (ZeroHelper[I, O]) OPtr() any { return new(O) }

/////////////////////////////////////////////////////////////////////
/////// UTILITIES
/////////////////////////////////////////////////////////////////////

// Simple alias for an empty struct
type None = struct{}

// Returns true if v is either an empty struct or a pointer to an empty struct
func IsNone(v any) bool {
	_, ok := v.(struct{})
	if ok {
		return true
	}
	_, ok = v.(*struct{})
	return ok
}

// Returns the zero value of type T
func Zero[T any]() T {
	var zero T
	return zero
}

// Returns v cast as type T if possible, otherwise returns the zero value of T
func AssertOrZero[T any](v any) T {
	if typedV, ok := v.(T); ok {
		return typedV
	}
	return Zero[T]()
}

// Returns field if it is not the zero value for its type, otherwise returns defaultVal
func OrDefault[F comparable](field F, defaultVal F) F {
	var zero F
	if field == zero {
		return defaultVal
	}
	return field
}
