package internal_azure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidAzureZone(t *testing.T) {
	tests := []struct {
		name     string
		zone     string
		expected bool
	}{
		{"Valid zone", "polandcentral", true},
		{"Valid zone with different case", "polandCentral", true},
		{"Invalid zone", "invalidzone", false},
		{"Empty zone", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAzureLocation(tt.zone)
			assert.Equal(
				t,
				tt.expected,
				result,
				fmt.Sprintf("%s: expected %v, got %v", tt.name, tt.expected, result),
			)
		})
	}
}
