package azure_test

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/suite"
)

type PkgProviderAzureProviderTestSuite struct {
	suite.Suite
	localModel *display.DisplayModel
	testConfig *viper.Viper
}

func TestPkgProviderAzureProviderTestSuite(t *testing.T) {
	suite.Run(t, new(PkgProviderAzureProviderTestSuite))
}

func (s *PkgProviderAzureProviderTestSuite) SetupTest() {
	s.testConfig = viper.New()
	s.testConfig.Set("azure.subscription_id", "test-subscription-id")

	s.localModel = &display.DisplayModel{}
	origGetGlobalModel := display.GetGlobalModelFunc
	s.T().Cleanup(func() { display.GetGlobalModelFunc = origGetGlobalModel })
	display.GetGlobalModelFunc = func() *display.DisplayModel { return s.localModel }
}

func (s *PkgProviderAzureProviderTestSuite) TestRandomServiceUpdates() {
	l := logger.Get()
	l.Debug("Starting TestRandomServiceUpdates")

	testMachines := []string{"vm1", "vm2", "vm3"}
	allServices := models.RequiredServices

	s.localModel.Deployment = &models.Deployment{
		Name:     "test-deployment",
		Machines: make(map[string]models.Machiner),
		Tags:     map[string]string{"test": "value"},
	}

	for _, machineName := range testMachines {
		s.localModel.Deployment.Machines[machineName] = &models.Machine{
			Name: machineName,
		}
		for _, service := range allServices {
			s.localModel.Deployment.Machines[machineName].SetServiceState(
				service.Name,
				models.ServiceStateNotStarted,
			)
		}
	}

	l.Debug("Deployment model initialized")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	updateChan := make(chan models.DisplayStatus, 300)
	defer close(updateChan)

	var wg sync.WaitGroup
	wg.Add(300)

	go func() {
		for range updateChan {
			wg.Done()
		}
	}()

	s.T().Log("Starting to send updates")
	go s.sendRandomUpdates(ctx, testMachines, updateChan)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		s.T().Errorf("Test timed out while sending updates")
	case <-done:
		s.T().Log("All updates processed successfully")
	}

	updatesSent := 300
	s.T().Logf("Updates sent: %d/300", updatesSent)
	s.Equal(300, updatesSent, "Not all updates were sent")

	s.verifyFinalStates(testMachines, allServices)

	l.Debug("Test completed successfully")
}

var numOfMachineUpdates = map[string]int{}

func (s *PkgProviderAzureProviderTestSuite) sendRandomUpdates(
	ctx context.Context,
	machines []string,
	updateChan chan<- models.DisplayStatus,
) {
	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 300; i++ {
		machine := machines[rng.Intn(len(machines))]
		service := models.RequiredServices[rng.Intn(len(models.RequiredServices))]
		newState := models.ServiceState(rng.Intn(int(models.ServiceStateFailed)-1) + 2)

		ds := models.DisplayStatus{
			ID: machine,
		}
		if service == models.ServiceTypeSSH {
			ds.SSH = newState
		} else if service == models.ServiceTypeDocker {
			ds.Docker = newState
		} else if service == models.ServiceTypeBacalhau {
			ds.Bacalhau = newState
		}

		select {
		case updateChan <- ds:
			numOfMachineUpdates[machine]++
		case <-ctx.Done():
			return
		}
		time.Sleep(time.Duration(rng.Intn(20)) * time.Millisecond)
	}
}

func (s *PkgProviderAzureProviderTestSuite) waitForUpdatesCompletion(
	ctx context.Context,
	wg *sync.WaitGroup,
	updatesSent *int32,
	expectedUpdates int32,
) {
	wgDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(wgDone)
	}()

	select {
	case <-wgDone:
		s.Require().
			Equal(expectedUpdates, atomic.LoadInt32(updatesSent), "Unexpected number of updates sent")
	case <-ctx.Done():
		s.Fail(
			"Test timed out while sending updates",
			"Updates sent: %d/%d",
			atomic.LoadInt32(updatesSent),
			expectedUpdates,
		)
	}
}

func (s *PkgProviderAzureProviderTestSuite) verifyFinalStates(
	testMachines []string,
	allServices []models.ServiceType,
) {
	stateMap := make(map[string]map[string]models.ServiceState)
	for _, machine := range testMachines {
		stateMap[machine] = make(map[string]models.ServiceState)
		for _, service := range allServices {
			state := s.localModel.Deployment.Machines[machine].GetServiceState(service.Name)
			stateMap[machine][service.Name] = state
			s.Require().
				GreaterOrEqual(int(state), int(models.ServiceStateNotStarted), "Unexpected final state for machine %s, service %s: %s", machine, service.Name, state)
			s.Require().
				LessOrEqual(int(state), int(models.ServiceStateFailed), "Unexpected final state for machine %s, service %s: %s", machine, service.Name, state)
		}
	}

	totalUpdates := 0
	for _, machine := range testMachines {
		totalUpdates += numOfMachineUpdates[machine]
	}

	s.Require().
		Equal(totalUpdates, 300, "Not all updates were sent")
}
