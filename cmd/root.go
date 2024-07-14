package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile                       string
	projectName                   string
	targetPlatform                string
	numberOfOrchestratorNodes     int
	numberOfComputeNodes          int
	targetRegions                 string
	orchestratorIP                string
	awsProfile                    string
	verboseMode                   bool
)

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

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.andaime.yaml)")
	rootCmd.PersistentFlags().BoolVar(&verboseMode, "verbose", false, "Enable verbose output")

	rootCmd.PersistentFlags().StringVar(&projectName, "project-name", "", "Set project name")
	rootCmd.PersistentFlags().StringVar(&targetPlatform, "target-platform", "", "Set target platform")
	rootCmd.PersistentFlags().IntVar(&numberOfOrchestratorNodes, "orchestrator-nodes", 1, "Set number of orchestrator nodes")
	rootCmd.PersistentFlags().IntVar(&numberOfComputeNodes, "compute-nodes", 2, "Set number of compute nodes")
	rootCmd.PersistentFlags().StringVar(&targetRegions, "target-regions", "us-east-1", "Comma-separated list of target AWS regions")
	rootCmd.PersistentFlags().StringVar(&orchestratorIP, "orchestrator-ip", "", "IP address of existing orchestrator node")
	rootCmd.PersistentFlags().StringVar(&awsProfile, "aws-profile", "default", "AWS profile to use for credentials")

	rootCmd.AddCommand(createCmd)
	rootCmd.AddCommand(destroyCmd)
	rootCmd.AddCommand(listCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".andaime" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".andaime")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}