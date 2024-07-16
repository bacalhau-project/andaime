package cmd

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/bacalhau-project/andaime/display"
	"github.com/spf13/cobra"
)

var testDisplayCmd = &cobra.Command{
	Use:   "testDisplay",
	Short: "Test the display functionality",
	Run:   runTestDisplay,
}

func getTestDisplayCmd() *cobra.Command {
	return testDisplayCmd
}

func runTestDisplay(cmd *cobra.Command, args []string) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	totalTasks := 20
	d := display.NewDisplay(totalTasks)

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		d.DebugLog.Debug("\nReceived interrupt, shutting down...")
		d.Stop()
		cancel()
	}()

	d.Start(sigChan)

	statusChan := make(chan *display.Status)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		generateEvents(ctx, statusChan, totalTasks)
	}()

	go func() {
		for {
			select {
			case status := <-statusChan:
				d.UpdateStatus(status)
			case <-ctx.Done():
				return
			}
		}
	}()

	// Wait for the context to be canceled (which happens when we receive an interrupt)
	<-ctx.Done()

	// Wait for goroutines to finish
	wg.Wait()

	// Wait for the display to fully stop
	d.WaitForStop()

	d.DebugLog.Debug("Exiting")
}

func generateEvents(ctx context.Context, statusChan chan<- *display.Status, totalTasks int) {
	statuses := make(map[string]*display.Status)
	ticker := time.NewTicker(500 * time.Millisecond) //nolint:gomnd
	defer ticker.Stop()

	for i := 0; i < totalTasks; i++ {
		newStatus := createRandomStatus()
		statuses[newStatus.ID] = newStatus
		statusChan <- newStatus
	}

	for {
		select {
		case <-ticker.C:
			if status := getRandomStatus(statuses); status != nil {
				updateRandomStatus(status)
				select {
				case statusChan <- status:
				case <-ctx.Done():
					return
				}
			}
		case <-ctx.Done():
			return
		}
	}
}

func createRandomStatus() *display.Status {
	id := fmt.Sprintf("i-%06d", rand.Intn(1000000))
	return &display.Status{
		ID:             id,
		Type:           "EC2",
		Region:         randomRegion(),
		Zone:           randomZone(),
		Status:         "Initializing",
		DetailedStatus: "Starting",
		ElapsedTime:    0,
		InstanceID:     id,
		PublicIP:       randomIP(),
		PrivateIP:      randomIP(),
	}
}

func updateRandomStatus(status *display.Status) {
	status.ElapsedTime += time.Duration(rand.Intn(10)) * time.Second
	status.Status = randomStatus()
	status.DetailedStatus = randomDetailedStatus(status.Status)
}

func getRandomStatus(statuses map[string]*display.Status) *display.Status {
	if len(statuses) == 0 {
		return nil
	}
	i := rand.Intn(len(statuses))
	for _, status := range statuses {
		if i == 0 {
			return status
		}
		i--
	}
	return nil
}

func randomRegion() string {
	regions := []string{"us-west-1", "us-west-2", "us-east-1", "us-east-2", "eu-west-1", "eu-central-1", "ap-southeast-1", "ap-northeast-1"}
	return regions[rand.Intn(len(regions))]
}

func randomZone() string {
	return "zone-" + string(rune('a'+rand.Intn(3)))
}

func randomStatus() string {
	statuses := []string{"Pending", "Running", "Stopping", "Stopped", "Terminated"}
	return statuses[rand.Intn(len(statuses))]
}

func randomDetailedStatus(status string) string {
	switch status {
	case "Pending":
		return "Launching"
	case "Running":
		return "Healthy"
	case "Stopping":
		return "Shutting down"
	case "Stopped":
		return "Powered off"
	case "Terminated":
		return "Deleted"
	default:
		return "Unknown"
	}
}

func randomIP() string {
	return fmt.Sprintf("%d.%d.%d.%d", rand.Intn(256), rand.Intn(256), rand.Intn(256), rand.Intn(256))
}
