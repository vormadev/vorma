package set

type Set[T comparable] struct {
	m map[T]struct{}
}

func (s *Set[T]) Add(val T) {
	if s.m == nil {
		s.m = make(map[T]struct{})
	}
	s.m[val] = struct{}{}
}

func (s *Set[T]) Contains(val T) bool {
	_, ok := s.m[val]
	return ok
}

func (s *Set[T]) ToSlice() []T {
	slice := make([]T, 0, len(s.m))
	for val := range s.m {
		slice = append(slice, val)
	}
	return slice
}
