package collections

import (
	"iter"
	"maps"
	"slices"
)

type Set[T comparable] map[T]struct{}

func NewSet[T comparable](values ...T) Set[T] {
	s := make(Set[T], len(values))
	s.Add(values...)

	return s
}

func SetFrom[T comparable](sequences ...iter.Seq[T]) Set[T] {
	s := NewSet[T]()
	for value := range Concat(sequences...) {
		s.Add(value)
	}

	return s
}

func (s Set[T]) Add(values ...T) {
	for _, value := range values {
		s[value] = struct{}{}
	}
}

func (s Set[T]) Contains(value T) bool {
	_, exists := s[value]
	return exists
}

func (s Set[T]) Remove(value T) {
	delete(s, value)
}

func (s Set[T]) Slice() []T {
	return slices.Collect(maps.Keys(s))
}
