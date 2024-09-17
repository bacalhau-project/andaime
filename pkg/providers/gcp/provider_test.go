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
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestRandomServiceUpdates(t *testing.T) {
	l := logger.Get()

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
			ProjectID: "test-project-id",
			Zone:      "us-central1-a",
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

	mockClient := new(MockGCPClient)
	provider := &GCPProvider{
		Client:      mockClient,
		updateQueue: make(chan display.UpdateAction, 100),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	processorDone := make(chan struct{})
	go func() {
		l.Debug("Update processor started")
		localModel.StartUpdateProcessor(ctx)
		processorDone <- struct{}{}
		l.Debug("Update processor finished")
	}()

	var wg sync.WaitGroup
	updatesSent := int32(0)
	updatesPerMachine := 1000 // Increase the number of updates per machine
	expectedUpdates := int32(len(testMachines) * updatesPerMachine)

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
				case localModel.UpdateQueue <- display.UpdateAction{
					MachineName: innerMachineName,
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
							innerMachineName,
							service.Name,
						),
					)
					return
				}
				time.Sleep(time.Duration(rng.Intn(20)) * time.Millisecond)
			}
		}(machineName)
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

	l.Debug("Test completed successfully")
}
