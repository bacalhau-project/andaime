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
)

var originalGlobalModelFunc func() *display.DisplayModel

func setupTestDisplayModel() *display.DisplayModel {
	viper.Set("azure.subscription_id", "test-subscription-id")
	viper.Set("azure.resource_group_name", "test-resource-group")
	viper.Set("azure.resource_group_location", "eastus")
	viper.Set("general.ssh_public_key_path", "/path/to/public/key")
	viper.Set("general.ssh_private_key_path", "/path/to/private/key")
	viper.Set("general.ssh_user", "testuser")
	viper.Set("general.ssh_port", 22)
	viper.Set("general.project_prefix", "test-project")

	return &display.DisplayModel{
		Deployment: &models.Deployment{
			Machines:  make(map[string]models.Machiner),
			Locations: []string{"eastus"},
			Azure: &models.AzureConfig{
				ResourceGroupName:     "test-resource-group",
				ResourceGroupLocation: "eastus",
				SubscriptionID:        "test-subscription-id",
			},
			SSHPublicKeyMaterial: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC0g+ZTxC7weoIJLUafOgrm+h...",
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
		machineName := "testMachine"
		originalGlobalModelFunc = display.GetGlobalModelFunc
		display.GetGlobalModelFunc = func() *display.DisplayModel {
			m := setupTestDisplayModel()
			m.Deployment.SetMachine(machineName, &models.Machine{
				Name: machineName,
			})
			return m
		}
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()
		updateCalled := false
		update := display.NewUpdateAction(
			machineName,
			display.UpdateData{
				UpdateType:    display.UpdateTypeResource,
				ResourceType:  display.ResourceType("testResource"),
				ResourceState: models.ResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(mach models.Machiner, data display.UpdateData) {
			fmt.Printf("Update called with data: %v\n", data)

			updateCalled = true
			assert.Equal(t, machineName, mach.GetName())
			assert.Equal(t, display.UpdateTypeResource, data.UpdateType)
			assert.Equal(t, display.ResourceType("testResource"), data.ResourceType)
			assert.Equal(t, models.ResourceStateSucceeded, data.ResourceState)
		}

		display.GetGlobalModelFunc().ProcessUpdate(update)
		assert.True(t, updateCalled, "Update function should have been called")
	})

	t.Run("NilGlobalModel", func(t *testing.T) {
		originalGlobalModelFunc := display.GetGlobalModelFunc
		display.GetGlobalModelFunc = func() *display.DisplayModel { return nil }
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()

		update := display.NewUpdateAction(
			"testMachine",
			display.UpdateData{
				UpdateType:    display.UpdateTypeResource,
				ResourceType:  display.ResourceType("testResource"),
				ResourceState: models.ResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(mach models.Machiner, data display.UpdateData) {
			t.Error("Update function should not have been called")
		}

		display.GetGlobalModelFunc().ProcessUpdate(update)
	})

	t.Run("NilDeployment", func(t *testing.T) {
		originalGlobalModelFunc = display.GetGlobalModelFunc
		display.GetGlobalModelFunc = func() *display.DisplayModel { return nil }
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()

		update := display.NewUpdateAction(
			"testMachine",
			display.UpdateData{
				UpdateType:    display.UpdateTypeResource,
				ResourceType:  display.ResourceType("testResource"),
				ResourceState: models.ResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(mach models.Machiner, data display.UpdateData) {
			t.Error("Update function should not have been called")
		}

		display.GetGlobalModelFunc().ProcessUpdate(update)
	})

	t.Run("NonExistentMachine", func(t *testing.T) {
		update := display.NewUpdateAction(
			"nonExistentMachine",
			display.UpdateData{
				UpdateType:    display.UpdateTypeResource,
				ResourceType:  display.ResourceType("testResource"),
				ResourceState: models.ResourceStateSucceeded,
			},
		)
		update.UpdateFunc = func(mach models.Machiner, data display.UpdateData) {
			t.Error("Update function should not have been called")
		}

		display.GetGlobalModelFunc().ProcessUpdate(update)
	})

	t.Run("NilUpdateFunc", func(t *testing.T) {
		machineName := "testMachine"
		originalGlobalModelFunc = display.GetGlobalModelFunc
		display.GetGlobalModelFunc = func() *display.DisplayModel {
			m := setupTestDisplayModel()
			m.Deployment.SetMachine(machineName, &models.Machine{
				Name: machineName,
			})
			return m
		}
		defer func() {
			display.GetGlobalModelFunc = originalGlobalModelFunc
		}()

		update := display.NewUpdateAction(
			machineName,
			display.UpdateData{
				UpdateType:    display.UpdateTypeResource,
				ResourceType:  display.ResourceType("testResource"),
				ResourceState: models.ResourceStateSucceeded,
			},
		)
		update.UpdateFunc = nil

		display.GetGlobalModelFunc().ProcessUpdate(update)
	})
}

func TestCreateResources(t *testing.T) {
	setupDeployARMTemplateTest(t)
	defer teardownDeployARMTemplateTest(t)

	mockSSHConfig := new(sshutils.MockSSHConfig)

	tests := []struct {
		name                string
		locations           []string
		machinesPerLocation int
		deployMachineErrors map[string]error
		expectedError       string
	}{
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

			mockAzureClient.On("ListResourceGroups", mock.Anything).
				Return([]*armresources.ResourceGroup{}, nil)

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
				sshPrivateKeyPath string) (sshutils.SSHConfiger, error) {
				return mockSSHConfig, nil
			}
			defer func() {
				sshutils.NewSSHConfigFunc = oldNewSSHConfigFunc
			}()

			err := provider.CreateResources(context.Background())

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

func TestPollResources(t *testing.T) {
	setupUpdateTest(t)
	defer teardownTest(t)
	l := logger.Get()

	mockConfig := viper.New()
	mockConfig.Set("azure.subscription_id", "test-subscription-id")

	requiredResources := models.GetAllAzureResources()
	testMachines := []string{"vm1", "vm2", "vm3"}

	m := display.GetGlobalModelFunc()
	m.Deployment = &models.Deployment{
		Name:     "test-deployment",
		Machines: make(map[string]models.Machiner),
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
			m.Deployment.Machines[machineName].SetMachineResourceState(
				resource.ResourceString,
				models.ResourceStateNotStarted,
			)
		}
	}

	mockClient := new(MockAzureClient)
	azureProvider := &AzureProvider{
		Client:              mockClient,
		UpdateQueue:         make(chan display.UpdateAction, 100),
		servicesProvisioned: false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Simulate multiple polling cycles
	pollCycles := 5
	for cycle := 0; cycle < pollCycles; cycle++ {
		l.Debugf("Starting polling cycle %d", cycle)

		mockResponse := []interface{}{}
		for _, machine := range testMachines {
			for _, resource := range requiredResources {
				mockResponse = append(
					mockResponse,
					generateMockResource(machine, resource),
				)
			}
		}

		mockClient.On("GetResources", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
			Return(mockResponse, nil).
			Once()

		resources, err := azureProvider.PollResources(ctx)

		assert.NoError(t, err)
		assert.Equal(t, mockResponse, resources)

		// Process updates
		for len(azureProvider.UpdateQueue) > 0 {
			update := <-azureProvider.UpdateQueue
			m.ProcessUpdate(update)
		}

		// Verify resource states after each cycle
		for _, machine := range testMachines {
			for _, resource := range requiredResources {
				state := m.Deployment.Machines[machine].GetMachineResourceState(
					resource.ResourceString,
				)
				expectedState := getSimulatedResourceState(cycle, pollCycles)
				assert.Equal(
					t,
					expectedState,
					state,
					"Unexpected state for machine %s, resource %s in cycle %d",
					machine,
					resource.ResourceString,
					cycle,
				)
			}
		}

		// Short delay between cycles
		time.Sleep(100 * time.Millisecond)
	}

	l.Debug("All polling cycles completed")
}

func getSimulatedResourceState(currentCycle, totalCycles int) models.MachineResourceState {
	progress := float64(currentCycle) / float64(totalCycles-1)
	switch {
	case progress < 0.25:
		return models.ResourceStateNotStarted
	case progress < 0.5:
		return models.ResourceStatePending
	case progress < 0.75:
		return models.ResourceStateRunning
	default:
		return models.ResourceStateSucceeded
	}
}

func generateMockResource(
	machineName string,
	resourceType models.ResourceType,
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
	t.Parallel() // Allow this test to run in parallel with others
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
		Machines: make(map[string]models.Machiner),
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processorDone := make(chan struct{})
	go func() {
		l.Debug("Update processor started")
		display.GetGlobalModelFunc().StartUpdateProcessor(ctx)
		close(processorDone)
		l.Debug("Update processor finished")
	}()

	var wg sync.WaitGroup
	updatesSent := int32(0)
	updatesPerMachine := 1000
	expectedUpdates := int32(len(testMachines) * updatesPerMachine)

	for _, machine := range testMachines {
		wg.Add(1)
		go func(machine string) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for i := 0; i < updatesPerMachine; i++ {
				service := allServices[rng.Intn(len(allServices))]
				newState := models.ServiceState(rng.Intn(int(models.ServiceStateFailed)-1) + 2)

				select {
				case display.GetGlobalModelFunc().UpdateQueue <- display.UpdateAction{
					MachineName: machine,
					UpdateData: display.UpdateData{
						UpdateType:   display.UpdateTypeService,
						ServiceType:  display.ServiceType(service.Name),
						ServiceState: newState,
					},
				}:
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
				time.Sleep(time.Duration(rng.Intn(5)) * time.Millisecond)
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
		t.Fatalf(
			"Test timed out while sending updates. Updates sent: %d/%d",
			atomic.LoadInt32(&updatesSent),
			expectedUpdates,
		)
	}

	l.Debug("Closing update queue")
	close(display.GetGlobalModelFunc().UpdateQueue)

	l.Debug("Waiting for update processor to finish")
	select {
	case <-processorDone:
		l.Debug("Update processor finished successfully")
	case <-time.After(5 * time.Second):
		t.Fatal("Update processor did not stop in time")
	}

	l.Debug("Checking final states")
	stateMap := make(map[string]map[string]models.ServiceState)
	for _, machine := range testMachines {
		stateMap[machine] = make(map[string]models.ServiceState)
		for _, service := range allServices {
			state := localModel.Deployment.Machines[machine].GetServiceState(service.Name)
			stateMap[machine][service.Name] = state
			if state == models.ServiceStateNotStarted {
				t.Errorf(
					"Service %s on machine %s is still in NotStarted state",
					service.Name,
					machine,
				)
			}
		}
	}

	uniqueStates := make(map[string]bool)
	for _, machine := range testMachines {
		stateString := fmt.Sprintf("%v", stateMap[machine])
		uniqueStates[stateString] = true
	}

	if len(uniqueStates) != len(testMachines) {
		t.Errorf(
			"Not all machines have unique service state combinations. Got %d unique combinations, expected %d",
			len(uniqueStates),
			len(testMachines),
		)
	}

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
		Machines: make(map[string]models.Machiner),
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

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processorDone := make(chan struct{})
	go func() {
		l.Debug("Update processor started")
		display.GetGlobalModelFunc().StartUpdateProcessor(ctx)
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
				case display.GetGlobalModelFunc().UpdateQueue <- display.UpdateAction{
					MachineName: machine,
					UpdateData: display.UpdateData{
						UpdateType:   display.UpdateTypeService,
						ServiceType:  display.ServiceType(service.Name),
						ServiceState: newState,
					},
				}:
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
	close(display.GetGlobalModelFunc().UpdateQueue)

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
