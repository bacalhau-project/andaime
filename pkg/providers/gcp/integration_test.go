package gcp_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/providers/gcp"

	"github.com/bacalhau-project/andaime/internal/clouds/general"
	"github.com/bacalhau-project/andaime/internal/testdata"
	gcp_mocks "github.com/bacalhau-project/andaime/mocks/gcp"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

var localCustomScriptPath string
var localCustomScriptContent []byte

type PkgProvidersGCPIntegrationTest struct {
	suite.Suite
	provider               *gcp.GCPProvider
	clusterDeployer        *common.ClusterDeployer
	origGetGlobalModelFunc func() *display.DisplayModel
	testDisplayModel       *display.DisplayModel
	mockGCPClient          *gcp_mocks.MockGCPClienter
	mockSSHConfig          *sshutils.MockSSHConfig
	cleanup                func()
}

func (s *PkgProvidersGCPIntegrationTest) SetupSuite() {
	tempConfigFile, err := os.CreateTemp("", "config*.yaml")
	s.Require().NoError(err)

	testConfig, err := testdata.ReadTestGCPConfig()
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

	viper.Set("gcp.project_id", "test-project-id")
	viper.Set("gcp.region", "us-central1")
	viper.Set("general.ssh_user", "testuser")
	viper.Set("general.ssh_port", 22)
	viper.Set("gcp.default_disk_size_gb", 30)

	s.mockGCPClient = new(gcp_mocks.MockGCPClienter)
	s.provider = &gcp.GCPProvider{
		Client: s.mockGCPClient,
	}

	s.clusterDeployer = common.NewClusterDeployer(models.DeploymentTypeGCP)

	s.cleanup = func() {
		_ = os.Remove(tempConfigFile.Name())
		_ = os.Remove(localCustomScriptPath)
		viper.Reset()
	}
}

func (s *PkgProvidersGCPIntegrationTest) TearDownSuite() {
	s.cleanup()
}

func (s *PkgProvidersGCPIntegrationTest) SetupTest() {
	s.mockGCPClient.On("EnsureProject", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return("test-project-id", nil)
	s.mockGCPClient.On("EnableAPI", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.mockGCPClient.On("CreateResources", mock.Anything).Return(nil)
	s.mockGCPClient.On("CreateVPCNetwork", mock.Anything, mock.Anything).Return(nil)
	s.mockGCPClient.On("CreateIP", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	s.mockGCPClient.On("CreateFirewallRules", mock.Anything, mock.Anything).Return(nil)
	s.mockGCPClient.On("CreateVM", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeGCPInstance(), nil)
	s.mockGCPClient.On("GetInstance", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeGCPInstance(), nil)
	s.mockGCPClient.On("GetOperation", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeGCPOperation(), nil)
	s.mockGCPClient.On("GetMachineType", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeGCPMachineType(), nil)
	s.mockGCPClient.On("GetNetwork", mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeGCPNetwork(), nil)
	s.mockGCPClient.On("GetSubnetwork", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(testdata.FakeGCPSubnetwork(), nil)

	deployment, err := models.NewDeployment()
	deployment.SetProjectID("test-project-id")
	s.Require().NoError(err)
	m := display.NewDisplayModel(deployment)

	machines := []struct {
		name         string
		location     string
		orchestrator bool
	}{
		{"orchestrator", "us-central1-a", true},
		{"worker1", "us-central1-b", false},
		{"worker2", "us-central1-c", false},
	}

	for _, machine := range machines {
		m, err := models.NewMachine(
			models.DeploymentTypeGCP,
			machine.location,
			"n1-standard-2",
			30,
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

	m.Deployment.GCP.DefaultRegion = "us-central1"
	m.Deployment.Locations = []string{"us-central1-a", "us-central1-b", "us-central1-c"}
	m.Deployment.SSHPublicKeyMaterial = "PUBLIC KEY MATERIAL"
	m.Deployment.CustomScriptPath = localCustomScriptPath
	bacalhauSettings, err := models.ReadBacalhauSettingsFromViper()
	s.Require().NoError(err)
	m.Deployment.BacalhauSettings = bacalhauSettings

	s.testDisplayModel = m

	display.SetGlobalModel(m)

	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{
				Dst:              "/tmp/get-node-config-metadata.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-docker.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-core-packages.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-bacalhau.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/install-run-bacalhau.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				Times:            3,
			},
			{
				Dst:              "/tmp/custom_script.sh",
				Executable:       true,
				ProgressCallback: mock.Anything,
				Error:            nil,
				FileContents:     localCustomScriptContent,
				Times:            3,
			},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:              "sudo /tmp/get-node-config-metadata.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-docker.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo docker run hello-world",
				ProgressCallback: mock.Anything,
				Output:           "Hello from Docker!",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-core-packages.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-bacalhau.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo /tmp/install-run-bacalhau.sh",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo bash /tmp/custom_script.sh | sudo tee /var/log/andaime-custom-script.log",
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "bacalhau node list --output json --api-host 0.0.0.0",
				ProgressCallback: mock.Anything,
				Output:           `[{"id": "node1"}]`,
				Error:            nil,
				Times:            1,
			},
			{
				Cmd:              "bacalhau node list --output json --api-host 35.200.100.100",
				ProgressCallback: mock.Anything,
				Output:           `[{"id": "node1"}]`,
				Error:            nil,
				Times:            2,
			},
			{
				Cmd:              `sudo bacalhau config set 'compute.allowlistedlocalpaths' '/tmp,/data'`,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'compute.heartbeat.interval' '15s'`,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'compute.heartbeat.infoupdateinterval' '16s'`,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'compute.heartbeat.resourceupdateinterval' '17s'`,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'orchestrator.nodemanager.disconnecttimeout' '18s'`,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              `sudo bacalhau config set 'jobadmissioncontrol.acceptnetworkedjobs' 'true'`,
				ProgressCallback: mock.Anything,
				Output:           "",
				Error:            nil,
				Times:            3,
			},
			{
				Cmd:              "sudo bacalhau config list --output json",
				ProgressCallback: mock.Anything,
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

	s.mockSSHConfig = sshutils.NewMockSSHConfigWithBehavior(sshBehavior)
	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return s.mockSSHConfig, nil
	}
}

func (s *PkgProvidersGCPIntegrationTest) TestProvisionResourcesSuccess() {
	l := logger.Get()
	l.Info("Starting TestProvisionResourcesSuccess")

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	s.mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil)

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
	for _, machine := range m.Deployment.Machines {
		err := s.provider.GetClusterDeployer().ProvisionPackagesOnMachine(ctx, machine.GetName())
		s.Require().NoError(err)
	}

	err = s.provider.GetClusterDeployer().ProvisionOrchestrator(ctx, "orchestrator")
	s.Require().NoError(err)

	for _, machine := range m.Deployment.Machines {
		if !machine.IsOrchestrator() {
			err := s.provider.GetClusterDeployer().ProvisionWorker(ctx, machine.GetName())
			s.Require().NoError(err)
		}
	}

	for _, machine := range m.Deployment.Machines {
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

func TestGCPIntegrationSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersGCPIntegrationTest))
}
