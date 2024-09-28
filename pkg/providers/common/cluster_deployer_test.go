package common

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type PkgProvidersCommonClusterDeployerTestSuite struct {
	suite.Suite
	ctx                context.Context
	clusterDeployer    *ClusterDeployer
	testPrivateKeyPath string
	validConfigOutput  string
	cleanup            func()
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) SetupSuite() {
	s.ctx = context.Background()
	_, cleanupPublicKey, testPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	s.cleanup = func() {
		cleanupPublicKey()
		cleanupPrivateKey()
	}
	s.testPrivateKeyPath = testPrivateKeyPath

	var configList []map[string]interface{}
	err := json.Unmarshal([]byte(fullValidConfigOutput), &configList)
	if err != nil {
		assert.NoError(s.T(), err)
	}

	// Convert configList to a JSON string
	configJSON, err := json.Marshal(configList)
	if err != nil {
		s.T().Fatalf("Failed to marshal configList: %v", err)
	}
	s.validConfigOutput = string(configJSON)
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) TearDownSuite() {
	s.cleanup()
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) SetupTest() {
	s.clusterDeployer = NewClusterDeployer(models.DeploymentTypeUnknown)
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) TestProvisionAllMachinesWithPackages() {
	tests := []struct {
		name          string
		sshBehavior   sshutils.ExpectedSSHBehavior
		expectedError string
	}{
		{
			name: "successful provisioning",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{Dst: "/tmp/install-docker.sh", Executable: true, Error: nil, Times: 1},
					{Dst: "/tmp/install-core-packages.sh", Executable: true, Error: nil, Times: 1},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{Cmd: "sudo /tmp/install-docker.sh", Error: nil, Times: 1},
					{
						Cmd:    "sudo docker run hello-world",
						Error:  nil,
						Times:  1,
						Output: "Hello from Docker!",
					},
					{Cmd: "sudo /tmp/install-core-packages.sh", Error: nil, Times: 1},
				},
			},
			expectedError: "",
		},
		{
			name: "SSH error",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        "/tmp/install-docker.sh",
						Executable: true,
						Error:      errors.New("failed to push file"),
						Times:      1,
					},
				},
			},
			expectedError: "failed to provision packages on all machines",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
				return sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior), nil
			}

			viper.Set("general.project_prefix", "test-project-prefix")
			deployment, err := models.NewDeployment()
			s.Require().NoError(err)
			deployment.SetProjectID("test-project-id")
			deployment.SetMachines(map[string]models.Machiner{
				"machine1": NewMockMachine("machine1", s.testPrivateKeyPath),
			})

			m := display.GetGlobalModelFunc()
			m.Deployment = deployment

			err = s.clusterDeployer.ProvisionAllMachinesWithPackages(s.ctx)

			if tt.expectedError != "" {
				s.Error(err)
				s.Contains(err.Error(), tt.expectedError)
			} else {
				s.NoError(err)
			}
		})
	}
}

func (s *PkgProvidersCommonClusterDeployerTestSuite) TestProvisionBacalhauCluster() {
	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{Dst: mock.Anything, Executable: true, Error: nil, Times: 3},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:    "sudo /tmp/get-node-config-metadata.sh",
				Output: "",
				Error:  nil,
				Times:  2,
			},
			{
				Cmd:    "sudo /tmp/install-bacalhau.sh",
				Output: "",
				Error:  nil,
				Times:  2,
			},
			{
				Cmd:    "sudo /tmp/install-run-bacalhau.sh",
				Output: "",
				Error:  nil,
				Times:  2,
			},
			{
				Cmd:    "bacalhau node list --output json --api-host 0.0.0.0",
				Output: `[{"name":"orch","address":"1.1.1.1"}]`,
				Error:  nil,
				Times:  1,
			},
			{
				Cmd:    "bacalhau node list --output json --api-host 1.1.1.1",
				Output: `[{"name":"orch","address":"1.1.1.1"},{"name":"worker","address":"2.2.2.2"}]`,
				Error:  nil,
				Times:  1,
			},
			{
				Cmd:    "sudo bacalhau config list --output json",
				Output: s.validConfigOutput,
				Error:  nil,
				Times:  2,
			},
		},
		InstallSystemdServiceExpectation: &sshutils.Expectation{Error: nil, Times: 2},
		RestartServiceExpectation:        &sshutils.Expectation{Error: nil, Times: 4},
	}

	sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
		return sshutils.NewMockSSHConfigWithBehavior(sshBehavior), nil
	}

	m := display.GetGlobalModelFunc()
	m.Deployment = &models.Deployment{
		Machines: map[string]models.Machiner{
			"orch":   NewMockMachine("orch", s.testPrivateKeyPath),
			"worker": NewMockMachine("worker", s.testPrivateKeyPath),
		},
	}
	m.Deployment.Machines["orch"].(*MockMachine).Orchestrator = true
	m.Deployment.Machines["orch"].(*MockMachine).PublicIP = "1.1.1.1"
	m.Deployment.Machines["worker"].(*MockMachine).PublicIP = "2.2.2.2"

	err := s.clusterDeployer.ProvisionBacalhauCluster(s.ctx)

	s.NoError(err)
	s.Equal("1.1.1.1", m.Deployment.OrchestratorIP)
}

