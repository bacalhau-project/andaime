package cmd

import (
	"context"
	"fmt"
	rand "math/rand/v2"
	"strings"
	"time"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/cobra"
)

var totalTasks = 20

var testDisplayCmd = &cobra.Command{
	Use:   "testDisplay",
	Short: "Test the display functionality",
	Run:   runTestDisplay,
}

func getTestDisplayCmd() *cobra.Command {
	return testDisplayCmd
}

func runTestDisplay(cmd *cobra.Command, args []string) {
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	totalTasks := 20
	runTestDisplayInternal(totalTasks)

	// sigChan := make(chan os.Signal, 1)
	// signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// go func() {
	// 	<-sigChan
	// 	d.DebugLog.Debug("\nReceived interrupt, shutting down...")
	// 	d.Stop()
	// 	cancel()
	// }()

	// d.Start(sigChan)

	// statusChan := make(chan *display.Status)
	// logChan := make(chan string)

	// var wg sync.WaitGroup
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	generateEvents(ctx, statusChan, logChan)
	// }()

	// go func() {
	// 	for {
	// 		select {
	// 		case status := <-statusChan:
	// 			d.UpdateStatus(status)
	// 		case logEntry := <-logChan:
	// 			d.AddLogEntry(logEntry)
	// 		case <-ctx.Done():
	// 			return
	// 		}
	// 	}
	// }()

	// // Wait for the context to be canceled (which happens when we receive an interrupt)
	// <-ctx.Done()

	// // Wait for goroutines to finish
	// wg.Wait()

	// // Wait for the display to fully stop
	// d.WaitForStop()

	// d.DebugLog.Debug("Exiting")
}

func runTestDisplayInternal(totalTasks int) {
	d := display.NewDisplay(totalTasks)
	d.Logger.Info("Display initialized")

	// Create 10 initial statuses
	for i := 0; i < 10; i++ {
		status := createRandomStatus()
		d.Logger.Infof("Created random status: %+v", status)
		d.UpdateStatus(status)
	}

	// Create a ticker for periodic updates
	ticker := time.NewTicker(500 * time.Millisecond) // Increased the interval
	defer ticker.Stop()

	// Start the update loop in a separate goroutine
	go func() {
		d.Log("Starting update loop")
		for {
			select {
			case <-ticker.C:
				d.Log("Update loop tick")
				status := getRandomStatus(d.Statuses)
				if status != nil {
					d.Logger.Infof("Got random status: %+v", status)
					updateRandomStatus(status)
					d.Logger.Infof("Updated random status: %+v", status)
					d.UpdateStatus(status)
					d.App.Draw()
				} else {
					d.Logger.Info("Got nil status")
				}
			case <-d.StopChan:
				d.Log("Stop signal received")
				return
			case <-d.Quit:
				d.Log("Quit signal received")
				return
			case <-d.Ctx.Done():
				d.Log("Context done")
				return
			}
		}
	}()

	// Run the application
	d.Logger.Info("Starting application")
	if err := d.App.Run(); err != nil {
		d.Logger.Errorf("Error running application: %s", err.Error())
	}
}

