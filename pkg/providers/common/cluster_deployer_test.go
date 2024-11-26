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

// func (s *PkgProvidersCommonClusterDeployerTestSuite) TestProvisionAllMachinesWithPackages() {
// 	tests := []struct {
// 		name          string
// 		sshBehavior   sshutils.ExpectedSSHBehavior
// 		expectedError string
// 	}{
// 		{
// 			name: "successful provisioning",
// 			sshBehavior: sshutils.ExpectedSSHBehavior{
// 				PushFileExpectations: []sshutils.PushFileExpectation{
// 					{Dst: "/tmp/install-docker.sh", Executable: true, Error: nil, Times: 1},
// 					{Dst: "/tmp/install-core-packages.sh", Executable: true, Error: nil, Times: 1},
// 				},
// 				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
// 					{Cmd: "sudo /tmp/install-docker.sh", Error: nil, Times: 1},
// 					{
// 						Cmd:    "sudo docker run hello-world",
// 						Error:  nil,
// 						Times:  1,
// 						Output: "Hello from Docker!",
// 					},
// 					{Cmd: "sudo /tmp/install-core-packages.sh", Error: nil, Times: 1},
// 				},
// 			},
// 			expectedError: "",
// 		},
// 		{
// 			name: "SSH error",
// 			sshBehavior: sshutils.ExpectedSSHBehavior{
// 				PushFileExpectations: []sshutils.PushFileExpectation{
// 					{
// 						Dst:        "/tmp/install-docker.sh",
// 						Executable: true,
// 						Error:      errors.New("failed to push file"),
// 						Times:      1,
// 					},
// 				},
// 			},
// 			expectedError: "failed to provision packages on all machines",
// 		},
// 	}

// 	for _, tt := range tests {
// 		s.Run(tt.name, func() {
// 			sshutils.NewSSHConfigFunc = func(host string, port int, user string, sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
// 				return sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior), nil
// 			}

// 			viper.Set("general.project_prefix", "test-project-prefix")
// 			deployment, err := models.NewDeployment()
// 			s.Require().NoError(err)
// 			deployment.SetProjectID("test-project-id")
// 			deployment.SetMachines(map[string]models.Machiner{
// 				"machine1": NewMockMachine("machine1", s.testPrivateKeyPath),
// 			})

// 			m := display.GetGlobalModelFunc()
// 			m.Deployment = deployment

// 			err = s.clusterDeployer.ProvisionAllMachinesWithPackages(s.ctx)

// 			if tt.expectedError != "" {
// 				s.Error(err)
// 				s.Contains(err.Error(), tt.expectedError)
// 			} else {
// 				s.NoError(err)
// 			}
// 		})
// 	}
// }

