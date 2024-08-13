package beta

import (
	"fmt"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
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
	m := display.InitialModel()
	p := tea.NewProgram(m)

	go func() {
		totalTasks := 5
		for i := 0; i < totalTasks; i++ {
			status := &models.Status{
				ID:        fmt.Sprintf("test%d", i+1),
				Type:      models.UpdateStatusResourceType("test"),
				Location:  "us-west-2",
				Status:    "Running",
				StartTime: time.Now(),
			}
			m.Deployment.UpdateStatus(status)
			time.Sleep(1 * time.Second)
		}
		p.Quit()
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	fmt.Println("Test display completed successfully")
	return nil
}
