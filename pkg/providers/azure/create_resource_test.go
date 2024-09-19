package azure

import (
	"fmt"
	"testing"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type CreateResourceTestSuite struct {
	BaseAzureTestSuite
}

func TestCreateResourceSuite(t *testing.T) {
	suite.Run(t, new(CreateResourceTestSuite))
}

func (suite *CreateResourceTestSuite) TestCreateResources() {
	tests := []struct {
		name                string
		locations           []string
		machinesPerLocation int
		deployMachineErrors map[string]error
		expectedError       string
	}{
		{
			name:                "Success - Single Location",
			locations:           []string{"eastus"},
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{},
			expectedError:       "",
		},
		{
			name:                "Success - Multiple Locations",
			locations:           []string{"eastus", "westus"},
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{},
			expectedError:       "",
		},
		{
			name:                "Failure - Deploy Machine Error",
			locations:           []string{"eastus"},
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{
				"machine-eastus-0": fmt.Errorf("deploy machine error"),
			},
			expectedError: "deploy machine error",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			provider := &AzureProvider{}

			testModel := &models.Deployment{
				Locations: tt.locations,
				Machines:  make(map[string]models.Machiner),
			}

			for _, location := range tt.locations {
				for i := 0; i < tt.machinesPerLocation; i++ {
					machineName := fmt.Sprintf("machine-%s-%d", location, i)
					testModel.Machines[machineName] = &models.Machine{
						Name:     machineName,
						Location: location,
					}
				}
			}

			err := provider.CreateResources(suite.Ctx)

			if tt.expectedError != "" {
				assert.ErrorContains(suite.T(), err, tt.expectedError)
			} else {
				assert.NoError(suite.T(), err)
				suite.MockAzureClient.AssertExpectations(suite.T())
			}
		})
	}
}
