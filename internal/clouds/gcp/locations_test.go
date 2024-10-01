package internal_gcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type InternalGCPTestSuite struct {
	suite.Suite
}

func (suite *InternalGCPTestSuite) TestIsValidGCPLocation() {
	testCases := []struct {
		name          string
		location      string
		expectedValid bool
	}{
		{
			name:          "Valid location",
			location:      "us-central1-a",
			expectedValid: true,
		},
		{
			name:          "Valid location with different case",
			location:      "US-CENTRAL1-A",
			expectedValid: true,
		},
		{
			name:          "Invalid location",
			location:      "invalid-location",
			expectedValid: false,
		},
		{
			name:          "Empty location",
			location:      "",
			expectedValid: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			isValid := IsValidGCPLocation(tc.location)
			assert.Equal(suite.T(), tc.expectedValid, isValid)
		})
	}
}

func (suite *InternalGCPTestSuite) TestIsValidGCPMachineType() {
	testCases := []struct {
		name          string
		location      string
		machineType   string
		expectedValid bool
	}{
		{
			name:          "Valid machine type",
			location:      "us-central1-a",
			machineType:   "n1-standard-1",
			expectedValid: true,
		},
		{
			name:          "Invalid machine type",
			location:      "us-central1-a",
			machineType:   "invalid-machine-type",
			expectedValid: false,
		},
		{
			name:          "Invalid location",
			location:      "invalid-location",
			machineType:   "n1-standard-1",
			expectedValid: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			isValid := IsValidGCPMachineType(tc.location, tc.machineType)
			assert.Equal(suite.T(), tc.expectedValid, isValid)
		})
	}
}

func (suite *InternalGCPTestSuite) TestIsValidGCPDiskImageFamily() {
	testCases := []struct {
		name            string
		location        string
		diskImageFamily string
		expectedValid   bool
	}{
		{
			name:            "Valid disk image family",
			location:        "us-central1-a",
			diskImageFamily: "ubuntu-2004-lts",
			expectedValid:   true,
		},
		{
			name:            "Invalid disk image family",
			location:        "us-central1-a",
			diskImageFamily: "invalid-family",
			expectedValid:   false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			imageURL, err := IsValidGCPDiskImageFamily(tc.location, tc.diskImageFamily)
			if tc.expectedValid {
				assert.NoError(suite.T(), err)
				assert.NotEmpty(suite.T(), imageURL)
			} else {
				assert.Error(suite.T(), err)
				assert.Empty(suite.T(), imageURL)
			}
		})
	}
}

func (suite *InternalGCPTestSuite) TestGetGCPDiskImageURL() {
	projectID := "ubuntu-os-cloud"
	family := "ubuntu-2004-lts"
	expectedURL := "https://www.googleapis.com/compute/v1/projects/ubuntu-os-cloud/global/images/family/ubuntu-2004-lts"

	url := GetGCPDiskImageURL(projectID, family)
	assert.Equal(suite.T(), expectedURL, url)
}

func TestInternalGCPTestSuite(t *testing.T) {
	suite.Run(t, new(InternalGCPTestSuite))
}
