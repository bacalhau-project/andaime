package gcp

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/internal/testdata"
	"github.com/bacalhau-project/andaime/internal/testutil"
	gcp_mocks "github.com/bacalhau-project/andaime/mocks/gcp"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	gcp_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	"github.com/bacalhau-project/andaime/pkg/providers/common"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type CmdBetaGCPCreateDeploymentSuite struct {
	suite.Suite
	ctx                    context.Context
	viperConfigFile        string
	testSSHPublicKeyPath   string
	testPrivateKeyPath     string
	cleanupPublicKey       func()
	cleanupPrivateKey      func()
	mockGCPClient          *gcp_mocks.MockGCPClienter
	gcpProvider            *gcp_provider.GCPProvider
	origGetGlobalModelFunc func() *display.DisplayModel
	origGetClientFunc      func(ctx context.Context, organizationID string) (gcp_interface.GCPClienter, func(), error)
}

func (suite *CmdBetaGCPCreateDeploymentSuite) SetupSuite() {
	suite.ctx = context.Background()
	suite.testSSHPublicKeyPath, suite.cleanupPublicKey, suite.testPrivateKeyPath, suite.cleanupPrivateKey = testutil.CreateSSHPublicPrivateKeyPairOnDisk()

	suite.mockGCPClient = new(gcp_mocks.MockGCPClienter)
	suite.mockGCPClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	suite.origGetGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		deployment, err := models.NewDeployment()
		suite.Require().NoError(err)
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	_ = logger.Get()
}

func (suite *CmdBetaGCPCreateDeploymentSuite) TearDownSuite() {
	suite.cleanupPublicKey()
	suite.cleanupPrivateKey()
	display.GetGlobalModelFunc = suite.origGetGlobalModelFunc
	gcp_provider.NewGCPClientFunc = suite.origGetClientFunc

	_ = os.Remove(suite.viperConfigFile)
}

func (suite *CmdBetaGCPCreateDeploymentSuite) SetupTest() {
	viper.Reset()
	suite.setupViper()

	suite.mockGCPClient = new(gcp_mocks.MockGCPClienter)
	suite.mockGCPClient.On("ValidateMachineType", mock.Anything, mock.Anything, mock.Anything).
		Return(true, nil)

	suite.origGetClientFunc = gcp_provider.NewGCPClientFunc
	gcp_provider.NewGCPClientFunc = func(ctx context.Context,
		organizationID string) (gcp_interface.GCPClienter, func(), error) {
		return suite.mockGCPClient, func() {}, nil
	}

	suite.origGetGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		deployment, err := models.NewDeployment()
		suite.Require().NoError(err)
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	var err error
	suite.gcpProvider, err = gcp_provider.NewGCPProviderFunc(
		suite.ctx,
		viper.GetString("gcp.project_id"),
		viper.GetString("gcp.organization_id"),
		viper.GetString("gcp.billing_account_id"),
	)
	suite.Require().NoError(err)
	suite.Require().NotNil(suite.gcpProvider)
	suite.gcpProvider.SetGCPClient(suite.mockGCPClient)
}

func (suite *CmdBetaGCPCreateDeploymentSuite) setupViper() {
	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.unique_id", "01234567890")
	viper.Set(
		"gcp.project_id",
		fmt.Sprintf("test-project-%s", viper.GetString("general.unique_id")),
	)
	viper.Set("general.ssh_public_key_path", suite.testSSHPublicKeyPath)
	viper.Set("general.ssh_private_key_path", suite.testPrivateKeyPath)
	viper.Set("gcp.organization_id", "test-org-id")
	viper.Set("gcp.billing_account_id", "test-billing-account-id")
	viper.Set("gcp.default_count_per_zone", 1)
	viper.Set("gcp.default_region", "us-central1")
	viper.Set("gcp.default_zone", "us-central1-a")
	viper.Set("gcp.default_machine_type", "n1-standard-2")
	viper.Set("gcp.default_disk_size_gb", 30)
	viper.Set("gcp.machines", []map[string]interface{}{
		{
			"location": "us-central1-a",
			"parameters": map[string]interface{}{
				"count":        1,
				"machine_type": "n1-standard-2",
				"orchestrator": true,
			},
		},
	})

	tempConfigFile, err := os.CreateTemp("", "config*.yaml")
	content, err := testdata.ReadTestGCPConfig()
	suite.Require().NoError(err)
	err = os.WriteFile(tempConfigFile.Name(), []byte(content), 0o644)
	suite.Require().NoError(err)

	suite.viperConfigFile = tempConfigFile.Name()
	viper.SetConfigFile(suite.viperConfigFile)
}

