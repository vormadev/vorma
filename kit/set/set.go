package set

import (
	"iter"
)

// Set is a generic set data structure that holds unique values of any comparable type T.
type Set[T comparable] struct {
	m map[T]struct{}
}

// New creates a new Set. If an optional slice of values is provided, those values are added to the set.
func New[T comparable](from ...[]T) *Set[T] {
	if len(from) == 0 {
		return &Set[T]{}
	}
	s := &Set[T]{}
	s.m = make(map[T]struct{}, len(from[0]))
	for _, val := range from[0] {
		s.Add(val)
	}
	return s
}

// Add adds the specified value to the set. If the value is already present, it has no effect.
func (s *Set[T]) Add(val T) {
	if s.m == nil {
		s.m = make(map[T]struct{}, 1)
	}
	s.m[val] = struct{}{}
}

// Remove removes the specified value from the set. If the value is not present, it has no effect.
func (s *Set[T]) Remove(val T) {
	delete(s.m, val)
}

// Has returns true if the set contains the specified value.
func (s *Set[T]) Has(val T) bool {
	_, ok := s.m[val]
	return ok
}

// HasAll returns true if the set contains all the specified values.
func (s *Set[T]) HasAll(val ...T) bool {
	for _, v := range val {
		if _, ok := s.m[v]; !ok {
			return false
		}
	}
	return true
}

// HasAny returns true if the set contains any of the specified values.
func (s *Set[T]) HasAny(val ...T) bool {
	for _, v := range val {
		if _, ok := s.m[v]; ok {
			return true
		}
	}
	return false
}

// Slice returns a slice containing all elements in the set. The order of elements is not guaranteed.
func (s *Set[T]) Slice() []T {
	slice := make([]T, 0, len(s.m))
	for val := range s.m {
		slice = append(slice, val)
	}
	return slice
}

// Range calls the provided function for each element in the set.
func (s *Set[T]) Range() iter.Seq[T] {
	return func(yield func(T) bool) {
		for val := range s.m {
			if !yield(val) {
				return
			}
		}
	}
}

// Diff returns a new Set containing elements in s that are not in other.
func (s *Set[T]) Diff(other *Set[T]) *Set[T] {
	result := &Set[T]{}
	for val := range s.Range() {
		if !other.Has(val) {
			result.Add(val)
		}
	}
	return result
}

// Clear removes all elements from the set.
func (s *Set[T]) Clear() {
	s.m = make(map[T]struct{})
}

// Len returns the number of elements in the set.
func (s *Set[T]) Len() int {
	return len(s.m)
}

// IsEmpty returns true if the set contains no elements.
func (s *Set[T]) IsEmpty() bool {
	return s.Len() == 0
}

// Union returns a new Set containing all elements that are in either s or other.
func (s *Set[T]) Union(other *Set[T]) *Set[T] {
	result := &Set[T]{}
	for val := range s.Range() {
		result.Add(val)
	}
	for val := range other.Range() {
		result.Add(val)
	}
	return result
}

// Intersect returns a new Set containing elements that are in both s and other.
func (s *Set[T]) Intersect(other *Set[T]) *Set[T] {
	result := &Set[T]{}
	for val := range s.Range() {
		if other.Has(val) {
			result.Add(val)
		}
	}
	return result
}

// Equals returns true if s and other contain the same elements, regardless of order.
func (s *Set[T]) Equals(other *Set[T]) bool {
	if s.Len() != other.Len() {
		return false
	}
	for val := range s.Range() {
		if !other.Has(val) {
			return false
		}
	}
	return true
}

// Replace replaces the contents of the set with the provided slice of values.
func (s *Set[T]) Replace(vals []T) {
	s.Clear()
	for _, val := range vals {
		s.Add(val)
	}
}