func (s *PkgProvidersCommonClusterDeployerTestSuite) TestProvisionBacalhauCluster() {
	sshBehavior := sshutils.ExpectedSSHBehavior{
		PushFileExpectations: []sshutils.PushFileExpectation{
			{Dst: mock.Anything, Executable: true, Error: nil, Times: 3},
		},
		ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
			{
				Cmd:    "sudo /tmp/install-docker.sh",
				Output: "",
				Error:  nil,
				Times:  2,
			},
			{
				Cmd:    "sudo docker run hello-world",
				Output: "Hello from Docker!",
				Error:  nil,
				Times:  2,
			},
			{
				Cmd:    "sudo /tmp/install-core-packages.sh",
				Output: "",
				Error:  nil,
				Times:  2,
			},
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

	viper.Set("general.project_prefix", "test-project-prefix")
	m := display.GetGlobalModelFunc()
	m.Deployment, _ = models.NewDeployment()
	m.Deployment.SetMachines(map[string]models.Machiner{
		"orch":   NewMockMachine("orch", s.testPrivateKeyPath),
		"worker": NewMockMachine("worker", s.testPrivateKeyPath),
	})
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
				"compute.allowlistedlocalpaths": `/tmp,/data:rw`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'compute.allowlistedlocalpaths'='/tmp,/data:rw'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[{"Key":"'compute.allowlistedlocalpaths'","Value":["/tmp","/data:rw"]}]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Multiple configurations",
			bacalhauSettings: map[string]string{
				"compute.allowlistedlocalpaths":           `/tmp,/data:rw`,
				"compute.heartbeat.interval":              "5s",
				"jobadmissioncontrol.acceptnetworkedjobs": "true",
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'compute.allowlistedlocalpaths'='/tmp,/data:rw'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    `sudo bacalhau config set 'compute.heartbeat.interval'='5s'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    `sudo bacalhau config set 'jobadmissioncontrol.acceptnetworkedjobs'='true'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd: "sudo bacalhau config list --output json",
						Output: `[{"Key":"'compute.allowlistedlocalpaths'","Value":["/tmp","/data:rw"]},
{"Key":"'compute.heartbeat.interval'","Value":"5s"},
{"Key":"'jobadmissioncontrol.acceptnetworkedjobs'","Value":"true"}]`,
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
						Cmd:    `sudo bacalhau config set 'invalid.config'='value'`,
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
				"compute.allowlistedlocalpaths": `/tmp,/data:rw`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'compute.allowlistedlocalpaths'='/tmp,/data:rw'`,
						Output: "Configuration set successfully",
						Error:  nil,
					},
					{
						Cmd:    "sudo bacalhau config list --output json",
						Output: `[{"Key":"'compute.allowlistedlocalpaths'","Value":["/tmp","/data:rw"]}]`,
						Error:  nil,
					},
				},
			},
			expectedError: "",
		},
		{
			name: "Missing configuration in final list",
			bacalhauSettings: map[string]string{
				"compute.allowlistedlocalpaths": `/tmp,/data:rw`,
			},
			sshBehavior: sshutils.ExpectedSSHBehavior{
				ExecuteCommandExpectations: []sshutils.ExecuteCommandExpectation{
					{
						Cmd:    `sudo bacalhau config set 'compute.allowlistedlocalpaths'='/tmp,/data:rw'`,
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

			// display.GetGlobalModelFunc = func() *display.DisplayModel {
			// 	return &display.DisplayModel{
			// 		Deployment: deployment,
			// 	}
			// }

			mockSSHConfig := sshutils.NewMockSSHConfigWithBehavior(tt.sshBehavior)

			cd := NewClusterDeployer(models.DeploymentTypeAzure)

			err = cd.ApplyBacalhauConfigs(context.Background(), mockSSHConfig, bacalhauSettings)

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
    "Key": "JobAdmissionControl.RejectStatelessJobs",
    "Value": false,
    "Description": "RejectStatelessJobs indicates whether to reject stateless jobs, i.e. jobs without inputs."
  },
  {
    "Key": "Orchestrator.Scheduler.HousekeepingInterval",
    "Value": "30s",
    "Description": "HousekeepingInterval specifies how often to run housekeeping tasks."
  },
  {
    "Key": "Orchestrator.Scheduler.QueueBackoff",
    "Value": "1m0s",
    "Description": "QueueBackoff specifies the time to wait before retrying a failed job."
  },
  {
    "Key": "ResultDownloaders.Timeout",
    "Value": "0s",
    "Description": "Timeout specifies the maximum time allowed for a download operation."
  },
  {
    "Key": "JobAdmissionControl.AcceptNetworkedJobs",
    "Value": true,
    "Description": "AcceptNetworkedJobs indicates whether to accept jobs that require network access."
  },
  {
    "Key": "Compute.AllocatedCapacity.CPU",
    "Value": "70%",
    "Description": "CPU specifies the amount of CPU a compute node allocates for running jobs. It can be expressed as a percentage (e.g., \"85%\") or a Kubernetes resource string (e.g., \"100m\")."
  },
  {
    "Key": "InputSources.Disabled",
    "Value": null,
    "Description": "Disabled specifies a list of storages that are disabled."
  },
  {
    "Key": "JobAdmissionControl.ProbeHTTP",
    "Value": "",
    "Description": "ProbeHTTP specifies the HTTP endpoint for probing job submission."
  },
  {
    "Key": "WebUI.Listen",
    "Value": "0.0.0.0:8438",
    "Description": "Listen specifies the address and port on which the Web UI listens."
  },
  {
    "Key": "API.TLS.Insecure",
    "Value": false,
    "Description": "Insecure allows insecure TLS connections (e.g., self-signed certificates)."
  },
  {
    "Key": "Compute.AllowListedLocalPaths",
    "Value": [
      "/tmp",
      "/data:rw"
    ],
    "Description": "AllowListedLocalPaths specifies a list of local file system paths that the compute node is allowed to access."
  },
  {
    "Key": "Compute.Auth.Token",
    "Value": "",
    "Description": "Token specifies the key for compute nodes to be able to access the orchestrator."
  },
  {
    "Key": "JobDefaults.Batch.Task.Resources.GPU",
    "Value": "",
    "Description": "GPU specifies the default number of GPUs allocated to a task. It uses Kubernetes resource string format (e.g., \"1\" for 1 GPU). This value is used when the task hasn't explicitly set its GPU requirement."
  },
  {
    "Key": "JobDefaults.Batch.Task.Resources.Memory",
    "Value": "1Gb",
    "Description": "Memory specifies the default amount of memory allocated to a task. It uses Kubernetes resource string format (e.g., \"256Mi\" for 256 megabytes). This value is used when the task hasn't explicitly set its memory requirement."
  },
  {
    "Key": "JobDefaults.Daemon.Priority",
    "Value": 0,
    "Description": "Priority specifies the default priority allocated to a service or daemon job. This value is used when the job hasn't explicitly set its priority requirement."
  },
  {
    "Key": "JobDefaults.Ops.Task.Resources.GPU",
    "Value": "",
    "Description": "GPU specifies the default number of GPUs allocated to a task. It uses Kubernetes resource string format (e.g., \"1\" for 1 GPU). This value is used when the task hasn't explicitly set its GPU requirement."
  },
  {
    "Key": "Orchestrator.Cluster.Advertise",
    "Value": "",
    "Description": "Advertise specifies the address to advertise to other cluster members."
  },
  {
    "Key": "Compute.AllocatedCapacity.Memory",
    "Value": "70%",
    "Description": "Memory specifies the amount of Memory a compute node allocates for running jobs. It can be expressed as a percentage (e.g., \"85%\") or a Kubernetes resource string (e.g., \"1Gi\")."
  },
  {
    "Key": "Publishers.Types.Local.Port",
    "Value": 6001,
    "Description": "Port specifies the port the publisher serves on."
  },
  {
    "Key": "JobDefaults.Ops.Task.Publisher.Type",
    "Value": "",
    "Description": "Type specifies the publisher type. e.g. \"s3\", \"local\", \"ipfs\", etc."
  },
  {
    "Key": "JobDefaults.Service.Task.Resources.GPU",
    "Value": "",
    "Description": "GPU specifies the default number of GPUs allocated to a task. It uses Kubernetes resource string format (e.g., \"1\" for 1 GPU). This value is used when the task hasn't explicitly set its GPU requirement."
  },
  {
    "Key": "Logging.LogDebugInfoInterval",
    "Value": "30s",
    "Description": "LogDebugInfoInterval specifies the interval for logging debug information."
  },
  {
    "Key": "FeatureFlags.ExecTranslation",
    "Value": false,
    "Description": "ExecTranslation enables the execution translation feature."
  },
  {
    "Key": "API.TLS.SelfSigned",
    "Value": false,
    "Description": "SelfSigned indicates whether to use a self-signed certificate."
  },
  {
    "Key": "Compute.Heartbeat.ResourceUpdateInterval",
    "Value": "30s",
    "Description": "ResourceUpdateInterval specifies the time between updates of resource information to the orchestrator."
  },
  {
    "Key": "InputSources.Types.IPFS.Endpoint",
    "Value": "",
    "Description": "Endpoint specifies the multi-address to connect to for IPFS. e.g /ip4/127.0.0.1/tcp/5001"
  },
  {
    "Key": "JobDefaults.Batch.Task.Timeouts.TotalTimeout",
    "Value": "0s",
    "Description": "TotalTimeout is the maximum total time allowed for a task"
  },
  {
    "Key": "JobDefaults.Ops.Task.Resources.Disk",
    "Value": "",
    "Description": "Disk specifies the default amount of disk space allocated to a task. It uses Kubernetes resource string format (e.g., \"1Gi\" for 1 gigabyte). This value is used when the task hasn't explicitly set its disk space requirement."
  },
  {
    "Key": "Orchestrator.Scheduler.HousekeepingTimeout",
    "Value": "2m0s",
    "Description": "HousekeepingTimeout specifies the maximum time allowed for a single housekeeping run."
  },
  {
    "Key": "API.Auth.AccessPolicyPath",
    "Value": "",
    "Description": "AccessPolicyPath is the path to a file or directory that will be loaded as the policy to apply to all inbound API requests. If unspecified, a policy that permits access to all API endpoints to both authenticated and unauthenticated users (the default as of v1.2.0) will be used."
  },
  {
    "Key": "Compute.AllocatedCapacity.GPU",
    "Value": "100%",
    "Description": "GPU specifies the amount of GPU a compute node allocates for running jobs. It can be expressed as a percentage (e.g., \"85%\") or a Kubernetes resource string (e.g., \"1\"). Note: When using percentages, the result is always rounded up to the nearest whole GPU."
  },
  {
    "Key": "JobDefaults.Ops.Task.Timeouts.ExecutionTimeout",
    "Value": "0s",
    "Description": "ExecutionTimeout is the maximum time allowed for task execution"
  },
  {
    "Key": "Labels",
    "Value": null,
    "Description": "Labels are key-value pairs used to describe and categorize the nodes."
  },
  {
    "Key": "Logging.Mode",
    "Value": "default",
    "Description": "Mode specifies the logging mode. One of: default, json."
  },
  {
    "Key": "Publishers.Types.IPFS.Endpoint",
    "Value": "",
    "Description": "Endpoint specifies the multi-address to connect to for IPFS. e.g /ip4/127.0.0.1/tcp/5001"
  },
  {
    "Key": "API.Host",
    "Value": "0.0.0.0",
    "Description": "Host specifies the hostname or IP address on which the API server listens or the client connects."
  },
  {
    "Key": "API.TLS.AutoCertCachePath",
    "Value": "",
    "Description": "AutoCertCachePath specifies the directory to cache auto-generated certificates."
  },
  {
    "Key": "API.TLS.CAFile",
    "Value": "",
    "Description": "CAFile specifies the path to the Certificate Authority file."
  },
  {
    "Key": "DisableAnalytics",
    "Value": false,
    "Description": "No description available"
  },
  {
    "Key": "Engines.Disabled",
    "Value": null,
    "Description": "Disabled specifies a list of engines that are disabled."
  },
  {
    "Key": "JobDefaults.Ops.Task.Publisher.Params",
    "Value": null,
    "Description": "Params specifies the publisher configuration data."
  },
  {
    "Key": "JobDefaults.Service.Task.Resources.Memory",
    "Value": "1Gb",
    "Description": "Memory specifies the default amount of memory allocated to a task. It uses Kubernetes resource string format (e.g., \"256Mi\" for 256 mebibytes). This value is used when the task hasn't explicitly set its memory requirement."
  },
  {
    "Key": "Orchestrator.Auth.Token",
    "Value": "",
    "Description": "Token specifies the key for compute nodes to be able to access the orchestrator"
  },
  {
    "Key": "API.Auth.Methods",
    "Value": {
      "ClientKey": {
        "Type": "challenge"
      }
    },
    "Description": "Methods maps \"method names\" to authenticator implementations. A method name is a human-readable string chosen by the person configuring the system that is shown to users to help them pick the authentication method they want to use. There can be multiple usages of the same Authenticator *type* but with different configs and parameters, each identified with a unique method name.  For example, if an implementation wants to allow users to log in with Github or Bitbucket, they might both use an authenticator implementation of type \"oidc\", and each would appear once on this provider with key / method name \"github\" and \"bitbucket\".  By default, only a single authentication method that accepts authentication via client keys will be enabled."
  },
  {
    "Key": "StrictVersionMatch",
    "Value": false,
    "Description": "StrictVersionMatch indicates whether to enforce strict version matching."
  },
  {
    "Key": "Engines.Types.Docker.ManifestCache.Size",
    "Value": 1000,
    "Description": "Size specifies the size of the Docker manifest cache."
  },
  {
    "Key": "JobAdmissionControl.ProbeExec",
    "Value": "",
    "Description": "ProbeExec specifies the command to execute for probing job submission."
  },
  {
    "Key": "JobDefaults.Batch.Task.Timeouts.ExecutionTimeout",
    "Value": "0s",
    "Description": "ExecutionTimeout is the maximum time allowed for task execution"
  },
  {
    "Key": "JobDefaults.Service.Priority",
    "Value": 0,
    "Description": "Priority specifies the default priority allocated to a service or daemon job. This value is used when the job hasn't explicitly set its priority requirement."
  },
  {
    "Key": "JobDefaults.Service.Task.Resources.CPU",
    "Value": "500m",
    "Description": "CPU specifies the default amount of CPU allocated to a task. It uses Kubernetes resource string format (e.g., \"100m\" for 0.1 CPU cores). This value is used when the task hasn't explicitly set its CPU requirement."
  },
  {
    "Key": "Compute.Heartbeat.InfoUpdateInterval",
    "Value": "1m0s",
    "Description": "InfoUpdateInterval specifies the time between updates of non-resource information to the orchestrator."
  },
  {
    "Key": "Orchestrator.Port",
    "Value": 4222,
    "Description": "Host specifies the port number on which the Orchestrator server listens for compute node connections."
  },
  {
    "Key": "Compute.Orchestrators",
    "Value": [
      "nats://127.0.0.1:4222"
    ],
    "Description": "Orchestrators specifies a list of orchestrator endpoints that this compute node connects to."
  },
  {
    "Key": "JobDefaults.Daemon.Task.Resources.Disk",
    "Value": "",
    "Description": "Disk specifies the default amount of disk space allocated to a task. It uses Kubernetes resource string format (e.g., \"1Gi\" for 1 gibibyte). This value is used when the task hasn't explicitly set its disk space requirement."
  },
  {
    "Key": "JobDefaults.Daemon.Task.Resources.GPU",
    "Value": "",
    "Description": "GPU specifies the default number of GPUs allocated to a task. It uses Kubernetes resource string format (e.g., \"1\" for 1 GPU). This value is used when the task hasn't explicitly set its GPU requirement."
  },
  {
    "Key": "JobDefaults.Daemon.Task.Resources.Memory",
    "Value": "1Gb",
    "Description": "Memory specifies the default amount of memory allocated to a task. It uses Kubernetes resource string format (e.g., \"256Mi\" for 256 mebibytes). This value is used when the task hasn't explicitly set its memory requirement."
  },
  {
    "Key": "Orchestrator.Advertise",
    "Value": "",
    "Description": "Advertise specifies URL to advertise to other servers."
  },
  {
    "Key": "Orchestrator.Cluster.Port",
    "Value": 0,
    "Description": "Port specifies the port number for cluster communication."
  },
  {
    "Key": "Orchestrator.Host",
    "Value": "0.0.0.0",
    "Description": "Host specifies the hostname or IP address on which the Orchestrator server listens for compute node connections."
  },
  {
    "Key": "Orchestrator.Scheduler.WorkerCount",
    "Value": 12,
    "Description": "WorkerCount specifies the number of concurrent workers for job scheduling."
  },
  {
    "Key": "InputSources.MaxRetryCount",
    "Value": 3,
    "Description": "ReadTimeout specifies the maximum number of attempts for reading from a storage."
  },
  {
    "Key": "ResultDownloaders.Types.IPFS.Endpoint",
    "Value": "",
    "Description": "Endpoint specifies the multi-address to connect to for IPFS. e.g /ip4/127.0.0.1/tcp/5001"
  },
  {
    "Key": "Publishers.Types.S3.PreSignedURLExpiration",
    "Value": "0s",
    "Description": "PreSignedURLExpiration specifies the duration before a pre-signed URL expires."
  },
  {
    "Key": "Engines.Types.Docker.ManifestCache.Refresh",
    "Value": "1h0m0s",
    "Description": "Refresh specifies the refresh interval for cache entries."
  },
  {
    "Key": "JobDefaults.Batch.Task.Publisher.Params",
    "Value": null,
    "Description": "Params specifies the publisher configuration data."
  },
  {
    "Key": "JobDefaults.Daemon.Task.Resources.CPU",
    "Value": "500m",
    "Description": "CPU specifies the default amount of CPU allocated to a task. It uses Kubernetes resource string format (e.g., \"100m\" for 0.1 CPU cores). This value is used when the task hasn't explicitly set its CPU requirement."
  },
  {
    "Key": "JobDefaults.Ops.Priority",
    "Value": 0,
    "Description": "Priority specifies the default priority allocated to a batch or ops job. This value is used when the job hasn't explicitly set its priority requirement."
  },
  {
    "Key": "Orchestrator.Enabled",
    "Value": false,
    "Description": "Enabled indicates whether the orchestrator node is active and available for job submission."
  },
  {
    "Key": "Publishers.Types.Local.Address",
    "Value": "127.0.0.1",
    "Description": "Address specifies the endpoint the publisher serves on."
  },
  {
    "Key": "API.TLS.CertFile",
    "Value": "",
    "Description": "CertFile specifies the path to the TLS certificate file."
  },
  {
    "Key": "Orchestrator.Cluster.Host",
    "Value": "",
    "Description": "Host specifies the hostname or IP address for cluster communication."
  },
  {
    "Key": "Orchestrator.EvaluationBroker.MaxRetryCount",
    "Value": 10,
    "Description": "MaxRetryCount specifies the maximum number of times an evaluation can be retried before being marked as failed."
  },
  {
    "Key": "Publishers.Disabled",
    "Value": null,
    "Description": "Disabled specifies a list of publishers that are disabled."
  },
  {
    "Key": "ResultDownloaders.Disabled",
    "Value": null,
    "Description": "Disabled is a list of downloaders that are disabled."
  },
  {
    "Key": "NameProvider",
    "Value": "puuid",
    "Description": "NameProvider specifies the method used to generate names for the node. One of: hostname, aws, gcp, uuid, puuid."
  },
  {
    "Key": "JobDefaults.Batch.Priority",
    "Value": 0,
    "Description": "Priority specifies the default priority allocated to a batch or ops job. This value is used when the job hasn't explicitly set its priority requirement."
  },
  {
    "Key": "JobDefaults.Batch.Task.Publisher.Type",
    "Value": "",
    "Description": "Type specifies the publisher type. e.g. \"s3\", \"local\", \"ipfs\", etc."
  },
  {
    "Key": "JobDefaults.Batch.Task.Resources.CPU",
    "Value": "500m",
    "Description": "CPU specifies the default amount of CPU allocated to a task. It uses Kubernetes resource string format (e.g., \"100m\" for 0.1 CPU cores). This value is used when the task hasn't explicitly set its CPU requirement."
  },
  {
    "Key": "Logging.Level",
    "Value": "info",
    "Description": "Level sets the logging level. One of: trace, debug, info, warn, error, fatal, panic."
  },
  {
    "Key": "Orchestrator.Cluster.Name",
    "Value": "",
    "Description": "Name specifies the unique identifier for this orchestrator cluster."
  },
  {
    "Key": "InputSources.ReadTimeout",
    "Value": "5m0s",
    "Description": "ReadTimeout specifies the maximum time allowed for reading from a storage."
  },
  {
    "Key": "Orchestrator.Cluster.Peers",
    "Value": null,
    "Description": "Peers is a list of other cluster members to connect to on startup."
  },
  {
    "Key": "Orchestrator.NodeManager.DisconnectTimeout",
    "Value": "5s",
    "Description": "DisconnectTimeout specifies how long to wait before considering a node disconnected."
  },
  {
    "Key": "Orchestrator.NodeManager.ManualApproval",
    "Value": false,
    "Description": "ManualApproval, if true, requires manual approval for new compute nodes joining the cluster."
  },
  {
    "Key": "API.TLS.KeyFile",
    "Value": "",
    "Description": "KeyFile specifies the path to the TLS private key file."
  },
  {
    "Key": "API.TLS.UseTLS",
    "Value": false,
    "Description": "UseTLS indicates whether to use TLS for client connections."
  },
  {
    "Key": "JobDefaults.Batch.Task.Resources.Disk",
    "Value": "",
    "Description": "Disk specifies the default amount of disk space allocated to a task. It uses Kubernetes resource string format (e.g., \"1Gi\" for 1 gibibyte). This value is used when the task hasn't explicitly set its disk space requirement."
  },
  {
    "Key": "JobDefaults.Ops.Task.Resources.CPU",
    "Value": "500m",
    "Description": "CPU specifies the default amount of CPU allocated to a task. It uses Kubernetes resource string format (e.g., \"100m\" for 0.1 CPU cores). This value is used when the task hasn't explicitly set its CPU requirement."
  },
  {
    "Key": "JobDefaults.Ops.Task.Resources.Memory",
    "Value": "1Gb",
    "Description": "Memory specifies the default amount of memory allocated to a task. It uses Kubernetes resource string format (e.g., \"256Mi\" for 256 mebibytes). This value is used when the task hasn't explicitly set its memory requirement."
  },
  {
    "Key": "JobDefaults.Ops.Task.Timeouts.TotalTimeout",
    "Value": "0s",
    "Description": "TotalTimeout is the maximum total time allowed for a task"
  },
  {
    "Key": "Orchestrator.EvaluationBroker.VisibilityTimeout",
    "Value": "1m0s",
    "Description": "VisibilityTimeout specifies how long an evaluation can be claimed before it's returned to the queue."
  },
  {
    "Key": "Publishers.Types.S3.PreSignedURLDisabled",
    "Value": false,
    "Description": "PreSignedURLDisabled specifies whether pre-signed URLs are enabled for the S3 provider."
  },
  {
    "Key": "API.Port",
    "Value": 1234,
    "Description": "Port specifies the port number on which the API server listens or the client connects."
  },
  {
    "Key": "WebUI.Backend",
    "Value": "",
    "Description": "Backend specifies the address and port of the backend API server. If empty, the Web UI will use the same address and port as the API server."
  },
  {
    "Key": "WebUI.Enabled",
    "Value": false,
    "Description": "Enabled indicates whether the Web UI is enabled."
  },
  {
    "Key": "UpdateConfig.Interval",
    "Value": "24h0m0s",
    "Description": "Interval specifies the time between update checks, when set to 0 update checks are not performed."
  },
  {
    "Key": "Compute.AllocatedCapacity.Disk",
    "Value": "70%",
    "Description": "Disk specifies the amount of Disk space a compute node allocates for running jobs. It can be expressed as a percentage (e.g., \"85%\") or a Kubernetes resource string (e.g., \"10Gi\")."
  },
  {
    "Key": "Compute.Enabled",
    "Value": false,
    "Description": "Enabled indicates whether the compute node is active and available for job execution."
  },
  {
    "Key": "Compute.Heartbeat.Interval",
    "Value": "5s",
    "Description": "Interval specifies the time between heartbeat signals sent to the orchestrator."
  },
  {
    "Key": "DataDir",
    "Value": "/Users/daaronch/.bacalhau",
    "Description": "DataDir specifies a location on disk where the bacalhau node will maintain state."
  },
  {
    "Key": "Engines.Types.Docker.ManifestCache.TTL",
    "Value": "1h0m0s",
    "Description": "TTL specifies the time-to-live duration for cache entries."
  },
  {
    "Key": "JobAdmissionControl.Locality",
    "Value": "anywhere",
    "Description": "Locality specifies the locality of the job input data."
  },
  {
    "Key": "JobDefaults.Service.Task.Resources.Disk",
    "Value": "",
    "Description": "Disk specifies the default amount of disk space allocated to a task. It uses Kubernetes resource string format (e.g., \"1Gi\" for 1 gibibyte). This value is used when the task hasn't explicitly set its disk space requirement."
  },
  {
    "Key": "API.TLS.AutoCert",
    "Value": "",
    "Description": "AutoCert specifies the domain for automatic certificate generation."
  }
]`
