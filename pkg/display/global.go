package display

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/utils"
	"github.com/briandowns/spinner"
)

var (
	globalDisplay *Display
	once          sync.Once
	shutdownChan  chan os.Signal
)

// GetGlobalDisplay returns the global Display instance
func GetGlobalDisplay() *Display {
	once.Do(func() {
		globalDisplay = NewDisplay(1)
		shutdownChan = utils.CreateSignalChannel(1)
	})
	return globalDisplay
}

// Start starts the global display
func Start(cancelChan <-chan struct{}) {
	go func() {
		<-cancelChan
		GracefulShutdown()
	}()
	GetGlobalDisplay().Start(shutdownChan)
}

// Stop stops the global display
func Stop() {
	utils.CloseChannel(shutdownChan)
	GetGlobalDisplay().Stop()
}

// GracefulShutdown handles the graceful shutdown process
func GracefulShutdown() {
	fmt.Println("\nGraceful shutdown initiated. Press Ctrl+C again to force exit.")
	fmt.Println("Warning: Forcing exit may result in partially deployed resources.")

	Stop()
	DisplayLongRunningTimer()
}

// DisplayLongRunningTimer displays a long-running timer with a spinner
func DisplayLongRunningTimer() {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	s.Suffix = " Deployment still running. Press Ctrl+C again to force exit."
	s.Start()

	startTime := time.Now()
	for {
		select {
		case <-shutdownChan:
			s.Stop()
			return
		default:
			elapsed := time.Since(startTime)
			s.Suffix = fmt.Sprintf(
				" Deployment still running (%.0fs). Press Ctrl+C again to force exit.",
				elapsed.Seconds(),
			)
			time.Sleep(1 * time.Second)
		}
	}
}

// UpdateStatus updates the status of a deployment or machine
func UpdateStatus(status *models.Status) {
	GetGlobalDisplay().UpdateStatus(status)
}
