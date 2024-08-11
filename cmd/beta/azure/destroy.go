package azure

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/display"
	"github.com/bacalhau-project/andaime/pkg/logger"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

type Deployment struct {
	Name         string
	Type         string // "Azure" or "AWS"
	ID           string // Resource Group for Azure, VPC ID for AWS
	FullViperKey string // The full key in the Viper config file
}

var DestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "List and destroy deployments",
	Long:  `List all active deployments by Resource Group in config.yaml, and allow the user to select one for destruction. Deployments that are already being destroyed will not be listed.`,
	RunE:  runDestroy,
}

func GetAzureDestroyCmd() *cobra.Command {
	DestroyCmd.Flags().StringP("name", "n", "", "The name of the deployment to destroy")
	DestroyCmd.Flags().IntP("index", "i", 0, "The index of the deployment to destroy")
	DestroyCmd.Flags().Bool("all", false, "Destroy all deployments")
	// Can only set either name or index
	DestroyCmd.MarkFlagsMutuallyExclusive("name", "index", "all")
	return DestroyCmd
}

func runDestroy(cmd *cobra.Command, args []string) error {
	l := logger.Get()

	// Get flags
	name := cmd.Flag("name").Value.String()
	index, err := strconv.Atoi(cmd.Flag("index").Value.String())
	if err != nil {
		l.Fatalf("Failed to convert index to int: %v", err)
	}
	destroyAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		l.Fatalf("Failed to get 'all' flag: %v", err)
	}

	// Read config.yaml
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		l.Fatalf("Error reading config file: %s", err)
	}

	// Extract deployments from config
	var deployments []Deployment
	azureDeployments := viper.Get("deployments.azure")
	if azureMap, ok := azureDeployments.(map[string]interface{}); ok {
		for name, details := range azureMap {
			deploymentDetails, ok := details.(map[string]interface{})
			if !ok {
				l.Warnf("Invalid deployment details for Azure deployment %s, skipping", name)
				continue
			}
			resourceGroupName, ok := deploymentDetails["resourcegroupname"].(string)
			if !ok {
				l.Warnf("Resource group name not found for Azure deployment %s, skipping", name)
				continue
			}
			dep := Deployment{
				Name: name,
				Type: "Azure",
				ID:   resourceGroupName,
			}
			deployments = append(deployments, dep)
		}
	} else {
		l.Warnf("Azure deployments are not in the expected format")
	}

	awsDeployments := viper.Get("deployments.aws")
	if awsMap, ok := awsDeployments.(map[string]interface{}); ok {
		for name, details := range awsMap {
			deploymentDetails, ok := details.(map[string]interface{})
			if !ok {
				l.Warnf("Invalid deployment details for AWS deployment %s, skipping", name)
				continue
			}
			vpcID, ok := deploymentDetails["vpc_id"].(string)
			if !ok {
				l.Warnf("VPC ID not found for AWS deployment %s, skipping", name)
				continue
			}
			dep := Deployment{
				Name: name,
				Type: "AWS",
				ID:   vpcID,
			}
			deployments = append(deployments, dep)
		}
	} else {
		l.Warnf("AWS deployments are not in the expected format")
	}

	if len(deployments) == 0 {
		l.Fatal("No deployments available for destruction")
	}

	// Sort deployments alphabetically by name
	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Name < deployments[j].Name
	})

	if destroyAll {
		l.Infof("Destroying all deployments:")
		for _, dep := range deployments {
			if err := destroyDeployment(dep); err != nil {
				l.Errorf("Failed to destroy deployment %s: %v", dep.Name, err)
			}
		}

		// Get all resource groups with tag "CreatedBy" containing "andaime"
		azureProvider, err := azure.NewAzureProvider()
		if err != nil {
			l.Errorf("Failed to create Azure provider: %v", err)
			return err
		}
		resourceGroups, err := azureProvider.GetClient().ListAllResourceGroups(cmd.Context())
		if err != nil {
			l.Errorf("Failed to get resource groups: %v", err)
			return err
		}
		for rgName, rgLocation := range resourceGroups {
			// Check if the resource group already exists
			rg, err := azureProvider.GetClient().
				GetResourceGroup(cmd.Context(), rgLocation, rgName)
			if err != nil {
				l.Errorf("Failed to get resource group %s: %v", rgName, err)
				continue
			}
			if rg.Tags["CreatedBy"] != nil && *rg.Tags["CreatedBy"] == "andaime" {
				if err := destroyDeployment(Deployment{Name: rgName, Type: "Azure", ID: rgName}); err != nil {
					l.Errorf("Failed to destroy resource group %s: %v", rgName, err)
				}
			}
		}

		l.Infof("Finished destroying all deployments")
		return nil
	}

	if index != 0 {
		if index < 1 || index > len(deployments) {
			l.Fatalf("Invalid index. Please enter a number between 1 and %d", len(deployments))
		}
		l.Debugf("Destroying deployment at index: %d", index)
		destroyDeployment(deployments[index-1])
		return nil
	}

	// Present list to user
	l.Debugf("Available deployments:")
	for i, dep := range deployments {
		l.Infof("%d. %s (%s) - %s\n", i+1, dep.Name, dep.Type, dep.ID)
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
		l.Debugf("Enter the number of the deployment to destroy: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		index := 0
		_, err = fmt.Sscanf(input, "%d", &index)
		if err != nil || index < 1 || index > len(deployments) {
			l.Fatalf(
				"Invalid selection. Please enter a number between 1 and %d",
				len(deployments),
			)
		}

		selected = deployments[index-1]
	}

	return destroyDeployment(selected)
}

