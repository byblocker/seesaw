package collections

import "iter"

func ExtractMapByKeys[K comparable, V any](source map[K]V, keys iter.Seq[K]) map[K]V {
	extracted := make(map[K]V)
	for key := range keys {
		if value, exists := source[key]; exists {
			extracted[key] = value
			delete(source, key)
		}
	}

	return extracted
}
