package utils

import (
	"strconv"
)

func StringPtr(s string) *string {
	return &s
}

func ParseStringToIntOrZero(row string) int {
	parsedInt, err := strconv.Atoi(row)
	if err != nil {
		// If there's an error in parsing, return 0
		return 0
	}
	return parsedInt
}
