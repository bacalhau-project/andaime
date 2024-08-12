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

	// Initialize the display with the mock output
	simScreen := tcell.NewSimulationScreen("UTF-8")
	simScreen.Init()
	simScreen.Set

	disp := display.NewMockDisplay(simScreen)
	go disp.Start()

	// Create a wait group to wait for all goroutines to finish
	var wg sync.WaitGroup

	// Simulate deployment
	for _, machine := range deployment.Machines {
		wg.Add(1)
		go func(m models.Machine) {
			defer wg.Done()
			// Simulate deployment process
			time.Sleep(50 * time.Millisecond)
			disp.UpdateStatus(&models.Status{
				ID:     m.Name,
				Status: "Provisioning",
			})
			time.Sleep(50 * time.Millisecond)
			disp.UpdateStatus(&models.Status{
				ID:     m.Name,
				Status: "Deployed",
			})
		}(machine)
	}

	// Wait for all deployments to finish
	wg.Wait()

	// Stop the display and close all channels
	disp.Stop()
	utils.CloseAllChannels()

	// Allow some time for channels to close
	time.Sleep(100 * time.Millisecond)

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after mock deployment")
	}

	a, b, c := simScreen.GetContents()
	_ = a
	_ = b
	_ = c
	for _, machine := range deployment.Machines {
		if !strings.Contains("", machine.Name) || !strings.Contains("", "Deployed") {
			t.Errorf("Expected output for machine %s not found", machine.Name)
		}
	}
}

func TestMockCancelledDeployment(t *testing.T) {
	// Create a mock output buffer
	var mockOutput bytes.Buffer

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

	// Initialize the display with the mock output
	screen := tcell.NewSimulationScreen("")
	disp := display.NewMockDisplay(screen)
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
			case <-time.After(200 * time.Millisecond):
				disp.UpdateStatus(&models.Status{
					ID:     m.Name,
					Status: "Deployed",
				})
			}
		}(machine)
	}

	// Cancel the deployment after a short delay
	time.AfterFunc(100*time.Millisecond, cancel)

	// Wait for all deployments to finish or be cancelled
	wg.Wait()

	// Stop the display and close all channels
	disp.Stop()
	utils.CloseAllChannels()

	// Allow some time for channels to close
	time.Sleep(100 * time.Millisecond)

	// Check if all channels are closed
	if !utils.AreAllChannelsClosed() {
		t.Error("Not all channels were closed after mock cancelled deployment")
	}

	// Check if the mock output contains expected content
	outputStr := mockOutput.String()
	cancelledCount := 0
	for _, machine := range deployment.Machines {
		if strings.Contains(outputStr, machine.Name) && strings.Contains(outputStr, "Cancelled") {
			cancelledCount++
		}
	}
	if cancelledCount == 0 {
		t.Error("No machines were cancelled in the mock deployment")
	}
}
