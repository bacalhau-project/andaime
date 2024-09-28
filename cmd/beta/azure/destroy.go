package azure

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	"github.com/bacalhau-project/andaime/pkg/providers/azure"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

var (
	configFileName = "config.yaml"
)

type ConfigDeployment struct {
	Name         string
	Type         models.DeploymentType
	ID           string
	UniqueID     string
	FullViperKey string
}

var DestroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "List and destroy deployments",
	Long: `List all active deployments by Resource Group in config-azure.yaml, 
and allow the user to select one for destruction. Deployments that are already being destroyed will not be listed.`,
	RunE: runDestroy,
}

func GetAzureDestroyCmd() *cobra.Command {
	DestroyCmd.Flags().StringP("name", "n", "", "The name of the deployment to destroy")
	DestroyCmd.Flags().IntP("index", "i", 0, "The index of the deployment to destroy")
	DestroyCmd.Flags().Bool("all", false, "Destroy all deployments")
	DestroyCmd.Flags().String("config", "", "Path to the config file")
	DestroyCmd.MarkFlagsMutuallyExclusive("name", "index", "all")
	return DestroyCmd
}

func runDestroy(cmd *cobra.Command, args []string) error {
	logger := logger.Get()
	logger.Debug("Starting runDestroy function")

	flags, err := parseFlags(cmd)
	if err != nil {
		return err
	}

	if err := initializeConfig(cmd); err != nil {
		return err
	}

	deployments, err := getDeployments()
	if err != nil {
		return err
	}

	if flags.destroyAll {
		return destroyAllDeployments(cmd.Context(), deployments)
	}

	selected, err := selectDeployment(deployments, flags)
	if err != nil {
		return err
	}

	return destroyDeployment(selected)
}

func parseFlags(cmd *cobra.Command) (struct {
	name       string
	index      int
	destroyAll bool
}, error) {
	name := cmd.Flag("name").Value.String()
	index, err := strconv.Atoi(cmd.Flag("index").Value.String())
	if err != nil {
		return struct {
			name       string
			index      int
			destroyAll bool
		}{}, fmt.Errorf("failed to convert index to int: %w", err)
	}
	destroyAll, err := cmd.Flags().GetBool("all")
	if err != nil {
		return struct {
			name       string
			index      int
			destroyAll bool
		}{}, fmt.Errorf("failed to get 'all' flag: %w", err)
	}

	config, err := cmd.Flags().GetString("config")
	if err == nil && config != "" {
		configFileName = config
	}

	return struct {
		name       string
		index      int
		destroyAll bool
	}{name, index, destroyAll}, nil
}

func initializeConfig(cmd *cobra.Command) error {
	// Check if a config file was specified on the command line
	configFile, _ := cmd.Flags().GetString("config")
	if configFile != "" {
		// Use the specified config file
		viper.SetConfigFile(configFile)
	} else {
		// Use the default config file
		viper.SetConfigName(configFileName)
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
	}

	if err := viper.ReadInConfig(); err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}
	return nil
}

func getDeployments() ([]ConfigDeployment, error) {
	var deployments []ConfigDeployment
	allDeployments := viper.Get("deployments")
	if allDeployments == nil {
		fmt.Println("No deployments found in config.")
		return nil, nil
	}

	deploymentMap, ok := allDeployments.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid deployments format in config")
	}

	for uniqueID, details := range deploymentMap {
		azureDeployments, err := extractAzureDeployments(uniqueID, details)
		if err != nil {
			fmt.Printf("Error processing deployment %s: %v\n", uniqueID, err)
			continue
		}
		deployments = append(deployments, azureDeployments...)
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Name < deployments[j].Name
	})

	return deployments, nil
}

func extractAzureDeployments(uniqueID string, details interface{}) ([]ConfigDeployment, error) {
	var deployments []ConfigDeployment
	deploymentClouds, ok := details.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid deployment details for %s", uniqueID)
	}

	azureDetails, ok := deploymentClouds["azure"].(map[string]interface{})
	if !ok {
		return nil, nil // Not an Azure deployment, skip
	}

	for rgName := range azureDetails {
		deployments = append(deployments, ConfigDeployment{
			Name:         rgName,
			Type:         models.DeploymentTypeAzure,
			ID:           rgName,
			UniqueID:     uniqueID,
			FullViperKey: fmt.Sprintf("deployments.%s.azure.%s", uniqueID, rgName),
		})
	}

	return deployments, nil
}

