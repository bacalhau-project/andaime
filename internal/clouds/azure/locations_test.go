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
		location       string
		expectedValid  bool
		expectedReason string
	}{
		{
			name:           "Valid location",
			location:       "polandcentral",
			expectedValid:  true,
			expectedReason: "",
		},
		{
			name:           "Valid location with different case",
			location:       "polandCentral",
			expectedValid:  true,
			expectedReason: "",
		},
		{
			name:           "Invalid location",
			location:       "invalidlocation",
			expectedValid:  false,
			expectedReason: "invalidlocation is not a valid Azure location",
		},
		{
			name:           "Empty location",
			location:       "",
			expectedValid:  false,
			expectedReason: "location cannot be empty",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			isValid := IsValidAzureLocation(tc.location)
			suite.Equal(
				tc.expectedValid,
				isValid,
				fmt.Sprintf(
					"Expected validity of location '%s' to be %v, but got %v",
					tc.location,
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
