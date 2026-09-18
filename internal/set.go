package internal

import "iter"

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](values ...T) Set[T] {
	s := make(Set[T], len(values))
	for _, value := range values {
		s.Add(value)
	}
	return s
}

func SetFrom[T comparable](sequences ...iter.Seq[T]) Set[T] {
	s := NewSet[T]()
	for value := range Concat(sequences...) {
		s.Add(value)
	}
	return s
}

func (s Set[T]) Add(value T) {
	s[value] = struct{}{}
}

func (s Set[T]) Contains(value T) bool {
	_, exists := s[value]
	return exists
}

func (s Set[T]) Remove(value T) {
	delete(s, value)
}
