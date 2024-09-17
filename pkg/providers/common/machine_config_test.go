package common

import (
	"context"
	"testing"

	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestProcessMachinesConfig(t *testing.T) {
	_, cleanupPublicKey, testPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	viper.Set("general.ssh_private_key_path", testPrivateKeyPath)
	viper.Set("general.ssh_port", 22)
	viper.Set("general.project_prefix", "test")

	// Mock viper configuration
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
	assert.NoError(t, err)

	origGetDeploymentFunc := display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return &display.DisplayModel{
			Deployment: deployment,
		}
	}
	defer func() {
		display.GetGlobalModelFunc = origGetDeploymentFunc
	}()
	mockValidateMachineType := func(ctx context.Context, location, vmSize string) (bool, error) {
		return true, nil
	}

	machines, locations, err := ProcessMachinesConfig(
		models.DeploymentTypeAzure,
		mockValidateMachineType,
	)
	assert.NoError(t, err)
	deployment.SetMachines(machines)
	deployment.SetLocations(locations)

	// Verify the results
	assert.Len(t, deployment.Machines, 3)
	assert.Len(t, deployment.Locations, 2)

	var orchestratorCount int
	for _, machine := range deployment.Machines {
		if machine.IsOrchestrator() {
			orchestratorCount++
		}
		assert.Equal(t, "azureuser", machine.GetSSHUser())
		assert.Equal(t, 22, machine.GetSSHPort())
		assert.NotNil(t, machine.GetSSHPrivateKeyMaterial())
	}
	assert.Equal(t, 1, orchestratorCount)
}

func TestProcessMachinesConfigErrors(t *testing.T) {
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
		t.Run(tt.name, func(t *testing.T) {
			viper.Reset()
			tt.viperSetup()

			deployment := &models.Deployment{
				SSHPrivateKeyPath: "test_key_path",
				SSHPort:           22,
			}

			origGetDeploymentFunc := display.GetGlobalModelFunc
			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return &display.DisplayModel{
					Deployment: deployment,
				}
			}
			defer func() {
				display.GetGlobalModelFunc = origGetDeploymentFunc
			}()

			mockValidateMachineType := func(ctx context.Context, location, vmSize string) (bool, error) {
				return true, nil
			}

			machines, locations, err := ProcessMachinesConfig(
				models.DeploymentTypeAzure,
				mockValidateMachineType,
			)

			assert.Nil(t, machines)
			assert.Nil(t, locations)
			assert.EqualError(t, err, tt.expectedError)
		})
	}
}
