package utils

import (
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/logger"
)

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

// GenerateUniqueID generates a unique ID of length 8
func GenerateUniqueID() string {
	return generateID(8) //nolint:mnd
}

// CreateShortID generates a short ID of length 6
func CreateShortID() string {
	return generateID(6) //nolint:mnd
}

func generateID(length int) string {
	var letters = []rune("bcdfghjklmnpqrstvwxz")
	b := make([]rune, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(letters))))
		if err != nil {
			log.Fatalf("Failed to generate unique ID: %v", err)
		}
		b[i] = letters[n.Int64()]
	}
	return string(b)
}

func ExpandPath(path string) (string, error) {
	if strings.HasPrefix(path, "~/") {
		usr, err := user.Current()
		if err != nil {
			return "", err
		}
		path = filepath.Join(usr.HomeDir, path[2:])
	}
	return path, nil
}

//nolint:mnd
func GenerateUniqueName(projectID, uniqueID string) string {
	// Take the first 4 characters of projectID and uniqueID
	shortProjectID := projectID
	if len(shortProjectID) > 4 {
		shortProjectID = shortProjectID[:4]
	}
	shortUniqueID := uniqueID
	if len(shortUniqueID) > 4 {
		shortUniqueID = shortUniqueID[:4]
	}

	// Combine the parts
	vmName := fmt.Sprintf("vm-%s-%s-%s", shortProjectID, shortUniqueID, GenerateUniqueID()[:4])

	// Ensure the total length is less than 20 characters
	if len(vmName) > 19 {
		vmName = vmName[:19]
	}

	return vmName
}

func ConvertStringPtrMapToStringMap(m map[string]*string) map[string]string {
	result := make(map[string]string)
	for k, v := range m {
		if v != nil {
			result[k] = *v
		}
	}
	return result
}

func ConvertStringMapToStringPtrMap(m map[string]string) map[string]*string {
	result := make(map[string]*string)
	for k, v := range m {
		value := v
		result[k] = &value
	}
	return result
}

func TruncateString(s string, maxLength int) string {
	ellipsis := "..."
	if len(s) > maxLength {
		return s[:maxLength-len(ellipsis)] + ellipsis
	}
	return s
}
