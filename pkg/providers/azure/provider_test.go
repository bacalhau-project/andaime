package azure

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/aws/smithy-go/ptr"
	"github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var originalGlobalModelFunc func() *display.DisplayModel

func resetTestDisplayModel() *display.DisplayModel {
	return &display.DisplayModel{
		Deployment: &models.Deployment{
			Machines: make(map[string]*models.Machine),
		},
	}
}

func setupUpdateTest(t *testing.T) {
	t.Helper()
	originalGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return resetTestDisplayModel()
	}
}

func teardownTest(t *testing.T) {
	t.Helper()
	display.GetGlobalModelFunc = originalGlobalModelFunc
}

func TestProcessUpdate(t *testing.T) {
	setupUpdateTest(t)
	defer teardownTest(t)

	t.Run("ValidUpdate", func(t *testing.T) {
		provider := &AzureProvider{}
		machineName := "testMachine"
		originalGlobalModelFunc = display.GetGlobalModelFunc
		display.GetGlobalModelFunc = func() *display.DisplayModel {
			m := resetTestDisplayModel()
			m.Deployment.Machines[machineName] = &models.Machine{
				Name: machineName,
			}
			return m
		}
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()
		updateCalled := false
		update := UpdateAction{
			MachineName: machineName,
			UpdateData: UpdatePayload{
				UpdateType:    UpdateTypeResource,
				ResourceType:  models.AzureResourceTypes{ResourceString: "testResource"},
				ResourceState: models.AzureResourceStateSucceeded,
			},
			UpdateFunc: func(m *models.Machine, data UpdatePayload) {
				fmt.Printf("Update called with data: %v\n", data)

				updateCalled = true
				assert.Equal(t, machineName, m.Name)
				assert.Equal(t, UpdateTypeResource, data.UpdateType)
				assert.Equal(t, "testResource", data.ResourceType.ResourceString)
				assert.Equal(t, models.AzureResourceStateSucceeded, data.ResourceState)
			},
		}

		provider.processUpdate(update)
		assert.True(t, updateCalled, "Update function should have been called")
	})

	t.Run("NilGlobalModel", func(t *testing.T) {
		provider := &AzureProvider{}

		originalGlobalModelFunc := display.GetGlobalModelFunc
		display.GetGlobalModelFunc = func() *display.DisplayModel { return nil }
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()

		update := UpdateAction{
			MachineName: "testMachine",
			UpdateFunc: func(m *models.Machine, data UpdatePayload) {
				t.Error("Update function should not have been called")
			},
		}

		provider.processUpdate(update)
	})

	t.Run("NilDeployment", func(t *testing.T) {
		provider := &AzureProvider{}

		resetTestDisplayModel().Deployment = nil

		update := UpdateAction{
			MachineName: "testMachine",
			UpdateFunc: func(m *models.Machine, data UpdatePayload) {
				t.Error("Update function should not have been called")
			},
		}

		provider.processUpdate(update)
	})

	t.Run("NonExistentMachine", func(t *testing.T) {
		provider := &AzureProvider{}

		update := UpdateAction{
			MachineName: "nonExistentMachine",
			UpdateFunc: func(m *models.Machine, data UpdatePayload) {
				t.Error("Update function should not have been called")
			},
		}

		provider.processUpdate(update)
	})

	t.Run("NilUpdateFunc", func(t *testing.T) {
		provider := &AzureProvider{}

		machineName := "testMachine"
		display.GetGlobalModelFunc().Deployment.Machines[machineName] = &models.Machine{
			Name: machineName,
		}

		update := UpdateAction{
			MachineName: machineName,
			UpdateFunc:  nil,
		}

		provider.processUpdate(update)
	})
}

