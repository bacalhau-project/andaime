package azure_test

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/smithy-go/ptr"
	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	// ... other imports
)

func TestProvider(t *testing.T) {
	// ... test implementation
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