func (suite *CmdBetaGCPCreateDeploymentSuite) TestProcessMachinesConfig() {
	tests := []struct {
		name                string
		machinesConfig      []map[string]interface{}
		orchestratorIP      string
		expectError         bool
		expectedErrorString string
		expectedNodes       int
	}{
		{
			name:                "No orchestrator node and no orchestrator IP",
			machinesConfig:      []map[string]interface{}{},
			expectError:         true,
			expectedErrorString: "no orchestrator node and orchestratorIP is not set",
			expectedNodes:       0,
		},
		{
			name: "No orchestrator node but orchestrator IP specified",
			machinesConfig: []map[string]interface{}{
				{
					"location":   "us-central1-a",
					"parameters": map[string]interface{}{"count": 1},
				},
			},
			orchestratorIP: "1.2.3.4",
			expectError:    false,
			expectedNodes:  1,
		},
		{
			name: "One orchestrator node, no other machines",
			machinesConfig: []map[string]interface{}{
				{
					"location":   "us-central1-a",
					"parameters": map[string]interface{}{"orchestrator": true},
				},
			},
			expectError:   false,
			expectedNodes: 1,
		},
		{
			name: "One orchestrator node and many other machines",
			machinesConfig: []map[string]interface{}{
				{
					"location":   "us-central1-a",
					"parameters": map[string]interface{}{"orchestrator": true},
				},
				{"location": "us-west1-b", "parameters": map[string]interface{}{"count": 3}},
			},
			expectError:   false,
			expectedNodes: 4,
		},
		{
			name: "Multiple orchestrator nodes (should error)",
			machinesConfig: []map[string]interface{}{
				{
					"location":   "us-central1-a",
					"parameters": map[string]interface{}{"orchestrator": true},
				},
				{
					"location":   "us-west1-b",
					"parameters": map[string]interface{}{"orchestrator": true},
				},
			},
			expectError:   true,
			expectedNodes: 0,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()

			deployment, err := models.NewDeployment()
			suite.Require().NoError(err)
			deployment.SSHPrivateKeyPath = suite.testPrivateKeyPath
			deployment.SSHPort = 22
			deployment.OrchestratorIP = tt.orchestratorIP

			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}

			viper.Set("gcp.machines", tt.machinesConfig)
			viper.Set("general.orchestrator_ip", tt.orchestratorIP)

			machines, locations, err := suite.gcpProvider.ProcessMachinesConfig(suite.ctx)

			if tt.expectError {
				suite.Error(err)
				if tt.expectedErrorString != "" {
					suite.Contains(err.Error(), tt.expectedErrorString)
				}
			} else {
				suite.NoError(err)
				suite.Len(machines, tt.expectedNodes)

				if tt.expectedNodes > 0 {
					suite.NotEmpty(locations, "Locations should not be empty")
				}

				if tt.name == "One orchestrator node, no other machines" ||
					tt.name == "One orchestrator node and many other machines" {
					orchestratorFound := false
					for _, machine := range machines {
						if machine.IsOrchestrator() {
							orchestratorFound = true
							break
						}
					}
					suite.True(orchestratorFound)
				} else if tt.name == "No orchestrator node but orchestrator IP specified" {
					suite.NotEmpty(deployment.OrchestratorIP)
					for _, machine := range machines {
						suite.False(machine.IsOrchestrator())
						suite.Equal(deployment.OrchestratorIP, machine.GetOrchestratorIP())
					}
				} else {
					suite.Empty(deployment.OrchestratorIP)
				}
			}
		})
	}
}

func (suite *CmdBetaGCPCreateDeploymentSuite) TestPrepareDeployment() {
	suite.SetupTest()

	deployment, err := suite.gcpProvider.PrepareDeployment(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().NotNil(deployment)

	suite.Equal(models.DeploymentTypeGCP, deployment.DeploymentType)
	suite.Contains(deployment.ProjectID, viper.GetString("general.project_prefix"))
	suite.Equal("us-central1", deployment.GCP.DefaultRegion)
	suite.Equal("us-central1-a", deployment.GCP.DefaultZone)
	suite.Equal("test-billing-account-id", deployment.GCP.BillingAccountID)
	suite.Equal("test-org-id", deployment.GCP.OrganizationID)
	suite.NotEmpty(deployment.SSHPublicKeyMaterial)
	suite.NotEmpty(deployment.SSHPrivateKeyMaterial)
	suite.WithinDuration(time.Now(), deployment.StartTime, 5*time.Second)
}

func (suite *CmdBetaGCPCreateDeploymentSuite) TestPrepareDeployment_MissingRequiredFields() {
	testCases := []struct {
		name        string
		setupConfig func()
		expectedErr string
	}{
		{
			name: "Missing project_id",
			setupConfig: func() {
				viper.Set("gcp.project_id", "")
			},
			expectedErr: "gcp.project_id is not set",
		},
		{
			name: "Missing organization_id",
			setupConfig: func() {
				viper.Set("gcp.organization_id", "")
			},
			expectedErr: "gcp.organization_id is not set",
		},
		{
			name: "Missing billing_account_id",
			setupConfig: func() {
				viper.Set("gcp.billing_account_id", "")
			},
			expectedErr: "gcp.billing_account_id is not set",
		},
		{
			name: "Missing machines configuration",
			setupConfig: func() {
				viper.Set("gcp.machines", nil)
			},
			expectedErr: "no machines configuration found for provider GCP",
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.setupConfig()

			tempConfigFile, err := os.CreateTemp("", "gcp_test_config_*.yaml")
			suite.Require().NoError(err, "Failed to create temporary config file")
			defer os.Remove(tempConfigFile.Name())
			viper.SetConfigFile(tempConfigFile.Name())

			_, err = common.PrepareDeployment(suite.ctx, models.DeploymentTypeGCP)
			suite.Require().Error(err)
			suite.Contains(err.Error(), tc.expectedErr)
		})
	}
}

