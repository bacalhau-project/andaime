package azure

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestDeployARMTemplate(t *testing.T) {
	_, cleanupPublicKey, testSSHPrivateKeyPath, cleanupPrivateKey := testutil.CreateSSHPublicPrivateKeyPairOnDisk()
	testSSHPrivateKeyMaterial, err := os.ReadFile(testSSHPrivateKeyPath)
	if err != nil {
		t.Fatalf("failed to read SSH private key: %v", err)
	}
	defer cleanupPublicKey()
	defer cleanupPrivateKey()

	mockSSHConfig := new(MockSSHConfig)

	tests := []struct {
		name                string
		locations           []string
		machinesPerLocation int
		deployMachineErrors map[string]error
		expectedError       string
	}{
		{
			name:                "No locations",
			locations:           []string{},
			machinesPerLocation: 1,
			expectedError:       "no locations provided",
		},
		{
			name:                "Single location, single machine, success",
			locations:           []string{"eastus"},
			machinesPerLocation: 1,
			deployMachineErrors: map[string]error{},
		},
		{
			name:                "Single location, multiple machines, success",
			locations:           []string{"eastus"},
			machinesPerLocation: 3,
			deployMachineErrors: map[string]error{},
		},
		{
			name:                "Multiple locations, single machine each, success",
			locations:           []string{"eastus", "westus", "northeurope"},
			machinesPerLocation: 1,
			deployMachineErrors: map[string]error{},
		},
		{
			name:                "Multiple locations, multiple machines each, success",
			locations:           []string{"eastus", "westus"},
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{},
		},
		{
			name:                "Single location, first machine fails",
			locations:           []string{"eastus"},
			machinesPerLocation: 3,
			deployMachineErrors: map[string]error{
				"machine-eastus-0": errors.New("deployment failed"),
			},
			expectedError: "deployment failed: failed to deploy first machine",
		},
		{
			name:                "Single location, later machine fails",
			locations:           []string{"eastus"},
			machinesPerLocation: 3,
			deployMachineErrors: map[string]error{
				"machine-eastus-2": errors.New("deployment failed"),
			},
			expectedError: "deployment failed: failed to deploy remaining machines in location eastus",
		},
		{
			name:                "Multiple locations, machine in first location fails",
			locations:           []string{"eastus", "westus"},
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{
				"machine-eastus-1": errors.New("deployment failed"),
			},
			expectedError: "deployment failed: failed to deploy first machine",
		},
		{
			name:                "Multiple locations, machine in later location fails",
			locations:           []string{"eastus", "westus"},
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{
				"machine-westus-0": errors.New("deployment failed"),
			},
			expectedError: "deployment failed: failed to deploy remaining machines in location",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Test panicked: %v", r)
					debug.PrintStack()
				}
			}()

			fmt.Printf("Running test: %s\n", tt.name)

			// Create a mock Azure client
			mockAzureClient := &MockAzureClient{}

			// Create the Azure provider with the mock client
			provider := &AzureProvider{
				Client: mockAzureClient,
			}

			// Set up expectations for the mock client
			for _, location := range tt.locations {
				for i := 0; i < tt.machinesPerLocation; i++ {
					machineName := fmt.Sprintf("machine-%s-%d", location, i)
					err, ok := tt.deployMachineErrors[machineName]
					if !ok {
						err = nil
					}

					props := armresources.DeploymentsClientCreateOrUpdateResponse{}
					var provisioningState armresources.ProvisioningState
					if err == nil {
						provisioningState = armresources.ProvisioningStateSucceeded
					} else {
						provisioningState = armresources.ProvisioningStateFailed
					}
					props.Properties = &armresources.DeploymentPropertiesExtended{
						ProvisioningState: &provisioningState,
					}
					mockArmDeploymentPoller := &MockPoller{}
					mockArmDeploymentPoller.On("PollUntilDone", mock.Anything, mock.Anything).
						Return(props, err).
						Once()
					mockAzureClient.On("DeployTemplate",
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
						mock.Anything,
					).
						Return(mockArmDeploymentPoller, nil).
						Once()

					// If deployment is successful, mock GetVMIPAddresses
					if err == nil {
						setupMockVMAndNetwork(mockAzureClient)
					}
				}
			}

			// Set up the global model
			display.SetGlobalModel(display.InitialModel())
			m := display.GetGlobalModelFunc()
			m.Deployment = &models.Deployment{
				UniqueLocations:       tt.locations,
				Machines:              make(map[string]*models.Machine),
				SSHPrivateKeyMaterial: string(testSSHPrivateKeyMaterial),
				SSHPort:               66000,
				SSHUser:               "fake-user",
			}

			// Add machines to the deployment
			for _, location := range tt.locations {
				for i := 0; i < tt.machinesPerLocation; i++ {
					machineName := fmt.Sprintf("machine-%s-%d", location, i)
					m.Deployment.Machines[machineName] = &models.Machine{
						Name:     machineName,
						Location: location,
					}
				}
			}

			oldNewSSHConfigFunc := sshutils.NewSSHConfigFunc
			fakeOrchestratorIP := "1.2.3.4"
			mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything).Return(nil)
			mockSSHConfig.On("PushFile", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			mockSSHConfig.On("ExecuteCommand", mock.Anything, "sudo docker version -f json").
				Return(`{"Client":{"Version":"1.2.3"},"Server":{"Version":"1.2.3"}}`, nil)
			mockSSHConfig.On("InstallSystemdService", mock.Anything, mock.Anything, mock.Anything).
				Return(nil)
			mockSSHConfig.On("RestartService", mock.Anything, mock.Anything).Return(nil)
			mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.MatchedBy(func(command string) bool {
				return command == fmt.Sprintf(
					"bacalhau node list --output json --api-host %s",
					fakeOrchestratorIP,
				) ||
					command == "bacalhau node list --output json --api-host 0.0.0.0"
			})).
				Return(`[{"id": "node1"}]`, nil)
			mockSSHConfig.On("ExecuteCommand", mock.Anything, mock.Anything).Return("", nil)
			sshutils.NewSSHConfigFunc = func(host string,
				port int,
				user string,
				privateKeyMaterial []byte) (sshutils.SSHConfiger, error) {
				return mockSSHConfig, nil
			}
			defer func() {
				sshutils.NewSSHConfigFunc = oldNewSSHConfigFunc
			}()

			// Call DeployARMTemplate
			err := provider.DeployARMTemplate(context.Background())

			if tt.expectedError != "" {
				assert.ErrorContains(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				mockAzureClient.AssertExpectations(t)
			}

		})
	}
}

