package internal_aws

import (
	"testing"
)

func TestIsValidAWSRegion(t *testing.T) {
	tests := []struct {
		name     string
		region   string
		expected bool
	}{
		{"Valid region", "us-east-1", true},
		{"Valid region case insensitive", "US-WEST-2", true},
		{"Invalid region", "invalid-region", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAWSRegion(tt.region)
			if result != tt.expected {
				t.Errorf("IsValidAWSRegion(%s) = %v, want %v", tt.region, result, tt.expected)
			}
		})
	}
}
func TestIsValidAWSInstanceType(t *testing.T) {
	tests := []struct {
		name         string
		region       string
		instanceType string
		expected     bool
	}{
		{"Valid instance type", "us-east-1", "t2.micro", true},
		{"Invalid instance type", "us-east-1", "invalid-type", false},
		{"Invalid region", "invalid-region", "t2.micro", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsValidAWSInstanceType(tt.region, tt.instanceType)
			if result != tt.expected {
				t.Errorf(
					"IsValidAWSInstanceType(%s, %s) = %v, want %v",
					tt.region,
					tt.instanceType,
					result,
					tt.expected,
				)
			}
		})
	}
}

func TestGetAllAWSRegions(t *testing.T) {
	regions, err := GetAllAWSRegions()
	if err != nil {
		t.Fatalf("GetAllAWSRegions() returned an error: %v", err)
	}

	if len(regions) == 0 {
		t.Error("GetAllAWSRegions() returned an empty list of regions")
	}

	// Check for some known regions
	knownRegions := []string{"us-east-1", "us-west-2", "eu-west-1"}
	for _, known := range knownRegions {
		if !contains(regions, known) {
			t.Errorf("GetAllAWSRegions() does not contain known region: %s", known)
		}
	}
}

func TestGetAWSInstanceTypes(t *testing.T) {
	tests := []struct {
		name             string
		region           string
		expectedErrorNil bool
	}{
		{"Valid region", "us-east-1", true},
		{"Invalid region", "invalid-region", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			types, err := GetAWSInstanceTypes(tt.region)
			if (err == nil) != tt.expectedErrorNil {
				t.Errorf(
					"GetAWSInstanceTypes(%s) error = %v, expectedErrorNil %v",
					tt.region,
					err,
					tt.expectedErrorNil,
				)
				return
			}
			if tt.expectedErrorNil && len(types) == 0 {
				t.Errorf(
					"GetAWSInstanceTypes(%s) returned an empty list for a valid region",
					tt.region,
				)
			}
		})
	}
}

// Helper function to check if a slice contains a string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
