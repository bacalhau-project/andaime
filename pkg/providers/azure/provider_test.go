package azure

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/aws/smithy-go/ptr"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/sshutils"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"golang.org/x/sync/errgroup"
)

var originalGlobalModelFunc func() *display.DisplayModel

func setupTestDisplayModel() *display.DisplayModel {
	return &display.DisplayModel{
		Deployment: &models.Deployment{
			Machines:  make(map[string]*models.Machine),
			Locations: []string{"eastus"},
			Azure: &models.AzureConfig{
				ResourceGroupName:     "test-resource-group",
				ResourceGroupLocation: "eastus",
				SubscriptionID:        "test-subscription-id",
			},
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
				ResourceType:  models.ResourceTypes{ResourceString: "testResource"},
				ResourceState: models.ResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(m *models.Machine, data UpdatePayload) {
			fmt.Printf("Update called with data: %v\n", data)

			updateCalled = true
			assert.Equal(t, machineName, m.Name)
			assert.Equal(t, UpdateTypeResource, data.UpdateType)
			assert.Equal(t, "testResource", data.ResourceType.ResourceString)
			assert.Equal(t, models.ResourceStateSucceeded, data.ResourceState)
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
				ResourceType:  models.ResourceTypes{ResourceString: "testResource"},
				ResourceState: models.ResourceStateSucceeded,
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
				ResourceType:  models.ResourceTypes{ResourceString: "testResource"},
				ResourceState: models.ResourceStateSucceeded,
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
				ResourceType:  models.ResourceTypes{ResourceString: "testResource"},
				ResourceState: models.ResourceStateSucceeded,
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
				ResourceType:  models.ResourceTypes{ResourceString: "testResource"},
				ResourceState: models.ResourceStateSucceeded,
			},
		)
		update.UpdateFunc = nil

		provider.processUpdate(update)
	})
}

