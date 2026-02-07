package set

type Set[T comparable] map[T]struct{}

func New[T comparable]() Set[T] {
	return make(Set[T])
}

func (s Set[T]) Add(val T) Set[T] {
	if s == nil {
		s = New[T]()
	}
	s[val] = struct{}{}
	return s
}

func (s Set[T]) Contains(val T) bool {
	_, ok := s[val]
	return ok
}
