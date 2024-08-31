package azure

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

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
	Long: `List all active deployments by Resource Group in config.yaml, and allow the user to select one for destruction. 
			Deployments that are already being destroyed will not be listed.`,
	RunE: runDestroy,
}

func GetAzureDestroyCmd() *cobra.Command {
	DestroyCmd.Flags().StringP("name", "n", "", "The name of the deployment to destroy")
	DestroyCmd.Flags().IntP("index", "i", 0, "The index of the deployment to destroy")
	DestroyCmd.Flags().Bool("all", false, "Destroy all deployments")
	// Can only set either name or index
	DestroyCmd.MarkFlagsMutuallyExclusive("name", "index", "all")
	return DestroyCmd
}

//nolint:funlen,gocyclo,unused
func runDestroy(cmd *cobra.Command, args []string) error {
	l := logger.Get()
	l.Debug("Starting runDestroy function")

	// Get flags
	name := cmd.Flag("name").Value.String()
	index, err := strconv.Atoi(cmd.Flag("index").Value.String())
	if err != nil {
		l.Errorf("Failed to convert index to int: %v", err)
		return fmt.Errorf("failed to convert index to int: %v", err)
	}
	destroyAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		l.Errorf("Failed to get 'all' flag: %v", err)
		return fmt.Errorf("failed to get 'all' flag: %v", err)
	}

	l.Debugf("Flags: name=%s, index=%d, destroyAll=%v", name, index, destroyAll)

	// Read config.yaml
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		l.Errorf("Error reading config file: %s", err)
		return fmt.Errorf("error reading config file: %s", err)
	}

	l.Debug("Config file read successfully")

	// Extract deployments from config
	var deployments []Deployment
	azureDeployments := viper.Get("deployments.azure")
	if azureMap, ok := azureDeployments.(map[string]interface{}); ok {
		for name, details := range azureMap {
			deploymentDetails, ok := details.(map[string]interface{})
			if !ok {
				l.Warnf("Invalid deployment details for Azure deployment %s, skipping", name)
				l.Debugf("Details: %v", details)
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
		l.Warn("Azure deployments are not in the expected format")
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
		l.Warn("AWS deployments are not in the expected format")
	}

	l.Debugf("Found %d deployments", len(deployments))

	// Sort deployments alphabetically by name
	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Name < deployments[j].Name
	})

	if destroyAll {
		// Query the subscription for any remaining resource groups with the CreatedBy tag
		azureProvider, err := azure.NewAzureProviderFunc()
		if err != nil {
			l.Errorf("Failed to create Azure provider: %v", err)
			return fmt.Errorf("failed to create Azure provider: %v", err)
		}
		resourceGroups, err := azureProvider.GetAzureClient().ListAllResourceGroups(cmd.Context())
		if err != nil {
			l.Errorf("Failed to get resource groups: %v", err)
			return fmt.Errorf("failed to get resource groups: %v", err)
		}

		for rgName, rgLocation := range resourceGroups {
			rg, err := azureProvider.GetAzureClient().
				GetResourceGroup(cmd.Context(), rgLocation, rgName)
			if err != nil {
				l.Errorf("Failed to get resource group %s: %v", rgName, err)
				continue
			}
			if createdBy, ok := rg.Tags["CreatedBy"]; ok && createdBy != nil &&
				strings.EqualFold(*createdBy, "andaime") {
				l.Infof("Found resource group %s with CreatedBy tag", rgName)
				// Only add the resource group if it is not already in the list
				found := false
				for _, dep := range deployments {
					if dep.Name == rgName {
						found = true
						break
					}
				}

				// Test to see if the resource group is already being destroyed
				if *rg.Properties.ProvisioningState == "Deleting" {
					l.Infof("Resource group %s is already being destroyed", rgName)
					continue
				}

				if !found {
					deployments = append(deployments, Deployment{
						Name: rgName,
						Type: "Azure",
						ID:   rgName,
					})
				}
			}

			l.Info("Destroying all deployments:")
			for _, dep := range deployments {
				if err := destroyDeployment(dep); err != nil {
					l.Errorf("Failed to destroy deployment %s: %v", dep.Name, err)
				}
			}
		}

		l.Info("Finished destroying all deployments")
		return nil
	}

	if index != 0 {
		if index < 1 || index > len(deployments) {
			l.Errorf("Invalid index. Please enter a number between 1 and %d", len(deployments))
			return fmt.Errorf(
				"invalid index. Please enter a number between 1 and %d",
				len(deployments),
			)
		}
		l.Debugf("Destroying deployment at index: %d", index)
		return destroyDeployment(deployments[index-1])
	}

	// Present list to user
	l.Debug("Available deployments:")
	for i, dep := range deployments {
		l.Infof("%d. %s (%s) - %s", i+1, dep.Name, dep.Type, dep.ID)
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
		l.Debug("Enter the number of the deployment to destroy: ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		index := 0
		_, err = fmt.Sscanf(input, "%d", &index)
		if err != nil || index < 1 || index > len(deployments) {
			l.Errorf("Invalid selection. Please enter a number between 1 and %d", len(deployments))
			return fmt.Errorf(
				"invalid selection. Please enter a number between 1 and %d",
				len(deployments),
			)
		}

		selected = deployments[index-1]
	}

	l.Debugf("Selected deployment: %s (%s) - %s", selected.Name, selected.Type, selected.ID)
	return destroyDeployment(selected)
}

