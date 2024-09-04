package testutils

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/bacalhau-project/andaime/pkg/models"
)

func GenerateRandomLogEntry() string {
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

	numWords := rand.IntN(5) + 3 //nolint:mnd,gosec
	var logWords []string
	for i := 0; i < numWords; i++ {
		logWords = append(logWords, words[rand.IntN(len(words))]) //nolint:mnd,gosec
	}

	return strings.Join(logWords, " ")
}

func CreateRandomStatus() *models.DisplayStatus {
	id := fmt.Sprintf("i-%06d", rand.IntN(1000000)) //nolint:mnd,gosec
	newDisplayStatus := models.NewDisplayVMStatus(
		id,
		models.ResourceStatePending,
	)
	newDisplayStatus.Location = RandomZone()
	newDisplayStatus.StatusMessage = "Initializing"
	newDisplayStatus.DetailedStatus = "Starting"
	newDisplayStatus.ElapsedTime = 0
	newDisplayStatus.InstanceID = id
	newDisplayStatus.PublicIP = RandomIP()
	newDisplayStatus.PrivateIP = RandomIP()
	return newDisplayStatus
}

func GetRandomStatus(statuses map[string]*models.DisplayStatus) *models.DisplayStatus {
	if len(statuses) == 0 {
		return nil
	}
	i := rand.IntN(len(statuses)) //nolint:mnd,gosec
	for _, status := range statuses {
		if i == 0 {
			return status
		}
		i--
	}
	return nil
}

func RandomRegion() string {
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
	return regions[rand.IntN(len(regions))] //nolint:mnd,gosec
}

func RandomZone() string {
	return "zone-" + string(rune('a'+rand.IntN(3))) //nolint:mnd,gosec
}

func RandomStatus() string {
	statuses := []string{"Pending", "Running", "Stopping", "Stopped", "Terminated"}
	return statuses[rand.IntN(len(statuses))] //nolint:mnd,gosec
}

func GetRandomDetailedStatus(status string) string {
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

func RandomIP() string {
	return fmt.Sprintf(
		"%d.%d.%d.%d",
		rand.IntN(256), //nolint:mnd,gosec
		rand.IntN(256), //nolint:mnd,gosec
		rand.IntN(256), //nolint:mnd,gosec
		rand.IntN(256), //nolint:mnd,gosec
	)
}
