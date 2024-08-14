package beta

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var words = []string{
	"apple", "banana", "cherry", "date", "elderberry",
	"fig", "grape", "honeydew", "kiwi", "lemon",
	"mango", "nectarine", "orange", "papaya", "quince",
	"raspberry", "strawberry", "tangerine", "ugli", "watermelon",
}

func getRandomWords(n int) string {
	rand.Shuffle(len(words), func(i, j int) {
		words[i], words[j] = words[j], words[i]
	})
	return strings.Join(words[:n], " ")
}

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
		for i := 0; i < totalTasks; i++ {
			status := &models.Status{
				Name:     fmt.Sprintf("test%d", i+1),
				Type:     models.UpdateStatusResourceType("test"),
				Location: "us-west-2",
				Status:   getRandomWords(3),
				Progress: 0,
			}
			p.Send(models.StatusUpdateMsg{Status: status})
		}

		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				for i := 0; i < totalTasks; i++ {
					status := &models.Status{
						Name:     fmt.Sprintf("test%d", i+1),
						Type:     models.UpdateStatusResourceType("test"),
						Location: "us-west-2",
						Status:   getRandomWords(3),
						Progress: (status.Progress + 1) % display.AzureTotalSteps,
					}
					p.Send(models.StatusUpdateMsg{Status: status})
				}
			}
		}
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	fmt.Println("Test display exited")
	return nil
}
