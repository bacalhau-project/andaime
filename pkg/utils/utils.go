package utils

import (
	"fmt"
	"math"
	"strings"
	"time"
)

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

func GetSafeDiskSize(i int) int32 {
	if i > math.MaxInt32 {
		i = math.MaxInt32
	} else if i < math.MinInt32 {
		i = math.MinInt32
	}

	//nolint:gosec
	return int32(i)
}

func SafeConvertToInt32(value int) int32 {
	if value > math.MaxInt32 {
		return math.MaxInt32
	}
	if value < math.MinInt32 {
		return math.MinInt32
	}

	//nolint:gosec
	return int32(value)
}

const (
	FormatHours = 1 << iota
	FormatMinutes
	FormatSeconds
	FormatMilliseconds
)

func FormatDuration(d time.Duration, format int) string {
	d = d.Round(time.Millisecond) // Round to nearest millisecond

	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond

	var parts []string

	if format&FormatHours != 0 && h > 0 {
		parts = append(parts, fmt.Sprintf("%dh", h))
	}

	if format&FormatMinutes != 0 {
		parts = append(parts, fmt.Sprintf("%02dm", m))
	}

	if format&FormatSeconds != 0 {
		parts = append(parts, fmt.Sprintf("%02ds", s))
	}

	if format&FormatMilliseconds != 0 {
		parts = append(parts, fmt.Sprintf("%03dms", ms))
	}

	if len(parts) == 0 {
		return "0s"
	}

	return strings.Join(parts, "")
}
