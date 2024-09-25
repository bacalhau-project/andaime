package azure

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	azure_provider "github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

type BacalhauConfig struct {
	Key   string
	Value string
}

type ConfigDeployment struct {
	Name         string
	Type         models.DeploymentType // "Azure" or "AWS" or "GCP"
	ID           string                // Resource Group for Azure, VPC ID for AWS
	FullViperKey string                // The full key in the Viper config file
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

	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	err = viper.ReadInConfig()
	if err != nil {
		l.Errorf("Error reading config file: %s", err)
		return fmt.Errorf("error reading config file: %s", err)
	}

	l.Debug("Config file read successfully")

	// Read Bacalhau configuration settings
	var bacalhauConfigs []BacalhauConfig
	bacalhauSettings := viper.GetStringSlice("general.bacalhau-settings")
	for _, setting := range bacalhauSettings {
		parts := strings.SplitN(setting, " ", 4)
		if len(parts) == 4 && parts[0] == "config" && parts[1] == "set" {
			bacalhauConfigs = append(bacalhauConfigs, BacalhauConfig{
				Key:   parts[2],
				Value: parts[3],
			})
		}
	}
	l.Debugf("Bacalhau configs: %+v", bacalhauConfigs)

	// Extract deployments from config
	var deployments []ConfigDeployment
	allDeployments := viper.Get("deployments")
	if deploymentMap, ok := allDeployments.(map[string]interface{}); ok {
		for _, deploymentDetails := range deploymentMap {
			deploymentClouds, ok := deploymentDetails.(map[string]interface{})
			if !ok {
				l.Warnf("Invalid deployment details for Azure deployment %s, skipping", name)
				l.Debugf("Details: %v", deploymentDetails)
				continue
			}
			if deploymentClouds["azure"] != nil {
				deploymentCloudsAzure, ok := deploymentClouds["azure"].(map[string]interface{})
				if !ok {
					l.Warnf("Does not appear to have a resource group name %s, skipping", name)
					l.Debugf("Details: %v", deploymentCloudsAzure)
					continue
				}

				for rgName := range deploymentCloudsAzure {
					dep := ConfigDeployment{
						Name: rgName,
						Type: models.DeploymentTypeAzure,
						ID:   rgName,
					}
					deployments = append(deployments, dep)
				}
			}
		}
	} else {
		l.Warn("Azure deployments are not in the expected format")
	}

	// Apply Bacalhau configuration settings after deployment and VerifyBacalhau
	if err := applyBacalhauConfigs(bacalhauConfigs); err != nil {
		l.Errorf("Failed to apply Bacalhau configurations: %v", err)
		return fmt.Errorf("failed to apply Bacalhau configurations: %w", err)
	}

	// awsDeployments := viper.Get("deployments.aws")
	// if awsMap, ok := awsDeployments.(map[string]interface{}); ok {
	// 	for name, details := range awsMap {
	// 		deploymentDetails, ok := details.(map[string]interface{})
	// 		if !ok {
	// 			l.Warnf("Invalid deployment details for AWS deployment %s, skipping", name)
	// 			continue
	// 		}
	// 		vpcID, ok := deploymentDetails["vpc_id"].(string)
	// 		if !ok {
	// 			l.Warnf("VPC ID not found for AWS deployment %s, skipping", name)
	// 			continue
	// 		}
	// 		dep := ConfigDeployment{
	// 			Name: name,
	// 			Type: "AWS",
	// 			ID:   vpcID,
	// 		}
	// 		deployments = append(deployments, dep)
	// 	}
	// } else {
	// 	l.Warn("AWS deployments are not in the expected format")
	// }

	l.Debugf("Found %d deployments", len(deployments))

	// Sort deployments alphabetically by name
	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Name < deployments[j].Name
	})

	if destroyAll {
		subscriptionID := viper.GetString("azure.subscription_id")
		azureProvider, err := azure_provider.NewAzureProviderFunc(cmd.Context(), subscriptionID)
		if err != nil {
			l.Errorf("Failed to create Azure provider: %v", err)
			return fmt.Errorf("failed to create Azure provider: %w", err)
		}

		if azureProvider == nil {
			l.Error("Azure provider is nil after creation")
			return fmt.Errorf("azure provider is nil after creation")
		}

		resourceGroups, err := azureProvider.ListAllResourceGroups(cmd.Context())
		if err != nil {
			l.Errorf("Failed to get resource groups: %v", err)
			return fmt.Errorf("failed to get resource groups: %v", err)
		}

		for _, rg := range resourceGroups {
			rg, err := azureProvider.
				GetOrCreateResourceGroup(
					cmd.Context(),
					*rg.Name,
					*rg.Location,
					map[string]string{},
				)
			if err != nil {
				l.Errorf("Failed to get resource group %s: %v", *rg.Name, err)
				continue
			}
			if createdBy, ok := rg.Tags["CreatedBy"]; ok && createdBy != nil &&
				strings.EqualFold(*createdBy, "andaime") {
				l.Infof("Found resource group %s with CreatedBy tag", *rg.Name)
				// Only add the resource group if it is not already in the list
				found := false
				for _, dep := range deployments {
					if dep.Name == *rg.Name {
						found = true
						break
					}
				}

				// Test to see if the resource group is already being destroyed
				if *rg.Properties.ProvisioningState == "Deleting" {
					l.Infof("Resource group %s is already being destroyed", *rg.Name)
					continue
				}

				if !found {
					deployments = append(deployments, ConfigDeployment{
						Name: *rg.Name,
						Type: "Azure",
						ID:   *rg.Name,
					})
				}
			}
		}

		l.Info("Destroying all deployments:")
		for i, dep := range deployments {
			fmt.Printf("%d. Destroying resources for %s\n", i+1, dep.Name)
			if err := destroyDeployment(dep); err != nil {
				l.Errorf("Failed to destroy deployment %s: %v", dep.Name, err)
			}
			fmt.Println()
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

	var selected ConfigDeployment

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

func destroyDeployment(dep ConfigDeployment) error {
	l := logger.Get()
	l.Infof("Starting destruction of %s (%s) - %s", dep.Name, dep.Type, dep.ID)

	ctx := context.Background()

	if dep.Type == models.DeploymentTypeAzure {
		azureProvider, err := azure_provider.NewAzureProviderFunc(
			ctx,
			viper.GetString("azure.subscription_id"),
		)
		dep.FullViperKey = fmt.Sprintf("deployments.azure.%s", dep.Name)
		if err != nil {
			l.Errorf("Failed to create Azure provider for %s: %v", dep.Name, err)
			return fmt.Errorf("failed to create Azure provider for %s: %v", dep.Name, err)
		}
		fmt.Printf("   Destroying Azure resources\n")
		err = azureProvider.DestroyResources(ctx, dep.ID)
		if err != nil {
			if strings.Contains(err.Error(), "ResourceGroupNotFound") {
				fmt.Printf("   -- Resource group is already destroyed.\n")
			} else {
				l.Errorf("Failed to destroy Azure deployment %s: %v", dep.Name, err)
				return fmt.Errorf("failed to destroy Azure deployment %s: %v", dep.Name, err)
			}
		} else {
			fmt.Printf("   -- Started successfully\n")
		}
	} else if dep.Type == models.DeploymentTypeAWS {
		l.Warnf("AWS destroy is not implemented yet")
	}

	fmt.Printf("   Removing keys from config\n")
	if err := utils.DeleteKeyFromConfig(dep.FullViperKey); err != nil {
		fmt.Printf("   -- Failed to delete key from config: %v\n", err)
		return fmt.Errorf("failed to delete key from config for %s: %v", dep.Name, err)
	}

	fmt.Printf("   -- Done\n")

	return nil
}

func applyBacalhauConfigs(configs []BacalhauConfig) error {
	l := logger.Get()
	l.Info("Applying Bacalhau configurations")

	for _, config := range configs {
		cmd := exec.Command("bacalhau", "config", "set", config.Key, config.Value)
		cmd.Stderr = os.Stderr
		
		output, err := cmd.Output()
		if err != nil {
			l.Errorf("Failed to apply Bacalhau config %s: %v", config.Key, err)
			return fmt.Errorf("failed to apply Bacalhau config %s: %w", config.Key, err)
		}
		
		l.Infof("Applied Bacalhau config %s: %s", config.Key, string(output))
	}

	l.Info("Bacalhau configurations applied successfully")
	return nil
}
