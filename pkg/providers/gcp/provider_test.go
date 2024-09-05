package gcp
package gcp

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/stretchr/testify/assert"
)

func TestRandomServiceUpdates(t *testing.T) {
	// Create a new provider
	provider := newMockGCPProvider(t)

	// Create a deployment with 3 machines
	deployment := &models.Deployment{
		Machines: map[string]*models.Machine{
			"machine1": {Name: "machine1"},
			"machine2": {Name: "machine2"},
			"machine3": {Name: "machine3"},
		},
	}

	// Set up the global model
	m := &display.DisplayModel{
		Deployment: deployment,
	}
	display.SetGlobalModelFunc(func() *display.DisplayModel {
		return m
	})

	// Start the update processor
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go provider.ProcessUpdates(ctx)

	// Generate random updates
	for i := 0; i < 500; i++ {
		machineName := fmt.Sprintf("machine%d", rand.Intn(3)+1)
		serviceType := models.ServiceType{Name: fmt.Sprintf("service%d", rand.Intn(5)+1)}
		serviceState := models.ServiceState(rand.Intn(4))

		update := NewUpdateAction(machineName, UpdatePayload{
			UpdateType:   UpdateTypeService,
			ServiceType:  serviceType,
			ServiceState: serviceState,
		})

		provider.updateQueue <- update
		time.Sleep(1 * time.Millisecond) // Small delay between updates
	}

	// Wait for updates to be processed
	time.Sleep(200 * time.Millisecond)

	// Check if all machines have different service state combinations
	combinations := make(map[string]bool)
	for _, machine := range m.Deployment.Machines {
		combination := ""
		for _, service := range machine.Services {
			combination += fmt.Sprintf("%s:%d,", service.Type.Name, service.State)
		}
		combinations[combination] = true
	}

	assert.Equal(t, 3, len(combinations), "Not all machines have unique service state combinations")
}

func newMockGCPProvider(t *testing.T) *GCPProvider {
	return &GCPProvider{
		Client:      &mockGCPClient{},
		updateQueue: make(chan UpdateAction, UpdateQueueSize),
	}
}
