package azure

import (
	"context"
	"fmt"
	"testing"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/internal/testutil"
	azure_mocks "github.com/bacalhau-project/andaime/mocks/azure"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	pkg_testutil "github.com/bacalhau-project/andaime/pkg/testutil"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type PkgProvidersAzureCreateResourceTestSuite struct {
	suite.Suite
	ctx                    context.Context
	origLogger             *logger.Logger
	testSSHPublicKeyPath   string
	testSSHPrivateKeyPath  string
	cleanupPublicKey       func()
	cleanupPrivateKey      func()
	mockAzureClient        *azure_mocks.MockAzureClienter
	azureProvider          *AzureProvider
	origGetGlobalModelFunc func() *display.DisplayModel
	origNewSSHConfigFunc   func(string, int, string, string) (sshutils.SSHConfiger, error)
	mockSSHConfig          *sshutils.MockSSHConfig
	deployment             *models.Deployment // Add deployment as suite field
}

func (suite *PkgProvidersAzureCreateResourceTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testSSHPublicKeyPath,
		suite.cleanupPublicKey,
		suite.testSSHPrivateKeyPath,
		suite.cleanupPrivateKey = testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	suite.origGetGlobalModelFunc = display.GetGlobalModelFunc
	suite.origLogger = logger.Get()
	suite.origNewSSHConfigFunc = sshutils.NewSSHConfigFunc
}

func (suite *PkgProvidersAzureCreateResourceTestSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
	sshutils.NewSSHConfigFunc = suite.origNewSSHConfigFunc
	logger.SetGlobalLogger(suite.origLogger)
}

func (suite *PkgProvidersAzureCreateResourceTestSuite) SetupTest() {
	// Reset everything for each test
	viper.Reset()
	pkg_testutil.SetupViper(models.DeploymentTypeAzure,
		suite.testSSHPublicKeyPath,
		suite.testSSHPrivateKeyPath,
	)

	// Create fresh mocks for each test
	suite.mockAzureClient = new(azure_mocks.MockAzureClienter)
	suite.mockSSHConfig = new(sshutils.MockSSHConfig)

	// Create fresh provider for each test
	suite.azureProvider = &AzureProvider{}
	suite.azureProvider.SetAzureClient(suite.mockAzureClient)

	// Create fresh deployment for each test
	suite.deployment = &models.Deployment{
		Machines: make(map[string]models.Machiner),
		Azure: &models.AzureConfig{
			ResourceGroupName: "test-rg-name",
		},
		SSHPublicKeyMaterial: "PUBLIC KEY MATERIAL",
	}

	// Set up fresh global model function for each test
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: suite.deployment,
		}
	}

	// Set up fresh SSH config function for each test
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return suite.mockSSHConfig, nil
	}

	// Set up fresh logger for each test
	logger.SetGlobalLogger(logger.NewTestLogger(suite.T()))
}

func (suite *PkgProvidersAzureCreateResourceTestSuite) TestCreateResources() {
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
			machinesPerLocation: 1, // Only create one machine since we're testing failure
			deployMachineErrors: map[string]error{
				"machine-eastus-0": fmt.Errorf("deploy machine error"),
			},
			expectedError: "deploy machine error",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset everything for each test
			suite.mockAzureClient = new(azure_mocks.MockAzureClienter)
			suite.mockSSHConfig = new(sshutils.MockSSHConfig)
			suite.azureProvider = &AzureProvider{} // Create fresh provider
			suite.azureProvider.SetAzureClient(suite.mockAzureClient)

			// Reset deployment state
			suite.deployment.Locations = tt.locations
			suite.deployment.Machines = make(map[string]models.Machiner)

			// Create the test machines
			for _, location := range tt.locations {
				for i := 0; i < tt.machinesPerLocation; i++ {
					machineName := fmt.Sprintf("machine-%s-%d", location, i)
					suite.deployment.SetMachine(machineName, &models.Machine{
						Name:              machineName,
						Location:          location,
						SSHPublicKeyPath:  suite.testSSHPublicKeyPath,
						SSHPrivateKeyPath: suite.testSSHPrivateKeyPath,
					})
				}
			}

			// Set up a new mock Azure client for each test case
			suite.mockAzureClient = new(azure_mocks.MockAzureClienter)
			suite.azureProvider.SetAzureClient(suite.mockAzureClient)

			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: suite.deployment,
				}
			}

			// Set up expectations for this specific test case
			for _, err := range tt.deployMachineErrors {
				suite.mockAzureClient.On("DeployTemplate",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(nil, err).Once()
			}

			if len(tt.deployMachineErrors) == 0 {
				mockPoller := new(MockPoller)
				mockPoller.On("PollUntilDone", mock.Anything, mock.Anything).
					Return(armresources.DeploymentsClientCreateOrUpdateResponse{
						DeploymentExtended: testdata.FakeDeployment(),
					}, nil)

				suite.mockAzureClient.On("DeployTemplate",
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(mockPoller, nil).Maybe()
				suite.mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).
					Return(testdata.FakeVirtualMachine(), nil)
				suite.mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
					Return(testdata.FakeNetworkInterface(), nil)
				suite.mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
					Return(testdata.FakePublicIPAddress("20.30.40.50"), nil)
				suite.mockSSHConfig.On("WaitForSSH",
					mock.Anything,
					mock.Anything,
					mock.Anything,
				).Return(nil)
			}
			// Execute the test
			err := suite.azureProvider.CreateResources(suite.ctx)

			// Verify expectations
			if tt.expectedError != "" {
				suite.Error(err, "Expected an error but got nil")
				if err != nil {
					suite.Contains(err.Error(), tt.expectedError)
				}
			} else {
				suite.NoError(err)
			}

			// Verify all mock expectations were met
			suite.mockAzureClient.AssertExpectations(suite.T())
			suite.mockSSHConfig.AssertExpectations(suite.T())
		})
	}
}
func TestPkgProvidersAzureCreateResourceSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersAzureCreateResourceTestSuite))
}
