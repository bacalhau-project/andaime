package testutil

import (
	"fmt"
	"math/rand"
	"strings"

	internal_testutil "github.com/bacalhau-project/andaime/internal/testutil"
	"github.com/bacalhau-project/andaime/pkg/globals"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/spf13/viper"
)

func InitializeTestViper(testConfig string) (*viper.Viper, error) {
	viper.Reset()
	configFile, cleanup, err := internal_testutil.WriteStringToTempFile(testConfig)
	if err != nil {
		return nil, err
	}
	defer cleanup()
	viper.SetConfigType("yaml")
	viper.SetConfigFile(configFile)
	err = viper.ReadInConfig()
	if err != nil {
		return nil, err
	}
	return viper.GetViper(), nil
}

func SetupViper(deploymentType models.DeploymentType,
	testSSHPublicKeyPath, testSSHPrivateKeyPath string,
) {
	viper.Set("general.project_prefix", "test-project")
	viper.Set("general.ssh_public_key_path", testSSHPublicKeyPath)
	viper.Set("general.ssh_private_key_path", testSSHPrivateKeyPath)
	if deploymentType == models.DeploymentTypeAzure {
		viper.Set("azure.subscription_id", "test-subscription-id")
		viper.Set("azure.default_count_per_zone", 1)
		viper.Set("azure.default_machine_type", "Standard_DS4_v2")
		viper.Set("azure.default_disk_size_gb", 30) //nolint:mnd
		viper.Set("azure.resource_group_location", "eastus")
		viper.Set("azure.machines", []map[string]interface{}{
			{
				"location": "eastus",
				"parameters": map[string]interface{}{
					"count":        1,
					"machine_type": "Standard_D2s_v3",
					"orchestrator": true,
				},
			},
		})
	} else if deploymentType == models.DeploymentTypeGCP {
		viper.Set("gcp.default_region", "us-central1")
		viper.Set("gcp.default_zone", "us-central1-a")
		viper.Set("gcp.default_machine_type", "e2-medium")
		viper.Set("gcp.disk_size_gb", globals.DefaultDiskSizeGB)
		viper.Set("gcp.allowed_ports", globals.DefaultAllowedPorts)
		viper.Set("gcp.machines", []map[string]interface{}{
			{
				"location": "us-central1",
				"parameters": map[string]interface{}{
					"count":        1,
					"machine_type": "n1-standard-2",
					"orchestrator": true,
				},
			},
		})
	} else if deploymentType == models.DeploymentTypeAWS {
		viper.Set("aws.account_id", "1234567890")
	}
}

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

	numWords := rand.Intn(5) + 3 //nolint:mnd,gosec
	var logWords []string
	for i := 0; i < numWords; i++ {
		logWords = append(logWords, words[rand.Intn(len(words))]) //nolint:mnd,gosec
	}

	return strings.Join(logWords, " ")
}

func CreateRandomStatus() *models.DisplayStatus {
	id := fmt.Sprintf("i-%06d", rand.Intn(1000000)) //nolint:mnd,gosec
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
	i := rand.Intn(len(statuses)) //nolint:mnd,gosec
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
	return regions[rand.Intn(len(regions))] //nolint:mnd,gosec
}

func RandomZone() string {
	return "zone-" + string(rune('a'+rand.Intn(3))) //nolint:mnd,gosec
}

func RandomStatus() string {
	statuses := []string{"Pending", "Running", "Stopping", "Stopped", "Terminated"}
	return statuses[rand.Intn(len(statuses))] //nolint:mnd,gosec
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
		rand.Intn(256), //nolint:mnd,gosec
		rand.Intn(256), //nolint:mnd,gosec
		rand.Intn(256), //nolint:mnd,gosec
		rand.Intn(256), //nolint:mnd,gosec
	)
}
