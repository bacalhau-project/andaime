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
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

var originalGlobalModelFunc func() *display.DisplayModel

func setupTestDisplayModel() *display.DisplayModel {
	return &display.DisplayModel{
		Deployment: &models.Deployment{
			Machines:  make(map[string]*models.Machine),
			Locations: []string{"eastus"},
		},
	}
}

var updateTestDisplayModel *display.DisplayModel
var deployARMTemplateTestDisplayModel *display.DisplayModel

func setupUpdateTest(t *testing.T) {
	t.Helper()
	updateTestDisplayModel = setupTestDisplayModel()
	originalGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return updateTestDisplayModel
	}
}

func setupDeployARMTemplateTest(t *testing.T) {
	t.Helper()
	deployARMTemplateTestDisplayModel = setupTestDisplayModel()
	originalGlobalModelFunc = display.GetGlobalModelFunc
	display.GetGlobalModelFunc = func() *display.DisplayModel {
		return deployARMTemplateTestDisplayModel
	}
}

func teardownTest(t *testing.T) {
	t.Helper()
	display.GetGlobalModelFunc = originalGlobalModelFunc
}

func teardownDeployARMTemplateTest(t *testing.T) {
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
			m := setupTestDisplayModel()
			m.Deployment.Machines[machineName] = &models.Machine{
				Name: machineName,
			}
			return m
		}
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()
		updateCalled := false
		update := NewUpdateAction(
			machineName,
			UpdatePayload{
				UpdateType:    UpdateTypeResource,
				ResourceType:  models.AzureResourceTypes{ResourceString: "testResource"},
				ResourceState: models.AzureResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(m *models.Machine, data UpdatePayload) {
			fmt.Printf("Update called with data: %v\n", data)

			updateCalled = true
			assert.Equal(t, machineName, m.Name)
			assert.Equal(t, UpdateTypeResource, data.UpdateType)
			assert.Equal(t, "testResource", data.ResourceType.ResourceString)
			assert.Equal(t, models.AzureResourceStateSucceeded, data.ResourceState)
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

		update := NewUpdateAction(
			"testMachine",
			UpdatePayload{
				UpdateType:    UpdateTypeResource,
				ResourceType:  models.AzureResourceTypes{ResourceString: "testResource"},
				ResourceState: models.AzureResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(m *models.Machine, data UpdatePayload) {
			t.Error("Update function should not have been called")
		}

		provider.processUpdate(update)
	})

	t.Run("NilDeployment", func(t *testing.T) {
		provider := &AzureProvider{}

		originalGlobalModelFunc = display.GetGlobalModelFunc
		display.GetGlobalModelFunc = func() *display.DisplayModel { return nil }
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()

		update := NewUpdateAction(
			"testMachine",
			UpdatePayload{
				UpdateType:    UpdateTypeResource,
				ResourceType:  models.AzureResourceTypes{ResourceString: "testResource"},
				ResourceState: models.AzureResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(m *models.Machine, data UpdatePayload) {
			t.Error("Update function should not have been called")
		}

		provider.processUpdate(update)
	})

	t.Run("NonExistentMachine", func(t *testing.T) {
		provider := &AzureProvider{}

		update := NewUpdateAction(
			"nonExistentMachine",
			UpdatePayload{
				UpdateType:    UpdateTypeResource,
				ResourceType:  models.AzureResourceTypes{ResourceString: "testResource"},
				ResourceState: models.AzureResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(m *models.Machine, data UpdatePayload) {
			t.Error("Update function should not have been called")
		}

		provider.processUpdate(update)
	})

	t.Run("NilUpdateFunc", func(t *testing.T) {
		provider := &AzureProvider{}

		machineName := "testMachine"
		originalGlobalModelFunc = display.GetGlobalModelFunc
		display.GetGlobalModelFunc = func() *display.DisplayModel {
			m := setupTestDisplayModel()
			m.Deployment.Machines[machineName] = &models.Machine{
				Name: machineName,
			}
			return m
		}
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()

		update := NewUpdateAction(
			machineName,
			UpdatePayload{
				UpdateType:    UpdateTypeResource,
				ResourceType:  models.AzureResourceTypes{ResourceString: "testResource"},
				ResourceState: models.AzureResourceStateSucceeded,
			},
		)
		update.UpdateFunc = nil

		provider.processUpdate(update)
	})
}

func TestDeployResources(t *testing.T) {
	setupDeployARMTemplateTest(t)
	defer teardownDeployARMTemplateTest(t)

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
			expectedError: "deployment failed",
		},
		{
			name:                "Single location, later machine fails",
			locations:           []string{"eastus"},
			machinesPerLocation: 3,
			deployMachineErrors: map[string]error{
				"machine-eastus-2": errors.New("deployment failed"),
			},
			expectedError: "deployment failed",
		},
		{
			name:                "Multiple locations, machine in first location fails",
			locations:           []string{"eastus", "westus"},
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{
				"machine-eastus-1": errors.New("deployment failed"),
			},
			expectedError: "deployment failed",
		},
		{
			name:                "Multiple locations, machine in later location fails",
			locations:           []string{"eastus", "westus"},
			machinesPerLocation: 2,
			deployMachineErrors: map[string]error{
				"machine-westus-0": errors.New("deployment failed"),
			},
			expectedError: "deployment failed",
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

			m := setupTestDisplayModel()
			m.Deployment = &models.Deployment{
				UniqueLocations:       tt.locations,
				Machines:              make(map[string]*models.Machine),
				SSHPrivateKeyMaterial: string(testSSHPrivateKeyMaterial),
				SSHPort:               66000,
				SSHUser:               "fake-user",
			}

			originalGlobalModelFunc := display.GetGlobalModelFunc
			defer func() {
				display.GetGlobalModelFunc = originalGlobalModelFunc
			}()
			display.GetGlobalModelFunc = func() *display.DisplayModel {
				return m
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
			mockSSHConfig.On("WaitForSSH", mock.Anything, mock.Anything, mock.Anything).Return(nil)
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

			err := provider.DeployResources(context.Background())

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
	setupUpdateTest(t)
	defer teardownTest(t)
	l := logger.Get()

	start := time.Now()
	log := func(msg string) {
		l.Debugf("%v: %s", time.Since(start).Round(time.Millisecond), msg)
	}
	log("TestPollAndUpdateResources started")
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

	mockResponse := []interface{}{}
	for _, machine := range testMachines {
		for _, resource := range requiredResources {
			mockResponse = append(
				mockResponse,
				generateMockResource(machine, resource),
			)
		}
	}

	log("Mock response generated")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	log("Starting update processor goroutine")
	processorDone := make(chan struct{})
	go func() {
		log("Update processor started")
		provider.startUpdateProcessor(ctx)
		processorDone <- struct{}{}
		log("Update processor goroutine finished")
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
				for i := 0; i < 10; i++ {
					for _, state := range []models.AzureResourceState{
						models.AzureResourceStateNotStarted,
						models.AzureResourceStatePending,
						models.AzureResourceStateRunning,
						models.AzureResourceStateSucceeded,
					} {
						log(
							fmt.Sprintf(
								"Sending update for machine %s, resource type %s, state %d",
								machine,
								resourceType.ResourceString,
								state,
							),
						)
						select {
						case provider.updateQueue <- NewUpdateAction(
							machine,
							UpdatePayload{
								UpdateType:    UpdateTypeResource,
								ResourceType:  resourceType,
								ResourceState: state,
							},
						):
							atomic.AddInt32(&updatesSent, 1)
						case <-ctx.Done():
							log(
								fmt.Sprintf(
									"Context cancelled while sending update for %s, %s",
									machine,
									resourceType.ResourceString,
								),
							)
							return
						}
						time.Sleep(50 * time.Millisecond)
					}
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
				models.AzureResourceStateSucceeded,
				state,
				"Unexpected final state for machine %s, resource %s",
				machine,
				resourceType.ResourceString,
			)
			log(
				fmt.Sprintf(
					"Final state for machine %s, resource %s: %d",
					machine,
					resourceType.ResourceString,
					state,
				),
			)
		}
	}

	log("Test completed successfully")
}

func generateMockResource(
	machineName string,
	resourceType models.AzureResourceTypes,
) map[string]interface{} {
	resource := map[string]interface{}{
		"name":              fmt.Sprintf("%s-%s", machineName, resourceType.ShortResourceName),
		"type":              resourceType.ResourceString,
		"provisioningState": "Succeeded",
	}

	switch resourceType.ShortResourceName {
	case "VM":
		resource["properties"] = map[string]interface{}{
			"hardwareProfile": map[string]interface{}{
				"vmSize": "Standard_DS1_v2",
			},
		}
	case "NIC":
		resource["properties"] = map[string]interface{}{
			"ipConfigurations": []interface{}{
				map[string]interface{}{
					"properties": map[string]interface{}{
						"privateIPAddress": fmt.Sprintf("10.0.0.%d", rand.Intn(255)),
					},
				},
			},
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
		resource["properties"] = map[string]interface{}{
			"ipAddress": fmt.Sprintf("1.2.3.%d", rand.Intn(255)),
		}
	}

	return resource
}

func TestRandomServiceUpdates(t *testing.T) {
	// ... (keep existing test code)
}

func TestProcessMachinesConfig(t *testing.T) {
	tests := []struct {
		name           string
		machinesConfig []models.MachineConfig
		mockSkus       map[string][]string
		expectedError  string
	}{
		{
			name: "All SKUs available",
			machinesConfig: []models.MachineConfig{
				{Name: "vm1", Location: "eastus", VMSize: "Standard_D2_v2"},
				{Name: "vm2", Location: "westus", VMSize: "Standard_D4_v3"},
			},
			mockSkus: map[string][]string{
				"eastus": {"Standard_D2_v2", "Standard_D4_v3"},
				"westus": {"Standard_D2_v2", "Standard_D4_v3"},
			},
			expectedError: "",
		},
		{
			name: "SKU not available in location",
			machinesConfig: []models.MachineConfig{
				{Name: "vm1", Location: "eastus", VMSize: "Standard_D2_v2"},
				{Name: "vm2", Location: "westus", VMSize: "Standard_D4_v3"},
			},
			mockSkus: map[string][]string{
				"eastus": {"Standard_D2_v2"},
				"westus": {"Standard_D2_v2"},
			},
			expectedError: "SKU Standard_D4_v3 is not available in location westus",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockAzureClient)
			provider := &AzureProvider{
				Client: mockClient,
			}

			for location, skus := range tt.mockSkus {
				mockClient.On("ListAvailableSkus", mock.Anything, location).Return(skus, nil)
			}

			display.SetGlobalModel(display.InitialModel())
			err := provider.ProcessMachinesConfig(context.Background(), tt.machinesConfig)

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
				m := display.GetGlobalModelFunc()
				assert.Equal(t, len(tt.machinesConfig), len(m.Deployment.Machines))
				for _, config := range tt.machinesConfig {
					machine, exists := m.Deployment.Machines[config.Name]
					assert.True(t, exists)
					assert.Equal(t, config.Location, machine.Location)
					assert.Equal(t, config.VMSize, machine.Parameters["vmSize"])
				}
			}

			mockClient.AssertExpectations(t)
		})
	}
}

	// Create a new Viper instance for this test
	testConfig := viper.New()
	testConfig.Set("azure.subscription_id", "test-subscription-id")

	// Create a local display model for this test
	localModel := &display.DisplayModel{}
	origGetGlobalModel := display.GetGlobalModelFunc
	t.Cleanup(func() { display.GetGlobalModelFunc = origGetGlobalModel })
	display.GetGlobalModelFunc = func() *display.DisplayModel { return localModel }

	testMachines := []string{"vm1", "vm2", "vm3"}
	allServices := models.RequiredServices

	localModel.Deployment = &models.Deployment{
		Name:     "test-deployment",
		Machines: make(map[string]*models.Machine),
		Tags:     map[string]*string{"test": ptr.String("value")},
	}

	for _, machineName := range testMachines {
		localModel.Deployment.Machines[machineName] = &models.Machine{
			Name: machineName,
		}
		for _, service := range allServices {
			localModel.Deployment.Machines[machineName].SetServiceState(
				service.Name,
				models.ServiceStateNotStarted,
			)
		}
	}

	log("Deployment model initialized")

	mockClient := new(MockAzureClient)
	provider := &AzureProvider{
		Client:              mockClient,
		Config:              testConfig,
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
	expectedUpdates := int32(len(testMachines) * 100) // 100 updates per machine

	for _, machine := range testMachines {
		wg.Add(1)
		go func(machine string) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for i := 0; i < 100; i++ {
				service := allServices[rng.Intn(len(allServices))]
				newState := models.ServiceState(
					rng.Intn(int(models.ServiceStateFailed)-1) + 2,
				) // Exclude NotStarted state

				select {
				case provider.updateQueue <- NewUpdateAction(
					machine,
					UpdatePayload{
						UpdateType:   UpdateTypeService,
						ServiceType:  service,
						ServiceState: newState,
					},
				):
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
				time.Sleep(time.Duration(rng.Intn(20)) * time.Millisecond)
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
			state := localModel.Deployment.Machines[machine].GetServiceState(service.Name)
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

	// Check for unique states across machines and ensure no NotStarted states
	stateMap := make(map[string]map[string]models.ServiceState)
	for _, machine := range testMachines {
		stateMap[machine] = make(map[string]models.ServiceState)
		for _, service := range allServices {
			state := localModel.Deployment.Machines[machine].GetServiceState(service.Name)
			stateMap[machine][service.Name] = state
			assert.NotEqual(t, models.ServiceStateNotStarted, state,
				"Service %s on machine %s is still in NotStarted state", service.Name, machine)
		}
	}

	uniqueStates := make(map[string]bool)
	for _, machine := range testMachines {
		stateString := fmt.Sprintf("%v", stateMap[machine])
		uniqueStates[stateString] = true
	}

	assert.Equal(t, len(testMachines), len(uniqueStates),
		"Not all machines have unique service state combinations")

	log("Test completed successfully")
}