func destroyDeployment(dep Deployment) error {
	l := logger.Get()
	l.Infof("Starting destruction of %s (%s) - %s", dep.Name, dep.Type, dep.ID)

	ctx := context.Background()
	started := false

	if dep.Type == "Azure" {
		azureProvider, err := azure.NewAzureProviderFunc()
		dep.FullViperKey = fmt.Sprintf("deployments.azure.%s", dep.Name)
		if err != nil {
			l.Errorf("Failed to create Azure provider for %s: %v", dep.Name, err)
			return fmt.Errorf("failed to create Azure provider for %s: %v", dep.Name, err)
		}
		l.Debugf("Destroying Azure resources for %s", dep.Name)
		err = azureProvider.DestroyResources(ctx, dep.ID)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceGroupNotFound") {
				l.Infof("Resource group '%s' is already destroyed.", dep.ID)
			} else {
				l.Errorf("Failed to destroy Azure deployment %s: %v", dep.Name, err)
				return fmt.Errorf("failed to destroy Azure deployment %s: %v", dep.Name, err)
			}
		} else {
			started = true
			l.Infof("Azure deployment %s destruction started successfully", dep.Name)
			// Stop the display
		}
	} else if dep.Type == "AWS" {
		awsProvider, err := awsprovider.NewAWSProvider(viper.GetViper())
		dep.FullViperKey = fmt.Sprintf("deployments.aws.%s", dep.Name)
		if err != nil {
			l.Errorf("Failed to create AWS provider for %s: %v", dep.Name, err)
			return fmt.Errorf("failed to create AWS provider for %s: %v", dep.Name, err)
		}
		l.Debugf("Destroying AWS resources for %s", dep.Name)
		err = awsProvider.DestroyResources(ctx, dep.ID)
		if err != nil {
			l.Errorf("Failed to destroy AWS deployment %s: %v", dep.Name, err)
			return fmt.Errorf("failed to destroy AWS deployment %s: %v", dep.Name, err)
		}
		started = true
	}

	if started {
		l.Info("Destruction process started.")
		fmt.Println("To watch the destruction progress, use the following CLI command:")
		if dep.Type == "Azure" {
			l.Infof("andaime beta azure destroy-status --resource-group %s", dep.ID)
		} else if dep.Type == "AWS" {
			l.Infof("andaime beta aws destroy-status --vpc-id %s", dep.ID)
		}
	} else {
		l.Warn("Destruction process did not start")
	}

	l.Debug("Removing key from config.")
	if err := utils.DeleteKeyFromConfig(dep.FullViperKey); err != nil {
		l.Errorf("Failed to delete key from config for %s: %v", dep.Name, err)
		return fmt.Errorf("failed to delete key from config for %s: %v", dep.Name, err)
	}

	l.Infof("Destruction process for %s (%s) - %s completed", dep.Name, dep.Type, dep.ID)
	return nil
}
