package azure

import (
	"context"
	"fmt"
	"strings"
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
	testPrivateKeyPath     string
	cleanupPublicKey       func()
	cleanupPrivateKey      func()
	mockAzureClient        *azure_mocks.MockAzureClienter
	azureProvider          *AzureProvider
	origGetGlobalModelFunc func() *display.DisplayModel
	origNewSSHConfigFunc   func(string, int, string, string) (sshutils.SSHConfiger, error)
	mockSSHConfig          *sshutils.MockSSHConfig
}

func (suite *PkgProvidersAzureCreateResourceTestSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testSSHPublicKeyPath,
		suite.cleanupPublicKey,
		suite.testPrivateKeyPath,
		suite.cleanupPrivateKey = testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	suite.mockAzureClient = new(azure_mocks.MockAzureClienter)
	suite.origGetGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		deployment, err := models.NewDeployment()
		suite.Require().NoError(err)
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	suite.origLogger = logger.Get() // Save the original logger
	testLogger := logger.NewTestLogger(suite.T())
	logger.SetGlobalLogger(testLogger)
}

func (suite *PkgProvidersAzureCreateResourceTestSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
	sshutils.NewSSHConfigFunc = suite.origNewSSHConfigFunc
}

func (suite *PkgProvidersAzureCreateResourceTestSuite) SetupTest() {
	viper.Reset()
	pkg_testutil.SetupViper(models.DeploymentTypeAzure,
		suite.testSSHPublicKeyPath,
		suite.testPrivateKeyPath,
	)

	suite.azureProvider = &AzureProvider{}
	suite.azureProvider.SetAzureClient(suite.mockAzureClient)

	suite.mockSSHConfig = new(sshutils.MockSSHConfig)
	suite.origNewSSHConfigFunc = sshutils.NewSSHConfigFunc
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return suite.mockSSHConfig, nil
	}
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
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{
				"machine-eastus-0": fmt.Errorf("deploy machine error"),
			},
			expectedError: "deploy machine error",
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			// Reset the suite for each test case
			suite.SetupTest()

			// Create a new deployment for each test case
			deployment := &models.Deployment{
				Locations: tt.locations,
				Machines:  make(map[string]models.Machiner),
				Azure: &models.AzureConfig{
					ResourceGroupName: "test-rg-name",
				},
			}

			for _, location := range tt.locations {
				for i := 0; i < tt.machinesPerLocation; i++ {
					machineName := fmt.Sprintf("machine-%s-%d", location, i)
					deployment.SetMachine(machineName, &models.Machine{
						Name:              machineName,
						Location:          location,
						SSHPublicKeyPath:  suite.testSSHPublicKeyPath,
						SSHPrivateKeyPath: suite.testPrivateKeyPath,
					})
				}
			}

			// Set up a new mock Azure client for each test case
			suite.mockAzureClient = new(azure_mocks.MockAzureClienter)
			suite.azureProvider.SetAzureClient(suite.mockAzureClient)

			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}

			// Set up expectations for this specific test case
			for machineName, err := range tt.deployMachineErrors {
				suite.mockAzureClient.On("DeployTemplate",
					mock.Anything,
					mock.Anything,
					mock.MatchedBy(func(deploymentName string) bool {
						return strings.Contains(deploymentName, machineName)
					}),
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
				).Return(mockPoller, nil)
				suite.mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).
					Return(testdata.FakeVirtualMachine(), nil)
				suite.mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
					Return(testdata.FakeNetworkInterface(), nil)
				suite.mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
					Return(testdata.FakePublicIPAddress("20.30.40.50"), nil)

				suite.mockSSHConfig.On("WaitForSSH",
					mock.Anything,
					mock.Anything,
					mock.Anything).Return(nil)

			}

			err := suite.azureProvider.CreateResources(suite.ctx)

			if tt.expectedError != "" {
				suite.Error(err)
				suite.Contains(err.Error(), tt.expectedError)
			} else {
				suite.NoError(err)
			}

			// Assert that all expectations were met
			suite.mockAzureClient.AssertExpectations(suite.T())
		})
	}
}

func TestPkgProvidersAzureCreateResourceSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersAzureCreateResourceTestSuite))
}
