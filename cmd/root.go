package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/bacalhau-project/andaime/providers/aws"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile                   string
	projectName               string
	targetPlatform            string
	numberOfOrchestratorNodes int
	numberOfComputeNodes      int
	targetRegions             string
	orchestratorIP            string
	awsProfile                string
	verboseMode               bool
)

var (
	NumberOfDefaultOrchestratorNodes = 1
	NumberOfDefaultComputeNodes      = 2
)

type CloudProvider struct {
	awsProvider aws.AWSProviderInterface
}

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "andaime",
	Short: "Andaime is a tool for managing cloud resources",
	Long: `Andaime is a comprehensive tool for managing cloud resources,
including deploying and managing Bacalhau nodes across multiple cloud providers.`,
}

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create resources for Bacalhau nodes",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement create functionality
		fmt.Println("Create command called")
	},
}

// destroyCmd represents the destroy command
var destroyCmd = &cobra.Command{
	Use:   "destroy",
	Short: "Destroy resources for Bacalhau nodes",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement destroy functionality
		fmt.Println("Destroy command called")
	},
}

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List resources for Bacalhau nodes",
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Implement list functionality
		fmt.Println("List command called")
	},
}

func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		if err != nil {
			fmt.Println("Unable to find home directory:", err)
			os.Exit(1)
		}

		// Search config in home directory with name ".andaime" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".andaime")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	} else if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
		// Config file was found but another error was produced
		fmt.Println("Error reading config file:", err)
		os.Exit(1)
	}
}

func SetupRootCommand() *cobra.Command {
	cobra.OnInitialize(initConfig)

	// Setup flags
	setupFlags()

	// Add commands
	rootCmd.AddCommand(createCmd, destroyCmd, listCmd)
	getBetaCmd(rootCmd)

	// Dynamically initialize required cloud providers based on configuration
	initializeCloudProviders()

	return rootCmd
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return SetupRootCommand().Execute()
}

func setupFlags() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.andaime.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verboseMode, "verbose", false, "Enable verbose output")

	rootCmd.PersistentFlags().StringVar(&projectName, "project-name", "", "Set project name")
	rootCmd.PersistentFlags().StringVar(&targetPlatform, "target-platform", "", "Set target platform")
	rootCmd.PersistentFlags().IntVar(&numberOfOrchestratorNodes,
		"orchestrator-nodes",
		NumberOfDefaultOrchestratorNodes,
		"Set number of orchestrator nodes")
	rootCmd.PersistentFlags().IntVar(&numberOfComputeNodes, "compute-nodes", numberOfComputeNodes, "Set number of compute nodes")
	rootCmd.PersistentFlags().StringVar(&targetRegions, "target-regions", "us-east-1", "Comma-separated list of target AWS regions")
	rootCmd.PersistentFlags().StringVar(&orchestratorIP, "orchestrator-ip", "", "IP address of existing orchestrator node")
	rootCmd.PersistentFlags().StringVar(&awsProfile, "aws-profile", "default", "AWS profile to use for credentials")
}

func initializeCloudProviders() *CloudProvider {
	cloudProvider := &CloudProvider{}

	// Example: Initialize AWS provider
	if shouldInitAWS() {
		if err := initAWSProvider(cloudProvider); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize AWS provider: %v\n", err)
			os.Exit(1)
		}
	}
	// TODO: Instantiate GCP
	// TODO: Instantiate Azure
	return cloudProvider
}

// initAWSProvider initializes the AWS provider with the given profile and region
func initAWSProvider(c *CloudProvider) error {
	awsProvider, err := aws.NewAWSProviderFunc(context.Background())
	if err != nil {
		return fmt.Errorf("failed to initialize AWS provider: %w", err)
	}
	c.awsProvider = awsProvider
	return nil
}

func shouldInitAWS() bool {
	// TODO: Detect if AWS should be instantiated
	return true
}