func TestDeployARMTemplate(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)

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

			mockAzureClient := &MockAzureClient{}

			provider := &AzureProvider{
				Client: mockAzureClient,
			}

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

					if err == nil {
						setupMockVMAndNetwork(mockAzureClient)
					}
				}
			}

			display.SetGlobalModel(display.InitialModel())
			m := display.GetGlobalModelFunc()
			m.Deployment = &models.Deployment{
				UniqueLocations:       tt.locations,
				Machines:              make(map[string]*models.Machine),
				SSHPrivateKeyMaterial: string(testSSHPrivateKeyMaterial),
				SSHPort:               66000,
				SSHUser:               "fake-user",
			}

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
	setupTest(t)
	defer teardownTest(t)

	t.Logf("TestPollAndUpdateResources started")
	start := time.Now()
	log := func(msg string) {
		t.Logf("%v: %s", time.Since(start).Round(time.Millisecond), msg)
	}

	log("Test started")
	mockConfig := viper.New()
	mockConfig.Set("azure.subscription_id", "test-subscription-id")

	requiredResources := models.GetAllAzureResources()
	testMachines := []string{"vm1", "vm2", "vm3"}

	m := display.GetGlobalModelFunc()
	m.Deployment = &models.Deployment{
		Name:     "test-deployment",
		Machines: make(map[string]*models.Machine),
		Tags:     map[string]*string{"test": ptr.String("value")},
	}

	for _, machineName := range testMachines {
		m.Deployment.Machines[machineName] = &models.Machine{
			Name: machineName,
		}
		for _, resource := range requiredResources {
			m.Deployment.Machines[machineName].SetResourceState(
				resource.ResourceString,
				models.AzureResourceStateNotStarted,
			)
		}
	}

	log("Deployment model initialized")

	mockClient := new(MockAzureClient)
	provider := &AzureProvider{
		Client:              mockClient,
		Config:              mockConfig,
		updateQueue:         make(chan UpdateAction, 100),
		servicesProvisioned: false,
	}

	defer mockClient.AssertExpectations(t)

	mockResponse := []map[string]interface{}{}
	for _, resource := range requiredResources {
		mockResponse = append(
			mockResponse,
			generateMockResource(resource.ShortResourceName, resource),
		)
	}

	log("Mock response generated")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	t.Logf("Starting update processor goroutine")
	processorDone := make(chan struct{})
	go func() {
		log("Update processor started")
		provider.startUpdateProcessor(ctx)
		processorDone <- struct{}{}
		t.Logf("Update processor goroutine finished")
	}()

	mockClient.On("GetResources", mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).
		Return(mockResponse, nil).
		Once()

	log("Calling PollAndUpdateResources")
	resources, err := provider.PollAndUpdateResources(ctx)

	assert.NoError(t, err)
	assert.Equal(t, mockResponse, resources)
	log("PollAndUpdateResources completed")

	var wg sync.WaitGroup
	updatesSent := int32(0)
	for _, machine := range testMachines {
		for _, resourceType := range requiredResources {
			wg.Add(1)
			go func(machine string, resourceType models.AzureResourceTypes) {
				defer wg.Done()
				for _, state := range []models.AzureResourceState{
					models.AzureResourceStateNotStarted,
					models.AzureResourceStatePending,
					models.AzureResourceStateRunning,
					models.AzureResourceStateSucceeded,
					models.AzureResourceStateFailed,
				} {
					t.Logf(
						"Sending update for machine %s, resource type %s",
						machine,
						resourceType.ResourceString,
					)
					select {
					case provider.updateQueue <- UpdateAction{
						MachineName: machine,
						UpdateData: UpdatePayload{
							UpdateType:    UpdateTypeResource,
							ResourceType:  resourceType,
							ResourceState: state,
						},
					}:
						atomic.AddInt32(&updatesSent, 1)
					case <-ctx.Done():
						t.Logf(
							"Context cancelled while sending update for %s, %s",
							machine,
							resourceType.ResourceString,
						)
						return
					}
					time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
				}
			}(machine, resourceType)
		}
	}

	log("Started sending updates")

	wg.Wait()
	log(
		fmt.Sprintf(
			"All updates sent successfully. Total updates: %d",
			atomic.LoadInt32(&updatesSent),
		),
	)

	log("Closing update queue")
	close(provider.updateQueue)

	log("TestPollAndUpdateResources completed")

	log("Waiting for update processor to finish")
	select {
	case <-processorDone:
		log("Update processor finished successfully")
	case <-time.After(5 * time.Second):
		log("Update processor did not stop in time")
		t.Fatal("Update processor did not stop in time")
	}

	log("Checking final states")
	for _, machine := range testMachines {
		for _, resourceType := range requiredResources {
			state := m.Deployment.Machines[machine].GetResourceState(resourceType.ResourceString)
			assert.Equal(
				t,
				models.AzureResourceStateFailed,
				state,
				"Unexpected final state for machine %s, resource %s",
				machine,
				resourceType.ResourceString,
			)
		}
	}

	log("Test completed successfully")
}

