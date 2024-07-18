package utils

import (
	"crypto/rand"
	"log"
	"math/big"
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

func GenerateUniqueID() string {
	var lettersAndDigits = []rune("abcdefghijklmnopqrstuvwxyz0123456789")
	b := make([]rune, 8) //nolint:gomnd
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(lettersAndDigits))))
		if err != nil {
			log.Fatalf("Failed to generate unique ID: %v", err)
		}
		b[i] = lettersAndDigits[n.Int64()]
	}
	return string(b)
}
