package azure

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

func TestChannelClosing(t *testing.T) {
	// Create a mock deployment
	deployment := &models.Deployment{
		Machines: []models.Machine{
			{ID: "1", Name: "vm1"},
			{ID: "2", Name: "vm2"},
			{ID: "3", Name: "vm3"},
		},
	}

	// Initialize the display
	disp := display.NewDisplay()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Simulate deployment
	for _, machine := range deployment.Machines {
		wg.Add(1)
		go func(m models.Machine) {
			defer wg.Done()
			// Simulate deployment process
			time.Sleep(100 * time.Millisecond)
			disp.UpdateStatus(&models.Status{
				ID:     m.Name,
				Status: "Deployed",
			})
		}(machine)
	}

	// Wait for all deployments to finish
	wg.Wait()

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after deployment")
	}
}

func TestCancelledDeployment(t *testing.T) {
	// Create a mock deployment
	deployment := &models.Deployment{
		Machines: []models.Machine{
			{ID: "1", Name: "vm1"},
			{ID: "2", Name: "vm2"},
			{ID: "3", Name: "vm3"},
		},
	}

	// Create a context with cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Initialize the display
	disp := display.NewDisplay()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Simulate deployment with cancellation
	for _, machine := range deployment.Machines {
		wg.Add(1)
		go func(m models.Machine) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				disp.UpdateStatus(&models.Status{
					ID:     m.Name,
					Status: "Cancelled",
				})
			case <-time.After(500 * time.Millisecond):
				disp.UpdateStatus(&models.Status{
					ID:     m.Name,
					Status: "Deployed",
				})
			}
		}(machine)
	}

	// Cancel the deployment after a short delay
	time.AfterFunc(200*time.Millisecond, cancel)

	// Wait for all deployments to finish or be cancelled
	wg.Wait()

	// Stop the display
	disp.Stop()

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after cancelled deployment")
	}
}
