package collections

import "iter"

func Find[T any](seq iter.Seq[T], match func(T) bool) (T, bool) {
	for value := range seq {
		if match(value) {
			return value, true
		}
	}
	var zero T
	return zero, false
}

func FlatMap[T, U any](seq iter.Seq[T], transform func(T) iter.Seq[U]) iter.Seq[U] {
	return func(yield func(U) bool) {
		for value := range seq {
			for element := range transform(value) {
				if !yield(element) {
					return
				}
			}
		}
	}
}

func Map[T, U any](seq iter.Seq[T], transform func(T) U) iter.Seq[U] {
	return func(yield func(U) bool) {
		for value := range seq {
			if !yield(transform(value)) {
				return
			}
		}
	}
}

func Filter[T any](seq iter.Seq[T], keep func(T) bool) iter.Seq[T] {
	return func(yield func(T) bool) {
		for value := range seq {
			if keep(value) && !yield(value) {
				return
			}
		}
	}
}

func Concat[T any](sequences ...iter.Seq[T]) iter.Seq[T] {
	return func(yield func(T) bool) {
		for _, seq := range sequences {
			for value := range seq {
				if !yield(value) {
					return
				}
			}
		}
	}
}
