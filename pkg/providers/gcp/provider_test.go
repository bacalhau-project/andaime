package gcp

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	gcp_interface "github.com/bacalhau-project/andaime/pkg/models/interfaces/gcp"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	gcp_mock "github.com/bacalhau-project/andaime/mocks/gcp"
)

func TestRandomServiceUpdates(t *testing.T) {
	l := logger.Get()
	l.Info("Starting TestRandomServiceUpdates")

	// Create a new Viper instance for this test
	testConfig := viper.New()
	testConfig.Set("gcp.project_id", "test-project-id")
	testConfig.Set("gcp.zone", "us-central1-a")
	testConfig.Set("general.ssh_public_key_path", "/path/to/public/key")
	testConfig.Set("general.ssh_private_key_path", "/path/to/private/key")
	testConfig.Set("general.ssh_user", "testuser")
	testConfig.Set("general.ssh_port", 22)
	testConfig.Set("general.project_prefix", "test-project")

	// Set the global Viper instance to our test config
	viper.Reset()
	viper.MergeConfigMap(testConfig.AllSettings())

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
		GCP: &models.GCPConfig{
			ProjectID:   "test-project-id",
			DefaultZone: "us-central1-a",
		},
		Tags:                 map[string]*string{"test": &[]string{"value"}[0]},
		SSHPublicKeyMaterial: "ssh-rsa AAAAB3NzaC1yc2EAAAADAQABAAABAQC0g+ZTxC7weoIJLUafOgrm+h...",
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

	mockClient := new(gcp_mock.MockGCPClienter)
	provider := &GCPProvider{
		updateQueue: make(chan display.UpdateAction, 100),
	}

	origClientFunc := NewGCPClientFunc
	NewGCPClientFunc = func(ctx context.Context, organizationID string) (gcp_interface.GCPClienter, func(), error) {
		return mockClient, func() {}, nil
	}
	defer func() {
		NewGCPClientFunc = origClientFunc
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processorDone := make(chan struct{})
	go func() {
		l.Debug("Update processor started")
		localModel.StartUpdateProcessor(ctx)
		close(processorDone)
		l.Debug("Update processor finished")
	}()

	l.Info("Setting up update sending goroutines")
	var wg sync.WaitGroup
	updatesSent := int32(0)
	updatesPerMachine := 20

	// Create a separate channel for sending updates
	updateChan := make(chan display.UpdateAction, 100)

	for _, machineName := range testMachines {
		wg.Add(1)
		go func(innerMachineName string) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			for i := 0; i < updatesPerMachine; i++ {
				service := allServices[rng.Intn(len(allServices))]
				newState := models.ServiceState(
					rng.Intn(int(models.ServiceStateFailed)-1) + 2,
				) // Exclude NotStarted state

				select {
				case updateChan <- display.UpdateAction{
					MachineName: innerMachineName,
					UpdateData: display.UpdateData{
						UpdateType:   display.UpdateTypeService,
						ServiceType:  display.ServiceType(service.Name),
						ServiceState: newState,
					},
				}:
					atomic.AddInt32(&updatesSent, 1)
				case <-ctx.Done():
					return
				}
				time.Sleep(time.Duration(rng.Intn(20)) * time.Millisecond)
			}
			l.Debugf("Finished sending updates for machine: %s", innerMachineName)
		}(machineName)
	}

	l.Info("Starting update forwarding goroutine")
	go func() {
		for update := range updateChan {
			select {
			case localModel.UpdateQueue <- update:
				l.Debugf(
					"Forwarded update for machine: %s, service: %s",
					update.MachineName,
					update.UpdateData.ServiceType,
				)
			case <-ctx.Done():
				l.Warn("Context cancelled while forwarding updates")
				return
			}
		}
		l.Debug("Update forwarding goroutine finished")
	}()

	l.Info("Waiting for all updates to be sent")
	go func() {
		wg.Wait()
		l.Info("All update senders finished, closing updateChan")
		close(updateChan)
	}()

	l.Info("Waiting for completion or timeout")
	select {
	case <-processorDone:
		l.Info("Update processor finished successfully")
	case <-time.After(5 * time.Second):
		l.Error("Update processor did not stop in time")
		t.Fatal("Update processor did not stop in time")
	}

	l.Info("Closing provider update queue")
	close(provider.updateQueue)

	l.Info("Waiting for update processor to finish")
	select {
	case <-processorDone:
		l.Info("Update processor finished successfully")
	case <-time.After(5 * time.Second):
		l.Error("Update processor did not stop in time")
		t.Fatal("Update processor did not stop in time")
	}

	l.Info("Checking final states")
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

	l.Info("Test completed successfully")
}