func generateMockResource(
	machineName string,
	resourceType models.AzureResourceTypes,
) map[string]interface{} {
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

func TestRandomServiceUpdates(t *testing.T) {
	setupTest(t)
	defer teardownTest(t)

	log := func(msg string) {
		t.Logf("%v: %s", time.Now().Format("15:04:05.000"), msg)
	}

	log("Test started")
	mockConfig := viper.New()
	mockConfig.Set("azure.subscription_id", "test-subscription-id")

	testMachines := []string{"vm1", "vm2", "vm3"}

	m := display.GetGlobalModelFunc()
	allServices := models.RequiredServices

	m.Deployment = &models.Deployment{
		Name:     "test-deployment",
		Machines: make(map[string]*models.Machine),
		Tags:     map[string]*string{"test": ptr.String("value")},
	}

	for _, machineName := range testMachines {
		m.Deployment.Machines[machineName] = &models.Machine{
			Name: machineName,
		}
		for _, service := range allServices {
			m.Deployment.Machines[machineName].SetServiceState(
				service.Name,
				models.ServiceStateNotStarted,
			)
		}
	}

	log("Deployment model initialized")

	mockClient := new(MockAzureClient)
	provider := &AzureProvider{
		Client:              mockClient,
		Config:              mockConfig,
		updateQueue:         make(chan UpdateAction, 100),
		servicesProvisioned: false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processorDone := make(chan struct{})
	go func() {
		log("Update processor started")
		provider.startUpdateProcessor(ctx)
		processorDone <- struct{}{}
		log("Update processor finished")
	}()

	var wg sync.WaitGroup
	updatesSent := int32(0)
	expectedUpdates := int32(len(testMachines) * 20) // 20 updates per machine

	for _, machine := range testMachines {
		wg.Add(1)
		go func(machine string) {
			defer wg.Done()
			for i := 0; i < 20; i++ { // Send 20 updates per machine
				service := allServices[rand.Intn(len(allServices))]
				state := models.ServiceState(rand.Intn(int(models.ServiceStateFailed) + 1))

				select {
				case provider.updateQueue <- UpdateAction{
					MachineName: machine,
					UpdateData: UpdatePayload{
						UpdateType:   UpdateTypeService,
						ServiceType:  service,
						ServiceState: state,
					},
				}:
					atomic.AddInt32(&updatesSent, 1)
				case <-ctx.Done():
					log(
						fmt.Sprintf(
							"Context cancelled while sending update for %s, %s",
							machine,
							service.Name,
						),
					)
					return
				}
				time.Sleep(time.Duration(rand.Intn(10)) * time.Millisecond)
			}
		}(machine)
	}

	log("Started sending updates")

	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		log(
			fmt.Sprintf(
				"All updates sent successfully. Total updates: %d",
				atomic.LoadInt32(&updatesSent),
			),
		)
	case <-ctx.Done():
		log(
			fmt.Sprintf(
				"Test timed out while sending updates. Updates sent: %d/%d",
				atomic.LoadInt32(&updatesSent),
				expectedUpdates,
			),
		)
		t.Fatal("Test timed out while sending updates")
	}

	log("Closing update queue")
	close(provider.updateQueue)

	log("Waiting for update processor to finish")
	select {
	case <-processorDone:
		log("Update processor finished successfully")
	case <-time.After(5 * time.Second):
		log("Update processor did not stop in time")
		t.Fatal("Update processor did not stop in time")
	}

	log("Checking final states")
	for _, machine := range testMachines {
		for _, service := range allServices {
			state := m.Deployment.Machines[machine].GetServiceState(service.Name)
			assert.True(
				t,
				state >= models.ServiceStateNotStarted && state <= models.ServiceStateFailed,
				"Unexpected final state for machine %s, service %s: %v",
				machine,
				service,
				state,
			)
		}
	}

	// Check for state bleed-over
	for i, machine1 := range testMachines {
		for j, machine2 := range testMachines {
			if i != j {
				for _, service := range allServices {
					assert.NotEqual(
						t,
						m.Deployment.Machines[machine1].GetServiceState(service.Name),
						m.Deployment.Machines[machine2].GetServiceState(service.Name),
						"State bleed-over detected between %s and %s for service %s",
						machine1,
						machine2,
						service.Name,
					)
				}
			}
		}
	}

	log("Test completed successfully")
}
