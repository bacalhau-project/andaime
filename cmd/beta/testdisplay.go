package beta

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/testutil"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

const statusLength = 30

// Constants
const (
	StatusUpdateInterval = 100 * time.Millisecond
	RandomWordsCount     = 3
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
	m := display.GetGlobalModelFunc()
	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		totalTasks := 5
		statuses := make([]*models.DisplayStatus, totalTasks)
		for i := 0; i < totalTasks; i++ {
			newDisplayStatus := models.NewDisplayVMStatus(
				fmt.Sprintf("testVM%d", i+1),
				models.ConvertFromAzureStringToResourceState("Not Started"),
				false, // Test display doesn't use spot instances
			)
			newDisplayStatus.Location = testutil.RandomRegion()
			newDisplayStatus.StatusMessage = "Initializing"
			newDisplayStatus.DetailedStatus = "Starting"
			newDisplayStatus.ElapsedTime = 0
			newDisplayStatus.InstanceID = fmt.Sprintf("test%d", i+1)

			if i%2 == 0 {
				newDisplayStatus.Orchestrator = false
				newDisplayStatus.SSH = models.ServiceStateSucceeded
				newDisplayStatus.Docker = models.ServiceStateFailed
				newDisplayStatus.Bacalhau = models.ServiceStateSucceeded
			} else {
				newDisplayStatus.Orchestrator = true
				newDisplayStatus.SSH = models.ServiceStateFailed
				newDisplayStatus.Docker = models.ServiceStateSucceeded
				newDisplayStatus.Bacalhau = models.ServiceStateFailed
			}
			newDisplayStatus.PublicIP = testutil.RandomIP()
			newDisplayStatus.PrivateIP = testutil.RandomIP()
			statuses[i] = newDisplayStatus
		}

		wordTicker := time.NewTicker(1 * time.Second)
		timeTicker := time.NewTicker(StatusUpdateInterval)
		defer wordTicker.Stop()
		defer timeTicker.Stop()

		for {
			select {
			case <-wordTicker.C:
				for _, machine := range m.Deployment.GetMachines() {
					rawStatus := getRandomWords(RandomWordsCount)
					statusMessage := ""
					if len(rawStatus) > statusLength {
						statusMessage = rawStatus[:statusLength]
					} else {
						statusMessage = fmt.Sprintf("%-*s", statusLength, rawStatus)
					}
					m.Deployment.Machines[machine.GetName()].SetStatusMessage(statusMessage)
				}
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
