package destroy

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/logger"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
)

type Deployment struct {
	Name string
	Type string // "Azure" or "AWS"
	ID   string // Resource Group for Azure, VPC ID for AWS
}

var (
	DestroyCmd *cobra.Command
	once       sync.Once
)

func GetDestroyCmd() *cobra.Command {
	once.Do(func() {
		DestroyCmd = &cobra.Command{
			Use:   "destroy",
			Short: "List and destroy deployments",
			Long:  `List all deployments by VPC or Resource Group in config.yaml, and allow the user to select one for destruction.`,
			Run:   runDestroy,
		}
		// Add any flags if needed
	})
	return DestroyCmd
}

func runDestroy(cmd *cobra.Command, args []string) {
	log := logger.Get()

	// Read config.yaml
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	// Extract deployments from config
	var deployments []Deployment
	azureDeployments := viper.GetStringMap("deployments.azure")
	for name, details := range azureDeployments {
		deploymentDetails := details.(map[string]interface{})
		resourceGroupName := deploymentDetails["resourcegroupname"].(string)
		deployments = append(deployments, Deployment{
			Name: name,
			Type: "Azure",
			ID:   resourceGroupName,
		})
	}

	awsDeployments := viper.GetStringMap("deployments.aws")
	for name, details := range awsDeployments {
		deploymentDetails := details.(map[string]interface{})
		vpcID, ok := deploymentDetails["vpc_id"].(string)
		if !ok {
			log.Warnf("VPC ID not found for AWS deployment %s, skipping", name)
			continue
		}
		deployments = append(deployments, Deployment{
			Name: name,
			Type: "AWS",
			ID:   vpcID,
		})
	}

	if len(deployments) == 0 {
		log.Fatal("No deployments found in the configuration")
	}

	// Present list to user
	fmt.Println("Available deployments:")
	for i, dep := range deployments {
		fmt.Printf("%d. %s (%s) - %s\n", i+1, dep.Name, dep.Type, dep.ID)
	}

	// Get user selection
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the number of the deployment to destroy: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	index := 0
	_, err = fmt.Sscanf(input, "%d", &index)
	if err != nil || index < 1 || index > len(deployments) {
		log.Fatalf("Invalid selection. Please enter a number between 1 and %d", len(deployments))
	}

	selected := deployments[index-1]

	// Start destruction process
	fmt.Printf("Starting destruction of %s (%s) - %s\n", selected.Name, selected.Type, selected.ID)

	ctx := context.Background()
	if selected.Type == "Azure" {
		azureProvider, err := azure.NewAzureProvider(viper.GetViper())
		if err != nil {
			log.Fatalf("Failed to create Azure provider: %v", err)
		}
		err = azureProvider.DestroyResources(ctx, selected.ID)
	} else if selected.Type == "AWS" {
		awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
		if err != nil {
			log.Fatalf("Failed to create AWS provider: %v", err)
		}
		err = awsProvider.DestroyResources(ctx, selected.ID)
	}

	if err != nil {
		log.Fatalf("Failed to destroy deployment: %v", err)
	}

	fmt.Println("Destruction process started.")
	fmt.Println("To watch the destruction progress, use the following CLI command:")
	if selected.Type == "Azure" {
		fmt.Printf("andaime beta azure destroy-status --resource-group %s\n", selected.ID)
	} else if selected.Type == "AWS" {
		fmt.Printf("andaime beta aws destroy-status --vpc-id %s\n", selected.ID)
	}
}
