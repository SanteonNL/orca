package slices

// Deduplicate removes duplicate elements from a slice.
func Deduplicate[T any](slice []T, comparer func(a, b T) bool) []T {
	var result []T
	for i, value := range slice {
		found := false
		for j, existing := range result {
			if i == j {
				continue
			}
			if comparer(value, existing) {
				found = true
				break
			}
		}
		if !found {
			result = append(result, value)
		}
	}
	return result
}