func destroyDeployment(dep Deployment) error {
	l := logger.Get()
	l.Infof("Starting destruction of %s (%s) - %s\n", dep.Name, dep.Type, dep.ID)

	ctx := context.Background()
	started := false

	if dep.Type == "Azure" {
		azureProvider, err := azure.NewAzureProvider()
		dep.FullViperKey = fmt.Sprintf("deployments.azure.%s", dep.Name)
		if err != nil {
			l.Errorf("Failed to create Azure provider for %s: %v", dep.Name, err)
			return err
		}
		err = azureProvider.DestroyResources(ctx, dep.ID)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceGroupNotFound") {
				l.Infof("Resource group '%s' is already destroyed.\n", dep.ID)
			} else {
				l.Errorf("Failed to destroy Azure deployment %s: %v", dep.Name, err)
				return err
			}
		} else {
			started = true
			l.Infof("Azure deployment %s completed successfully", dep.Name)
			// Stop the display
			if disp := display.GetGlobalDisplay(); disp != nil {
				disp.Stop()
			}
		}
	} else if dep.Type == "AWS" {
		awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
		dep.FullViperKey = fmt.Sprintf("deployments.aws.%s", dep.Name)
		if err != nil {
			l.Errorf("Failed to create AWS provider for %s: %v", dep.Name, err)
			return err
		}
		err = awsProvider.DestroyResources(ctx, dep.ID)
		if err != nil {
			l.Errorf("Failed to destroy AWS deployment %s: %v", dep.Name, err)
			return err
		}
		started = true
	}

	if started {
		l.Infof("Destruction process started.")
		fmt.Println("To watch the destruction progress, use the following CLI command:")
		if dep.Type == "Azure" {
			l.Infof("andaime beta azure destroy-status --resource-group %s\n", dep.ID)
		} else if dep.Type == "AWS" {
			l.Infof("andaime beta aws destroy-status --vpc-id %s\n", dep.ID)
		}
	}

	fmt.Println("Removing key from config.")
	if err := utils.DeleteKeyFromConfig(dep.FullViperKey); err != nil {
		l.Errorf("Failed to delete key from config for %s: %v", dep.Name, err)
		return err
	}

	return nil
}
