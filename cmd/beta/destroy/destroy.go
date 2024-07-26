package destroy

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/logger"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/utils"
)

type Deployment struct {
	Name         string
	Type         string // "Azure" or "AWS"
	ID           string // Resource Group for Azure, VPC ID for AWS
	FullViperKey string // The full key in the Viper config file
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
			Long:  `List all active deployments by VPC or Resource Group in config.yaml, and allow the user to select one for destruction. Deployments that are already being destroyed will not be listed.`,
			Run:   runDestroy,
		}
		DestroyCmd.Flags().StringP("name", "n", "", "The name of the deployment to destroy")
		DestroyCmd.Flags().IntP("index", "i", 0, "The index of the deployment to destroy")

		// Can only set either name or index
		DestroyCmd.MarkFlagsMutuallyExclusive("name", "index")
	})
	return DestroyCmd
}

func runDestroy(cmd *cobra.Command, args []string) {
	log := logger.Get()

	// Get flags
	name := cmd.Flag("name").Value.String()
	index, err := strconv.Atoi(cmd.Flag("index").Value.String())
	if err != nil {
		log.Fatalf("Failed to convert index to int: %v", err)
	}

	// Read config.yaml
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		log.Fatalf("Error reading config file: %s", err)
	}

	// Extract deployments from config
	var deployments []Deployment
	azureDeployments := viper.GetStringMap("deployments.azure")
	for name, details := range azureDeployments {
		deploymentDetails := details.(map[string]interface{})
		resourceGroupName := deploymentDetails["resourcegroupname"].(string)
		dep := Deployment{
			Name: name,
			Type: "Azure",
			ID:   resourceGroupName,
		}
		deployments = append(deployments, dep)
	}

	awsDeployments := viper.GetStringMap("deployments.aws")
	for name, details := range awsDeployments {
		deploymentDetails := details.(map[string]interface{})
		vpcID, ok := deploymentDetails["vpc_id"].(string)
		if !ok {
			log.Warnf("VPC ID not found for AWS deployment %s, skipping", name)
			continue
		}
		_ = Deployment{
			Name: name,
			Type: "AWS",
			ID:   vpcID,
		}
	}

	if len(deployments) == 0 {
		log.Fatal("No deployments available for destruction")
	}

	// Sort deployments alphabetically by name
	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Name < deployments[j].Name
	})

	// Present list to user
	fmt.Println("Available deployments:")
	for i, dep := range deployments {
		fmt.Printf("%d. %s (%s) - %s\n", i+1, dep.Name, dep.Type, dep.ID)
	}

	var selected Deployment

	// If the user has set a rgname to destroy, use that
	if name != "" {
		for _, dep := range deployments {
			if dep.Name == name {
				selected = dep
				break
			}
		}
	} else if index != 0 {
		selected = deployments[index-1]
	}

	if selected.Name == "" {
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

		selected = deployments[index-1]
	}

	// Start destruction process
	fmt.Printf("Starting destruction of %s (%s) - %s\n", selected.Name, selected.Type, selected.ID)

	ctx := context.Background()
	started := false
	if selected.Type == "Azure" {
		azureProvider, err := azure.NewAzureProvider(viper.GetViper())
		selected.FullViperKey = fmt.Sprintf("deployments.azure.%s", selected.Name)
		if err != nil {
			log.Fatalf("Failed to create Azure provider: %v", err)
		}
		err = azureProvider.DestroyResources(ctx, selected.ID)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceGroupNotFound") {
				fmt.Printf("Resource group '%s' is already destroyed.\n", selected.ID)
			} else {
				log.Fatalf("Failed to destroy deployment: %v", err)
			}
		} else {
			started = true
		}
	} else if selected.Type == "AWS" {
		awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
		selected.FullViperKey = fmt.Sprintf("deployments.aws.%s", selected.Name)
		if err != nil {
			log.Fatalf("Failed to create AWS provider: %v", err)
		}
		err = awsProvider.DestroyResources(ctx, selected.ID)
		if err != nil {
			log.Fatalf("Failed to destroy deployment: %v", err)
		}
	}

	if err != nil {
		log.Fatalf("Failed to destroy deployment: %v", err)
	}

	if started {
		fmt.Println("Destruction process started.")
		fmt.Println("To watch the destruction progress, use the following CLI command:")
		if selected.Type == "Azure" {
			fmt.Printf("andaime beta azure destroy-status --resource-group %s\n", selected.ID)
		} else if selected.Type == "AWS" {
			fmt.Printf("andaime beta aws destroy-status --vpc-id %s\n", selected.ID)
		}
	}

	fmt.Println("Removing key from config.")
	if err := utils.DeleteKeyFromConfig(selected.FullViperKey); err != nil {
		log.Fatalf("Failed to delete key from config: %v", err)
	}
}
