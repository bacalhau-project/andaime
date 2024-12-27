package azure

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/runtime"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/internal/testdata"
	azure_mocks "github.com/bacalhau-project/andaime/mocks/azure"
	ssh_mocks "github.com/bacalhau-project/andaime/mocks/sshutils"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	sshutils_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/sshutils"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var localCustomScriptPath string
var localCustomScriptContent []byte

type PkgProvidersAzureIntegrationTest struct {
	suite.Suite
	provider               *AzureProvider
	clusterDeployer        *common.ClusterDeployer
	origGetGlobalModelFunc func() *display.DisplayModel
	testDisplayModel       *display.DisplayModel
	mockAzureClient        *azure_mocks.MockAzureClienter
	mockSSHConfig          *ssh_mocks.MockSSHConfiger
	cleanup                func()
}

func (s *PkgProvidersAzureIntegrationTest) SetupSuite() {
	tempConfigFile, err := os.CreateTemp("", "config*.yaml")
	s.Require().NoError(err)

	testConfig, err := testdata.ReadTestAzureConfig()
	s.Require().NoError(err)

	f, err := os.CreateTemp("", "local_custom_script*.sh")
	s.Require().NoError(err)

	localCustomScriptContent, err = general.GetLocalCustomScript()
	s.Require().NoError(err)

	_, err = f.Write(localCustomScriptContent)
	s.Require().NoError(err)

	_, err = tempConfigFile.Write([]byte(testConfig))
	s.Require().NoError(err)

	localCustomScriptPath = f.Name()

	viper.SetConfigFile(tempConfigFile.Name())
	err = viper.ReadInConfig()
	s.Require().NoError(err)

	viper.Set("azure.subscription_id", "test-subscription-id")
	viper.Set("azure.resource_group_location", "eastus")
	viper.Set("general.ssh_user", "testuser")
	viper.Set("general.ssh_port", 22)
	viper.Set("general.custom_script_path", localCustomScriptPath)
	viper.Set("azure.default_disk_size_gb", 30)

	s.mockAzureClient = new(azure_mocks.MockAzureClienter)
	s.provider = &AzureProvider{
		Client: s.mockAzureClient,
	}

	s.clusterDeployer = common.NewClusterDeployer(models.DeploymentTypeAzure)

	s.cleanup = func() {
		_ = os.Remove(tempConfigFile.Name())
		_ = os.Remove(localCustomScriptPath)
		viper.Reset()
	}
}

func (s *PkgProvidersAzureIntegrationTest) TearDownSuite() {
	s.cleanup()
}