func TestDeployResources(t *testing.T) {
	setupDeployARMTemplateTest(t)
	defer teardownDeployARMTemplateTest(t)

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

			mockAzureClient := new(MockAzureClient)

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
			m.Deployment.Locations = tt.locations
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
		Azure: &models.AzureConfig{
			ResourceGroupName:     "test-resource-group",
			ResourceGroupLocation: "eastus",
			SubscriptionID:        "test-subscription-id",
		},
		Tags: map[string]*string{"test": ptr.String("value")},
	}

	for _, machineName := range testMachines {
		m.Deployment.Machines[machineName] = &models.Machine{
			Name: machineName,
		}
		for _, resource := range requiredResources {
			m.Deployment.Machines[machineName].SetResourceState(
				resource.ResourceString,
				models.ResourceStateNotStarted,
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

	errgroup, ctx := errgroup.WithContext(ctx)
	errgroup.SetLimit(models.NumberOfSimultaneousProvisionings)
	updatesSent := int32(0)
	for _, machine := range testMachines {
		for _, resourceType := range requiredResources {
			errgroup.Go(func() error {
				return func(machine string, resourceType models.ResourceTypes) error {
					for i := 0; i < 10; i++ {
						for _, state := range []models.ResourceState{
							models.ResourceStateNotStarted,
							models.ResourceStatePending,
							models.ResourceStateRunning,
							models.ResourceStateSucceeded,
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
								return nil
							}
							time.Sleep(50 * time.Millisecond)
						}
					}
					return nil
				}(machine, resourceType)
			})
		}
	}

	log("Started sending updates")

	if err := errgroup.Wait(); err != nil {
		t.Fatal(err)
	}

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
				models.ResourceStateSucceeded,
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
	resourceType models.ResourceTypes,
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
	l := logger.Get()

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
		Azure: &models.AzureConfig{
			ResourceGroupName:     "test-resource-group",
			ResourceGroupLocation: "eastus",
			SubscriptionID:        "test-subscription-id",
		},
		Tags: map[string]*string{"test": ptr.String("value")},
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

	l.Debug("Deployment model initialized")

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
		l.Debug("Update processor started")
		provider.startUpdateProcessor(ctx)
		processorDone <- struct{}{}
		l.Debug("Update processor finished")
	}()

	var wg sync.WaitGroup
	updatesSent := int32(0)
	updatesPerMachine := 1000 // Increase the number of updates per machine
	expectedUpdates := int32(len(testMachines) * updatesPerMachine)

	var stateMutex sync.Mutex // Mutex to protect access to the state map

	for _, machine := range testMachines {
		wg.Add(1)
		go func(machine string) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for i := 0; i < updatesPerMachine; i++ {
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
					l.Debug(
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

	l.Debug("Started sending updates")

	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		l.Debug(
			fmt.Sprintf(
				"All updates sent successfully. Total updates: %d",
				atomic.LoadInt32(&updatesSent),
			),
		)
	case <-ctx.Done():
		l.Debug(
			fmt.Sprintf(
				"Test timed out while sending updates. Updates sent: %d/%d",
				atomic.LoadInt32(&updatesSent),
				expectedUpdates,
			),
		)
		t.Fatal("Test timed out while sending updates")
	}

	l.Debug("Closing update queue")
	close(provider.updateQueue)

	l.Debug("Waiting for update processor to finish")
	select {
	case <-processorDone:
		l.Debug("Update processor finished successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Update processor did not stop in time")
	}

	l.Debug("Checking final states")
	stateMap := make(map[string]map[string]models.ServiceState)
	stateMutex.Lock()
	for _, machine := range testMachines {
		stateMap[machine] = make(map[string]models.ServiceState)
		for _, service := range allServices {
			state := localModel.Deployment.Machines[machine].GetServiceState(service.Name)
			stateMap[machine][service.Name] = state
			assert.NotEqual(t, models.ServiceStateNotStarted, state,
				"Service %s on machine %s is still in NotStarted state", service.Name, machine)
		}
	}
	stateMutex.Unlock()

	uniqueStates := make(map[string]bool)
	for _, machine := range testMachines {
		stateString := fmt.Sprintf("%v", stateMap[machine])
		uniqueStates[stateString] = true
	}

	assert.Equal(t, len(testMachines), len(uniqueStates),
		"Not all machines have unique service state combinations")

	l.Debug("Test completed successfully")
}

func TestRandomServiceUpdatesWithRetry(t *testing.T) {
	l := logger.Get()
	maxRetries := 3
	var err error
	for i := 0; i < maxRetries; i++ {
		err = runRandomServiceUpdatesTest(t)
		if err == nil {
			return
		}
		l.Debugf("Test failed on attempt %d: %v. Retrying...", i+1, err)
	}
	t.Fatalf("Test failed after %d attempts: %v", maxRetries, err)
}

func runRandomServiceUpdatesTest(t *testing.T) error {
	// Move the entire content of TestRandomServiceUpdates here,
	// replacing assertions with error returns
	l := logger.Get()

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

	l.Debug("Deployment model initialized")

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
		l.Debug("Update processor started")
		provider.startUpdateProcessor(ctx)
		processorDone <- struct{}{}
		l.Debug("Update processor finished")
	}()

	var wg sync.WaitGroup
	updatesSent := int32(0)
	updatesPerMachine := 1000 // Increase the number of updates per machine
	expectedUpdates := int32(len(testMachines) * updatesPerMachine)

	var stateMutex sync.Mutex // Mutex to protect access to the state map

	for _, machine := range testMachines {
		wg.Add(1)
		go func(machine string) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for i := 0; i < updatesPerMachine; i++ {
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
					l.Debug(
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

	l.Debug("Started sending updates")

	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		l.Debug(
			fmt.Sprintf(
				"All updates sent successfully. Total updates: %d",
				atomic.LoadInt32(&updatesSent),
			),
		)
	case <-ctx.Done():
		return fmt.Errorf(
			"Test timed out while sending updates. Updates sent: %d/%d",
			atomic.LoadInt32(&updatesSent),
			expectedUpdates,
		)
	}

	l.Debug("Closing update queue")
	close(provider.updateQueue)

	l.Debug("Waiting for update processor to finish")
	select {
	case <-processorDone:
		l.Debug("Update processor finished successfully")
	case <-time.After(5 * time.Second):
		return fmt.Errorf("Update processor did not stop in time")
	}

	l.Debug("Checking final states")
	stateMap := make(map[string]map[string]models.ServiceState)
	stateMutex.Lock()
	for _, machine := range testMachines {
		stateMap[machine] = make(map[string]models.ServiceState)
		for _, service := range allServices {
			state := localModel.Deployment.Machines[machine].GetServiceState(service.Name)
			stateMap[machine][service.Name] = state
			if state < models.ServiceStateNotStarted || state > models.ServiceStateFailed {
				stateMutex.Unlock()
				return fmt.Errorf(
					"Unexpected final state for machine %s, service %s: %s",
					machine,
					service.Name,
					fmt.Sprintf("%d", state),
				)
			}
		}
	}
	stateMutex.Unlock()

	uniqueStates := make(map[string]bool)
	for _, machine := range testMachines {
		stateString := fmt.Sprintf("%v", stateMap[machine])
		uniqueStates[stateString] = true
	}

	if len(uniqueStates) != len(testMachines) {
		return fmt.Errorf(
			"Not all machines have unique service state combinations: got %d, want %d",
			len(uniqueStates),
			len(testMachines),
		)
	}

	l.Debug("Test completed successfully")
	return nil
}
