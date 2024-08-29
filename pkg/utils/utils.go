package utils

import "strings"

// ToPtr returns a pointer to the given value
func ToPtr[T any](v T) *T {
	return &v
}

func CaseInsensitiveContains(s []string, t string) bool {
	for _, v := range s {
		if strings.EqualFold(v, t) {
			return true
		}
	}
	return false
}

func RemoveDuplicates(s []string) []string {
	seen := make(map[string]struct{})
	uniqueCount := 0
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			uniqueCount++
		}
	}

	result := make([]string, 0, uniqueCount)
	seen = make(map[string]struct{}) // Reset the map
	for _, v := range s {
		if _, ok := seen[v]; !ok {
			seen[v] = struct{}{}
			result = append(result, v)
		}
	}
	return result
}
