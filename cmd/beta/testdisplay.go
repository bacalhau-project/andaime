package beta

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/testutils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

const statusLength = 30

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
		statuses := make([]*models.DisplayStatus, totalTasks)
		for i := 0; i < totalTasks; i++ {
			newDisplayStatus := models.NewDisplayStatus(
				fmt.Sprintf("test%d", i+1),
				models.AzureResourceTypeVM,
				models.AzureResourceStateNotStarted,
			)
			newDisplayStatus.Location = testutils.RandomRegion()
			newDisplayStatus.StatusMessage = "Initializing"
			newDisplayStatus.DetailedStatus = "Starting"
			newDisplayStatus.ElapsedTime = 0
			newDisplayStatus.InstanceID = fmt.Sprintf("test%d", i+1)

			if i%2 == 0 {
				newDisplayStatus.Orchestrator = false
				newDisplayStatus.SSH = models.DisplayEmojiSuccess
				newDisplayStatus.Docker = models.DisplayEmojiFailed
				newDisplayStatus.Bacalhau = models.DisplayEmojiSuccess
			} else {
				newDisplayStatus.Orchestrator = true
				newDisplayStatus.SSH = models.DisplayEmojiFailed
				newDisplayStatus.Docker = models.DisplayEmojiSuccess
				newDisplayStatus.Bacalhau = models.DisplayEmojiFailed
			}
			newDisplayStatus.PublicIP = testutils.RandomIP()
			newDisplayStatus.PrivateIP = testutils.RandomIP()
			statuses[i] = newDisplayStatus
			p.Send(models.StatusUpdateMsg{Status: statuses[i]})
		}

		wordTicker := time.NewTicker(1 * time.Second)
		timeTicker := time.NewTicker(100 * time.Millisecond)
		defer wordTicker.Stop()
		defer timeTicker.Stop()

		for {
			select {
			case <-wordTicker.C:
				for i := 0; i < totalTasks; i++ {
					rawStatus := getRandomWords(3)
					if len(rawStatus) > statusLength {
						statuses[i].StatusMessage = rawStatus[:statusLength]
					} else {
						statuses[i].StatusMessage = fmt.Sprintf("%-*s", statusLength, rawStatus)
					}
					statuses[i].Progress = (statuses[i].Progress + 1) % display.AzureTotalSteps
					p.Send(models.StatusUpdateMsg{Status: statuses[i]})

					// Log the length of each status string
					// log.Infof("Task %d status length: %d", i+1, len(statuses[i].Status))
				}

				// Log the total width of all statuses
				// totalWidth := statusLength * totalTasks
				// log.Infof("Total width of all statuses: %d", totalWidth)

				// Log a constant value for terminal width (adjust as needed)
				// log.Infof("Assumed terminal width: %d", 80)
			case <-timeTicker.C:
				p.Send(models.TimeUpdateMsg{})
			}
		}
	}()

	if _, err := p.Run(); err != nil {
		return err
	}

	return nil
}
