package internal_azure

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type InternalAzureTestSuite struct {
	suite.Suite
}

func (suite *InternalAzureTestSuite) TestIsValidAzureLocation() {
	testCases := []struct {
		name           string
		region         string
		expectedValid  bool
		expectedReason string
	}{
		{
			name:           "Valid location",
			region:         "polandcentral",
			expectedValid:  true,
			expectedReason: "",
		},
		{
			name:           "Valid location with different case",
			region:         "polandCentral",
			expectedValid:  true,
			expectedReason: "",
		},
		{
			name:           "Invalid location",
			region:         "invalidlocation",
			expectedValid:  false,
			expectedReason: "invalidlocation is not a valid Azure region",
		},
		{
			name:           "Empty region",
			region:         "",
			expectedValid:  false,
			expectedReason: "region cannot be empty",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			isValid := IsValidAzureRegion(tc.region)
			suite.Equal(
				tc.expectedValid,
				isValid,
				fmt.Sprintf(
					"Expected validity of region '%s' to be %v, but got %v",
					tc.region,
					tc.expectedValid,
					isValid,
				),
			)
		})
	}
}

func TestInternalAzureTestSuite(t *testing.T) {
	suite.Run(t, new(InternalAzureTestSuite))
}