func generateEvents(ctx context.Context, statusChan chan<- *models.Status, logChan chan<- string) {
	log := logger.Get()

	statuses := make(map[string]*models.Status)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for i := 0; i < totalTasks; i++ {
		newStatus := createRandomStatus()
		statuses[newStatus.ID] = newStatus
		statusChan <- newStatus
	}

	logTicker := time.NewTicker(2 * time.Second)
	defer logTicker.Stop()

	for {
		select {
		case <-ticker.C:
			if status := getRandomStatus(statuses); status != nil {
				if updateRandomStatus(status) {
					select {
					case statusChan <- status:
					case <-ctx.Done():
						return
					}
				}
			}
		case <-logTicker.C:
			logEntry := generateRandomLogEntry()
			log.Infof(logEntry)
			select {
			case logChan <- logEntry:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
func updateRandomStatus(status *models.Status) bool {
	oldStatus := *status
	status.ElapsedTime += time.Duration(rand.IntN(10)) * time.Second
	status.Status = randomStatus()
	status.DetailedStatus = randomDetailedStatus(status.Status)
	return oldStatus != *status // Return true if there's a change
}

func generateRandomLogEntry() string {
	words := []string{
		"Deploying", "Configuring", "Initializing", "Updating", "Processing",
		"Resource", "Network", "Storage", "Compute", "Database",
		"Server", "Cloud", "Virtual", "Container", "Cluster",
		"Scaling", "Balancing", "Routing", "Firewall", "Gateway",
		"Backup", "Recovery", "Monitoring", "Logging", "Analytics",
		"API", "Microservice", "Function", "Queue", "Cache",
		"Encryption", "Authentication", "Authorization", "Endpoint", "Protocol",
		"Bandwidth", "Latency", "Throughput", "Packet", "Payload",
		"Instance", "Volume", "Snapshot", "Image", "Template",
		"Orchestration", "Provisioning", "Deprovisioning", "Allocation", "Deallocation",
		"Replication", "Synchronization", "Failover", "Redundancy", "Resilience",
		"Optimization", "Compression", "Indexing", "Partitioning", "Sharding",
		"Namespace", "Repository", "Registry", "Artifact", "Pipeline",
		"Webhook", "Trigger", "Event", "Stream", "Batch",
		"Scheduler", "Cron", "Task", "Job", "Workflow",
		"Module", "Package", "Library", "Framework", "SDK",
		"Compiler", "Interpreter", "Runtime", "Debugger", "Profiler",
		"Algorithm", "Hashing", "Encryption", "Decryption", "Encoding",
		"Socket", "Port", "Interface", "Bridge", "Tunnel",
		"Proxy", "Reverse-proxy", "Load-balancer", "CDN", "DNS",
		"Certificate", "Key", "Token", "Session", "Cookie",
		"Thread", "Process", "Daemon", "Service", "Middleware",
	}

	numWords := rand.IntN(5) + 3 //nolint:gomnd,gosec
	var logWords []string
	for i := 0; i < numWords; i++ {
		logWords = append(logWords, words[rand.IntN(len(words))]) //nolint:gomnd,gosec
	}

	return strings.Join(logWords, " ")
}

func createRandomStatus() *models.Status {
	id := fmt.Sprintf("i-%06d", rand.IntN(1000000)) //nolint:gomnd,gosec
	return &models.Status{
		ID:             id,
		Type:           "EC2",
		Location:       randomRegion(),
		Status:         "Initializing",
		DetailedStatus: "Starting",
		ElapsedTime:    0,
		InstanceID:     id,
		PublicIP:       randomIP(),
		PrivateIP:      randomIP(),
	}
}

func getRandomStatus(statuses map[string]*models.Status) *models.Status {
	if len(statuses) == 0 {
		return nil
	}
	i := rand.IntN(len(statuses)) //nolint:gomnd,gosec
	for _, status := range statuses {
		if i == 0 {
			return status
		}
		i--
	}
	return nil
}

func randomRegion() string {
	regions := []string{
		"us-west-1",
		"us-west-2",
		"us-east-1",
		"us-east-2",
		"eu-west-1",
		"eu-central-1",
		"ap-southeast-1",
		"ap-northeast-1",
	}
	return regions[rand.IntN(len(regions))] //nolint:gomnd,gosec
}

func randomZone() string {
	return "zone-" + string(rune('a'+rand.IntN(3))) //nolint:gomnd,gosec
}

func randomStatus() string {
	statuses := []string{"Pending", "Running", "Stopping", "Stopped", "Terminated"}
	return statuses[rand.IntN(len(statuses))] //nolint:gomnd,gosec
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
	return fmt.Sprintf(
		"%d.%d.%d.%d",
		rand.IntN(256), //nolint:gomnd,gosec
		rand.IntN(256), //nolint:gomnd,gosec
		rand.IntN(256), //nolint:gomnd,gosec
		rand.IntN(256), //nolint:gomnd,gosec
	)
}