func destroyAllDeployments(ctx context.Context, deployments []ConfigDeployment) error {
	azureProvider, err := createAzureProvider(ctx)
	if err != nil {
		return err
	}

	resourceGroups, err := azureProvider.ListAllResourceGroups(ctx)
	if err != nil {
		return fmt.Errorf("failed to get resource groups: %w", err)
	}

	deployments = appendAndaimeDeployments(deployments, resourceGroups)

	if len(deployments) == 0 {
		fmt.Println("No deployments to destroy")
		return nil
	}

	for i, dep := range deployments {
		fmt.Printf("%d. Destroying resources for %s\n", i+1, dep.Name)
		if err := destroyDeployment(dep); err != nil {
			fmt.Printf("Failed to destroy deployment %s: %v\n", dep.Name, err)
		}
		fmt.Println()
	}

	fmt.Printf("Finished destroying %d deployment(s)\n", len(deployments))
	return nil
}

func createAzureProvider(ctx context.Context) (*azure.AzureProvider, error) {
	subscriptionID := viper.GetString("azure.subscription_id")
	provider, err := azure.NewAzureProviderFunc(ctx, subscriptionID)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure provider: %w", err)
	}
	if provider == nil {
		return nil, fmt.Errorf("azure provider is nil after creation")
	}
	return provider, nil
}

func appendAndaimeDeployments(
	deployments []ConfigDeployment,
	resourceGroups []*armresources.ResourceGroup,
) []ConfigDeployment {
	for _, rg := range resourceGroups {
		if isAndaimeDeployment(rg) && !isBeingDestroyed(rg) &&
			!isAlreadyListed(deployments, *rg.Name) {
			deployments = append(deployments, ConfigDeployment{
				Name: *rg.Name,
				Type: models.DeploymentTypeAzure,
				ID:   *rg.Name,
			})
		}
	}
	return deployments
}

func isAndaimeDeployment(rg *armresources.ResourceGroup) bool {
	createdBy, ok := rg.Tags["deployed-by"]
	return ok && createdBy != nil && strings.EqualFold(*createdBy, "andaime")
}

func isBeingDestroyed(rg *armresources.ResourceGroup) bool {
	return *rg.Properties.ProvisioningState == "Deleting"
}

func isAlreadyListed(deployments []ConfigDeployment, name string) bool {
	for _, dep := range deployments {
		if dep.Name == name {
			return true
		}
	}
	return false
}

func selectDeployment(deployments []ConfigDeployment, flags struct {
	name       string
	index      int
	destroyAll bool
}) (ConfigDeployment, error) {
	if flags.name != "" {
		return findDeploymentByName(deployments, flags.name)
	}
	if flags.index != 0 {
		return selectDeploymentByIndex(deployments, flags.index)
	}
	return selectDeploymentInteractively(deployments)
}

func findDeploymentByName(deployments []ConfigDeployment, name string) (ConfigDeployment, error) {
	for _, dep := range deployments {
		if dep.Name == name {
			return dep, nil
		}
	}
	return ConfigDeployment{}, fmt.Errorf("deployment with name %s not found", name)
}

func selectDeploymentByIndex(deployments []ConfigDeployment, index int) (ConfigDeployment, error) {
	if index < 1 || index > len(deployments) {
		return ConfigDeployment{}, fmt.Errorf(
			"invalid index. Please enter a number between 1 and %d",
			len(deployments),
		)
	}
	return deployments[index-1], nil
}

func selectDeploymentInteractively(deployments []ConfigDeployment) (ConfigDeployment, error) {
	fmt.Println("Available deployments:")
	for i, dep := range deployments {
		fmt.Printf("%d. %s (%s) - %s\n", i+1, dep.Name, dep.Type, dep.ID)
	}

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Enter the number of the deployment to destroy: ")
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)
	index := 0
	_, err := fmt.Sscanf(input, "%d", &index)
	if err != nil || index < 1 || index > len(deployments) {
		return ConfigDeployment{}, fmt.Errorf(
			"invalid selection. Please enter a number between 1 and %d",
			len(deployments),
		)
	}

	selected := deployments[index-1]
	fmt.Printf("Selected deployment: %s (%s) - %s\n", selected.Name, selected.Type, selected.ID)
	return selected, nil
}

func destroyDeployment(dep ConfigDeployment) error {
	logger := logger.Get()
	logger.Infof("Starting destruction of %s (%s) - %s", dep.Name, dep.Type, dep.ID)

	ctx := context.Background()

	return destroyAzureDeployment(ctx, dep)
}

func destroyAzureDeployment(ctx context.Context, dep ConfigDeployment) error {
	azureProvider, err := createAzureProvider(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("   Destroying Azure resources\n")
	err = azureProvider.DestroyResources(ctx, dep.ID)
	if err != nil {
		if strings.Contains(err.Error(), "ResourceGroupNotFound") {
			fmt.Printf("   -- Resource group is already destroyed.\n")
		} else {
			return fmt.Errorf("failed to destroy Azure deployment %s: %w", dep.Name, err)
		}
	} else {
		fmt.Printf("   -- Started successfully\n")
	}

	fmt.Printf("   Removing deployment from config\n")
	if err := utils.DeleteKeyFromConfig(dep.FullViperKey); err != nil {
		return err
	}

	return nil
}
