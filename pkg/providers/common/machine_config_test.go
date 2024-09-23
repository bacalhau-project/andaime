package common

import (
	"context"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

type PkgProvidersCommonMachineConfigTestSuite struct {
	suite.Suite
	ctx                    context.Context
	testPrivateKeyPath     string
	cleanup                func()
	originalGetGlobalModel func() *display.DisplayModel
}

func (s *PkgProvidersCommonMachineConfigTestSuite) SetupSuite() {
	s.ctx = context.Background()
	_, cleanupPublicKey, testPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	s.cleanup = func() {
		cleanupPublicKey()
		cleanupPrivateKey()
	}
	s.testPrivateKeyPath = testPrivateKeyPath
	s.originalGetGlobalModel = display.GetGlobalModelFunc
}

func (s *PkgProvidersCommonMachineConfigTestSuite) TearDownSuite() {
	s.cleanup()
	display.GetGlobalModelFunc = s.originalGetGlobalModel
}

func (s *PkgProvidersCommonMachineConfigTestSuite) SetupTest() {
	viper.Reset()
	viper.Set("general.ssh_private_key_path", s.testPrivateKeyPath)
	viper.Set("general.ssh_port", 22)
	viper.Set("general.project_prefix", "test")
}

func (s *PkgProvidersCommonMachineConfigTestSuite) TestProcessMachinesConfig() {
	viper.Set("azure.machines", []map[string]interface{}{
		{
			"location": "eastus",
			"parameters": map[string]interface{}{
				"count":        1,
				"type":         "Standard_D2s_v3",
				"orchestrator": true,
			},
		},
		{
			"location": "westus",
			"parameters": map[string]interface{}{
				"count": 2,
				"type":  "Standard_D4s_v3",
			},
		},
	})
	viper.Set("azure.default_count_per_zone", 1)
	viper.Set("azure.default_machine_type", "Standard_D2s_v3")
	viper.Set("azure.default_disk_size_gb", 30)

	deployment, err := models.NewDeployment()
	s.Require().NoError(err)

	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}

	mockValidateMachineType := func(ctx context.Context, location, vmSize string) (bool, error) {
		return true, nil
	}

	machines, locations, err := ProcessMachinesConfig(
		models.DeploymentTypeAzure,
		mockValidateMachineType,
	)
	s.Require().NoError(err)
	deployment.SetMachines(machines)
	deployment.SetLocations(locations)

	s.Len(deployment.Machines, 3)
	s.Len(deployment.Locations, 2)

	orchestratorCount := 0
	for _, machine := range deployment.Machines {
		if machine.IsOrchestrator() {
			orchestratorCount++
		}
		s.Equal("azureuser", machine.GetSSHUser())
		s.Equal(22, machine.GetSSHPort())
		s.NotNil(machine.GetSSHPrivateKeyMaterial())
	}
	s.Equal(1, orchestratorCount)
}

func (s *PkgProvidersCommonMachineConfigTestSuite) TestProcessMachinesConfigErrors() {
	tests := []struct {
		name          string
		viperSetup    func()
		expectedError string
	}{
		{
			name: "Missing default count",
			viperSetup: func() {
				viper.Set("azure.default_count_per_zone", 0)
			},
			expectedError: "azure.default_count_per_zone is empty",
		},
		{
			name: "Missing default machine type",
			viperSetup: func() {
				viper.Set("azure.default_count_per_zone", 1)
				viper.Set("azure.default_machine_type", "")
			},
			expectedError: "azure.default_machine_type is empty",
		},
		{
			name: "Missing disk size",
			viperSetup: func() {
				viper.Set("azure.default_count_per_zone", 1)
				viper.Set("azure.default_machine_type", "Standard_D2s_v3")
				viper.Set("azure.default_disk_size_gb", 0)
			},
			expectedError: "azure.default_disk_size_gb is empty",
		},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			s.SetupTest()
			tt.viperSetup()

			deployment := &models.Deployment{
				SSHPrivateKeyPath: "test_key_path",
				SSHPort:           22,
			}

			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}

			mockValidateMachineType := func(ctx context.Context, location, vmSize string) (bool, error) {
				return true, nil
			}

			machines, locations, err := ProcessMachinesConfig(
				models.DeploymentTypeAzure,
				mockValidateMachineType,
			)

			s.Nil(machines)
			s.Nil(locations)
			s.EqualError(err, tt.expectedError)
		})
	}
}

func TestMachineConfigSuite(t *testing.T) {
	suite.Run(t, new(PkgProvidersCommonMachineConfigTestSuite))
}