func TestPkgProvidersCommonClusterDeployerTestSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersCommonClusterDeployerTestSuite))
}

type MockMachine struct {
	mock.Mock
	*models.Machine
}

func NewMockMachine(name string, privateKeyPath string) *MockMachine {
	m := &MockMachine{
		Machine: &models.Machine{
			Name:              name,
			SSHUser:           "test-user",
			SSHPrivateKeyPath: privateKeyPath,
			SSHPort:           22,
			PublicIP:          "1.1.1.1",
			Orchestrator:      false,
		},
	}
	m.On("SetServiceState", mock.Anything, mock.Anything).Return()
	m.On("SetComplete").Return()
	return m
}

func (m *MockMachine) SetServiceState(serviceName string, state models.ServiceState) {
	m.Called(serviceName, state)
}

func (m *MockMachine) SetComplete() {
	m.Called()
}

func TestExecuteCustomScript(t *testing.T) {
	tests := []struct {
		name             string
		customScriptPath string
		sshBehavior      sshutils.ExpectedSSHBehavior
		expectedError    string
	}{
		{
			name:             "Successful script execution",
			customScriptPath: "/tmp/custom_script.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        "/tmp/custom_script.sh",
						Executable: true,
						Error:      nil,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    "sudo bash /tmp/custom_script.sh | sudo tee /var/log/andaime-custom-script.log",
						Output: "Script output",
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name:             "Script execution failure",
			customScriptPath: "/path/to/valid/script_1.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        "/tmp/custom_script.sh",
						Executable: true,
						Error:      nil,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    "sudo bash /tmp/custom_script.sh | sudo tee /var/log/andaime-custom-script.log",
						Output: "",
						Error:  errors.New("script execution failed"),
					},
				},
			},
			expectedError: "failed to execute custom script: script execution failed",
		},
		{
			name:             "Script execution timeout",
			customScriptPath: "/path/to/valid/script_2.sh",
			sshBehavior: sshutils.ExpectedSSHBehavior{
				PushFileExpectations: []sshutils.PushFileExpectation{
					{
						Dst:        "/tmp/custom_script.sh",
						Executable: true,
						Error:      nil,
					},
				},
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    "sudo bash /tmp/custom_script.sh | sudo tee /var/log/andaime-custom-script.log",
						Output: "",
						Error:  context.DeadlineExceeded,
					},
				},
			},
			expectedError: "failed to execute custom script: context deadline exceeded",
		},
		{
			name:             "Empty custom script path",
			customScriptPath: "",
			sshBehavior:      sshutils.ExpectedSSHBehavior{},
			expectedError:    "",
		},
		{
			name:             "Non-existent custom script",
			customScriptPath: "/path/to/non-existent/script.sh",
			sshBehavior:      sshutils.ExpectedSSHBehavior{},
			expectedError:    "failed to read custom script: open /path/to/non-existent/script.sh: no such file or directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary file for the custom script if it's supposed to exist
			if tt.customScriptPath != "" && !strings.Contains(tt.name, "Non-existent") {
				tmpfile, err := os.CreateTemp("", "testscript")
				if err != nil {
					t.Fatal(err)
				}
				defer os.Remove(tmpfile.Name())
				tt.customScriptPath = tmpfile.Name()
			}

			viper.Set("general.project_prefix", "test-project-prefix")
			deployment, err := models.NewDeployment()
			if err != nil {
				t.Fatal(err)
			}
			deployment.CustomScriptPath = tt.customScriptPath
			deployment.SetProjectID("test-project")
			origGetGlobalModelFunc := display.GetGlobalModelFunc
			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}
			defer func() { display.GetGlobalModelFunc = origGetGlobalModelFunc }()

			mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior)

			mockMachine := &MockMachine{
				Machine: &models.Machine{
					Name:     "test-machine",
					PublicIP: "1.2.3.4",
				},
			}
			mockMachine.On("SetServiceState", mock.Anything, mock.Anything).Return()
			mockMachine.On("SetComplete").Return()

			cd := NewClusterDeployer(models.DeploymentTypeAzure)

			err = cd.ExecuteCustomScript(
				context.Background(),
				mockSSHConfig,
				mockMachine,
			)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestApplyBacalhauConfigs(t *testing.T) {
	type testCase struct {
		name             string
		bacalhauSettings map[string]string
		sshBehavior      sshutils.ExpectedSSHBehavior
		expectedError    string
	}

	tests := []testCase{
		{
			name:             "Empty configuration",
			bacalhauSettings: map[string]string{},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: "[]",
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Single configuration",
			bacalhauSettings: map[string]string{
				"node.allowlistedlocalpaths": `"/tmp","/data"`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'node.allowlistedlocalpaths' '"/tmp","/data"'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[{"Key":"'node.allowlistedlocalpaths'","Value":["/tmp","/data"]}]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Multiple configurations",
			bacalhauSettings: map[string]string{
				"node.allowlistedlocalpaths":                            `"/tmp","/data"`,
				"node.compute.controlplanesettings.infoupdatefrequency": "5s",
				"node.compute.jobselection.acceptnetworkedjobs":         "true",
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'node.allowlistedlocalpaths' '"/tmp","/data"'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    `sudo bacalhau config set 'node.compute.controlplanesettings.infoupdatefrequency' '5s'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    `sudo bacalhau config set 'node.compute.jobselection.acceptnetworkedjobs' 'true'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd: "sudo bacalhau config list --output json",
						Output: `[{"Key":"'node.allowlistedlocalpaths'","Value":["/tmp","/data"]},
{"Key":"'orchestrator.nodemanager.disconnecttimeout'","Value":"5s"},
{"Key":"'node.compute.jobselection.acceptnetworkedjobs'","Value":true}]`,
						Error: nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Configuration with error",
			bacalhauSettings: map[string]string{
				"invalid.config": "value",
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'invalid.config' 'value'`,
						Output: "",
						Error:  errors.New("invalid bacalhau_settings keys: invalid.config"),
					},
				},
			},
			expectedError: "invalid bacalhau_settings keys: invalid.config",
		},
		{
			name: "Unexpected value in configuration",
			bacalhauSettings: map[string]string{
				"node.allowlistedlocalpaths": `"/tmp","/data"`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'node.allowlistedlocalpaths' '"/tmp","/data"'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[{"Key":"'node.allowlistedlocalpaths'","Value":["/tmp","/data"]}]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Missing configuration in final list",
			bacalhauSettings: map[string]string{
				"node.allowlistedlocalpaths": `"/tmp","/data"`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'node.allowlistedlocalpaths' '"/tmp","/data"'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set up Viper configuration
			defer viper.Reset()
			viper.Set("general.project_prefix", "test-project")
			viper.Set("general.bacalhau_settings", tt.bacalhauSettings)

			bacalhauSettings, err := models.ReadBacalhauSettingsFromViper()
			if err != nil && tt.expectedError != "" {
				assert.Contains(t, err.Error(), tt.expectedError)
				return
			} else {
				assert.NoError(t, err)
			}

			deployment := &models.Deployment{
				BacalhauSettings: bacalhauSettings,
			}

			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}

			mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior)

			cd := NewClusterDeployer(models.DeploymentTypeAzure)

			err = cd.ApplyBacalhauConfigs(context.Background(), mockSSHConfig)

			if tt.expectedError != "" {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedError)
			} else {
				assert.NoError(t, err)
			}

			// Verify that all expected commands were executed
			mockSSHConfig.AssertExpectations(t)
		})
	}
}

const fullValidConfigOutput = `
[
  {
    "Key": "node.serverapi.clienttls.insecure",
    "Value": false
  },
  {
    "Key": "node.compute.localpublisher.port",
    "Value": 6001
  },
  {
    "Key": "node.compute.capacity.ignorephysicalresourcelimits",
    "Value": false
  },
  {
    "Key": "node.compute.jobtimeouts.minjobexecutiontimeout",
    "Value": 500000000
  },
  {
    "Key": "node.requester.worker.workerevaldequeuemaxbackoff",
    "Value": 30000000000
  },
  {
    "Key": "node.requester.evaluationbroker.evalbrokermaxretrycount",
    "Value": 10
  },
  {
    "Key": "node.compute.executionstore.path",
    "Value": "/root/.bacalhau/compute_store/executions.db"
  },
  {
    "Key": "update.checkstatepath",
    "Value": "/root/.bacalhau/update.json"
  },
  {
    "Key": "node.clientapi.clienttls.usetls",
    "Value": false
  },
  {
    "Key": "node.clientapi.clienttls.insecure",
    "Value": false
  },
  {
    "Key": "node.computestoragepath",
    "Value": "/root/.bacalhau/executor_storages"
  },
  {
    "Key": "node.compute.logstreamconfig.channelbuffersize",
    "Value": 10
  },
  {
    "Key": "node.compute.capacity.totalresourcelimits.disk",
    "Value": ""
  },
  {
    "Key": "node.requester.worker.workerevaldequeuetimeout",
    "Value": 5000000000
  },
  {
    "Key": "node.allowlistedlocalpaths",
    "Value": [
      "/tmp,/data"
    ]
  },
  {
    "Key": "node.requester.jobselectionpolicy.probehttp",
    "Value": ""
  },
  {
    "Key": "node.clientapi.host",
    "Value": "bootstrap.production.bacalhau.org"
  },
  {
    "Key": "node.serverapi.port",
    "Value": 1234
  },
  {
    "Key": "node.compute.localpublisher.address",
    "Value": "public"
  },
  {
    "Key": "node.compute.capacity.jobresourcelimits.cpu",
    "Value": ""
  },
  {
    "Key": "node.requester.jobstore.path",
    "Value": "/root/.bacalhau/orchestrator_store/jobs.db"
  },
  {
    "Key": "auth.tokenspath",
    "Value": "/root/.bacalhau/tokens.json"
  },
  {
    "Key": "node.serverapi.clienttls.usetls",
    "Value": false
  },
  {
    "Key": "node.compute.logging.logrunningexecutionsinterval",
    "Value": 10000000000
  },
  {
    "Key": "node.compute.manifestcache.frequency",
    "Value": 3600000000000
  },
  {
    "Key": "node.name",
    "Value": "n-c5f1f4b7-8ad7-4446-8531-c1d24d3c249b"
  },
  {
    "Key": "node.compute.jobtimeouts.jobnegotiationtimeout",
    "Value": 180000000000
  },
  {
    "Key": "node.requester.externalverifierhook",
    "Value": ""
  },
  {
    "Key": "node.requester.tagcache.size",
    "Value": 0
  },
  {
    "Key": "node.webui.port",
    "Value": 8483
  },
  {
    "Key": "node.network.advertisedaddress",
    "Value": ""
  },
  {
    "Key": "node.compute.jobselection.probehttp",
    "Value": ""
  },
  {
    "Key": "node.disabledfeatures.engines",
    "Value": []
  },
  {
    "Key": "node.serverapi.tls.autocertcachepath",
    "Value": ""
  },
  {
    "Key": "node.compute.capacity.defaultjobresourcelimits.cpu",
    "Value": "500m"
  },
  {
    "Key": "node.requester.worker.workercount",
    "Value": 2
  },
  {
    "Key": "node.requester.translationenabled",
    "Value": false
  },
  {
    "Key": "node.labels",
    "Value": {}
  },
  {
    "Key": "node.compute.capacity.jobresourcelimits.disk",
    "Value": ""
  },
  {
    "Key": "node.compute.capacity.defaultjobresourcelimits.memory",
    "Value": "1Gb"
  },
  {
    "Key": "node.compute.jobtimeouts.defaultjobexecutiontimeout",
    "Value": 600000000000
  },
  {
    "Key": "node.requester.failureinjectionconfig.isbadactor",
    "Value": false
  },
  {
    "Key": "node.requester.controlplanesettings.nodedisconnectedafter",
    "Value": 5000000000
  },
  {
    "Key": "node.compute.capacity.totalresourcelimits.gpu",
    "Value": ""
  },
  {
    "Key": "node.compute.capacity.jobresourcelimits.memory",
    "Value": ""
  },
  {
    "Key": "node.requester.worker.workerevaldequeuebasebackoff",
    "Value": 1000000000
  },
  {
    "Key": "auth.accesspolicypath",
    "Value": ""
  },
  {
    "Key": "node.serverapi.tls.selfsigned",
    "Value": false
  },
  {
    "Key": "node.requester.scheduler.queuebackoff",
    "Value": 60000000000
  },
  {
    "Key": "node.network.cluster.port",
    "Value": 0
  },
  {
    "Key": "node.downloadurlrequestretries",
    "Value": 3
  },
  {
    "Key": "node.executorpluginpath",
    "Value": "/root/.bacalhau/plugins"
  },
  {
    "Key": "node.compute.executionstore.type",
    "Value": "BoltDB"
  },
  {
    "Key": "node.requester.evaluationbroker.evalbrokervisibilitytimeout",
    "Value": 60000000000
  },
  {
    "Key": "node.webui.enabled",
    "Value": false
  },
  {
    "Key": "node.serverapi.tls.serverkey",
    "Value": ""
  },
  {
    "Key": "node.compute.jobselection.acceptnetworkedjobs",
    "Value": true
  },
  {
    "Key": "node.requester.controlplanesettings.heartbeatcheckfrequency",
    "Value": 5000000000
  },
  {
    "Key": "node.disabledfeatures.publishers",
    "Value": []
  },
  {
    "Key": "node.ipfs.connect",
    "Value": ""
  },
  {
    "Key": "node.compute.capacity.totalresourcelimits.cpu",
    "Value": ""
  },
  {
    "Key": "node.compute.jobtimeouts.maxjobexecutiontimeout",
    "Value": 9223372036000000000
  },
  {
    "Key": "node.requester.jobdefaults.totaltimeout",
    "Value": 1800000000000
  },
  {
    "Key": "update.skipchecks",
    "Value": false
  },
  {
    "Key": "update.checkfrequency",
    "Value": 86400000000000
  },
  {
    "Key": "node.clientapi.port",
    "Value": 1234
  },
  {
    "Key": "node.volumesizerequesttimeout",
    "Value": 120000000000
  },
  {
    "Key": "node.requester.jobselectionpolicy.rejectstatelessjobs",
    "Value": false
  },
  {
    "Key": "node.requester.scheduler.nodeoversubscriptionfactor",
    "Value": 1.5
  },
  {
    "Key": "node.requester.controlplanesettings.heartbeattopic",
    "Value": "heartbeat"
  },
  {
    "Key": "node.compute.controlplanesettings.heartbeatfrequency",
    "Value": 5000000000
  },
  {
    "Key": "node.requester.noderankrandomnessrange",
    "Value": 5
  },
  {
    "Key": "node.network.storedir",
    "Value": "/root/.bacalhau/orchestrator_store/nats-store"
  },
  {
    "Key": "node.network.cluster.peers",
    "Value": null
  },
  {
    "Key": "node.requester.jobselectionpolicy.acceptnetworkedjobs",
    "Value": true
  },
  {
    "Key": "node.requester.jobstore.type",
    "Value": "BoltDB"
  },
  {
    "Key": "auth.methods",
    "Value": {
      "ClientKey": {
        "Type": "challenge",
        "PolicyPath": ""
      }
    }
  },
  {
    "Key": "node.network.port",
    "Value": 4222
  },
  {
    "Key": "node.clientapi.tls.serverkey",
    "Value": ""
  },
  {
    "Key": "node.clientapi.tls.autocert",
    "Value": ""
  },
  {
    "Key": "node.compute.controlplanesettings.infoupdatefrequency",
    "Value": 60000000000
  },
  {
    "Key": "node.compute.capacity.defaultjobresourcelimits.disk",
    "Value": ""
  },
  {
    "Key": "node.serverapi.host",
    "Value": "0.0.0.0"
  },
  {
    "Key": "node.serverapi.clienttls.cacert",
    "Value": ""
  },
  {
    "Key": "node.compute.manifestcache.duration",
    "Value": 3600000000000
  },
  {
    "Key": "node.requester.manualnodeapproval",
    "Value": false
  },
  {
    "Key": "node.requester.storageprovider.s3.presignedurlexpiration",
    "Value": 1800000000000
  },
  {
    "Key": "node.clientapi.tls.servercertificate",
    "Value": ""
  },
  {
    "Key": "node.compute.capacity.jobresourcelimits.gpu",
    "Value": ""
  },
  {
    "Key": "node.network.orchestrators",
    "Value": null
  },
  {
    "Key": "node.network.cluster.name",
    "Value": ""
  },
  {
    "Key": "node.nameprovider",
    "Value": "puuid"
  },
  {
    "Key": "node.clientapi.tls.selfsigned",
    "Value": false
  },
  {
    "Key": "node.strictversionmatch",
    "Value": false
  },
  {
    "Key": "node.requester.tagcache.frequency",
    "Value": 0
  },
  {
    "Key": "node.loggingmode",
    "Value": "default"
  },
  {
    "Key": "node.disabledfeatures.storages",
    "Value": []
  },
  {
    "Key": "node.compute.controlplanesettings.heartbeattopic",
    "Value": "heartbeat"
  },
  {
    "Key": "node.requester.defaultpublisher",
    "Value": ""
  },
  {
    "Key": "node.requester.jobdefaults.queuetimeout",
    "Value": 0
  },
  {
    "Key": "node.clientapi.clienttls.cacert",
    "Value": ""
  },
  {
    "Key": "node.compute.capacity.totalresourcelimits.memory",
    "Value": ""
  },
  {
    "Key": "node.requester.tagcache.duration",
    "Value": 0
  },
  {
    "Key": "node.compute.jobselection.rejectstatelessjobs",
    "Value": false
  },
  {
    "Key": "node.compute.jobtimeouts.jobexecutiontimeoutclientidbypasslist",
    "Value": []
  },
  {
    "Key": "node.requester.evaluationbroker.evalbrokerinitialretrydelay",
    "Value": 1000000000
  },
  {
    "Key": "node.network.authsecret",
    "Value": ""
  },
  {
    "Key": "node.requester.overaskforbidsfactor",
    "Value": 3
  },
  {
    "Key": "user.installationid",
    "Value": "BACA14A0-eeee-eeee-eeee-194519911992"
  },
  {
    "Key": "node.requester.jobdefaults.executiontimeout",
    "Value": 0
  },
  {
    "Key": "node.compute.jobselection.probeexec",
    "Value": ""
  },
  {
    "Key": "node.compute.jobselection.locality",
    "Value": 1
  },
  {
    "Key": "node.compute.manifestcache.size",
    "Value": 1000
  },
  {
    "Key": "node.requester.jobselectionpolicy.locality",
    "Value": 1
  },
  {
    "Key": "node.requester.evaluationbroker.evalbrokersubsequentretrydelay",
    "Value": 30000000000
  },
  {
    "Key": "node.requester.storageprovider.s3.presignedurldisabled",
    "Value": false
  },
  {
    "Key": "node.requester.nodeinfostorettl",
    "Value": 600000000000
  },
  {
    "Key": "node.clientapi.tls.autocertcachepath",
    "Value": "/root/.bacalhau/autocert-cache"
  },
  {
    "Key": "node.requester.jobselectionpolicy.probeexec",
    "Value": ""
  },
  {
    "Key": "user.keypath",
    "Value": "/root/.bacalhau/user_id.pem"
  },
  {
    "Key": "node.type",
    "Value": [
      "requester"
    ]
  },
  {
    "Key": "node.serverapi.tls.autocert",
    "Value": ""
  },
  {
    "Key": "node.compute.capacity.defaultjobresourcelimits.gpu",
    "Value": ""
  },
  {
    "Key": "node.requester.housekeepingbackgroundtaskinterval",
    "Value": 30000000000
  },
  {
    "Key": "metrics.eventtracerpath",
    "Value": "/dev/null"
  },
  {
    "Key": "node.network.cluster.advertisedaddress",
    "Value": ""
  },
  {
    "Key": "node.serverapi.tls.servercertificate",
    "Value": ""
  },
  {
    "Key": "node.downloadurlrequesttimeout",
    "Value": 300000000000
  },
  {
    "Key": "node.compute.localpublisher.directory",
    "Value": ""
  },
  {
    "Key": "node.compute.controlplanesettings.resourceupdatefrequency",
    "Value": 30000000000
  }
]`
