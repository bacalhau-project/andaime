package azure

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsValidAzureZone(t *testing.T) {
	tests := []struct {
		name     string
		zone     string
		expected bool
	}{
		{"Valid zone", "westus", true},
		{"Valid zone with different case", "EastUS", true},
		{"Invalid zone", "invalidzone", false},
		{"Empty zone", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAzureZone(tt.zone)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSetMachineZone(t *testing.T) {
	tests := []struct {
		name        string
		zone        string
		expectError bool
	}{
		{"Valid zone", "westus", false},
		{"Valid zone with different case", "EastUS", false},
		{"Invalid zone", "invalidzone", true},
		{"Empty zone", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			machine := &Machine{}
			err := SetMachineZone(machine, tt.zone)

			if tt.expectError {
				assert.Error(t, err)
				assert.Empty(t, machine.Zone)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, strings.ToLower(tt.zone), machine.Zone)
			}
		})
	}
}