type MockResourceGraphClient struct {
	mock.Mock
}

func TestPollAndUpdateResources(t *testing.T) {
	// Create a mock configuration
	mockConfig := viper.New()
	mockConfig.Set("azure.subscription_id", "test-subscription-id")

	// Define the required resources for each machine
	requiredResources := []models.AzureResourceTypes{
		{ResourceString: "Microsoft.Compute/virtualMachines", ShortResourceName: "VM"},
		{ResourceString: "Microsoft.Network/publicIPAddresses", ShortResourceName: "PublicIP"},
		{ResourceString: "Microsoft.Network/networkInterfaces", ShortResourceName: "NetworkInterface"},
		{ResourceString: "Microsoft.Network/virtualNetworks", ShortResourceName: "VirtualNetwork"},
		{ResourceString: "Microsoft.Network/networkSecurityGroups", ShortResourceName: "NetworkSecurityGroup"},
		{ResourceString: "Microsoft.Storage/storageAccounts", ShortResourceName: "StorageAccount"},
	}

	// Set up test machines
	testMachines := []string{"vm1", "vm2", "vm3"}

	// Set up the global model
	m := display.GetGlobalModelFunc()
	m.Deployment = &models.Deployment{
		Name:     "test-deployment",
		Machines: make(map[string]*models.Machine),
	}

	for _, machineName := range testMachines {
		m.Deployment.Machines[machineName] = &models.Machine{
			Name: machineName,
		}
	}

	// Create a provider with a mock client
	provider := &AzureProvider{
		Client:      &MockAzureClient{},
		Config:      mockConfig,
		updateQueue: make(chan UpdateAction, 100),
	}

	// Start the update processor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go provider.startUpdateProcessor(ctx)

	// Run multiple iterations to simulate resource state changes
	iterations := 5
	for i := 0; i < iterations; i++ {
		mockResponse := []interface{}{}

		// Generate mock response for each machine and its resources
		for _, machineName := range testMachines {
			for _, resourceType := range requiredResources {
				resource := generateMockResource(machineName, resourceType)
				mockResponse = append(mockResponse, resource)
			}
		}

		// Set up mock client expectation
		provider.Client.(*MockAzureClient).On("GetResources", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(mockResponse, nil).Once()

		// Call the function
		resources, err := provider.PollAndUpdateResources(ctx)

		// Check the results
		assert.NoError(t, err)
		assert.Equal(t, mockResponse, resources)

		// Wait for updates to be processed
		time.Sleep(100 * time.Millisecond)

		// Verify machine states
		for _, machineName := range testMachines {
			machine := m.Deployment.Machines[machineName]
			for _, resourceType := range requiredResources {
				state := machine.GetResourceState(resourceType.ShortResourceName)
				assert.Contains(t, []string{"", "Creating", "Updating", "Succeeded", "Failed"}, state)
				
				switch resourceType.ShortResourceName {
				case "IP":
					if state == "Succeeded" {
						assert.NotEmpty(t, machine.PublicIP)
					}
				case "NIC":
					if state == "Succeeded" {
						assert.NotEmpty(t, machine.PrivateIP)
					}
				case "VM":
					if state == "Succeeded" {
						assert.NotEmpty(t, machine.VMSize)
					}
				case "DISK":
					if state == "Succeeded" {
						assert.NotEmpty(t, machine.GetMachineResources()[resourceType.ShortResourceName].ResourceValue)
					}
				case "VNET", "SNET", "NSG":
					if state == "Succeeded" {
						assert.NotEmpty(t, machine.GetMachineResources()[resourceType.ShortResourceName].ResourceValue)
					}
				}
			}
		}
	}

	// Verify that the mock expectations were met
	provider.Client.(*MockAzureClient).AssertExpectations(t)
}

func generateMockResource(machineName string, resourceType models.AzureResourceTypes) map[string]interface{} {
	states := []string{"Creating", "Updating", "Succeeded", "Failed"}
	state := states[rand.Intn(len(states))]

	resource := map[string]interface{}{
		"name":              fmt.Sprintf("%s-%s", machineName, resourceType.ShortResourceName),
		"type":              resourceType.ResourceString,
		"provisioningState": state,
	}

	switch resourceType.ShortResourceName {
	case "VM":
		resource["properties"] = map[string]interface{}{
			"hardwareProfile": map[string]interface{}{
				"vmSize": "Standard_DS1_v2",
			},
		}
	case "NIC":
		if state == "Succeeded" {
			resource["properties"] = map[string]interface{}{
				"ipConfigurations": []interface{}{
					map[string]interface{}{
						"properties": map[string]interface{}{
							"privateIPAddress": fmt.Sprintf("10.0.0.%d", rand.Intn(255)),
						},
					},
				},
			}
		}
	case "VNET":
		resource["properties"] = map[string]interface{}{
			"addressSpace": map[string]interface{}{
				"addressPrefixes": []string{"10.0.0.0/16"},
			},
		}
	case "SNET":
		resource["properties"] = map[string]interface{}{
			"addressPrefix": "10.0.0.0/24",
		}
	case "NSG":
		resource["properties"] = map[string]interface{}{
			"securityRules": []interface{}{
				map[string]interface{}{
					"name": "AllowSSH",
					"properties": map[string]interface{}{
						"protocol":                 "Tcp",
						"sourcePortRange":          "*",
						"destinationPortRange":     "22",
						"sourceAddressPrefix":      "*",
						"destinationAddressPrefix": "*",
						"access":                   "Allow",
						"priority":                 1000,
						"direction":                "Inbound",
					},
				},
			},
		}
	case "DISK":
		resource["properties"] = map[string]interface{}{
			"diskSizeGB": 30,
			"diskState":  "Attached",
		}
	case "IP":
		if state == "Succeeded" {
			resource["properties"] = map[string]interface{}{
				"ipAddress": fmt.Sprintf("1.2.3.%d", rand.Intn(255)),
			}
		}
	}

	return resource
}
