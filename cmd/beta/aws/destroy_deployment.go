package aws

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	awsprovider "github.com/bacalhau-project/andaime/pkg/providers/aws"
	"github.com/bacalhau-project/andaime/pkg/utils"
)

type ConfigDeployment struct {
	Name         string
	Type         models.DeploymentType
	ID           string
	UniqueID     string
	FullViperKey string
}

func GetAwsDestroyCmd() *cobra.Command {
	destroyCmd := &cobra.Command{
		Use:   "destroy",
		Short: "Destroy AWS deployments",
		Long:  `List and destroy AWS deployments from the configuration file.`,
		RunE:  runDestroy,
	}

	destroyCmd.Flags().StringP("name", "n", "", "The name of the deployment to destroy")
	destroyCmd.Flags().IntP("index", "i", 0, "The index of the deployment to destroy")
	destroyCmd.Flags().Bool("all", false, "Destroy all deployments")
	destroyCmd.Flags().
		Bool("dry-run", false, "Perform a dry run without actually destroying resources")
	destroyCmd.MarkFlagsMutuallyExclusive("name", "index", "all")

	return destroyCmd
}

func runDestroy(cmd *cobra.Command, args []string) error {
	logger := logger.Get()
	logger.Debug("Starting runDestroy function")

	flags, err := parseFlags(cmd)
	if err != nil {
		return err
	}

	deployments, err := getDeployments()
	if err != nil {
		return err
	}

	// Filter out deployments with empty VPC IDs
	var validDeployments []ConfigDeployment
	for _, dep := range deployments {
		if viper.GetString(fmt.Sprintf("%s.aws.vpc_id", dep.FullViperKey)) != "" {
			validDeployments = append(validDeployments, dep)
		}
	}

	if len(validDeployments) == 0 {
		fmt.Println("No deployments found to destroy")
		return nil
	}

	deployments = validDeployments

	if flags.destroyAll {
		return destroyAllDeployments(cmd.Context(), deployments, flags.dryRun)
	}

	selected, err := selectDeployment(deployments, flags)
	if err != nil {
		return err
	}

	return destroyDeployment(cmd.Context(), selected, flags.dryRun)
}

func parseFlags(cmd *cobra.Command) (struct {
	name       string
	index      int
	destroyAll bool
	dryRun     bool
}, error) {
	name, _ := cmd.Flags().GetString("name")
	index, _ := cmd.Flags().GetInt("index")
	destroyAll, _ := cmd.Flags().GetBool("all")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	return struct {
		name       string
		index      int
		destroyAll bool
		dryRun     bool
	}{name, index, destroyAll, dryRun}, nil
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
		awsDeployments, err := extractAwsDeployments(uniqueID, details)
		if err != nil {
			fmt.Printf("Error processing deployment %s: %v\n", uniqueID, err)
			continue
		}
		deployments = append(deployments, awsDeployments...)
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Name < deployments[j].Name
	})

	return deployments, nil
}

func extractAwsDeployments(uniqueID string, details interface{}) ([]ConfigDeployment, error) {
	var deployments []ConfigDeployment
	deploymentClouds, ok := details.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid deployment details for %s", uniqueID)
	}

	awsDetails, ok := deploymentClouds["aws"].(map[string]interface{})
	if !ok {
		return nil, nil // Not an AWS deployment, skip
	}

	for stackName, stackDetails := range awsDetails {
		stackMap, ok := stackDetails.(map[string]interface{})
		if !ok {
			continue
		}
		stackID, _ := stackMap["stack_id"].(string)
		deployments = append(deployments, ConfigDeployment{
			Name:         stackName,
			Type:         models.DeploymentTypeAWS,
			ID:           stackID,
			UniqueID:     uniqueID,
			FullViperKey: fmt.Sprintf("deployments.%s.aws.%s", uniqueID, stackName),
		})
	}

	return deployments, nil
}

func destroyAllDeployments(ctx context.Context, deployments []ConfigDeployment, dryRun bool) error {
	if len(deployments) == 0 {
		fmt.Println("No deployments to destroy")
		return nil
	}

	fmt.Println("The following deployments will be destroyed:")
	for i, dep := range deployments {
		fmt.Printf("%d. %s (%s) - %s\n", i+1, dep.Name, dep.Type, dep.ID)
	}

	if !dryRun {
		fmt.Print("\nAre you sure you want to destroy these deployments? (y/N): ")
		reader := bufio.NewReader(os.Stdin)
		confirm, _ := reader.ReadString('\n')
		if strings.ToLower(strings.TrimSpace(confirm)) != "y" {
			fmt.Println("Operation cancelled.")
			return nil
		}
	}

	for _, dep := range deployments {
		err := destroyDeployment(ctx, dep, dryRun)
		if err != nil {
			fmt.Printf("Failed to destroy deployment %s: %v\n", dep.Name, err)
		} else {
			fmt.Printf("Successfully destroyed deployment %s\n", dep.Name)
		}
	}

	if dryRun {
		fmt.Println("Dry run completed. No resources were actually destroyed.")
	} else {
		fmt.Printf("Finished destroying %d deployment(s)\n", len(deployments))
	}
	return nil
}

func selectDeployment(deployments []ConfigDeployment, flags struct {
	name       string
	index      int
	destroyAll bool
	dryRun     bool
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
	if len(deployments) == 0 {
		return ConfigDeployment{}, fmt.Errorf("no deployments available to destroy")
	}

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

func destroyDeployment(ctx context.Context, dep ConfigDeployment, dryRun bool) error {
	logger := logger.Get()
	logger.Infof("Starting destruction of %s (%s) - %s", dep.Name, dep.Type, dep.ID)

	awsProvider, err := createAwsProvider()
	if err != nil {
		return err
	}

	fmt.Printf("   Destroying AWS resources\n")
	if dryRun {
		fmt.Printf("   -- Dry run: Would destroy AWS resources for VPC %s\n", dep.ID)
	} else {
		err = awsProvider.DestroyResources(ctx, dep.ID)
		if err != nil {
			return fmt.Errorf("failed to destroy AWS deployment %s: %w", dep.Name, err)
		}
		fmt.Printf("   -- Resources destroyed successfully\n")

		fmt.Printf("   Removing deployment from config\n")
		if err := utils.DeleteKeyFromConfig(dep.FullViperKey); err != nil {
			return err
		}
	}

	return nil
}

func createAwsProvider() (*awsprovider.AWSProvider, error) {
	accountID := viper.GetString("aws.account_id")
	region := viper.GetString("aws.region")
	provider, err := awsprovider.NewAWSProviderFunc(accountID, region)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS provider: %w", err)
	}
	if provider == nil {
		return nil, fmt.Errorf("AWS provider is nil after creation")
	}
	return provider, nil
}
