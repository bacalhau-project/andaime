package gcp

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/resourcemanager/apiv3/resourcemanagerpb"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/sync/errgroup"

	"github.com/bacalhau-project/andaime/pkg/logger"
	"github.com/bacalhau-project/andaime/pkg/models"
	gcp_provider "github.com/bacalhau-project/andaime/pkg/providers/gcp"
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
	Short: "List and destroy GCP deployments",
	Long: `List all active GCP deployments in config.yaml, 
and allow the user to select one for destruction. Projects that are already being destroyed will not be listed.`,
	RunE: runDestroy,
}

func GetGCPDestroyCmd() *cobra.Command {
	DestroyCmd.Flags().StringP("name", "n", "", "The name of the deployment to destroy")
	DestroyCmd.Flags().IntP("index", "i", 0, "The index of the deployment to destroy")
	DestroyCmd.Flags().Bool("all", false, "Destroy all deployments")
	DestroyCmd.Flags().String("config", "", "Path to the config file")
	DestroyCmd.Flags().
		Bool("dry-run", false, "Perform a dry run without actually destroying resources")
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
		return destroyAllDeployments(cmd.Context(), deployments, flags.dryRun)
	}

	selected, err := selectDeployment(deployments, flags)
	if err != nil {
		return err
	}

	return destroyDeployment(selected, flags.dryRun)
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
		gcpDeployments, err := extractGCPDeployments(uniqueID, details)
		if err != nil {
			fmt.Printf("Error processing deployment %s: %v\n", uniqueID, err)
			continue
		}
		deployments = append(deployments, gcpDeployments...)
	}

	sort.Slice(deployments, func(i, j int) bool {
		return deployments[i].Name < deployments[j].Name
	})

	return deployments, nil
}

func extractGCPDeployments(uniqueID string, details interface{}) ([]ConfigDeployment, error) {
	var deployments []ConfigDeployment
	deploymentClouds, ok := details.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid deployment details for %s", uniqueID)
	}

	gcpDetails, ok := deploymentClouds["gcp"].(map[string]interface{})
	if !ok {
		return nil, nil // Not a GCP deployment, skip
	}

	for projectID := range gcpDetails {
		deployments = append(deployments, ConfigDeployment{
			Name:         projectID,
			Type:         models.DeploymentTypeGCP,
			ID:           projectID,
			UniqueID:     uniqueID,
			FullViperKey: fmt.Sprintf("deployments.%s.gcp.%s", uniqueID, projectID),
		})
	}

	return deployments, nil
}

func destroyAllDeployments(ctx context.Context, deployments []ConfigDeployment, dryRun bool) error {
	gcpProvider, err := createGCPProvider(ctx)
	if err != nil {
		return err
	}

	projects, err := gcpProvider.ListProjects(ctx)
	if err != nil {
		return fmt.Errorf("failed to get projects: %w", err)
	}

	deployments = appendAndaimeDeployments(deployments, projects)

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

	g, ctx := errgroup.WithContext(ctx)
	var mu sync.Mutex
	for _, dep := range deployments {
		dep := dep // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			err := destroyDeployment(dep, dryRun)
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fmt.Printf("Failed to destroy deployment %s: %v\n", dep.Name, err)
			} else {
				fmt.Printf("Successfully destroyed deployment %s\n", dep.Name)
			}
			return err
		})
	}

	if err := g.Wait(); err != nil {
		return fmt.Errorf("error occurred while destroying deployments: %w", err)
	}

	if dryRun {
		fmt.Println("Dry run completed. No resources were actually destroyed.")
	} else {
		fmt.Printf("Finished destroying %d deployment(s)\n", len(deployments))
	}
	return nil
}

func createGCPProvider(ctx context.Context) (*gcp_provider.GCPProvider, error) {
	provider, err := gcp_provider.NewGCPProviderFunc(
		ctx,
		viper.GetString("gcp.organization_id"),
		viper.GetString("gcp.billing_account_id"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCP provider: %w", err)
	}
	if provider == nil {
		return nil, fmt.Errorf("GCP provider is nil after creation")
	}
	return provider, nil
}

func appendAndaimeDeployments(
	deployments []ConfigDeployment,
	projects []*resourcemanagerpb.Project,
) []ConfigDeployment {
	for _, project := range projects {
		if isAndaimeDeployment(project) && !isBeingDestroyed(project) &&
			!isAlreadyListed(deployments, project.ProjectId) {
			deployments = append(deployments, ConfigDeployment{
				Name: project.DisplayName,
				Type: models.DeploymentTypeGCP,
				ID:   project.ProjectId,
			})
		}
	}
	return deployments
}

func isAndaimeDeployment(project *resourcemanagerpb.Project) bool {
	return project.Labels["deployed-by"] == "andaime"
}

func isBeingDestroyed(project *resourcemanagerpb.Project) bool {
	return project.State == resourcemanagerpb.Project_DELETE_REQUESTED
}

func isAlreadyListed(deployments []ConfigDeployment, projectID string) bool {
	for _, dep := range deployments {
		if dep.ID == projectID {
			return true
		}
	}
	return false
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

func destroyDeployment(dep ConfigDeployment, dryRun bool) error {
	logger := logger.Get()
	logger.Infof("Starting destruction of %s (%s) - %s", dep.Name, dep.Type, dep.ID)

	ctx := context.Background()

	return destroyGCPDeployment(ctx, dep, dryRun)
}

func destroyGCPDeployment(ctx context.Context, dep ConfigDeployment, dryRun bool) error {
	gcpProvider, err := createGCPProvider(ctx)
	if err != nil {
		return err
	}

	fmt.Printf("   Destroying GCP resources for %s\n", dep.Name)
	if dryRun {
		fmt.Printf("   -- Dry run: Would destroy project %s\n", dep.ID)
	} else {
		err = retry(func() error {
			return gcpProvider.DestroyProject(ctx, dep.ID)
		}, 3, time.Second)

		if err != nil {
			if strings.Contains(err.Error(), "Project not found") {
				fmt.Printf("   -- Project is already destroyed.\n")
			} else {
				return fmt.Errorf("failed to destroy GCP deployment %s: %w", dep.Name, err)
			}
		} else {
			fmt.Printf("   -- Project %s has been scheduled for deletion\n", dep.ID)
		}

		fmt.Printf("   Removing deployment from config\n")
		if err := utils.DeleteKeyFromConfig(dep.FullViperKey); err != nil {
			return err
		}
	}

	return nil
}

func retry(f func() error, attempts int, sleep time.Duration) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}
		if i >= (attempts - 1) {
			break
		}
		time.Sleep(sleep)
		fmt.Printf("Retrying after error: %v\n", err)
	}
	return fmt.Errorf("after %d attempts, last error: %s", attempts, err)
}

func initializeConfig(cmd *cobra.Command) error {
	l := logger.Get()
	// The config should already be initialized by the root command
	// We can just log the config file being used
	l.Debugf("Using config file: %s", viper.ConfigFileUsed())
	return nil
}