func (s *PkgProvidersAzureIntegrationTest) SetupTest() {
	mockPoller := new(MockPoller)
	mockPoller.On("PollUntilDone", mock.Anything, mock.Anything).
		Return(armresources.DeploymentsClientCreateOrUpdateResponse{
			DeploymentExtended: testdata.FakeDeployment(),
		}, nil)

	s.mockAzureClient.On("DeployTemplate",
		mock.Anything,
		mock.Anything,
		mock.AnythingOfType("string"),
		mock.AnythingOfType("map[string]interface {}"),
		mock.AnythingOfType("map[string]interface {}"),
		mock.AnythingOfType("map[string]*string"),
	).Return(mockPoller, nil).Maybe()

	s.mockAzureClient.On("GetVirtualMachine", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeVirtualMachine(), nil)

	s.mockAzureClient.On("GetNetworkInterface", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeNetworkInterface(), nil)
	s.mockAzureClient.On("GetPublicIPAddress", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakePublicIPAddress("20.30.40.50"), nil)

	deployment, err := models.NewDeployment()
	s.Require().NoError(err)
	m := display.NewDisplayModel(deployment)

	machines := []struct {
		name         string
		location     string
		region       string
		zone         string
		orchestrator bool
	}{
		{"orchestrator", "eastus", "eastus", "eastus-1", true},
		{"worker1", "eastus2", "eastus2", "eastus2-1", false},
		{"worker2", "westus", "westus", "westus-1", false},
	}

	for _, machine := range machines {
		m, err := models.NewMachine(
			models.DeploymentTypeAzure,
			machine.location,
			"Standard_D2s_v3",
			30,
			machine.region,
			machine.zone,
			models.CloudSpecificInfo{},
		)
		s.Require().NoError(err)
		m.SetName(machine.name)
		m.SetOrchestrator(machine.orchestrator)
		m.SetServiceState(models.ServiceTypeSSH.Name, models.ServiceStateNotStarted)
		m.SetServiceState(models.ServiceTypeDocker.Name, models.ServiceStateNotStarted)
		m.SetServiceState(models.ServiceTypeBacalhau.Name, models.ServiceStateNotStarted)
		deployment.SetMachine(machine.name, m)
	}

	m.Deployment.Azure.ResourceGroupLocation = "eastus"
	m.Deployment.Locations = []string{"eastus", "eastus2", "westus"}
	m.Deployment.SSHPublicKeyMaterial = "PUBLIC KEY MATERIAL"

	bacalhauSettings, err := models.ReadBacalhauSettingsFromViper()
	s.Require().NoError(err)
	m.Deployment.BacalhauSettings = bacalhauSettings

	m.Deployment.CustomScriptPath = localCustomScriptPath

	s.testDisplayModel = m

	display.SetGlobalModel(m)

	sshBehavior := sshutils.ExpectedSSHBehavior{
		WaitForSSHCount: 3,
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              "/tmp/get-node-config-metadata.sh",
				Executable:       true,
				ProgressCallback: func(int64, int64) {},
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-docker.sh",
				Executable:       true,
				ProgressCallback: func(int64, int64) {},
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-core-packages.sh",
				Executable:       true,
				ProgressCallback: func(int64, int64) {},
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-bacalhau.sh",
				Executable:       true,
				ProgressCallback: func(int64, int64) {},
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-run-bacalhau.sh",
				Executable:       true,
				ProgressCallback: func(int64, int64) {},
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/custom_script.sh",
				Executable:       true,
				ProgressCallback: func(int64, int64) {},
				Error:            nil,
				Times:            3,
			},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:              "sudo /tmp/get-node-config-metadata.sh",
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-docker.sh",
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              models.ExpectedDockerHelloWorldCommand,
				ProgressCallback: func(int64, int64) {},
				Output:           models.ExpectedDockerOutput,
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-core-packages.sh",
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-bacalhau.sh",
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-run-bacalhau.sh",
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo bash /tmp/custom_script.sh | sudo tee /var/log/andaime-custom-script.log",
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "bacalhau node list --output json --api-host 0.0.0.0",
				ProgressCallback: func(int64, int64) {},
				Output:           `[{"id": "node1"}]`,
				Error:            nil,
				Times:            1,
			},
			{
				Cmd:              "bacalhau node list --output json --api-host 20.30.40.50",
				ProgressCallback: func(int64, int64) {},
				Output:           `[{"id": "node1"}]`,
				Error:            nil,
				Times:            2,
			},
			{
				Cmd:              `sudo bacalhau config set 'compute.allowlistedlocalpaths'='/tmp,/data:rw'`,
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'compute.heartbeat.interval'='5s'`,
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'compute.heartbeat.infoupdateinterval'='6s'`,
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'compute.heartbeat.resourceupdateinterval'='7s'`,
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'orchestrator.nodemanager.disconnecttimeout'='8s'`,
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'jobadmissioncontrol.acceptnetworkedjobs'='true'`,
				ProgressCallback: func(int64, int64) {},
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo bacalhau config list --output json",
				ProgressCallback: func(int64, int64) {},
				Output:           "[]",
				Error:            nil,
				Times:            3,
			},
		},
		InstallSystemdServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 3,
		},
		RestartServiceExpectation: &sshutils.Expectation{
			Error: nil,
			Times: 6,
		},
	}

	s.mockSSHConfig = sshutils.NewMockSSHConfigWithBehavior(sshBehavior).(*ssh_mocks.MockSSHConfiger)
	sshutils.NewSSHConfigFunc = func(host string,
		port int,
		user string,
		sshPrivateKeyPath string) (sshutils_interface.SSHConfiger, error) {
		return s.mockSSHConfig, nil
	}

}

func (s *PkgProvidersAzureIntegrationTest) TestProvisionResourcesSuccess() {
	l := logger.Get()
	l.Info("Starting TestProvisionResourcesSuccess")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s.origGetGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return s.testDisplayModel
	}
	defer func() {
		display.GetGlobalModelFunc = s.origGetGlobalModelFunc
	}()

	err := s.provider.CreateResources(ctx)
	s.Require().NoError(err)

	s.provider.SetClusterDeployer(s.clusterDeployer)
	m := display.GetGlobalModelFunc()
	err = s.provider.GetClusterDeployer().ProvisionOrchestrator(ctx, "orchestrator")
	s.Require().NoError(err)

	for _, machine := range m.Deployment.GetMachines() {
		if !machine.IsOrchestrator() {
			err := s.provider.GetClusterDeployer().ProvisionWorker(ctx, machine.GetName())
			s.Require().NoError(err)
		}
	}

	for _, machine := range m.Deployment.GetMachines() {
		s.Equal(models.ServiceStateSucceeded, machine.GetServiceState(models.ServiceTypeSSH.Name))
		s.Equal(
			models.ServiceStateSucceeded,
			machine.GetServiceState(models.ServiceTypeDocker.Name),
		)
		s.Equal(
			models.ServiceStateSucceeded,
			machine.GetServiceState(models.ServiceTypeBacalhau.Name),
		)
		s.Equal(
			models.ServiceStateSucceeded,
			machine.GetServiceState(models.ServiceTypeScript.Name),
		)
	}

	s.mockSSHConfig.AssertExpectations(s.T())
}

func TestAzureIntegrationSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersAzureIntegrationTest))
}

type MockPoller struct {
	mock.Mock
}

func (m *MockPoller) PollUntilDone(
	ctx context.Context,
	options *runtime.PollUntilDoneOptions,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx, options)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPoller) ResumeToken() (string, error) {
	args := m.Called()
	return args.String(0), args.Error(1)
}

func (m *MockPoller) Result(
	ctx context.Context,
) (armresources.DeploymentsClientCreateOrUpdateResponse, error) {
	args := m.Called(ctx)
	return args.Get(0).(armresources.DeploymentsClientCreateOrUpdateResponse), args.Error(1)
}

func (m *MockPoller) Done() bool {
	args := m.Called()
	return args.Bool(0)
}

func (m *MockPoller) Poll(ctx context.Context) (*http.Response, error) {
	args := m.Called(ctx)
	return args.Get(0).(*http.Response), args.Error(1)
}
