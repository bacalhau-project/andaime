package beta

import (
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func GetTestDisplayCmd() *cobra.Command {
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
	m := display.GetGlobalModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		totalTasks := 5
		for i := 0; i < display.AzureTotalSteps; i++ {
			status := &models.Status{
				Name:     fmt.Sprintf("test%d", i+1),
				Type:     models.UpdateStatusResourceType("test"),
				Location: "us-west-2",
				Status:   "Running",
				Progress: i,
			}
			p.Send(models.StatusUpdateMsg{Status: status})
			time.Sleep(2 * time.Second)
		}

		// Set final status for all machines
		for i := 0; i < totalTasks; i++ {
			status := &models.Status{
				Name:      fmt.Sprintf("test%d", i+1),
				Type:      models.UpdateStatusResourceType("test"),
				Location:  "us-west-2",
				Status:    "Successfully Deployed",
				Progress:  display.AzureTotalSteps,
				PublicIP:  fmt.Sprintf("192.0.2.%d", i+1),
				PrivateIP: fmt.Sprintf("10.0.0.%d", i+1),
			}
			p.Send(models.StatusUpdateMsg{Status: status})
		}

	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	fmt.Println("Test display exited")
	return nil
}