func (suite *CmdBetaGCPCreateDeploymentSuite) TestPrepareDeployment_CustomMachineConfiguration() {
	suite.SetupTest()
	viper.Set("gcp.machines", []map[string]interface{}{
		{
			"location": "us-west1-b",
			"parameters": map[string]interface{}{
				"machine_type": "n1-standard-4",
				"disk_size_gb": 50,
				"count":        2,
			},
		},
	})

	deployment, err := suite.gcpProvider.PrepareDeployment(suite.ctx)
	suite.Require().NoError(err)
	suite.Require().NotNil(deployment)

	suite.Equal("us-central1", deployment.GCP.DefaultRegion)
	suite.Equal("us-central1-a", deployment.GCP.DefaultZone)

	// Verify custom machine configuration
	suite.Require().Len(deployment.Machines, 2, "Expected 2 machines to be created")
	for _, m := range deployment.Machines {
		machine, ok := m.(*models.Machine)
		suite.Require().True(ok, "Expected machine to be of type *models.Machine")
		suite.Equal("us-west1-b", machine.GetLocation(), "Expected location to be us-west1-b")
		suite.Equal(
			"n1-standard-4",
			machine.GetVMSize(),
			"Expected machine type to be n1-standard-4",
		)
		suite.Equal(50, machine.GetDiskSizeGB(), "Expected disk size to be 50")
		suite.False(machine.IsOrchestrator(), "Expected machine to not be an orchestrator")
	}
}

func (suite *CmdBetaGCPCreateDeploymentSuite) TestPrepareDeployment_InvalidSSHKeyPaths() {
	suite.SetupTest()
	viper.Set("general.ssh_public_key_path", "/nonexistent/path/to/public/key")
	viper.Set("general.ssh_private_key_path", "/nonexistent/path/to/private/key")

	_, err := suite.gcpProvider.PrepareDeployment(suite.ctx)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to extract SSH keys")
	suite.Contains(err.Error(), "key file does not exist")
}

func (suite *CmdBetaGCPCreateDeploymentSuite) TestPrepareDeployment_EmptySSHKeyPaths() {
	suite.SetupTest()
	viper.Set("general.ssh_public_key_path", "")
	viper.Set("general.ssh_private_key_path", "")

	_, err := suite.gcpProvider.PrepareDeployment(suite.ctx)
	suite.Require().Error(err)
	suite.Contains(err.Error(), "failed to extract SSH keys")
	suite.Contains(err.Error(), "general.ssh_public_key_path is empty")
}

func (suite *CmdBetaGCPCreateDeploymentSuite) TestValidateMachineType() {
	suite.SetupTest()

	testCases := []struct {
		name        string
		machineType string
		location    string
		isValid     bool
		expectError bool
	}{
		{
			name:        "Valid machine type",
			machineType: "n1-standard-2",
			location:    "us-central1-a",
			isValid:     true,
			expectError: false,
		},
		{
			name:        "Invalid machine type",
			machineType: "invalid-machine-type",
			location:    "us-central1-a",
			isValid:     false,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.mockGCPClient.On("ValidateMachineType", mock.Anything, tc.location, tc.machineType).
				Return(tc.isValid, nil).
				Once()

			isValid, err := suite.gcpProvider.ValidateMachineType(
				suite.ctx,
				tc.location,
				tc.machineType,
			)

			if tc.expectError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)
				suite.Equal(tc.isValid, isValid)
			}
		})
	}
}

func TestCmdBetaGCPCreateDeploymentSuite(t *testing.T) {
	suite.Run(t, new(CmdBetaGCPCreateDeploymentSuite))
}
