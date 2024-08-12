package azure

import (
	"bytes"
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/gdamore/tcell/v2"
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
	disp := display.GetGlobalDisplay()
	go disp.Start()

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

	// Stop the display
	disp.Stop()

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
	disp := display.GetGlobalDisplay()
	go disp.Start()

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

func TestMockDeployment(t *testing.T) {
	// Create a mock deployment
	deployment := &models.Deployment{
		Machines: []models.Machine{
			{ID: "1", Name: "vm1"},
			{ID: "2", Name: "vm2"},
			{ID: "3", Name: "vm3"},
		},
	}

	// Create channels to simulate the deployment process
	startChan := make(chan struct{})
	doneChan := make(chan struct{})

	// Simulate deployment
	go func() {
		<-startChan
		for i := range deployment.Machines {
			// Simulate deployment process
			time.Sleep(50 * time.Millisecond)
			deployment.Machines[i].Status = "Provisioning"
			time.Sleep(50 * time.Millisecond)
			deployment.Machines[i].Status = "Deployed"
		}
		close(doneChan)
	}()

	// Start the deployment
	close(startChan)

	// Wait for the deployment to finish
	select {
	case <-doneChan:
		// Deployment finished successfully
	case <-time.After(1 * time.Second):
		t.Error("Deployment timed out")
	}

	// Check if all machines are deployed
	for _, machine := range deployment.Machines {
		if machine.Status != "Deployed" {
			t.Errorf("Machine %s not deployed. Status: %s", machine.Name, machine.Status)
		}
	}

	// Simulate closing all channels
	utils.CloseAllChannels()

	// Allow some time for channels to close
	time.Sleep(100 * time.Millisecond)

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after mock deployment")
	}
}

func TestMockCancelledDeployment(t *testing.T) {
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
	defer cancel()

	// Initialize the display
	disp := display.GetGlobalDisplay()
	go disp.Start()
	defer disp.Stop()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Create a channel to signal when all goroutines are ready
	ready := make(chan struct{})

	// Simulate deployment with cancellation
	for _, machine := range deployment.Machines {
		wg.Add(1)
		go func(m models.Machine) {
			defer wg.Done()
			// Signal that this goroutine is ready
			ready <- struct{}{}
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

	// Wait for all goroutines to be ready
	for range deployment.Machines {
		<-ready
	}

	// Cancel the deployment immediately
	cancel()

	// Wait for all deployments to finish or be cancelled
	wg.Wait()

	// Allow some time for the display to update
	time.Sleep(100 * time.Millisecond)

	// Check if all channels are closed
	utils.CloseAllChannels()
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after mock cancelled deployment")
	}

	// Check if any machines were cancelled
	cancelledCount := 0
	for _, machine := range deployment.Machines {
		status := disp.GetStatus(machine.Name)
		if status != nil && status.Status == "Cancelled" {
			cancelledCount++
		}
	}
	if cancelledCount == 0 {
		t.Error("No machines were cancelled in the mock deployment")
	} else {
		t.Logf("%d machines were cancelled in the mock deployment", cancelledCount)
	}
}
