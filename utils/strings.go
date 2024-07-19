package utils

import (
	"crypto/rand"
	"log"
	"math/big"
	"strconv"

	"github.com/bacalhau-project/andaime/logger"
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

// safeDeref safely dereferences a string pointer. If the pointer is nil, it returns a placeholder.
func SafeDeref(s *string) string {
	log := logger.Get()

	// If s is a string pointer, dereference it
	if s != nil {
		return *s
	} else {
		log.Debug("State is nil")
		return ""
	}
}
