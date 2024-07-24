package beta

import (
	"fmt"
	"os"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/spf13/cobra"
)

func newTestDisplayCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "testDisplay",
		Short: "Test the display functionality",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTestDisplay()
		},
	}

	return cmd
}

func runTestDisplay() error {
	totalTasks := 5
	testDisplay := display.NewTestDisplay(totalTasks)
	sigChan := make(chan os.Signal, 1)

	go testDisplay.Start(sigChan)

	for i := 0; i < totalTasks; i++ {
		status := &display.Status{
			ID:     fmt.Sprintf("test%d", i+1),
			Type:   "test",
			Region: "us-west-2",
			Status: "Running",
		}
		testDisplay.UpdateStatus(status)
		time.Sleep(1 * time.Second)
	}

	testDisplay.Stop()
	testDisplay.WaitForStop()

	fmt.Println("Test display completed successfully")
	return nil
}
