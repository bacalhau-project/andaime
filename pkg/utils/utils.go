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
